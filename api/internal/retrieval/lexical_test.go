package retrieval

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

func TestExtractCandidateIdentifiers(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"what does asyncio.create_task do?", []string{"asyncio.create_task", "asyncio", "create_task"}},
		{"who calls tailor_node", []string{"tailor_node"}},
		{"what is this about", nil},
	}

	for _, c := range cases {
		got := extractCandidateIdentifiers(c.query)
		if len(got) != len(c.want) {
			t.Errorf("extractCandidateIdentifiers(%q) = %v, want %v", c.query, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("extractCandidateIdentifiers(%q)[%d] = %q, want %q", c.query, i, got[i], c.want[i])
			}
		}
	}
}

func TestHybridRetriever_LexicalSearch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable"
	}

	dbStore, err := store.NewStore(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	fileID, err := dbStore.InsertFile(ctx, "repo_lex_1", "bot/main.py", "python", "")
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	if _, err := dbStore.InsertSymbol(ctx, store.Symbol{
		FileID: fileID, Name: "post_init", Type: "function", StartLine: 10, EndLine: 25,
	}); err != nil {
		t.Fatalf("failed to insert symbol: %v", err)
	}

	// asyncio.create_task is never a locally-defined symbol (it's a stdlib
	// call), so it only ever shows up as a call_edges.callee_symbol - this
	// mirrors the exact real-world case this feature was built to catch.
	if err := dbStore.InsertCallEdge(ctx, store.CallEdge{
		RepoID: "repo_lex_1", CallerSymbol: "post_init", CallerFilePath: "bot/main.py",
		CalleeSymbol: "asyncio.create_task",
	}); err != nil {
		t.Fatalf("failed to insert call edge: %v", err)
	}

	sidecarClient := sidecar.NewClient("http://localhost:0") // never called by LexicalSearch
	retriever := NewHybridRetriever(dbStore, sidecarClient, logger)

	results, err := retriever.LexicalSearch(ctx, "repo_lex_1", "what does asyncio.create_task do?")
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	foundCallee := false
	for _, r := range results {
		if r.Symbol == "asyncio.create_task" {
			foundCallee = true
			if len(r.Edges) != 1 || r.Edges[0].Caller != "post_init" {
				t.Errorf("expected one edge from post_init, got %+v", r.Edges)
			}
		}
	}
	if !foundCallee {
		t.Fatalf("expected a result for asyncio.create_task (found via call_edges even though it's not a local symbol), got %+v", results)
	}

	// A question naming the caller instead should find the locally-defined
	// symbol via the symbol-table path.
	results2, err := retriever.LexicalSearch(ctx, "repo_lex_1", "what does post_init do?")
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}
	foundSymbol := false
	for _, r := range results2 {
		if r.Symbol == "post_init" && r.FilePath == "bot/main.py" {
			foundSymbol = true
		}
	}
	if !foundSymbol {
		t.Fatalf("expected a result for post_init (locally-defined symbol), got %+v", results2)
	}
}
