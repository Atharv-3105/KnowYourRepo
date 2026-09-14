package answer

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

// fakeSyncer lets the test assert whether a background sync was triggered
// without needing a real repo-sync implementation.
type fakeSyncer struct {
	called chan string
}

func (f *fakeSyncer) SyncIfStale(ctx context.Context, repoID string) error {
	f.called <- repoID
	return nil
}

func newTestService(t *testing.T, sidecarURL string, syncer RepoSyncer) (*Service, *store.Store) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable"
	}

	dbStore, err := store.NewStore(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sidecarClient := sidecar.NewClient(sidecarURL)
	retriever := retrieval.NewHybridRetriever(dbStore, sidecarClient, logger)
	analyzer := architecture.NewAnalyzer(logger, dbStore)
	architectureService := architecture.NewService(logger, analyzer, dbStore, sidecarClient)
	builder := contextbuilder.NewBuilder(logger)
	ragService := rag.NewService(builder, sidecarClient, logger)

	return NewService(retriever, architectureService, ragService, syncer, logger), dbStore
}

func TestService_Answer_SemanticOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			results := []sidecar.SearchResult{
				{
					ID:       "setup_part_0",
					Document: "func setup() { configure() }",
					Metadata: map[string]interface{}{
						"repo_id": "repo_answer_1", "file_path": "main.go", "symbol": "setup",
					},
					Distance: 0.1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "setup calls configure"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sync := &fakeSyncer{called: make(chan string, 1)}
	svc, dbStore := newTestService(t, server.URL, sync)
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	if _, err := dbStore.InsertFile(ctx, "repo_answer_1", "main.go", "go", ""); err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}
	if err := dbStore.InsertCallEdge(ctx, store.CallEdge{
		RepoID: "repo_answer_1", CallerSymbol: "setup", CallerFilePath: "main.go", CalleeSymbol: "configure",
	}); err != nil {
		t.Fatalf("failed to insert call edge: %v", err)
	}

	answer, refreshing, results, err := svc.Answer(ctx, "repo_answer_1", "what does setup do?", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}
	if answer != "setup calls configure" {
		t.Errorf("expected answer from sidecar, got %q", answer)
	}
	if refreshing {
		t.Error("expected refreshing=false for a non-freshness question")
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result (semantic only, no architecture overview), got %d: %+v", len(results), results)
	}
	if results[0].Symbol != "setup" {
		t.Errorf("expected semantic result for 'setup', got %+v", results[0])
	}

	select {
	case <-sync.called:
		t.Error("did not expect a background sync for a non-freshness question")
	case <-time.After(200 * time.Millisecond):
		// expected: no sync triggered
	}
}

func TestService_Answer_ArchitectureOverviewAppended(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]sidecar.SearchResult{})
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "this repo has 1 file"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, dbStore := newTestService(t, server.URL, nil)
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	if _, err := dbStore.InsertFile(ctx, "repo_answer_2", "main.go", "go", ""); err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	_, _, results, err := svc.Answer(ctx, "repo_answer_2", "give me a high-level overview of this repo", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Symbol == "architecture_overview" {
			found = true
			if r.FilePath != "repo_answer_2" {
				t.Errorf("expected architecture overview FilePath to be the repo ID, got %q", r.FilePath)
			}
		}
	}
	if !found {
		t.Fatalf("expected an architecture_overview result to be appended, got %+v", results)
	}
}

func TestService_Answer_TriggersBackgroundSyncOnFreshnessQuestion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]sidecar.SearchResult{})
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sync := &fakeSyncer{called: make(chan string, 1)}
	svc, dbStore := newTestService(t, server.URL, sync)
	defer dbStore.Close()

	answer, refreshing, _, err := svc.Answer(ctx, "repo_answer_3", "has this changed recently?", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}
	if !refreshing {
		t.Error("expected refreshing=true for a freshness question")
	}
	if answer != "ok" {
		t.Errorf("expected answer to still be produced, got %q", answer)
	}

	select {
	case repoID := <-sync.called:
		if repoID != "repo_answer_3" {
			t.Errorf("expected sync for repo_answer_3, got %q", repoID)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected background sync to be triggered")
	}
}
