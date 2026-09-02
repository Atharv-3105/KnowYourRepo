package store

import (
	"context"
	"testing"
	"time"
)

func TestInsertFile_RepositoryScoping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := newTestStore(t)

	// 1. Insert file "main.go" under repo_1
	id1, err := s.InsertFile(ctx, "repo_1", "main.go", "go", "")
	if err != nil {
		t.Fatalf("InsertFile repo_1 failed: %v", err)
	}

	// 2. Insert same file "main.go" under repo_2 (should succeed and return a new ID)
	id2, err := s.InsertFile(ctx, "repo_2", "main.go", "go", "")
	if err != nil {
		t.Fatalf("InsertFile repo_2 failed: %v", err)
	}

	if id1 == id2 {
		t.Errorf("Expected different file IDs for different repos, got %d for both", id1)
	}

	// 3. Re-insert file "main.go" under repo_1 (should handle conflict and return id1)
	id1Retry, err := s.InsertFile(ctx, "repo_1", "main.go", "go", "")
	if err != nil {
		t.Fatalf("Re-insert file repo_1 failed: %v", err)
	}

	if id1 != id1Retry {
		t.Errorf("Expected same file ID for re-inserting same file in repo_1, got %d and %d", id1, id1Retry)
	}
}

func TestCallGraph_RepositoryScoping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := newTestStore(t)

	// Insert CallEdge for repo_1
	err := s.InsertCallEdge(ctx, CallEdge{
		RepoID:         "repo_1",
		CallerSymbol:   "main",
		CallerFilePath: "main.go",
		CalleeSymbol:   "setup",
	})
	if err != nil {
		t.Fatalf("InsertCallEdge repo_1 failed: %v", err)
	}

	// Insert CallEdge with same caller/filepath but in repo_2 (calling a different callee)
	err = s.InsertCallEdge(ctx, CallEdge{
		RepoID:         "repo_2",
		CallerSymbol:   "main",
		CallerFilePath: "main.go",
		CalleeSymbol:   "run",
	})
	if err != nil {
		t.Fatalf("InsertCallEdge repo_2 failed: %v", err)
	}

	// Test Outgoing Calls scoping
	outgoing1, err := s.GetOutgoingCalls(ctx, "repo_1", "main.go", "main")
	if err != nil {
		t.Fatalf("GetOutgoingCalls repo_1 failed: %v", err)
	}
	if len(outgoing1) != 1 || outgoing1[0].CalleeSymbol != "setup" {
		t.Errorf("Expected 1 outgoing call to 'setup' for repo_1, got: %+v", outgoing1)
	}

	outgoing2, err := s.GetOutgoingCalls(ctx, "repo_2", "main.go", "main")
	if err != nil {
		t.Fatalf("GetOutgoingCalls repo_2 failed: %v", err)
	}
	if len(outgoing2) != 1 || outgoing2[0].CalleeSymbol != "run" {
		t.Errorf("Expected 1 outgoing call to 'run' for repo_2, got: %+v", outgoing2)
	}

	// Test Incoming Calls scoping
	incoming1, err := s.GetIncomingCalls(ctx, "repo_1", "setup")
	if err != nil {
		t.Fatalf("GetIncomingCalls repo_1 failed: %v", err)
	}
	if len(incoming1) != 1 || incoming1[0].CallerSymbol != "main" {
		t.Errorf("Expected 1 incoming call to 'setup' from 'main' in repo_1, got: %+v", incoming1)
	}

	// Callee 'run' should have no incoming calls in repo_1
	incomingRunInRepo1, err := s.GetIncomingCalls(ctx, "repo_1", "run")
	if err != nil {
		t.Fatalf("GetIncomingCalls repo_1 for 'run' failed: %v", err)
	}
	if len(incomingRunInRepo1) != 0 {
		t.Errorf("Expected 0 incoming calls to 'run' in repo_1, got: %+v", incomingRunInRepo1)
	}

	// Callee 'run' should have 1 incoming call in repo_2
	incomingRunInRepo2, err := s.GetIncomingCalls(ctx, "repo_2", "run")
	if err != nil {
		t.Fatalf("GetIncomingCalls repo_2 for 'run' failed: %v", err)
	}
	if len(incomingRunInRepo2) != 1 || incomingRunInRepo2[0].CallerSymbol != "main" {
		t.Errorf("Expected 1 incoming call to 'run' from 'main' in repo_2, got: %+v", incomingRunInRepo2)
	}
}
