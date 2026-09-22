package retrieval

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// TestHybridRetriever_LexicalSearch_ReturnsRealSource covers a real bug: a
// symbol found via LexicalSearch's exact/substring symbol-table path
// previously only ever got a one-line "X is a Y defined at Z, lines A-B"
// location stub as its Document - never the actual function body - even
// though the file was already on disk and the line range was already
// known. That starved the LLM of the one piece of context it actually
// needed to answer "what does this function do".
func TestHybridRetriever_LexicalSearch_ReturnsRealSource(t *testing.T) {
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

	// A real file on disk, standing in for a clone directory - LexicalSearch
	// must read this, not just cite it.
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "room.go")
	content := "package room\n\nfunc HandleWord(clientID, word string) bool {\n\treturn len(word) > 0\n}\n"
	if err := os.WriteFile(realFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	fileID, err := dbStore.InsertFile(ctx, "repo_lex_2", realFile, "go", "")
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	if _, err := dbStore.InsertSymbol(ctx, store.Symbol{
		FileID: fileID, Name: "HandleWord", Type: "function", StartLine: 3, EndLine: 5,
	}); err != nil {
		t.Fatalf("failed to insert symbol: %v", err)
	}

	sidecarClient := sidecar.NewClient("http://localhost:0") // never called by LexicalSearch
	retriever := NewHybridRetriever(dbStore, sidecarClient, logger)

	results, err := retriever.LexicalSearch(ctx, "repo_lex_2", "what does HandleWord do?")
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	var found *RetrievalResult
	for i, r := range results {
		if r.Symbol == "HandleWord" {
			found = &results[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a result for HandleWord, got %+v", results)
	}
	if !strings.Contains(found.Document, "return len(word) > 0") {
		t.Errorf("expected HandleWord's Document to contain its real source, got a location-only stub instead: %q", found.Document)
	}
}

// TestHybridRetriever_LexicalSearch_ExactNameMatchRanksFirst covers the
// second half of the same bug: ListSymbols' substring search returns exact
// and substring matches in file/line order, not relevance order, so an
// unrelated function whose name merely *contains* the searched identifier
// (e.g. a test function named TestRoom_HandleWord_EmptyWordRejected) could
// take a result slot away from the actual symbol the question named, once
// maxLexicalResults caps the list.
func TestHybridRetriever_LexicalSearch_ExactNameMatchRanksFirst(t *testing.T) {
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

	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "room_test.go")
	if err := os.WriteFile(realFile, []byte("package room\n"), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	fileID, err := dbStore.InsertFile(ctx, "repo_lex_3", realFile, "go", "")
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	// Deliberately insert the substring-matching noise symbols BEFORE the
	// exact match, in a file that sorts alphabetically before the exact
	// match's own file would (mirroring the real "_test.go sorts after the
	// real file" case would NOT save us here - this fixture puts the noise
	// in the earlier-sorting file on purpose, so only real ranking logic,
	// not incidental path ordering, can make the test pass).
	for i := 0; i < 6; i++ {
		if _, err := dbStore.InsertSymbol(ctx, store.Symbol{
			FileID: fileID, Name: fmt.Sprintf("TestRoom_HandleWord_Case%d", i), Type: "function", StartLine: i + 1, EndLine: i + 1,
		}); err != nil {
			t.Fatalf("failed to insert noise symbol: %v", err)
		}
	}
	if _, err := dbStore.InsertSymbol(ctx, store.Symbol{
		FileID: fileID, Name: "HandleWord", Type: "function", StartLine: 50, EndLine: 55,
	}); err != nil {
		t.Fatalf("failed to insert exact-match symbol: %v", err)
	}

	sidecarClient := sidecar.NewClient("http://localhost:0")
	retriever := NewHybridRetriever(dbStore, sidecarClient, logger)

	results, err := retriever.LexicalSearch(ctx, "repo_lex_3", "what does HandleWord do?")
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	foundExact := false
	for _, r := range results {
		if r.Symbol == "HandleWord" {
			foundExact = true
		}
	}
	if !foundExact {
		t.Fatalf("expected the exact-name match \"HandleWord\" to survive the maxLexicalResults cap ahead of substring-matching noise, got %+v", results)
	}
}
