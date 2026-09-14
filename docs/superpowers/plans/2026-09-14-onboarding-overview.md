# Onboarding Overview (narrative summary, reading path, concepts) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Overview page into a real onboarding surface by adding an LLM-generated narrative summary, LLM-generated "notable concepts" list, and a deterministically-computed reading path, so a new user gets an actual explanation of the repo instead of just stats and a flat entrypoint list.

**Architecture:** Ingestion gains a step (after existing call-graph extraction) that reads a root-level README, sends it plus entrypoints/directory-structure/symbol samples to a new sidecar endpoint backed by the existing `LLMRouter`, and persists the result. The existing `GET /architecture/:repoID` endpoint is extended with three new fields: `narrative_summary`/`concepts` (read straight from storage) and `reading_path` (computed fresh every request via BFS over the call graph - no LLM, no storage, always current). `Overview.tsx` renders all three.

**Tech Stack:** Go (`api/`) - `database/sql` + pgx driver, Gin. Python (`extraction-service/`) - FastAPI, existing `LLMRouter`. React + TS (`frontend/`) - TanStack Query.

**Spec:** [docs/superpowers/specs/2026-09-14-onboarding-overview-design.md](../specs/2026-09-14-onboarding-overview-design.md)

## Global Constraints

- Go tests that touch Postgres use `TEST_DATABASE_URL` (falls back to `postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable`) - see `api/internal/store/testhelper_test.go`.
- Python has no test suite in this project (confirmed in CLAUDE.md) - Python changes are verified manually via `curl`, not automated tests.
- All new JSON field names are `snake_case` on both the Go and Python sides - no aliasing needed, the shapes match directly.
- File paths stored in `files.path` (and therefore `EntryPoint.FilePath`, call edge file paths) are the **absolute on-disk clone path** (e.g. `..\data\repos\<repoID>\internal\api\server.go` on this Windows dev machine), not repo-relative - this is existing, established behavior (see `frontend/src/lib/path.ts`'s comment). New code that needs a clean repo-relative path must strip it the same way that file already does; don't assume `file.Path` is already relative.
- Non-nil slice initialization for anything JSON-marshaled to the frontend (`concepts := []Concept{}`, not `var concepts []Concept`) - a nil slice marshals to JSON `null`, which breaks frontend code assuming an array. This is an established, deliberate pattern in this codebase (see the comment in `internal/store/architecture_queries.go`'s `GetLanguages`).
- Ingestion must never fail because overview generation failed - log and continue, same as the existing non-fatal-failure pattern used elsewhere in `ingestRepository`.

---

## Task 1: `repo_overview` schema + store queries

**Files:**
- Modify: `api/internal/store/schema.sql`
- Create: `api/internal/store/overview_queries.go`
- Create: `api/internal/store/overview_queries_test.go`

**Interfaces:**
- Produces: `store.Concept{Term, Explanation string}`, `store.Overview{RepoID, NarrativeSummary string, Concepts []Concept, GeneratedAt time.Time}`, `(s *Store) SaveOverview(ctx context.Context, repoID, narrativeSummary string, concepts []Concept) error`, `(s *Store) GetOverview(ctx context.Context, repoID string) (*Overview, error)` (returns `nil, nil` when no row exists for that repo yet).

- [ ] **Step 1: Add the `repo_overview` table to schema.sql**

Append to `api/internal/store/schema.sql` (after the existing `ingestion_jobs ADD COLUMN` line at the end of the file):

```sql

-- Generated once per ingestion/sync by architecture.Service.GenerateOverview
-- (narrative summary + notable concepts, LLM-generated). ON DELETE CASCADE
-- so this row disappears automatically if the repository row is ever
-- deleted - no separate cleanup path needed.
CREATE TABLE IF NOT EXISTS repo_overview (
    repo_id           TEXT PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
    narrative_summary TEXT,
    concepts          JSONB,
    generated_at      TIMESTAMPTZ
);
```

- [ ] **Step 2: Write the failing test**

Create `api/internal/store/overview_queries_test.go`:

```go
package store

import (
	"context"
	"testing"
	"time"
)

func TestSaveAndGetOverview(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertRepository(ctx, Repository{ID: "repo_overview_test", RepoURL: "https://github.com/example/example"}); err != nil {
		t.Fatalf("failed to insert repository fixture: %v", err)
	}

	concepts := []Concept{
		{Term: "Worker pool", Explanation: "internal/worker.Pool processes ingestion jobs concurrently."},
	}

	if err := s.SaveOverview(ctx, "repo_overview_test", "This project ingests repos and answers questions about them.", concepts); err != nil {
		t.Fatalf("SaveOverview failed: %v", err)
	}

	got, err := s.GetOverview(ctx, "repo_overview_test")
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil overview, got nil")
	}
	if got.NarrativeSummary != "This project ingests repos and answers questions about them." {
		t.Errorf("unexpected narrative_summary: %q", got.NarrativeSummary)
	}
	if len(got.Concepts) != 1 || got.Concepts[0].Term != "Worker pool" {
		t.Errorf("unexpected concepts: %+v", got.Concepts)
	}

	// Re-saving (the re-ingestion/sync case) must overwrite, not duplicate.
	if err := s.SaveOverview(ctx, "repo_overview_test", "Updated summary.", nil); err != nil {
		t.Fatalf("SaveOverview (overwrite) failed: %v", err)
	}
	got, err = s.GetOverview(ctx, "repo_overview_test")
	if err != nil {
		t.Fatalf("GetOverview after overwrite failed: %v", err)
	}
	if got.NarrativeSummary != "Updated summary." {
		t.Errorf("expected overwrite to replace narrative_summary, got: %q", got.NarrativeSummary)
	}
}

func TestGetOverview_NoRow(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.InsertRepository(ctx, Repository{ID: "repo_no_overview", RepoURL: "https://github.com/example/none"}); err != nil {
		t.Fatalf("failed to insert repository fixture: %v", err)
	}

	got, err := s.GetOverview(ctx, "repo_no_overview")
	if err != nil {
		t.Fatalf("GetOverview should not error when no row exists, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil overview when no row exists, got: %+v", got)
	}
}
```

- [ ] **Step 2b: Run test to verify it fails**

Run: `cd api && go test ./internal/store/ -run TestSaveAndGetOverview -v`
Expected: FAIL with a compile error (`SaveOverview`/`GetOverview`/`Concept`/`Overview` undefined).

- [ ] **Step 3: Implement `overview_queries.go`**

Create `api/internal/store/overview_queries.go`:

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Concept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
}

type Overview struct {
	RepoID           string
	NarrativeSummary string
	Concepts         []Concept
	GeneratedAt      time.Time
}

// SaveOverview upserts the generated overview for a repo - called once per
// ingestion/sync from architecture.Service.GenerateOverview. A concepts
// slice of nil or zero length is stored as a JSON empty array, not SQL NULL,
// so GetOverview's caller never has to distinguish "no concepts" from
// "generation hasn't run" by inspecting this column alone.
func (s *Store) SaveOverview(ctx context.Context, repoID, narrativeSummary string, concepts []Concept) error {
	if concepts == nil {
		concepts = []Concept{}
	}

	conceptsJSON, err := json.Marshal(concepts)
	if err != nil {
		return fmt.Errorf("failed to marshal concepts: %w", err)
	}

	query := `
		INSERT INTO repo_overview (repo_id, narrative_summary, concepts, generated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (repo_id) DO UPDATE SET
			narrative_summary = EXCLUDED.narrative_summary,
			concepts = EXCLUDED.concepts,
			generated_at = EXCLUDED.generated_at
	`

	if _, err := s.db.ExecContext(ctx, query, repoID, narrativeSummary, conceptsJSON); err != nil {
		return fmt.Errorf("failed to save overview: %w", err)
	}

	return nil
}

// GetOverview returns (nil, nil) - not an error - when no overview row
// exists yet for repoID (generation hasn't run, or failed and left nothing
// to store). Callers (architecture.Analyzer) treat that as "no narrative
// summary/concepts available", not a request failure.
func (s *Store) GetOverview(ctx context.Context, repoID string) (*Overview, error) {
	query := `
		SELECT repo_id, narrative_summary, concepts, generated_at
		FROM repo_overview
		WHERE repo_id = $1
	`

	var o Overview
	var conceptsJSON []byte

	err := s.db.QueryRowContext(ctx, query, repoID).Scan(&o.RepoID, &o.NarrativeSummary, &conceptsJSON, &o.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get overview: %w", err)
	}

	if err := json.Unmarshal(conceptsJSON, &o.Concepts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal concepts: %w", err)
	}

	return &o, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/store/ -run TestSaveAndGetOverview -v` and `go test ./internal/store/ -run TestGetOverview_NoRow -v`
Expected: both PASS. (`NewStore` embeds and re-runs `schema.sql` on every startup - see `store.go`'s `//go:embed schema.sql` - so the new table exists automatically, no separate migration step.)

- [ ] **Step 5: Commit**

```bash
git add api/internal/store/schema.sql api/internal/store/overview_queries.go api/internal/store/overview_queries_test.go
git commit -m "feat(store): add repo_overview table and SaveOverview/GetOverview queries"
```

---

## Task 2: Deterministic reading path builder

**Files:**
- Create: `api/internal/architecture/reading_path.go`
- Create: `api/internal/architecture/reading_path_test.go`

**Interfaces:**
- Consumes: `EntryPoint{Name, FilePath, Language, Type string}` (from `models.go`, already exists), `store.ArchitectureCallEdge{CallerSymbol, CallerFilePath, CalleeeSymbol string}` (already exists).
- Produces: `architecture.ReadingStep{Symbol, FilePath, Reason string}`, `func BuildReadingPath(entrypoints []EntryPoint, callEdges []store.ArchitectureCallEdge) []ReadingStep`.

- [ ] **Step 1: Write the failing tests**

Create `api/internal/architecture/reading_path_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/architecture/ -run TestBuildReadingPath -v`
Expected: FAIL with a compile error (`BuildReadingPath`/`ReadingStep` undefined).

- [ ] **Step 3: Implement `reading_path.go`**

Create `api/internal/architecture/reading_path.go`:

```go
// This file computes a suggested "where to start reading" path through a
// repo's call graph - deterministic, no LLM involved, so it's free and
// always in sync with the current index. See the design spec's Reading
// path section for the reasoning behind this vs. an LLM-generated order.
package architecture

import (
	"fmt"
	"sort"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type ReadingStep struct {
	Symbol   string `json:"symbol"`
	FilePath string `json:"file_path"`
	Reason   string `json:"reason"`
}

const maxReadingPathSteps = 12

// BuildReadingPath ranks symbols by how central they are to the call graph:
// BFS outward from every detected entrypoint (depth ascending), broken by
// distinct-caller count (descending) within each depth. With zero
// entrypoints (a real case - some repos have none detected), falls back to
// ranking every symbol seen as a caller by caller count, with no BFS anchor.
func BuildReadingPath(entrypoints []EntryPoint, callEdges []store.ArchitectureCallEdge) []ReadingStep {
	fileOf := make(map[string]string)
	for _, ep := range entrypoints {
		fileOf[ep.Name] = ep.FilePath
	}
	for _, e := range callEdges {
		if _, ok := fileOf[e.CallerSymbol]; !ok {
			fileOf[e.CallerSymbol] = e.CallerFilePath
		}
	}

	// Distinct-caller count per callee symbol, used as a centrality proxy -
	// a symbol called from many places is more "core" than one called once.
	callerCount := make(map[string]int)
	callersSeen := make(map[string]map[string]struct{})
	for _, e := range callEdges {
		if callersSeen[e.CalleeeSymbol] == nil {
			callersSeen[e.CalleeeSymbol] = make(map[string]struct{})
		}
		if _, dup := callersSeen[e.CalleeeSymbol][e.CallerSymbol]; !dup {
			callersSeen[e.CalleeeSymbol][e.CallerSymbol] = struct{}{}
			callerCount[e.CalleeeSymbol]++
		}
	}

	adjacency := make(map[string][]string)
	for _, e := range callEdges {
		adjacency[e.CallerSymbol] = append(adjacency[e.CallerSymbol], e.CalleeeSymbol)
	}

	type discovered struct {
		symbol string
		depth  int
	}

	visited := make(map[string]bool)
	var order []discovered

	if len(entrypoints) > 0 {
		var queue []discovered
		for _, ep := range entrypoints {
			if !visited[ep.Name] {
				visited[ep.Name] = true
				queue = append(queue, discovered{ep.Name, 0})
			}
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			order = append(order, cur)
			for _, callee := range adjacency[cur.symbol] {
				if !visited[callee] {
					visited[callee] = true
					queue = append(queue, discovered{callee, cur.depth + 1})
				}
			}
		}
	} else {
		// No entrypoints detected - rank every symbol seen as a caller,
		// no BFS anchor to depth-order them, so depth is uniform.
		for symbol := range fileOf {
			order = append(order, discovered{symbol, 1})
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		if order[i].depth != order[j].depth {
			return order[i].depth < order[j].depth
		}
		return callerCount[order[i].symbol] > callerCount[order[j].symbol]
	})

	entrypointSet := make(map[string]bool, len(entrypoints))
	for _, ep := range entrypoints {
		entrypointSet[ep.Name] = true
	}

	steps := make([]ReadingStep, 0, maxReadingPathSteps)
	for _, d := range order {
		if len(steps) >= maxReadingPathSteps {
			break
		}

		reason := fmt.Sprintf("Called by %d function(s) in the core flow", callerCount[d.symbol])
		if entrypointSet[d.symbol] {
			reason = "Entrypoint"
		}

		steps = append(steps, ReadingStep{
			Symbol:   d.symbol,
			FilePath: fileOf[d.symbol], // empty for symbols never seen as a caller (e.g. stdlib/external calls) - same convention as GraphNode.file_path
			Reason:   reason,
		})
	}

	return steps
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/architecture/ -run TestBuildReadingPath -v`
Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/architecture/reading_path.go api/internal/architecture/reading_path_test.go
git commit -m "feat(architecture): add deterministic BuildReadingPath over the call graph"
```

---

## Task 3: README capture in the ingestion Walker

**Files:**
- Modify: `api/internal/ingestion/walker.go`
- Modify: `api/internal/ingestion/walkter_test.go` (existing filename, not a typo to fix here - matches what's already on disk)

**Interfaces:**
- Produces: `(w *Walker) ReadReadme(repoRoot string) (string, error)` - returns `("", nil)` when no README is present (not an error).

- [ ] **Step 1: Write the failing tests**

Add to the end of `api/internal/ingestion/walkter_test.go` (check the top of that file first for its existing imports - add `strings` if not already imported):

```go
func TestReadReadme_FindsRootReadme(t *testing.T) {
	dir := t.TempDir()
	content := "# My Project\n\nThis project does things."
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if got != content {
		t.Errorf("expected README content %q, got %q", content, got)
	}
}

func TestReadReadme_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	content := "lowercase readme"
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if got != content {
		t.Errorf("expected README content %q, got %q", content, got)
	}
}

func TestReadReadme_NoReadmePresent(t *testing.T) {
	dir := t.TempDir()

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("expected no error when no README is present, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string when no README is present, got: %q", got)
	}
}

func TestReadReadme_TruncatesLongReadme(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("x", maxReadmeChars+500)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if len(got) != maxReadmeChars {
		t.Errorf("expected truncation to %d chars, got %d", maxReadmeChars, len(got))
	}
}
```

If `io` isn't already imported in that test file, add `"io"` and `"strings"` to its import block.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/ingestion/ -run TestReadReadme -v`
Expected: FAIL with a compile error (`ReadReadme`/`maxReadmeChars` undefined).

- [ ] **Step 3: Implement `ReadReadme` in walker.go**

Add to `api/internal/ingestion/walker.go` (add `"strings"` is already imported; the rest of the imports already cover what's needed):

```go
const maxReadmeChars = 8000

var readmeCandidates = []string{"README.md", "README.rst", "README.txt", "README"}

// ReadReadme reads a root-level README (case-insensitive, not scanned into
// subdirectories - see the design spec's README ingestion section) as plain
// text, capped at maxReadmeChars. Returns ("", nil), not an error, when no
// README is present - overview generation degrades gracefully without one.
func (w *Walker) ReadReadme(repoRoot string) (string, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return "", fmt.Errorf("failed to read repo root: %w", err)
	}

	byLowerName := make(map[string]os.DirEntry, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			byLowerName[strings.ToLower(e.Name())] = e
		}
	}

	for _, candidate := range readmeCandidates {
		entry, ok := byLowerName[strings.ToLower(candidate)]
		if !ok {
			continue
		}

		content, err := os.ReadFile(filepath.Join(repoRoot, entry.Name()))
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}

		text := string(content)
		if len(text) > maxReadmeChars {
			text = text[:maxReadmeChars]
		}
		return text, nil
	}

	return "", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/ingestion/ -run TestReadReadme -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/ingestion/walker.go api/internal/ingestion/walkter_test.go
git commit -m "feat(ingestion): add Walker.ReadReadme to capture root-level README text"
```

---

## Task 4: Sidecar Go client - GenerateOverview

**Files:**
- Create: `api/internal/sidecar/generate_overview.go`
- Create: `api/internal/sidecar/generate_overview_test.go`

**Interfaces:**
- Produces: `sidecar.OverviewEntrypoint{Name, FilePath, Language string}`, `sidecar.OverviewConcept{Term, Explanation string}`, `sidecar.GenerateOverviewRequest{ReadmeText string, Entrypoints []OverviewEntrypoint, DirectoryStructure []string, RepresentativeSymbols []string}`, `sidecar.GenerateOverviewResponse{NarrativeSummary string, Concepts []OverviewConcept}`, `(c *Client) GenerateOverview(ctx context.Context, req GenerateOverviewRequest) (*GenerateOverviewResponse, error)`.

- [ ] **Step 1: Write the failing test**

Create `api/internal/sidecar/generate_overview_test.go`:

```go
package sidecar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate-overview" {
			t.Errorf("expected path /generate-overview, got %s", r.URL.Path)
		}

		var req GenerateOverviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.ReadmeText != "# Test" {
			t.Errorf("unexpected readme_text: %q", req.ReadmeText)
		}

		resp := GenerateOverviewResponse{
			NarrativeSummary: "This is a test project.",
			Concepts: []OverviewConcept{
				{Term: "Worker pool", Explanation: "Processes jobs concurrently."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	resp, err := client.GenerateOverview(context.Background(), GenerateOverviewRequest{
		ReadmeText:            "# Test",
		Entrypoints:           []OverviewEntrypoint{{Name: "main", FilePath: "main.go", Language: "go"}},
		DirectoryStructure:    []string{"internal", "cmd"},
		RepresentativeSymbols: []string{"main (function)"},
	})
	if err != nil {
		t.Fatalf("GenerateOverview failed: %v", err)
	}
	if resp.NarrativeSummary != "This is a test project." {
		t.Errorf("unexpected narrative_summary: %q", resp.NarrativeSummary)
	}
	if len(resp.Concepts) != 1 || resp.Concepts[0].Term != "Worker pool" {
		t.Errorf("unexpected concepts: %+v", resp.Concepts)
	}
}

func TestGenerateOverview_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("provider exhausted"))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	_, err := client.GenerateOverview(context.Background(), GenerateOverviewRequest{})
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/sidecar/ -run TestGenerateOverview -v`
Expected: FAIL with a compile error (`GenerateOverview` and related types undefined).

- [ ] **Step 3: Implement `generate_overview.go`**

Create `api/internal/sidecar/generate_overview.go` (mirrors `chat.go`'s structure exactly):

```go
package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OverviewEntrypoint struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
}

type OverviewConcept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
}

type GenerateOverviewRequest struct {
	ReadmeText             string               `json:"readme_text"`
	Entrypoints            []OverviewEntrypoint `json:"entrypoints"`
	DirectoryStructure     []string             `json:"directory_structure"`
	RepresentativeSymbols  []string             `json:"representative_symbols"`
}

type GenerateOverviewResponse struct {
	NarrativeSummary string            `json:"narrative_summary"`
	Concepts         []OverviewConcept `json:"concepts"`
}

func (c *Client) GenerateOverview(ctx context.Context, req GenerateOverviewRequest) (*GenerateOverviewResponse, error) {

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal generate-overview request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate-overview", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create generate-overview request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("generate-overview request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("generate-overview request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result GenerateOverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode generate-overview response: %w", err)
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/sidecar/ -run TestGenerateOverview -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/sidecar/generate_overview.go api/internal/sidecar/generate_overview_test.go
git commit -m "feat(sidecar): add Client.GenerateOverview for the new /generate-overview route"
```

---

## Task 5: Python `/generate-overview` route

**Files:**
- Create: `extraction-service/app/models/overview.py`
- Create: `extraction-service/app/services/overview.py`
- Create: `extraction-service/app/routes/overview.py`
- Modify: `extraction-service/app/main.py`
- Modify: `extraction-service/app/providers/router.py`

**Interfaces:**
- Consumes: `app.providers.factory.get_llm_router()` (existing), `LLMRouter.complete(prompt, task_type="overview")` (existing method, new task_type key).
- Produces: `POST /generate-overview` accepting `{readme_text, entrypoints: [{name, file_path, language}], directory_structure: [str], representative_symbols: [str]}`, returning `{narrative_summary: str, concepts: [{term, explanation}]}`. This is the exact shape `api/internal/sidecar/generate_overview.go` (Task 4) already sends/expects.

No automated tests for this task - this project has no Python test suite (see Global Constraints). Verified manually via `curl` in Step 5.

- [ ] **Step 1: Add the "overview" task type to the router**

In `extraction-service/app/providers/router.py`, find `TASK_PROVIDER_ORDER` (near the top of the file) and add a new entry:

```python
TASK_PROVIDER_ORDER: dict[str, list[str]] = {
    "answer" : ["groq", "gemini", "cerebras", "openrouter"],
    "overview": ["groq", "gemini", "cerebras", "openrouter"],
    "default": ["groq", "gemini", "cerebras", "openrouter"],
}
```

- [ ] **Step 2: Create the Pydantic models**

Create `extraction-service/app/models/overview.py`:

```python
from pydantic import BaseModel


class OverviewEntrypoint(BaseModel):
    name: str
    file_path: str
    language: str


class GenerateOverviewRequest(BaseModel):
    readme_text: str = ""
    entrypoints: list[OverviewEntrypoint] = []
    directory_structure: list[str] = []
    representative_symbols: list[str] = []


class OverviewConcept(BaseModel):
    term: str
    explanation: str


class GenerateOverviewResponse(BaseModel):
    narrative_summary: str
    concepts: list[OverviewConcept] = []
```

- [ ] **Step 3: Create the generation service**

Create `extraction-service/app/services/overview.py`:

```python
import json
import logging

from app.models.overview import GenerateOverviewResponse, OverviewConcept, OverviewEntrypoint
from app.providers.factory import get_llm_router

logger = logging.getLogger(__name__)
router = get_llm_router()


def _build_prompt(
    readme_text: str,
    entrypoints: list[OverviewEntrypoint],
    directory_structure: list[str],
    representative_symbols: list[str],
) -> str:
    readme_section = readme_text.strip() or "(no README found in this repository)"
    entrypoint_lines = "\n".join(f"- {e.name} ({e.file_path})" for e in entrypoints) or "(none detected)"
    dir_lines = "\n".join(f"- {d}" for d in directory_structure) or "(none)"
    symbol_lines = "\n".join(f"- {s}" for s in representative_symbols) or "(none)"

    return f"""You are helping a new engineer understand an unfamiliar codebase.

README
{readme_section}

Detected entrypoints
{entrypoint_lines}

Top-level directories
{dir_lines}

Representative symbols
{symbol_lines}

Respond with ONLY a single JSON object, no markdown code fences, no commentary, in exactly this shape:
{{"narrative_summary": "2-4 sentences describing what this project does and why, written for someone who has never seen it before", "concepts": [{{"term": "short name of a notable pattern, convention, or domain term used in this codebase", "explanation": "1-2 sentence explanation of it, specific to this codebase"}}]}}

Include at most 5 concepts. Only include concepts that are genuinely non-obvious - skip anything a competent engineer would already recognize on sight."""


def _parse_response(raw: str) -> GenerateOverviewResponse:
    text = raw.strip()
    if text.startswith("```"):
        text = text.strip("`")
        if text.lower().startswith("json"):
            text = text[4:]
        text = text.strip()

    try:
        parsed = json.loads(text)
    except json.JSONDecodeError as e:
        logger.warning("overview_generation_parse_failed error=%s raw=%s", e, raw[:500])
        raise ValueError(f"failed to parse overview generation response as JSON: {e}") from e

    return GenerateOverviewResponse(
        narrative_summary=parsed.get("narrative_summary", ""),
        concepts=[OverviewConcept(**c) for c in parsed.get("concepts", [])],
    )


async def generate_overview(
    readme_text: str,
    entrypoints: list[OverviewEntrypoint],
    directory_structure: list[str],
    representative_symbols: list[str],
) -> GenerateOverviewResponse:
    prompt = _build_prompt(readme_text, entrypoints, directory_structure, representative_symbols)
    raw = await router.complete(prompt, task_type="overview")
    return _parse_response(raw)
```

- [ ] **Step 4: Create the route**

Create `extraction-service/app/routes/overview.py`:

```python
from fastapi import APIRouter, HTTPException

from app.models.overview import GenerateOverviewRequest, GenerateOverviewResponse
from app.services.overview import generate_overview

router = APIRouter()


@router.post("/generate-overview", response_model=GenerateOverviewResponse)
async def generate_overview_route(request: GenerateOverviewRequest):
    try:
        return await generate_overview(
            request.readme_text,
            request.entrypoints,
            request.directory_structure,
            request.representative_symbols,
        )
    except ValueError as e:
        raise HTTPException(status_code=502, detail=str(e))
```

Register it in `extraction-service/app/main.py` - add the import next to the existing route imports and `include_router` call next to the existing ones:

```python
from app.routes.overview import router as overview_router
```

```python
app.include_router(overview_router)
```

- [ ] **Step 5: Manually verify**

Start the extraction service (`cd extraction-service && env/Scripts/python.exe run.py` on this Windows dev machine), then:

```bash
curl -X POST http://localhost:8000/generate-overview -H "Content-Type: application/json" -d "{\"readme_text\": \"# Demo\\n\\nA demo project.\", \"entrypoints\": [{\"name\": \"main\", \"file_path\": \"main.go\", \"language\": \"go\"}], \"directory_structure\": [\"internal\", \"cmd\"], \"representative_symbols\": [\"main (function)\"]}"
```

Expected: a `200` response with a JSON body containing non-empty `narrative_summary` and a `concepts` array (possibly empty - the LLM isn't guaranteed to find any).

- [ ] **Step 6: Commit**

```bash
git add extraction-service/app/models/overview.py extraction-service/app/services/overview.py extraction-service/app/routes/overview.py extraction-service/app/main.py extraction-service/app/providers/router.py
git commit -m "feat(sidecar): add POST /generate-overview route"
```

---

## Task 6: Wire overview generation into `architecture.Service` and the ingestion job

**Files:**
- Modify: `api/internal/architecture/service.go`
- Modify: `api/internal/api/repos.go`

**Interfaces:**
- Consumes: `store.SaveOverview` (Task 1), `sidecar.Client.GenerateOverview` (Task 4), `Walker.ReadReadme` (Task 3), `DetectEntrypoints` (existing, `entrypoints.go`), `store.GetFiles`/`GetSymbols` (existing, `architecture_queries.go`).
- Produces: `(s *Service) GenerateOverview(ctx context.Context, repoID, readmeText string) error`.

- [ ] **Step 1: Add `store` and `sidecar` fields to `architecture.Service` and a `GenerateOverview` method**

Modify `api/internal/architecture/service.go` in full:

```go
package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type Service struct {
	logger   *slog.Logger
	analyzer *Analyzer
	store    *store.Store
	sidecar  *sidecar.Client
}

func NewService(logger *slog.Logger, analyzer *Analyzer, store *store.Store, sidecar *sidecar.Client) *Service {

	return &Service{
		logger:   logger,
		analyzer: analyzer,
		store:    store,
		sidecar:  sidecar,
	}
}

//This function analyzes a repo and returns its architectural overview
func (s *Service) BuildSummary(ctx context.Context, repoID string) (*Summary, error) {

	s.logger.Info("architecture_service_started", "repo_id", repoID)

	summary, err := s.analyzer.AnalyzeRepository(ctx, repoID)

	if err != nil {

		s.logger.Error("architecture_service_failed", "repo_id", repoID, "error", err)

		return nil, err
	}

	s.logger.Info("architecture_service_completed",
		"repo_id", repoID, "files", summary.Statistics.FileCount,
		"symbols", summary.Statistics.SymbolCount, "components", len(summary.Components),
		"entrypoints", len(summary.EntryPoints))

	return summary, nil
}

const maxRepresentativeSymbols = 30

// GenerateOverview builds the LLM prompt inputs from what's already in the
// store (entrypoints, directory structure, a sample of symbols), calls the
// sidecar's /generate-overview route, and persists the result. Called once
// per ingestion/sync job, after call-graph extraction has completed (see
// ingestRepository in api/internal/api/repos.go). readmeText may be empty -
// generation still runs, just without that section of the prompt.
func (s *Service) GenerateOverview(ctx context.Context, repoID, readmeText string) error {

	files, err := s.store.GetFiles(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to load files for overview generation: %w", err)
	}

	symbols, err := s.store.GetSymbols(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to load symbols for overview generation: %w", err)
	}

	entrypoints := DetectEntrypoints(files, symbols)

	overviewEntrypoints := make([]sidecar.OverviewEntrypoint, 0, len(entrypoints))
	for _, ep := range entrypoints {
		overviewEntrypoints = append(overviewEntrypoints, sidecar.OverviewEntrypoint{
			Name:     ep.Name,
			FilePath: ep.FilePath,
			Language: ep.Language,
		})
	}

	resp, err := s.sidecar.GenerateOverview(ctx, sidecar.GenerateOverviewRequest{
		ReadmeText:            readmeText,
		Entrypoints:           overviewEntrypoints,
		DirectoryStructure:    topLevelDirs(files, repoID),
		RepresentativeSymbols: representativeSymbolStrings(symbols, maxRepresentativeSymbols),
	})
	if err != nil {
		return fmt.Errorf("overview generation request failed: %w", err)
	}

	concepts := make([]store.Concept, 0, len(resp.Concepts))
	for _, c := range resp.Concepts {
		concepts = append(concepts, store.Concept{Term: c.Term, Explanation: c.Explanation})
	}

	if err := s.store.SaveOverview(ctx, repoID, resp.NarrativeSummary, concepts); err != nil {
		return fmt.Errorf("failed to save generated overview: %w", err)
	}

	s.logger.Info("overview_generation_completed", "repo_id", repoID, "concepts", len(concepts))

	return nil
}

// repoRelativeSegments splits a stored file path (the clone directory's own
// absolute on-disk location - see the Global Constraints note in this
// plan/the design spec - not a clean repo-relative path) into segments after
// the repoID, mirroring frontend/src/lib/path.ts's repoRelativePath.
func repoRelativeSegments(filePath, repoID string) []string {
	segments := strings.FieldsFunc(filePath, func(r rune) bool { return r == '\\' || r == '/' })

	idx := -1
	for i, seg := range segments {
		if seg == repoID {
			idx = i
			break
		}
	}

	if idx >= 0 && idx+1 < len(segments) {
		return segments[idx+1:]
	}
	return segments
}

// topLevelDirs returns the deduplicated, sorted set of top-level directory
// names in the repo (relative to the repo root), for the overview prompt's
// "directory structure" section.
func topLevelDirs(files []store.ArchitectureFile, repoID string) []string {
	seen := make(map[string]struct{})

	for _, f := range files {
		rel := repoRelativeSegments(f.Path, repoID)
		dir := "(root)"
		if len(rel) > 1 {
			dir = rel[0]
		}
		seen[dir] = struct{}{}
	}

	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	return dirs
}

// representativeSymbolStrings caps how many symbols reach the overview
// prompt, formatted as "name (type)" - token-budget hygiene, see the design
// spec's Error handling section.
func representativeSymbolStrings(symbols []store.ArchitectureSymbol, limit int) []string {
	out := make([]string, 0, limit)

	for i, sym := range symbols {
		if i >= limit {
			break
		}
		out = append(out, fmt.Sprintf("%s (%s)", sym.Name, sym.Type))
	}

	return out
}
```

- [ ] **Step 2: Update `NewRepoHandler`'s call to `architecture.NewService`**

In `api/internal/api/repos.go`, find this line inside `NewRepoHandler`:

```go
	architectureService := architecture.NewService(logger, architectureAnalyzer)
```

Replace it with:

```go
	architectureService := architecture.NewService(logger, architectureAnalyzer, store, sidecar)
```

(`store` and `sidecar` are already `NewRepoHandler`'s own parameters at this point in the function, so this is just passing them through.)

- [ ] **Step 3: Call `GenerateOverview` from `ingestRepository`**

In `api/internal/api/repos.go`, inside `ingestRepository`, find this block near the end of the function:

```go
	//Save IR File Representation
	irPath := filepath.Join(repoDir, "repository_ir.json")

	if err := representation.SaveRepository(repoIR, irPath); err != nil {
		h.logger.Error("failed to save repository IR", "error", err)
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageDone); err != nil {
		h.logger.Error("failed to mark job stage done", "job_id", jobID, "error", err)
	}
```

Insert the new step between the IR save and the `StageDone` update:

```go
	//Save IR File Representation
	irPath := filepath.Join(repoDir, "repository_ir.json")

	if err := representation.SaveRepository(repoIR, irPath); err != nil {
		h.logger.Error("failed to save repository IR", "error", err)
	}

	readmeText, err := h.walker.ReadReadme(repoDir)
	if err != nil {
		h.logger.Warn("failed to read readme", "repo_id", repoID, "error", err)
		readmeText = ""
	}

	// Non-fatal: overview generation failing (all LLM providers exhausted,
	// zero entrypoints detected, etc.) must never fail the whole ingestion
	// job - symbols/embeddings are the core value, the overview is
	// supplementary. See the design spec's Error handling section.
	if err := h.architectureService.GenerateOverview(ctx, repoID, readmeText); err != nil {
		h.logger.Warn("overview_generation_failed", "repo_id", repoID, "error", err)
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageDone); err != nil {
		h.logger.Error("failed to mark job stage done", "job_id", jobID, "error", err)
	}
```

- [ ] **Step 4: Build to verify it compiles**

Run: `cd api && go build ./...`
Expected: no errors.

- [ ] **Step 5: Run the full architecture/store/sidecar test suites to check nothing broke**

Run: `cd api && go test ./internal/architecture/... ./internal/store/... ./internal/sidecar/... -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/architecture/service.go api/internal/api/repos.go
git commit -m "feat(architecture): wire GenerateOverview into the ingestion job"
```

---

## Task 7: Surface `narrative_summary`/`concepts`/`reading_path` in `GET /architecture/:repoID`

**Files:**
- Modify: `api/internal/architecture/models.go`
- Modify: `api/internal/architecture/analyzer.go`

**Interfaces:**
- Consumes: `store.GetOverview` (Task 1), `BuildReadingPath` (Task 2).
- Produces: `Summary` gains `NarrativeSummary string`, `Concepts []Concept`, `ReadingPath []ReadingStep` fields (JSON: `narrative_summary`, `concepts`, `reading_path`).

- [ ] **Step 1: Add the new fields and `Concept` type to `models.go`**

Modify `api/internal/architecture/models.go`:

```go
package architecture

type Summary struct {
	RepoID string `json:"repo_id"`

	Statistics       Statistics    `json:"statistics"`
	Languages        []string      `json:"languages"`
	EntryPoints      []EntryPoint  `json:"entrypoints"`
	Components       []Component   `json:"components"`
	NarrativeSummary string        `json:"narrative_summary"`
	Concepts         []Concept     `json:"concepts"`
	ReadingPath      []ReadingStep `json:"reading_path"`
}

//Statistics contains stats info about the repository
type Statistics struct {
	FileCount   int `json:"file_count"`
	SymbolCount int `json:"symbol_count"`
	CallEdges   int `json:"call_edges"`
}

//Component represents an imp architectural building block
type Component struct {
	Name      string `json:"name"`
	FilePath  string `json:"file_path"`
	Type      string `json:"type"`
	Language  string `json:"language"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

//EntryPoint represents an executable entrypoint of the repo
type EntryPoint struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
	Type     string `json:"type"`
}

// Concept is a notable pattern/convention/domain term the LLM-generated
// overview flagged as worth explaining to a newcomer - see
// architecture.Service.GenerateOverview.
type Concept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
}
```

- [ ] **Step 2: Populate the new fields in `AnalyzeRepository`**

Modify `api/internal/architecture/analyzer.go` in full:

```go
package architecture

import (
	"context"
	"log/slog"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type Analyzer struct {
	logger *slog.Logger
	store  *store.Store
}

func NewAnalyzer(logger *slog.Logger, store *store.Store) *Analyzer {

	return &Analyzer{
		logger: logger,
		store:  store,
	}
}

func (a *Analyzer) AnalyzeRepository(ctx context.Context, repoID string) (*Summary, error) {

	a.logger.Info("architecture_analysis_started", "repo_id:", repoID)

	files, err := a.store.GetFiles(ctx, repoID)
	if err != nil {
		return nil, err
	}

	symbols, err := a.store.GetSymbols(ctx, repoID)
	if err != nil {
		return nil, err
	}

	callEdges, err := a.store.GetCallEdges(ctx, repoID)
	if err != nil {
		return nil, err
	}

	languages, err := a.store.GetLanguages(ctx, repoID)
	if err != nil {
		return nil, err
	}

	a.logger.Info("architecture_repo_loaded", "repo_id", repoID, "files", len(files), "symbols", len(symbols), "call_edges", len(callEdges), "languages", len(languages))

	components := DetectComponents(files, symbols)

	a.logger.Info("architecture_components_detected", "count", len(components))

	//=======Detect Entrypoints===========
	entrypoints := DetectEntrypoints(files, symbols)

	a.logger.Info("architecture_entrypoints_detected", "count", len(entrypoints))

	//=======Build the deterministic reading path (no LLM - see reading_path.go)===========
	readingPath := BuildReadingPath(entrypoints, callEdges)

	//=======Load the LLM-generated narrative summary/concepts, if generation has run===========
	overview, err := a.store.GetOverview(ctx, repoID)
	if err != nil {
		return nil, err
	}

	var narrativeSummary string
	// Non-nil so a repo where generation hasn't run yet (or failed) still
	// marshals this as `[]` in JSON, not `null` - same reasoning as
	// languages/entrypoints/components elsewhere in this package.
	concepts := []Concept{}

	if overview != nil {
		narrativeSummary = overview.NarrativeSummary
		for _, c := range overview.Concepts {
			concepts = append(concepts, Concept{Term: c.Term, Explanation: c.Explanation})
		}
	}

	//=========Build the Statistics=========
	stats := Statistics{
		FileCount:   len(files),
		SymbolCount: len(symbols),
		CallEdges:   len(callEdges),
	}

	summary := &Summary{
		RepoID:           repoID,
		Statistics:       stats,
		Languages:        languages,
		EntryPoints:      entrypoints,
		Components:       components,
		NarrativeSummary: narrativeSummary,
		Concepts:         concepts,
		ReadingPath:      readingPath,
	}

	a.logger.Info("architecture_analysis_complete", "repo_id", repoID, "files", stats.FileCount, "symbols", stats.SymbolCount, "components", len(summary.Components), "entrypoints", len(summary.EntryPoints), "reading_path_steps", len(summary.ReadingPath))

	return summary, nil
}
```

- [ ] **Step 3: Build and run the architecture package's tests**

Run: `cd api && go build ./... && go test ./internal/architecture/... -v`
Expected: build succeeds, all tests PASS (including Task 2's reading-path tests, unaffected by this change).

- [ ] **Step 4: Manually verify the live endpoint**

With the API server running against a repo that's already been ingested (or re-synced) after Task 6 landed:

```bash
curl http://localhost:8080/architecture/<repoID>
```

Expected: the JSON response now includes `narrative_summary`, `concepts`, and `reading_path` alongside the existing fields. For a repo ingested *before* this change (no `repo_overview` row yet), expect `narrative_summary: ""` and `concepts: []`, with `reading_path` still populated (it's computed fresh, not stored).

- [ ] **Step 5: Commit**

```bash
git add api/internal/architecture/models.go api/internal/architecture/analyzer.go
git commit -m "feat(architecture): surface narrative_summary/concepts/reading_path from GET /architecture/:repoID"
```

---

## Task 8: Frontend - render narrative summary, reading path, concepts on Overview

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/pages/Overview.tsx`

**Interfaces:**
- Consumes: `ArchitectureSummary` (extended, Task 7's response shape), existing `getArchitecture` (`frontend/src/api/architecture.ts`, unchanged), existing `repoRelativePath` (`frontend/src/lib/path.ts`, unchanged).

- [ ] **Step 1: Extend the frontend types**

Modify `frontend/src/api/types.ts` - find the existing `ArchitectureSummary` interface and the block around it:

```ts
export interface ArchitectureEntrypoint {
  name: string;
  file_path: string;
  language: string;
  type: string;
}

export interface ArchitectureComponent {
  name: string;
  file_path: string;
  type: string;
  language: string;
  start_line: number;
  end_line: number;
}

export interface ArchitectureConcept {
  term: string;
  explanation: string;
}

export interface ArchitectureReadingStep {
  symbol: string;
  file_path: string;
  reason: string;
}

export interface ArchitectureSummary {
  repo_id: string;
  statistics: ArchitectureStatistics;
  languages: string[];
  entrypoints: ArchitectureEntrypoint[];
  components: ArchitectureComponent[];
  narrative_summary: string;
  concepts: ArchitectureConcept[];
  reading_path: ArchitectureReadingStep[];
}
```

(Only `ArchitectureConcept`, `ArchitectureReadingStep`, and the three new fields on `ArchitectureSummary` are new - `ArchitectureEntrypoint`/`ArchitectureComponent` are shown for location context, leave them as they are.)

- [ ] **Step 2: Add the narrative summary, reading path, and concepts sections to Overview.tsx**

Modify `frontend/src/pages/Overview.tsx`. First, add `useNavigate` to the existing `react-router-dom` import:

```tsx
import { useNavigate, useParams } from "react-router-dom";
```

Then add a `navigate` call near the top of the component body, right after the existing `repoId` line:

```tsx
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
```

Then replace the existing `{data.entrypoints.length > 0 && (...)}` block (the last section in the returned JSX) with the following three sections - narrative summary first (placed right after the header, before the language tags), reading path (replacing the old entrypoints block), and concepts (new, at the end):

Insert immediately after the closing `</div>` of the repo name/link header block (before the `{data.languages.length > 0 && (...)}` block):

```tsx
      {data.narrative_summary && (
        <p className="max-w-2xl text-sm leading-relaxed text-ink">{data.narrative_summary}</p>
      )}
```

Then replace the existing entrypoints block at the end of the file:

```tsx
      {data.entrypoints.length > 0 && (
        <section>
          <h2 className="text-sm text-ink-dim">Entrypoints</h2>
          <ul className="mt-2 space-y-1 font-mono text-sm">
            {data.entrypoints.map((ep, i) => (
              <li key={i} className="text-ink">
                {ep.name} <span className="text-ink-dim">&middot; {repoRelativePath(ep.file_path, repoId)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
```

with:

```tsx
      {data.reading_path.length > 0 ? (
        <section>
          <h2 className="text-sm text-ink-dim">Where to start reading</h2>
          <ol className="mt-2 space-y-2 font-mono text-sm">
            {data.reading_path.map((step, i) => (
              <li key={`${step.symbol}-${i}`} className="text-ink">
                <div className="flex flex-wrap items-baseline gap-2">
                  <span>{step.symbol}</span>
                  {step.file_path && (
                    <span className="text-xs text-ink-dim">{repoRelativePath(step.file_path, repoId!)}</span>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-ink-faint">
                  <span>{step.reason}</span>
                  <button
                    type="button"
                    onClick={() =>
                      navigate(`/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does ${step.symbol} do?`)}`)
                    }
                    className="border border-line-faint px-1.5 py-0.5 text-ink-dim transition-colors hover:border-accent hover:text-ink"
                  >
                    ask about this
                  </button>
                </div>
              </li>
            ))}
          </ol>
        </section>
      ) : (
        data.entrypoints.length > 0 && (
          <section>
            <h2 className="text-sm text-ink-dim">Entrypoints</h2>
            <ul className="mt-2 space-y-1 font-mono text-sm">
              {data.entrypoints.map((ep, i) => (
                <li key={i} className="text-ink">
                  {ep.name} <span className="text-ink-dim">&middot; {repoRelativePath(ep.file_path, repoId!)}</span>
                </li>
              ))}
            </ul>
          </section>
        )
      )}

      {data.concepts.length > 0 && (
        <section>
          <h2 className="text-sm text-ink-dim">Notable concepts</h2>
          <ul className="mt-2 space-y-2 font-mono text-sm">
            {data.concepts.map((concept, i) => (
              <li key={`${concept.term}-${i}`} className="text-ink">
                <div className="flex flex-wrap items-baseline gap-2">
                  <span>{concept.term}</span>
                  <button
                    type="button"
                    onClick={() =>
                      navigate(
                        `/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does "${concept.term}" mean in this codebase?`)}`,
                      )
                    }
                    className="border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
                  >
                    ask about this
                  </button>
                </div>
                <p className="text-xs text-ink-faint">{concept.explanation}</p>
              </li>
            ))}
          </ul>
        </section>
      )}
```

- [ ] **Step 3: Typecheck**

Run: `cd frontend && npx tsc -b`
Expected: no errors.

- [ ] **Step 4: Lint**

Run: `cd frontend && npm run lint`
Expected: no new errors introduced by this change (pre-existing warnings elsewhere in the codebase are not this task's concern).

- [ ] **Step 5: Live verification in the browser**

With the full stack running (Postgres, API, extraction service, frontend dev server) and a repo re-synced after Tasks 6-7 landed (so it has a `repo_overview` row):

1. Navigate to `/repos/<repoId>/overview`.
2. Confirm the narrative summary paragraph renders below the repo name/link.
3. Confirm "Where to start reading" renders an ordered list with symbol, file path, and reason for each step.
4. Click "ask about this" on a reading-path step - confirm it navigates to Chat with the question pre-filled.
5. Confirm "Notable concepts" renders (if the LLM returned any) with term, explanation, and its own "ask about this" link.
6. Load Overview for a repo that has *not* been re-synced since this change (no `repo_overview` row) - confirm the page still renders correctly: narrative summary section hidden, reading path falls back to the old flat entrypoints list (since `reading_path` is always computed fresh regardless of `repo_overview`, this fallback path mainly matters if a repo also has zero entrypoints and zero call edges - otherwise reading_path will already be populated even without stored overview data).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/types.ts frontend/src/pages/Overview.tsx
git commit -m "feat(frontend): render narrative summary, reading path, and concepts on Overview"
```
