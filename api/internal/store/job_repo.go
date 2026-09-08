package store 

import(
	"context"
	"fmt"
)


type IngestionJob  struct {
	ID			string
	RepoURL		string
	Status 		string
	ErrorMessage	*string
	Stage		*string
}

const (
	JobStatusPending = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted = "completed"
	JobStatusFailed = "failed"
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
	SELECT id, repo_url, status, error_message, stage
	FROM ingestion_jobs
	WHERE id = $1
	`

	var job IngestionJob

	err := s.db.QueryRowContext(ctx, query, jobID).Scan(&job.ID, &job.RepoURL, &job.Status, &job.ErrorMessage, &job.Stage)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingestion job: %w", err)
	}

	return &job, nil
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
