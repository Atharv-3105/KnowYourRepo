package api

import (
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

func TestBuildSources_ExcludesArchitectureOverview(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "setup", FilePath: "main.go", Metadata: map[string]interface{}{"start_line": float64(10), "end_line": float64(20)}},
		{Symbol: "architecture_overview", FilePath: "repo_123"},
		{Symbol: "configure", FilePath: "config.go"},
	}

	sources := buildSources(results)

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources (architecture_overview excluded), got %d: %+v", len(sources), sources)
	}
	for _, s := range sources {
		if s.Symbol == "architecture_overview" {
			t.Errorf("architecture_overview leaked into sources: %+v", s)
		}
	}
	if sources[0].Symbol != "setup" || sources[0].StartLine != 10 || sources[0].EndLine != 20 {
		t.Errorf("expected first source to be setup with line 10-20, got %+v", sources[0])
	}
}

func TestBuildSources_ExcludesEmptyFilePath(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "setup", FilePath: "main.go"},
		// A lexical fallback hit for a call to a symbol never locally
		// defined (e.g. a stdlib call like "asyncio.create_task") - real
		// grounding for the LLM prompt, but not a real, clickable citation
		// since there's no file in this repo to jump to.
		{Symbol: "asyncio.create_task", FilePath: ""},
	}

	sources := buildSources(results)

	if len(sources) != 1 {
		t.Fatalf("expected 1 source (empty-FilePath result excluded), got %d: %+v", len(sources), sources)
	}
	if sources[0].Symbol != "setup" {
		t.Errorf("expected the remaining source to be setup, got %+v", sources[0])
	}
}

func TestBuildSources_EmptyResultsReturnsEmptySlice(t *testing.T) {
	sources := buildSources(nil)
	if sources == nil {
		t.Error("expected buildSources to return an empty slice, not nil, so it serializes as [] not null")
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestToolsUsed_SemanticOnly(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "setup", FilePath: "main.go", Origin: "semantic"},
	}

	tools := toolsUsed(results)

	if len(tools) != 1 || tools[0] != "semantic" {
		t.Errorf("expected [\"semantic\"], got %+v", tools)
	}
}

func TestToolsUsed_IncludesArchitectureWhenOverviewPresent(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "setup", FilePath: "main.go", Origin: "semantic"},
		{Symbol: "architecture_overview", FilePath: "repo_123"},
	}

	tools := toolsUsed(results)

	if len(tools) != 2 || tools[0] != "semantic" || tools[1] != "architecture" {
		t.Errorf("expected [\"semantic\", \"architecture\"], got %+v", tools)
	}
}

func TestToolsUsed_IncludesLexicalWhenItContributed(t *testing.T) {
	// The exact real-world case this was built for: semantic search only
	// returns irrelevant noise, and the lexical fallback is what actually
	// grounds the answer - tools_used should say so, not silently claim
	// "semantic" contributed when it didn't.
	results := []retrieval.RetrievalResult{
		{Symbol: "test_pipeline", FilePath: "test_orchestrator.py", Origin: "semantic"},
		{Symbol: "asyncio.create_task", FilePath: "", Origin: "lexical"},
	}

	tools := toolsUsed(results)

	if len(tools) != 2 || tools[0] != "semantic" || tools[1] != "lexical" {
		t.Errorf("expected [\"semantic\", \"lexical\"], got %+v", tools)
	}
}

func TestToolsUsed_LexicalOnlyWhenSemanticFoundNothing(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "asyncio.create_task", FilePath: "", Origin: "lexical"},
	}

	tools := toolsUsed(results)

	if len(tools) != 1 || tools[0] != "lexical" {
		t.Errorf("expected [\"lexical\"], got %+v", tools)
	}
}

func TestToolsUsed_EmptyResultsReturnsEmptySlice(t *testing.T) {
	tools := toolsUsed(nil)
	if tools == nil {
		t.Error("expected toolsUsed to return an empty slice, not nil, so it serializes as [] not null")
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}
