package architecture

import (
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

func TestBuildReadingPath_BFSFromEntrypoint(t *testing.T) {
	entrypoints := []EntryPoint{
		{Name: "main", FilePath: "cmd/server/main.go", Language: "go", Type: "main"},
	}
	callEdges := []store.ArchitectureCallEdge{
		{CallerSymbol: "main", CallerFilePath: "cmd/server/main.go", CalleeeSymbol: "NewServer"},
		{CallerSymbol: "NewServer", CallerFilePath: "internal/api/server.go", CalleeeSymbol: "registerRoutes"},
	}

	steps := BuildReadingPath(entrypoints, callEdges)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps (main, NewServer, registerRoutes), got %d: %+v", len(steps), steps)
	}
	if steps[0].Symbol != "main" || steps[0].Reason != "Entrypoint" {
		t.Errorf("expected main first with reason \"Entrypoint\", got %+v", steps[0])
	}
	if steps[0].FilePath != "cmd/server/main.go" {
		t.Errorf("expected main's file path to come from the entrypoint, got %q", steps[0].FilePath)
	}
	// NewServer and registerRoutes should follow, in BFS order (depth 1 then depth 2)
	if steps[1].Symbol != "NewServer" {
		t.Errorf("expected NewServer at depth 1, got %+v", steps[1])
	}
	if steps[2].Symbol != "registerRoutes" {
		t.Errorf("expected registerRoutes at depth 2, got %+v", steps[2])
	}
}

func TestBuildReadingPath_RanksByCallerCount(t *testing.T) {
	entrypoints := []EntryPoint{{Name: "main", FilePath: "main.go", Language: "go", Type: "main"}}
	callEdges := []store.ArchitectureCallEdge{
		{CallerSymbol: "main", CallerFilePath: "main.go", CalleeeSymbol: "helperA"},
		{CallerSymbol: "main", CallerFilePath: "main.go", CalleeeSymbol: "helperB"},
		// helperB is called by two distinct symbols, helperA by only one -
		// at the same BFS depth, helperB should rank first.
		{CallerSymbol: "other", CallerFilePath: "other.go", CalleeeSymbol: "helperB"},
	}

	steps := BuildReadingPath(entrypoints, callEdges)

	helperAIdx, helperBIdx := -1, -1
	for i, s := range steps {
		if s.Symbol == "helperA" {
			helperAIdx = i
		}
		if s.Symbol == "helperB" {
			helperBIdx = i
		}
	}
	if helperAIdx == -1 || helperBIdx == -1 {
		t.Fatalf("expected both helperA and helperB in the path, got %+v", steps)
	}
	if helperBIdx > helperAIdx {
		t.Errorf("expected helperB (2 callers) to rank before helperA (1 caller), got order %+v", steps)
	}
	if steps[helperAIdx].Reason != "Called by 1 function(s) in the core flow" {
		t.Errorf("unexpected reason for helperA: %q", steps[helperAIdx].Reason)
	}
}

func TestBuildReadingPath_NoEntrypointsFallsBackToCallerRanking(t *testing.T) {
	callEdges := []store.ArchitectureCallEdge{
		{CallerSymbol: "coreLogic", CallerFilePath: "core.go", CalleeeSymbol: "helper"},
		{CallerSymbol: "otherCaller", CallerFilePath: "other.go", CalleeeSymbol: "helper"},
	}

	steps := BuildReadingPath(nil, callEdges)

	if len(steps) == 0 {
		t.Fatal("expected a non-empty fallback reading path when there are call edges but no entrypoints")
	}
	for _, s := range steps {
		if s.Reason == "Entrypoint" {
			t.Errorf("no entrypoints were given, no step should be reasoned \"Entrypoint\": %+v", s)
		}
	}
}

func TestBuildReadingPath_CapsAtTwelveSteps(t *testing.T) {
	entrypoints := []EntryPoint{{Name: "root", FilePath: "root.go", Language: "go", Type: "main"}}
	var callEdges []store.ArchitectureCallEdge
	prev := "root"
	for i := 0; i < 20; i++ {
		next := "sym" + string(rune('A'+i))
		callEdges = append(callEdges, store.ArchitectureCallEdge{
			CallerSymbol: prev, CallerFilePath: "root.go", CalleeeSymbol: next,
		})
		prev = next
	}

	steps := BuildReadingPath(entrypoints, callEdges)

	if len(steps) > 12 {
		t.Errorf("expected at most 12 steps, got %d", len(steps))
	}
}

func TestBuildReadingPath_HandlesCyclesWithoutInfiniteLoop(t *testing.T) {
	entrypoints := []EntryPoint{{Name: "a", FilePath: "a.go", Language: "go", Type: "main"}}
	callEdges := []store.ArchitectureCallEdge{
		{CallerSymbol: "a", CallerFilePath: "a.go", CalleeeSymbol: "b"},
		{CallerSymbol: "b", CallerFilePath: "b.go", CalleeeSymbol: "a"}, // cycle back to a
	}

	steps := BuildReadingPath(entrypoints, callEdges)

	if len(steps) != 2 {
		t.Fatalf("expected exactly 2 steps (a, b) despite the cycle, got %d: %+v", len(steps), steps)
	}
}
