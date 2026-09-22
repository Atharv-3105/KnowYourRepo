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
