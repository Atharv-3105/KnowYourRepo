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
		{Symbol: "setup", FilePath: "main.go"},
	}

	tools := toolsUsed(results)

	if len(tools) != 1 || tools[0] != "semantic" {
		t.Errorf("expected [\"semantic\"], got %+v", tools)
	}
}

func TestToolsUsed_IncludesArchitectureWhenOverviewPresent(t *testing.T) {
	results := []retrieval.RetrievalResult{
		{Symbol: "setup", FilePath: "main.go"},
		{Symbol: "architecture_overview", FilePath: "repo_123"},
	}

	tools := toolsUsed(results)

	if len(tools) != 2 || tools[0] != "semantic" || tools[1] != "architecture" {
		t.Errorf("expected [\"semantic\", \"architecture\"], got %+v", tools)
	}
}
