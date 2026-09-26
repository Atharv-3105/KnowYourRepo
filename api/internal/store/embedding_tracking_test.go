package store

import (
	"context"
	"testing"
	"time"
)

func TestEmbeddingTracking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := newTestStore(t)
	const repo = "repo_embed_track"

	for _, p := range []string{"a.go", "b.go", "c.go"} {
		if _, err := s.InsertFile(ctx, repo, p, "go", "h1"); err != nil {
			t.Fatalf("InsertFile(%s): %v", p, err)
		}
	}

	pending := func(model string) int {
		t.Helper()
		n, err := s.CountFilesPendingEmbedding(ctx, repo, model)
		if err != nil {
			t.Fatalf("CountFilesPendingEmbedding: %v", err)
		}
		return n
	}

	if got := pending("m1"); got != 3 {
		t.Fatalf("new files should all be pending, got %d", got)
	}

	if err := s.MarkFilesEmbedded(ctx, repo, []string{"a.go", "b.go"}, "m1"); err != nil {
		t.Fatalf("MarkFilesEmbedded: %v", err)
	}
	if got := pending("m1"); got != 1 {
		t.Errorf("expected only c.go pending under m1, got %d", got)
	}

	// A model change makes every file pending again.
	if got := pending("m2"); got != 3 {
		t.Errorf("expected all files pending under a new model, got %d", got)
	}

	// Re-inserting a file (its content changed) clears its embedded state.
	if _, err := s.InsertFile(ctx, repo, "a.go", "go", "h2"); err != nil {
		t.Fatalf("InsertFile re-insert: %v", err)
	}
	records, err := s.GetFileHashesByRepo(ctx, repo)
	if err != nil {
		t.Fatalf("GetFileHashesByRepo: %v", err)
	}
	if records["a.go"].EmbeddedModel != "" {
		t.Errorf("changed file should be unembedded, got %q", records["a.go"].EmbeddedModel)
	}
	if records["b.go"].EmbeddedModel != "m1" {
		t.Errorf("untouched file should stay embedded with m1, got %q", records["b.go"].EmbeddedModel)
	}
}
