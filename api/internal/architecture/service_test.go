package architecture

import (
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

// TestResolveConceptCitation_MatchesRealSymbol covers the core safety
// property: a concept's citation is only ever populated from the real,
// currently-indexed symbol table - never trusted directly from whatever
// location the LLM might have claimed. When the LLM names a symbol that
// genuinely exists, its real file path and line range are resolved.
func TestResolveConceptCitation_MatchesRealSymbol(t *testing.T) {
	files := []store.ArchitectureFile{
		{ID: 1, Path: "internal/worker/pool.go"},
	}
	symbols := []store.ArchitectureSymbol{
		{FileID: 1, Name: "Pool", Type: "struct", StartLine: 12, EndLine: 40},
	}

	filePath, startLine, endLine, ok := resolveConceptCitation("Pool", files, symbols)

	if !ok {
		t.Fatal("expected resolution to succeed for a real symbol")
	}
	if filePath != "internal/worker/pool.go" || startLine != 12 || endLine != 40 {
		t.Errorf("unexpected resolution: filePath=%q startLine=%d endLine=%d", filePath, startLine, endLine)
	}
}

// TestResolveConceptCitation_UnknownSymbolReturnsFalse covers the failure
// case: the LLM can name a symbol that doesn't actually exist (a
// hallucination, a typo, or a symbol outside the sample it was shown) -
// resolution must fail cleanly rather than fabricate a location.
func TestResolveConceptCitation_UnknownSymbolReturnsFalse(t *testing.T) {
	files := []store.ArchitectureFile{{ID: 1, Path: "main.go"}}
	symbols := []store.ArchitectureSymbol{{FileID: 1, Name: "main", Type: "function", StartLine: 1, EndLine: 5}}

	_, _, _, ok := resolveConceptCitation("DoesNotExist", files, symbols)

	if ok {
		t.Error("expected resolution to fail for a symbol that isn't in the index")
	}
}

// TestResolveConceptCitation_EmptyClaimReturnsFalse covers the common case:
// the LLM didn't name a symbol at all for this concept (a general
// convention, not tied to one specific location) - not an error, just no
// citation.
func TestResolveConceptCitation_EmptyClaimReturnsFalse(t *testing.T) {
	_, _, _, ok := resolveConceptCitation("", nil, nil)
	if ok {
		t.Error("expected an empty claimed symbol to resolve to no citation")
	}
}
