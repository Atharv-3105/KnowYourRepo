package graph

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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