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
}

const (
	JobStatusPending = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted = "completed"
	JobStatusFailed = "failed"
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
	SELECT id, repo_url, status, error_message
	FROM ingestion_jobs
	WHERE id = $1
	`

	var job IngestionJob

	err := s.db.QueryRowContext(ctx, query, jobID).Scan(&job.ID, &job.RepoURL, &job.Status, &job.ErrorMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingestion job: %w", err)
	}

	return &job, nil 
}
