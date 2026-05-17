package graph

import (
	"os"
	"path/filepath"
	"testing"
	"log/slog"
	"context"

	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
)

func  TestExtractGoSymbols(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	parser := ingestion.NewParser(logger)
	extractor := NewExtractor(logger)

	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "main.go")

	code := `
	package main
	
	func hello() {}
	
	func world() {}
	
	type User struct {}
	
	func (u User) GetName() string {
				return "atharva"
	}
	`

	err := os.WriteFile(file, []byte(code), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	//Parse the file 
	result, err := parser.ParseFile(ctx, file, "go")
	if err != nil{
		t.Fatalf("failed to parse the file: %v", err)
	}

	//Extract symbols
	symbols, err := extractor.ExtractSymbols(result.Root, result.Source, "go")

	if err != nil{
		t.Fatalf("failed to extract symbols: %v", err)
	}

	expectedCount := 3

	if len(symbols) != expectedCount {
		t.Fatalf("expected %d symbols, got %d", expectedCount, len(symbols))
	}

	expectedNames := map[string]bool{
		"hello": false, 
		"world": false,
		"GetName": false,
	}

	for _, sym := range symbols{
		
		if _, ok := expectedNames[sym.Name]; ok {
			expectedNames[sym.Name] = true
		}
		
		t.Logf("Extracted Symbol => Name = %s Type = %s Start = %d End = %d", sym.Name, sym.Type, sym.StartLine, sym.EndLine)
	}

	for name, found := range expectedNames {
		if !found{
			t.Fatalf("expected symbol %s not found", name)
		}
	}
}