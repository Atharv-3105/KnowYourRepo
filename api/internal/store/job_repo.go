package store 

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)


type IngestionJob  struct {
	ID			string
	RepoURL		string
	Status 		string
	ErrorMessage	*string
	Stage		*string
	ParseStatus	*string
	EmbedStatus	*string
}

const (
	JobStatusPending = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted = "completed"
	JobStatusFailed = "failed"
	// JobStatusSkipped is only ever set on ParseStatus/EmbedStatus (not the
	// job's own overall status) - e.g. an incremental sync where no files
	// changed, so there was nothing to embed.
	JobStatusSkipped = "skipped"
)

// Stage values mirror the real, sequential phases ingestRepository actually
// runs through (repos.go) - not invented UI steps. A job's stage is NULL
// until the worker picks it up and sets StageCloning; it stays NULL for a
// job that's still sitting in the queue.
const (
	StageCloning   = "cloning"
	StageWalking   = "walking"
	StageParsing   = "parsing"
	StageEmbedding = "embedding"
	StageDone      = "done"
)

//Function to insert a Job into DB
// InsertJob creates or resets a job row for jobID (jobID == repoID). On a
// repeat ingestion of the same repo, the same row is reused and reset back
// to pending rather than inserted fresh - job history isn't a requirement
// here, and this avoids a duplicate primary key on re-ingestion.
func(s *Store) InsertJob(ctx context.Context, jobID, repoURL string) error {
	query := `
	INSERT INTO ingestion_jobs(id, repo_url, status)
	VALUES ($1, $2, $3)
	ON CONFLICT (id) DO UPDATE SET
		repo_url = EXCLUDED.repo_url,
		status = EXCLUDED.status,
		error_message = NULL,
		parse_status = NULL,
		embed_status = NULL,
		updated_at = now()
	`

	_, err := s.db.ExecContext(ctx, query, jobID, repoURL, JobStatusPending)
	if err != nil {
		return fmt.Errorf("failed to insert ingestion job: %w", err)
	}

	return nil
}

func(s *Store) UpdateJobStatus(ctx context.Context, jobID, status string, errMsg *string) error {
	query := `
	UPDATE ingestion_jobs
	SET status = $1, error_message = $2, updated_at = now()
	WHERE id = $3
	`

	_, err := s.db.ExecContext(ctx, query, status, errMsg, jobID)
	if err != nil {
		return fmt.Errorf("failed to update ingestion job status: %w", err)
	}

	return nil 
}

func(s *Store) GetJob(ctx context.Context, jobID string) (*IngestionJob, error){
	query := `
	SELECT id, repo_url, status, error_message, stage, parse_status, embed_status
	FROM ingestion_jobs
	WHERE id = $1
	`

	var job IngestionJob

	err := s.db.QueryRowContext(ctx, query, jobID).Scan(&job.ID, &job.RepoURL, &job.Status, &job.ErrorMessage, &job.Stage, &job.ParseStatus, &job.EmbedStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingestion job: %w", err)
	}

	return &job, nil
}

// SetParseStatus records whether the parsing/symbol/call-graph-extraction
// phase completed - independent of the job's overall status and of
// embedding's outcome. See the schema.sql comment on these columns for why
// they're tracked separately.
func(s *Store) SetParseStatus(ctx context.Context, jobID, status string) error {
	query := `
	UPDATE ingestion_jobs
	SET parse_status = $1, updated_at = now()
	WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, status, jobID)
	if err != nil {
		return fmt.Errorf("failed to update ingestion job parse_status: %w", err)
	}

	return nil
}

// SetEmbedStatus records whether the embedding phase completed, failed, or
// was skipped (nothing to embed) - independent of the job's overall status.
func(s *Store) SetEmbedStatus(ctx context.Context, jobID, status string) error {
	query := `
	UPDATE ingestion_jobs
	SET embed_status = $1, updated_at = now()
	WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, status, jobID)
	if err != nil {
		return fmt.Errorf("failed to update ingestion job embed_status: %w", err)
	}

	return nil
}

// UpdateJobStage records which real phase of the ingestion pipeline a job
// is currently in (see the Stage* constants above) - called from
// ingestRepository at each actual phase transition, not synthesized.
func(s *Store) UpdateJobStage(ctx context.Context, jobID, stage string) error {
	query := `
	UPDATE ingestion_jobs
	SET stage = $1, updated_at = now()
	WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, stage, jobID)
	if err != nil {
		return fmt.Errorf("failed to update ingestion job stage: %w", err)
	}

	return nil
}

// GetActiveJobByURL returns a pending or processing job for repoURL
// (case-insensitive), or nil if there is none. A brand-new repository has
// no repositories row until its clone succeeds, so this is the only way to
// notice that the same URL is already being ingested and avoid starting a
// second, racing ingestion into a different directory.
func (s *Store) GetActiveJobByURL(ctx context.Context, repoURL string) (*IngestionJob, error) {
	query := `
	SELECT id, repo_url, status, error_message, stage, parse_status, embed_status
	FROM ingestion_jobs
	WHERE LOWER(repo_url) = LOWER($1) AND status IN ($2, $3)
	ORDER BY created_at DESC
	LIMIT 1
	`

	var job IngestionJob
	err := s.db.QueryRowContext(ctx, query, repoURL, JobStatusPending, JobStatusProcessing).
		Scan(&job.ID, &job.RepoURL, &job.Status, &job.ErrorMessage, &job.Stage, &job.ParseStatus, &job.EmbedStatus)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active job by url: %w", err)
	}
	return &job, nil
}

// FindJob is GetJob that returns (nil, nil) instead of an error when the
// job doesn't exist.
func (s *Store) FindJob(ctx context.Context, jobID string) (*IngestionJob, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// ListUnfinishedJobIDs returns every job left pending or processing -
// after a restart, these are jobs that were queued or mid-flight when the
// previous process stopped (a deploy, or a free-tier instance spinning
// down) and need to be resumed.
func (s *Store) ListUnfinishedJobIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM ingestion_jobs WHERE status IN ($1, $2) ORDER BY updated_at`,
		JobStatusPending, JobStatusProcessing)
	if err != nil {
		return nil, fmt.Errorf("failed to list unfinished jobs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
