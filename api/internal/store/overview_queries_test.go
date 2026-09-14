package store

import (
	"context"
	"testing"
	"time"
)

func TestSaveAndGetOverview(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertRepository(ctx, Repository{ID: "repo_overview_test", RepoURL: "https://github.com/example/example"}); err != nil {
		t.Fatalf("failed to insert repository fixture: %v", err)
	}

	concepts := []Concept{
		{Term: "Worker pool", Explanation: "internal/worker.Pool processes ingestion jobs concurrently."},
	}

	if err := s.SaveOverview(ctx, "repo_overview_test", "This project ingests repos and answers questions about them.", concepts); err != nil {
		t.Fatalf("SaveOverview failed: %v", err)
	}

	got, err := s.GetOverview(ctx, "repo_overview_test")
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil overview, got nil")
	}
	if got.NarrativeSummary != "This project ingests repos and answers questions about them." {
		t.Errorf("unexpected narrative_summary: %q", got.NarrativeSummary)
	}
	if len(got.Concepts) != 1 || got.Concepts[0].Term != "Worker pool" {
		t.Errorf("unexpected concepts: %+v", got.Concepts)
	}

	// Re-saving (the re-ingestion/sync case) must overwrite, not duplicate.
	if err := s.SaveOverview(ctx, "repo_overview_test", "Updated summary.", nil); err != nil {
		t.Fatalf("SaveOverview (overwrite) failed: %v", err)
	}
	got, err = s.GetOverview(ctx, "repo_overview_test")
	if err != nil {
		t.Fatalf("GetOverview after overwrite failed: %v", err)
	}
	if got.NarrativeSummary != "Updated summary." {
		t.Errorf("expected overwrite to replace narrative_summary, got: %q", got.NarrativeSummary)
	}
}

func TestGetOverview_NoRow(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertRepository(ctx, Repository{ID: "repo_no_overview", RepoURL: "https://github.com/example/none"}); err != nil {
		t.Fatalf("failed to insert repository fixture: %v", err)
	}

	got, err := s.GetOverview(ctx, "repo_no_overview")
	if err != nil {
		t.Fatalf("GetOverview should not error when no row exists, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil overview when no row exists, got: %+v", got)
	}
}
