package store

import (
	"context"
	"testing"
	"time"
)

func TestSetParseStatusAndSetEmbedStatus(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertJob(ctx, "job_phase_test", "https://github.com/example/example"); err != nil {
		t.Fatalf("failed to insert job: %v", err)
	}

	// Freshly-inserted job: neither phase has run yet.
	job, err := s.GetJob(ctx, "job_phase_test")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.ParseStatus != nil {
		t.Errorf("expected ParseStatus to be nil for a fresh job, got %v", *job.ParseStatus)
	}
	if job.EmbedStatus != nil {
		t.Errorf("expected EmbedStatus to be nil for a fresh job, got %v", *job.EmbedStatus)
	}

	if err := s.SetParseStatus(ctx, "job_phase_test", JobStatusCompleted); err != nil {
		t.Fatalf("SetParseStatus failed: %v", err)
	}
	if err := s.SetEmbedStatus(ctx, "job_phase_test", JobStatusFailed); err != nil {
		t.Fatalf("SetEmbedStatus failed: %v", err)
	}

	job, err = s.GetJob(ctx, "job_phase_test")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.ParseStatus == nil || *job.ParseStatus != JobStatusCompleted {
		t.Errorf("expected ParseStatus %q, got %v", JobStatusCompleted, job.ParseStatus)
	}
	if job.EmbedStatus == nil || *job.EmbedStatus != JobStatusFailed {
		t.Errorf("expected EmbedStatus %q, got %v", JobStatusFailed, job.EmbedStatus)
	}

	// A re-ingestion (InsertJob on the same id) must reset both phase
	// statuses back to unset, the same way it already resets error_message -
	// a stale "embed failed" from a previous attempt must not survive into
	// a fresh run's initial (pending, not-yet-run) state.
	if err := s.InsertJob(ctx, "job_phase_test", "https://github.com/example/example"); err != nil {
		t.Fatalf("failed to re-insert job: %v", err)
	}
	job, err = s.GetJob(ctx, "job_phase_test")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.ParseStatus != nil {
		t.Errorf("expected ParseStatus to be reset to nil on re-ingestion, got %v", *job.ParseStatus)
	}
	if job.EmbedStatus != nil {
		t.Errorf("expected EmbedStatus to be reset to nil on re-ingestion, got %v", *job.EmbedStatus)
	}
}

func TestSetEmbedStatus_Skipped(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertJob(ctx, "job_skip_test", "https://github.com/example/example"); err != nil {
		t.Fatalf("failed to insert job: %v", err)
	}

	if err := s.SetEmbedStatus(ctx, "job_skip_test", JobStatusSkipped); err != nil {
		t.Fatalf("SetEmbedStatus failed: %v", err)
	}

	job, err := s.GetJob(ctx, "job_skip_test")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.EmbedStatus == nil || *job.EmbedStatus != JobStatusSkipped {
		t.Errorf("expected EmbedStatus %q, got %v", JobStatusSkipped, job.EmbedStatus)
	}
}

func TestGetActiveJobByURL_CaseInsensitiveAndStatusScoped(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.DB().ExecContext(ctx, `DELETE FROM ingestion_jobs WHERE id LIKE 'job_active_%'`)

	if err := s.InsertJob(ctx, "job_active_1", "https://github.com/Owner/Repo"); err != nil {
		t.Fatalf("InsertJob failed: %v", err)
	}

	got, err := s.GetActiveJobByURL(ctx, "https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("GetActiveJobByURL failed: %v", err)
	}
	if got == nil || got.ID != "job_active_1" {
		t.Fatalf("expected the pending job to be found case-insensitively, got %+v", got)
	}

	if err := s.UpdateJobStatus(ctx, "job_active_1", JobStatusCompleted, nil); err != nil {
		t.Fatalf("UpdateJobStatus failed: %v", err)
	}
	got, err = s.GetActiveJobByURL(ctx, "https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("GetActiveJobByURL failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected a completed job not to count as active, got %+v", got)
	}
}

func TestListUnfinishedJobIDs(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.DB().ExecContext(ctx, `DELETE FROM ingestion_jobs WHERE id LIKE 'job_unfinished_%'`)

	for _, id := range []string{"job_unfinished_pending", "job_unfinished_processing", "job_unfinished_done"} {
		if err := s.InsertJob(ctx, id, "https://github.com/o/"+id); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
	}
	s.UpdateJobStatus(ctx, "job_unfinished_processing", JobStatusProcessing, nil)
	s.UpdateJobStatus(ctx, "job_unfinished_done", JobStatusCompleted, nil)

	ids, err := s.ListUnfinishedJobIDs(ctx)
	if err != nil {
		t.Fatalf("ListUnfinishedJobIDs failed: %v", err)
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["job_unfinished_pending"] || !found["job_unfinished_processing"] {
		t.Errorf("expected pending and processing jobs to be listed, got %v", ids)
	}
	if found["job_unfinished_done"] {
		t.Errorf("expected a completed job not to be listed, got %v", ids)
	}
}

func TestFindJob_MissingReturnsNil(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	job, err := s.FindJob(ctx, "job_that_does_not_exist_xyz")
	if err != nil || job != nil {
		t.Errorf("expected (nil, nil) for a missing job, got (%+v, %v)", job, err)
	}
}
