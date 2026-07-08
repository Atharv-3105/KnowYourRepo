package graph

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
)

func TestExtractGo_ScopeIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.go")

	code := `
	package main

	func first() {
		helper()
	}

	// Any call expression outside functions (should NOT be attributed to first)
	var x = globalCall()

	func second() {
		anotherHelper()
	}
	`

	err := os.WriteFile(file, []byte(code), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "go")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractGoCallGraph(result.Root, result.Source)

	// We expect only helper() to be attributed to first, and anotherHelper() to second.
	// globalCall() should NOT be attributed to first() or second().
	for _, edge := range edges {
		if edge.Caller == "first" && edge.Callee == "globalCall" {
			t.Errorf("Bleeding detected: globalCall was incorrectly attributed to first")
		}
		if edge.Caller == "second" && edge.Callee == "globalCall" {
			t.Errorf("Bleeding detected: globalCall was incorrectly attributed to second")
		}
	}
}

func TestExtractJS_ScopeIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.js")

	code := `
	function outer() {
		const inner = () => {
			innerCall();
		};
		outerCall();
	}

	topLevelCall();
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

	// We expect:
	// - outerCall to be called by outer
	// - innerCall to be called by inner
	// - topLevelCall to not be attributed to outer or inner
	foundInnerCall := false
	foundOuterCall := false

	for _, edge := range edges {
		if edge.Caller == "inner" && edge.Callee == "innerCall" {
			foundInnerCall = true
		}
		if edge.Caller == "outer" && edge.Callee == "outerCall" {
			foundOuterCall = true
		}
		if (edge.Caller == "outer" || edge.Caller == "inner") && edge.Callee == "topLevelCall" {
			t.Errorf("Bleeding detected: topLevelCall was incorrectly attributed to %s", edge.Caller)
		}
	}

	if !foundInnerCall {
		t.Errorf("Expected to find call from inner to innerCall")
	}
	if !foundOuterCall {
		t.Errorf("Expected to find call from outer to outerCall")
	}
}

func TestExtractPython_ScopeIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := ingestion.NewParser(logger)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "main.py")

	code := `
def first_func():
    helper_one()

# Top level call
global_call()

def second_func():
    helper_two()
`

	err := os.WriteFile(file, []byte(code), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	result, err := parser.ParseFile(context.Background(), file, "python")
	if err != nil {
		t.Fatal(err)
	}

	edges := ExtractPythonCallGraph(result.Root, result.Source)

	for _, edge := range edges {
		if (edge.Caller == "first_func" || edge.Caller == "second_func") && edge.Callee == "global_call" {
			t.Errorf("Bleeding detected: global_call was incorrectly attributed to %s", edge.Caller)
		}
	}
}
