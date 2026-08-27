package retrieval

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

func TestHybridRetriever_Search(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Spin up mock HTTP server for the sidecar
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify POST search request
		var req sidecar.SearchRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.RepoID != "repo_1" {
			http.Error(w, "invalid repo_id", http.StatusBadRequest)
			return
		}

		// Return mock semantic search results
		results := []sidecar.SearchResult{
			{
				ID:       "setup_part_0",
				Document: "func setup() { configure() }",
				Metadata: map[string]interface{}{
					"repo_id":   "repo_1",
					"file_path": "main.go",
					"language":  "go",
					"symbol":    "setup",
				},
				Distance: 0.15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}))
	defer server.Close()

	// 2. Initialize Store
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable"
	}

	dbStore, err := store.NewStore(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	// Populate SQLite metadata and call edges
	_, err = dbStore.InsertFile(ctx, "repo_1", "main.go", "go")
	if err != nil {
		t.Fatalf("Failed to insert file: %v", err)
	}

	err = dbStore.InsertCallEdge(ctx, store.CallEdge{
		RepoID:         "repo_1",
		CallerSymbol:   "setup",
		CallerFilePath: "main.go",
		CalleeSymbol:   "configure",
	})
	if err != nil {
		t.Fatalf("Failed to insert call edge: %v", err)
	}

	// 3. Initialize HybridRetriever
	sidecarClient := sidecar.NewClient(server.URL)
	retriever := NewHybridRetriever(dbStore, sidecarClient, logger)

	// 4. Perform Search
	results, err := retriever.Search(ctx, "repo_1", "setup query")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// 5. Assert results
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Symbol != "setup" || res.FilePath != "main.go" {
		t.Errorf("Expected symbol 'setup' from 'main.go', got symbol '%s' from '%s'", res.Symbol, res.FilePath)
	}

	if len(res.Edges) != 1 || res.Edges[0].Callee != "configure" {
		t.Errorf("Expected 1 call edge to 'configure', got: %+v", res.Edges)
	}
}
