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

func TestExtractPythonCallGraph(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.py")

	code := `
def helper():
	pass

def world():
	helper()

def hello():
	world()
`

	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "python")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractPythonCallGraph(result.Root, result.Source)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d: %+v", len(edges), edges)
	}

	t.Logf("edges: %+v", edges)
}

// TestExtractPythonCallGraph_AnonymousLambda covers the same closure-literal
// bug as the Go/JS extractors: an immediately-invoked lambda previously
// became the callee's entire raw source text instead of a short synthetic
// name.
func TestExtractPythonCallGraph_AnonymousLambda(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.py")

	code := `
def helper():
	pass

def main():
	(lambda: helper() and helper() and helper() and helper() and helper())()
`

	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "python")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractPythonCallGraph(result.Root, result.Source)

	for _, e := range edges {
		if len(e.Callee) > 50 {
			t.Errorf("callee name too long (%d chars), likely a raw lambda body: %q", len(e.Callee), e.Callee)
		}
		if strings.Contains(e.Callee, "\n") {
			t.Errorf("callee name contains a newline, likely a raw lambda body: %q", e.Callee)
		}
	}

	found := false
	for _, e := range edges {
		if e.Callee == "helper" {
			found = true
			if e.Caller == "main" {
				t.Errorf("expected helper()'s caller to be the anonymous lambda, not main directly: %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("expected a call edge to helper(), got: %+v", edges)
	}

	t.Logf("edges: %+v", edges)
}
