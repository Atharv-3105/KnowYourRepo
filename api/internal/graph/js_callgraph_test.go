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

func TestExtractJSCallGraph(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	parser := ingestion.NewParser(logger)

	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "main.js")

	code := `
	function helper() {}
	
	function world() {
		helper()
	}
		
	const hello = () => {
		world()
	}
	`

	err := os.WriteFile(file, []byte(code), 0o644)

	if err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "javascript")

	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractJSCallGraph(result.Root, result.Source)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	t.Logf("edges: %+v", edges)
}

// TestExtractJSCallGraph_AnonymousFunctionLiteral covers the same
// closure-literal bug as the Go extractor: an immediately-invoked anonymous
// function expression or arrow function previously became the callee's
// entire raw source text instead of a short synthetic name.
func TestExtractJSCallGraph_AnonymousFunctionLiteral(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.js")

	code := `
	function helper() {}

	(function() {
		helper();
	})();

	(() => {
		helper();
	})();
	`

	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "javascript")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractJSCallGraph(result.Root, result.Source)

	for _, e := range edges {
		if len(e.Callee) > 50 {
			t.Errorf("callee name too long (%d chars), likely a raw closure body: %q", len(e.Callee), e.Callee)
		}
		if strings.Contains(e.Callee, "\n") {
			t.Errorf("callee name contains a newline, likely a raw closure body: %q", e.Callee)
		}
	}

	helperCalls := 0
	for _, e := range edges {
		if e.Callee == "helper" {
			helperCalls++
		}
	}
	if helperCalls != 2 {
		t.Fatalf("expected 2 calls to helper() (one from each IIFE), got %d: %+v", helperCalls, edges)
	}

	t.Logf("edges: %+v", edges)
}