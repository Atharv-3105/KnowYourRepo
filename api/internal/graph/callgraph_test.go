package graph

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
)

func TestExtractGo(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	parser := ingestion.NewParser(logger)

	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "main.go")

	code := `
	package main
	func helper() {}
	
	func world() {
		helper()
	}
		
	func hello() {
		world()
	}
	`

	err := os.WriteFile(file, []byte(code), 0o644)

	if err != nil{
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "go")

	edges := ExtractGoCallGraph(result.Root, result.Source)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	t.Logf("edges: %+v", edges)

}

// TestExtractGo_AnonymousFunctionLiteral covers the closure-literal bug: an
// immediately-invoked anonymous function (a common goroutine-launch pattern,
// `go func() { ... }()`) previously became the callee's *entire source text*
// (the whole closure body, including further nested calls) instead of a
// short synthetic name - bloating context sent to the LLM and once caused a
// real Groq 413 in production.
func TestExtractGo_AnonymousFunctionLiteral(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.go")

	code := `
	package main

	func helper() {}

	func main() {
		go func() {
			helper()
		}()
	}
	`

	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "go")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractGoCallGraph(result.Root, result.Source)

	for _, e := range edges {
		if len(e.Callee) > 50 {
			t.Errorf("callee name too long (%d chars), likely a raw closure body: %q", len(e.Callee), e.Callee)
		}
		if strings.Contains(e.Callee, "\n") {
			t.Errorf("callee name contains a newline, likely a raw closure body: %q", e.Callee)
		}
		if strings.Contains(e.Caller, "\n") {
			t.Errorf("caller name contains a newline, likely a raw closure body: %q", e.Caller)
		}
	}

	// The call to helper() inside the closure must still be captured, and
	// must not be misattributed to "main" (the enclosing named function) -
	// it happens inside the anonymous closure, not directly inside main.
	found := false
	for _, e := range edges {
		if e.Callee == "helper" {
			found = true
			if e.Caller == "main" {
				t.Errorf("expected helper()'s caller to be the anonymous closure, not main directly: %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("expected a call edge to helper(), got: %+v", edges)
	}

	t.Logf("edges: %+v", edges)
}