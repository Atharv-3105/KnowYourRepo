# Collapse Agent/Tool-Calling Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `agent` package's `Planner`/`HybridPlanner`/`Executor`/four-`Tool` framework with a single retrieval path: always run semantic search (which already auto-expands call-graph edges), and conditionally append an architecture-overview summary when the question's phrasing calls for it.

**Architecture:** A new `api/internal/answer` package replaces `api/internal/agent` entirely. Its `Service.Answer()` does: freshness check → `HybridRetriever.Search` (unconditional) → `architecture.Service.BuildSummary` (only if `WantsArchitectureOverview(query)`) → `rag.Service.AnswerQuestion`. The two existing chat routes (`POST /chat`, `POST /agent/chat`) collapse into one `POST /chat` backed by this new service. The Python sidecar's `/classify` route (and everything that only existed to serve it) is deleted since nothing calls it anymore.

**Tech Stack:** Go 1.25 (Gin, `pgx`), Python/FastAPI sidecar, React/TS frontend (Vite, TanStack Query).

**Spec:** `docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md`

## Global Constraints

- `ChatRequest`/`AgentChatResponse` Go types and their JSON field names (including `tools_used`, `sources`) do not change — the frontend depends on these exact names (spec's explicit non-goal).
- The architecture-overview pseudo-result (`Symbol: "architecture_overview"`, `FilePath: repoID`) must be excluded from `buildSources()`'s output — it is not a real file-grounded citation.
- No new LLM/network round trip is introduced to replace the deleted classify call — `WantsArchitectureOverview` is a deterministic keyword check, ported as-is from the existing `architectureKeywords` list in `api/internal/agent/planner.go`.
- Do not touch: ingestion pipeline, `chat.Store`/session memory, `contextbuilder`/`rag.Service` internals, freshness-sync mechanics beyond moving `WantsReingestion` to its new package, or any frontend page other than `Chat.tsx`'s one API call path.

---

## Task 1: `answer` package — intent-detection functions

**Files:**
- Create: `api/internal/answer/intent.go`
- Test: `api/internal/answer/intent_test.go`

**Interfaces:**
- Produces: `answer.WantsReingestion(query string) bool`, `answer.WantsArchitectureOverview(query string) bool` — both pure functions, no dependencies. Task 2 consumes both by name.

- [ ] **Step 1: Write the failing test**

```go
package answer

import "testing"

func TestWantsReingestion(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"what is the latest state of this repo?", true},
		{"has this changed recently?", true},
		{"is this up to date?", true},
		{"what does the setup function do?", false},
		{"", false},
	}

	for _, c := range cases {
		if got := WantsReingestion(c.query); got != c.want {
			t.Errorf("WantsReingestion(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestWantsArchitectureOverview(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"give me a high-level overview of this repo", true},
		{"what are the entrypoints?", true},
		{"what languages does this repository use?", true},
		{"what does the setup function do?", false},
		{"who calls configure?", false},
	}

	for _, c := range cases {
		if got := WantsArchitectureOverview(c.query); got != c.want {
			t.Errorf("WantsArchitectureOverview(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run (from `api/`): `go test ./internal/answer/... -run TestWants -v`
Expected: FAIL — `package answer is not in std` / `undefined: WantsReingestion` (the package doesn't exist yet).

- [ ] **Step 3: Write minimal implementation**

```go
package answer

import "strings"

// reingestKeywords are checked to decide whether a question implies the
// user wants the repository's current/latest state, independent of what
// else the question needs to answer it. Ported from
// api/internal/agent/reingest_intent.go - unrelated to tool selection,
// it just needed a new home once the agent package was removed.
var reingestKeywords = []string{
	"latest", "recent changes", "recently changed", "up to date", "up-to-date",
	"did anything change", "has this changed", "new commits", "newest version",
	"current state", "most recent",
}

// architectureKeywords are checked to decide whether a question is asking
// about the repository's overall structure rather than a specific symbol.
// Ported verbatim from api/internal/agent/planner.go's architectureKeywords.
var architectureKeywords = []string{
	"architecture", "entrypoint", "entry point", "component", "overview",
	"structure of the repo", "repository structure", "statistics", "high level",
	"high-level", "what languages",
}

func containsAny(query string, words []string) bool {
	for _, w := range words {
		if strings.Contains(query, w) {
			return true
		}
	}
	return false
}

// WantsReingestion reports whether a question implies the user wants the
// repository re-synced against its remote before/while being answered.
// Deliberately deterministic, not LLM-classified - the phrasing patterns
// here are mechanical enough that keyword matching is reliable.
func WantsReingestion(query string) bool {
	return containsAny(strings.ToLower(query), reingestKeywords)
}

// WantsArchitectureOverview reports whether a question is asking about the
// repository's overall structure (entrypoints, components, statistics,
// languages) rather than a specific symbol's behavior.
func WantsArchitectureOverview(query string) bool {
	return containsAny(strings.ToLower(query), architectureKeywords)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/answer/... -run TestWants -v`
Expected: PASS (both `TestWantsReingestion` and `TestWantsArchitectureOverview`).

- [ ] **Step 5: Commit**

```bash
git add api/internal/answer/intent.go api/internal/answer/intent_test.go
git commit -m "feat(answer): add reingest/architecture-overview intent detection

Ported from api/internal/agent/reingest_intent.go and planner.go's
architectureKeywords - first piece of the new answer package that
replaces the agent/tool-calling framework."
```

---

## Task 2: `answer.Service` — the new single retrieval path

**Files:**
- Create: `api/internal/answer/service.go`
- Test: `api/internal/answer/service_test.go`

**Interfaces:**
- Consumes: `answer.WantsReingestion`, `answer.WantsArchitectureOverview` (Task 1); `retrieval.NewHybridRetriever(store, sidecar, logger) *HybridRetriever` with method `Search(ctx, repoID, query string) ([]retrieval.RetrievalResult, error)`; `architecture.NewService(logger, analyzer) *Service` with method `BuildSummary(ctx, repoID string) (*architecture.Summary, error)`; `rag.NewService(builder, sidecar, logger) *Service` with method `AnswerQuestion(ctx, query, history string, results []retrieval.RetrievalResult) (string, error)`.
- Produces: `answer.RepoSyncer` interface (`SyncIfStale(ctx, repoID string) error`), `answer.NewService(retriever *retrieval.HybridRetriever, architectureService *architecture.Service, ragService *rag.Service, syncer RepoSyncer, logger *slog.Logger) *Service`, and `(*Service) Answer(ctx, repoID, query, history string) (answer string, refreshing bool, results []retrieval.RetrievalResult, err error)`. Task 3 consumes `NewService` and `Answer` with these exact signatures.

- [ ] **Step 1: Write the failing test**

```go
package answer

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

// fakeSyncer lets the test assert whether a background sync was triggered
// without needing a real repo-sync implementation.
type fakeSyncer struct {
	called chan string
}

func (f *fakeSyncer) SyncIfStale(ctx context.Context, repoID string) error {
	f.called <- repoID
	return nil
}

func newTestService(t *testing.T, sidecarURL string, syncer RepoSyncer) (*Service, *store.Store) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable"
	}

	dbStore, err := store.NewStore(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sidecarClient := sidecar.NewClient(sidecarURL)
	retriever := retrieval.NewHybridRetriever(dbStore, sidecarClient, logger)
	analyzer := architecture.NewAnalyzer(logger, dbStore)
	architectureService := architecture.NewService(logger, analyzer)
	builder := contextbuilder.NewBuilder(logger)
	ragService := rag.NewService(builder, sidecarClient, logger)

	return NewService(retriever, architectureService, ragService, syncer, logger), dbStore
}

func TestService_Answer_SemanticOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			results := []sidecar.SearchResult{
				{
					ID:       "setup_part_0",
					Document: "func setup() { configure() }",
					Metadata: map[string]interface{}{
						"repo_id": "repo_answer_1", "file_path": "main.go", "symbol": "setup",
					},
					Distance: 0.1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "setup calls configure"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sync := &fakeSyncer{called: make(chan string, 1)}
	svc, dbStore := newTestService(t, server.URL, sync)
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	if _, err := dbStore.InsertFile(ctx, "repo_answer_1", "main.go", "go", ""); err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}
	if err := dbStore.InsertCallEdge(ctx, store.CallEdge{
		RepoID: "repo_answer_1", CallerSymbol: "setup", CallerFilePath: "main.go", CalleeSymbol: "configure",
	}); err != nil {
		t.Fatalf("failed to insert call edge: %v", err)
	}

	answer, refreshing, results, err := svc.Answer(ctx, "repo_answer_1", "what does setup do?", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}
	if answer != "setup calls configure" {
		t.Errorf("expected answer from sidecar, got %q", answer)
	}
	if refreshing {
		t.Error("expected refreshing=false for a non-freshness question")
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result (semantic only, no architecture overview), got %d: %+v", len(results), results)
	}
	if results[0].Symbol != "setup" {
		t.Errorf("expected semantic result for 'setup', got %+v", results[0])
	}

	select {
	case <-sync.called:
		t.Error("did not expect a background sync for a non-freshness question")
	case <-time.After(200 * time.Millisecond):
		// expected: no sync triggered
	}
}

func TestService_Answer_ArchitectureOverviewAppended(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]sidecar.SearchResult{})
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "this repo has 1 file"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, dbStore := newTestService(t, server.URL, nil)
	defer dbStore.Close()
	defer func() {
		dbStore.DB().ExecContext(context.Background(), "TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE")
	}()

	if _, err := dbStore.InsertFile(ctx, "repo_answer_2", "main.go", "go", ""); err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	_, _, results, err := svc.Answer(ctx, "repo_answer_2", "give me a high-level overview of this repo", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Symbol == "architecture_overview" {
			found = true
			if r.FilePath != "repo_answer_2" {
				t.Errorf("expected architecture overview FilePath to be the repo ID, got %q", r.FilePath)
			}
		}
	}
	if !found {
		t.Fatalf("expected an architecture_overview result to be appended, got %+v", results)
	}
}

func TestService_Answer_TriggersBackgroundSyncOnFreshnessQuestion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]sidecar.SearchResult{})
		case "/chat":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sidecar.ChatResponse{Answer: "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sync := &fakeSyncer{called: make(chan string, 1)}
	svc, dbStore := newTestService(t, server.URL, sync)
	defer dbStore.Close()

	answer, refreshing, _, err := svc.Answer(ctx, "repo_answer_3", "has this changed recently?", "")
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}
	if !refreshing {
		t.Error("expected refreshing=true for a freshness question")
	}
	if answer != "ok" {
		t.Errorf("expected answer to still be produced, got %q", answer)
	}

	select {
	case repoID := <-sync.called:
		if repoID != "repo_answer_3" {
			t.Errorf("expected sync for repo_answer_3, got %q", repoID)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected background sync to be triggered")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run (from `api/`): `go test ./internal/answer/... -run TestService_Answer -v`
Expected: FAIL — `undefined: RepoSyncer` / `undefined: NewService` (Service doesn't exist yet).

- [ ] **Step 3: Write minimal implementation**

```go
package answer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

// RepoSyncer lets the service trigger a background re-ingestion check
// without depending on the ingestion/api packages directly - implemented
// by RepoHandler and injected at construction time. Ported unchanged from
// api/internal/agent/service.go.
type RepoSyncer interface {
	SyncIfStale(ctx context.Context, repoID string) error
}

const maxArchitectureListItems = 15

// Service is the single entry point into answering a repository question:
// always runs semantic search (which already auto-expands call-graph
// edges via HybridRetriever), and only conditionally adds an architecture
// overview when the question's phrasing calls for one. Replaces the old
// Planner -> HybridPlanner -> Executor -> Tool framework in api/internal/agent,
// which duplicated work HybridRetriever already does automatically or data
// (history) already passed separately - see
// docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md.
type Service struct {
	retriever           *retrieval.HybridRetriever
	architectureService *architecture.Service
	ragService          *rag.Service
	syncer              RepoSyncer
	logger              *slog.Logger
}

func NewService(
	retriever *retrieval.HybridRetriever,
	architectureService *architecture.Service,
	ragService *rag.Service,
	syncer RepoSyncer,
	logger *slog.Logger,
) *Service {
	return &Service{
		retriever:           retriever,
		architectureService: architectureService,
		ragService:          ragService,
		syncer:              syncer,
		logger:              logger,
	}
}

// Answer plans, executes, and answers a repository question. If the
// question implies the user wants the repo's latest state, a background
// re-ingestion check is triggered (fire-and-forget - the answer is still
// built from whatever's currently indexed; refreshing reports true so the
// caller can tell the user this answer might be slightly stale).
//
// The merged retrieval results are also returned (not just the final answer
// string) so the caller can surface them as structured citations - these are
// exactly the results that were fed into the LLM prompt for this answer.
func (s *Service) Answer(ctx context.Context, repoID, query, history string) (answer string, refreshing bool, results []retrieval.RetrievalResult, err error) {

	s.logger.Info("answer_service_started", "repo_id", repoID, "query", query)

	if WantsReingestion(query) {
		refreshing = true
		s.triggerBackgroundSync(repoID)
	}

	results, err = s.retriever.Search(ctx, repoID, query)
	if err != nil {
		s.logger.Error("answer_service_search_failed", "repo_id", repoID, "error", err)
		return "", refreshing, nil, err
	}

	if WantsArchitectureOverview(query) {
		if overview, ovErr := s.architectureService.BuildSummary(ctx, repoID); ovErr != nil {
			// Not fatal - a request that already has real semantic results
			// shouldn't fail over a missed architecture overview.
			s.logger.Warn("answer_service_architecture_overview_failed", "repo_id", repoID, "error", ovErr)
		} else {
			results = append(results, overviewAsResult(repoID, overview))
		}
	}

	s.logger.Info("answer_service_retrieval_complete", "repo_id", repoID, "results", len(results))

	answer, err = s.ragService.AnswerQuestion(ctx, query, history, results)
	if err != nil {
		s.logger.Error("answer_service_failed", "repo_id", repoID, "error", err)
		return "", refreshing, nil, err
	}

	s.logger.Info("answer_service_completed", "repo_id", repoID)

	return answer, refreshing, results, nil
}

func (s *Service) triggerBackgroundSync(repoID string) {
	if s.syncer == nil {
		return
	}

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.syncer.SyncIfStale(syncCtx, repoID); err != nil {
			s.logger.Warn("answer_background_sync_failed", "repo_id", repoID, "error", err)
		}
	}()
}

// overviewAsResult builds the same text block the old ArchitectureTool
// produced, as a pseudo RetrievalResult so it flows into the LLM prompt
// through the same path as real search results. FilePath is set to the
// repo ID (not a real file) - callers building citations from these
// results must filter this one out by Symbol == "architecture_overview".
func overviewAsResult(repoID string, summary *architecture.Summary) retrieval.RetrievalResult {
	var b strings.Builder

	fmt.Fprintf(&b, "Repository statistics: %d files, %d symbols, %d call edges.\n",
		summary.Statistics.FileCount, summary.Statistics.SymbolCount, summary.Statistics.CallEdges)
	fmt.Fprintf(&b, "Languages: %s\n", strings.Join(summary.Languages, ", "))

	b.WriteString("EntryPoints:\n")
	for i, ep := range summary.EntryPoints {
		if i >= maxArchitectureListItems {
			break
		}
		fmt.Fprintf(&b, "- %s (%s) in %s\n", ep.Name, ep.Type, ep.FilePath)
	}

	b.WriteString("Components:\n")
	for i, c := range summary.Components {
		if i >= maxArchitectureListItems {
			break
		}
		fmt.Fprintf(&b, "- %s (%s) in %s\n", c.Name, c.Type, c.FilePath)
	}

	return retrieval.RetrievalResult{
		Symbol:   "architecture_overview",
		FilePath: repoID,
		Document: b.String(),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/answer/... -v`
Expected: PASS for all of `TestWantsReingestion`, `TestWantsArchitectureOverview`, `TestService_Answer_SemanticOnly`, `TestService_Answer_ArchitectureOverviewAppended`, `TestService_Answer_TriggersBackgroundSyncOnFreshnessQuestion`.

Note: like the existing `internal/retrieval/hybrid_test.go`, these tests need a real reachable Postgres (set `TEST_DATABASE_URL`, or ensure `postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable` is reachable — the docker-compose `postgres` service works if its port is mapped, or run against your local dev Postgres).

- [ ] **Step 5: Commit**

```bash
git add api/internal/answer/service.go api/internal/answer/service_test.go
git commit -m "feat(answer): add Service.Answer - single retrieval path

Replaces agent.Service's Planner->Executor pipeline with an
unconditional semantic search plus a conditional architecture-overview
addition, per the approved design spec."
```

---

## Task 3: Wire `RepoHandler` to `answer.Service`, collapse to one `/chat` route, delete `agent` package

**Files:**
- Modify: `api/internal/api/repos.go:53-100` (NewRepoHandler wiring)
- Modify: `api/internal/api/agent_chat.go` (rename handler logic to use `answer.Service`)
- Delete: `api/internal/api/chat.go`'s `Chat` handler function and its now-unused `ChatResponse` type (keep `ChatRequest` — still used)
- Modify: `api/internal/api/server.go:67-101` (route registration)
- Delete: `api/internal/agent/` (entire directory: `planner.go`, `hybrid_planner.go`, `executor.go`, `models.go`, `service.go`, `reingest_intent.go`, `tools/semantic.go`, `tools/graph.go`, `tools/architecture.go`, `tools/memory.go`, and any `*_test.go` alongside them)

**Interfaces:**
- Consumes: `answer.NewService(retriever, architectureService, ragService, syncer, logger) *answer.Service` and `(*answer.Service).Answer(ctx, repoID, query, history string) (string, bool, []retrieval.RetrievalResult, error)` (Task 2).
- Produces: `RepoHandler.answerService *answer.Service` field (replaces `agentService *agent.Service`); handler function `RepoHandler.Chat` now serves `POST /chat` using `answerService`.

- [ ] **Step 1: Update `RepoHandler` struct and `NewRepoHandler`**

In `api/internal/api/repos.go`, replace the `agentService *agent.Service` field with `answerService *answer.Service`, and replace this block (currently lines ~66-93):

```go
	agentTools := map[agent.ToolName]agent.Tool{
		agent.ToolSemantic:     tools.NewSemanticTool(hybridRetriever),
		agent.ToolGraph:        tools.NewGraphTool(store),
		agent.ToolMemory:       tools.NewMemoryTool(),
		agent.ToolArchitecture: tools.NewArchitectureTool(architectureService),
	}

	fallbackPlanner := agent.NewPlanner()
	hybridPlanner := agent.NewHybridPlanner(sidecar, fallbackPlanner, logger)
	executor := agent.NewExecutor(agentTools, logger)

	h := &RepoHandler{
		...
	}

	h.agentService = agent.NewService(hybridPlanner, executor, ragService, h, logger)
```

with:

```go
	h := &RepoHandler{
		logger:              logger,
		store:               store,
		sidecar:             sidecar,
		parser:              ingestion.NewParser(logger),
		extractor:           graph.NewExtractor(logger),
		cloner:              ingestion.NewCloner(logger),
		walker:              ingestion.NewWalker(logger),
		hybridRetriever:     hybridRetriever,
		contextBuilder:      builder,
		ragService:          ragService,
		chatStore:           chat.NewStore(),
		architectureService: architectureService,
	}

	// answerService needs h as its RepoSyncer (for freshness-triggered
	// background sync), so it's constructed after h exists - same
	// deferred-wiring pattern as workerPool below.
	h.answerService = answer.NewService(hybridRetriever, architectureService, ragService, h, logger)
```

Update the import block: remove `"github.com/atharva-3105/KnowYourRepo/internal/agent"` and `"github.com/atharva-3105/KnowYourRepo/internal/agent/tools"`, add `"github.com/atharva-3105/KnowYourRepo/internal/answer"`.

Update the `RepoHandler` struct field: change `agentService *agent.Service` to `answerService *answer.Service`.

- [ ] **Step 2: Rewrite the chat handler**

Delete `api/internal/api/chat.go`'s `Chat` function and the now-unused `ChatResponse` type — keep `ChatRequest` (it stays in `chat.go`, still used by the handler below).

In `api/internal/api/agent_chat.go`, replace the `AgentChat` function (and rename the file's role — it's now the only chat handler) with:

```go
func (h *RepoHandler) Chat(c *gin.Context) {

	var req ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	history := h.chatStore.RecentMessages(req.SessionID, 6)
	conversationContext := chat.BuildConversationContext(history)

	h.logger.Info("chat_conversation_loaded", "session_id", req.SessionID, "messages", len(history))

	h.chatStore.AddMessage(req.SessionID, "user", req.Question)

	answer, refreshing, results, err := h.answerService.Answer(
		c.Request.Context(),
		req.RepoID,
		req.Question,
		conversationContext,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.chatStore.AddMessage(req.SessionID, "assistant", answer)
	h.logger.Info("chat_message_saved", "session_id", req.SessionID, "role", "assistant")

	c.JSON(http.StatusOK, AgentChatResponse{
		Answer:     answer,
		Tools:      toolsUsed(results),
		Refreshing: refreshing,
		Sources:    buildSources(results),
	})
}

// toolsUsed reports which retrieval capabilities actually contributed to
// this answer, for the frontend's per-tool citation badges (Brick 25).
// "semantic" always ran; "architecture" only shows up if an overview
// result was actually appended.
func toolsUsed(results []retrieval.RetrievalResult) []string {
	tools := []string{"semantic"}
	for _, r := range results {
		if r.Symbol == "architecture_overview" {
			tools = append(tools, "architecture")
			break
		}
	}
	return tools
}
```

Also update `buildSources` (same file) to exclude the architecture-overview pseudo-result from citations:

```go
func buildSources(results []retrieval.RetrievalResult) []Source {

	sources := make([]Source, 0, len(results))

	for _, r := range results {
		if r.Symbol == "architecture_overview" {
			continue
		}

		sources = append(sources, Source{
			Symbol:    r.Symbol,
			FilePath:  r.FilePath,
			StartLine: metadataInt(r.Metadata, "start_line"),
			EndLine:   metadataInt(r.Metadata, "end_line"),
		})
	}

	return sources
}
```

- [ ] **Step 3: Update route registration**

In `api/internal/api/server.go`, in `registerRoutes()`, replace:

```go
	//Chat Route
	s.router.POST("/chat", repoHandler.Chat)
	...
	//Agent-Based Chat Route
	s.router.POST("/agent/chat", repoHandler.AgentChat)
```

with a single:

```go
	//Chat Route
	s.router.POST("/chat", repoHandler.Chat)
```

(Before this task, `POST /chat` routed to the old `Chat` handler in `chat.go` and `POST /agent/chat` routed to the separate `AgentChat` handler in `agent_chat.go`. Step 2 deleted the old `Chat` handler and renamed `AgentChat`'s logic into a new `Chat` method, so after this step there is exactly one `Chat` method and exactly one route pointing to it.)

- [ ] **Step 4: Delete the `agent` package**

```bash
rm -rf api/internal/agent
```

- [ ] **Step 5: Build and vet**

Run (from `api/`):
```bash
go build ./...
go vet ./...
```
Expected: both succeed with no errors. If `go build` reports an unused import or a leftover reference to `agent.` anywhere, grep for it:
```bash
grep -rn "internal/agent\"" --include="*.go" .
```
and remove that import/usage — every reference to the old `agent` package must be gone.

- [ ] **Step 6: Run the full Go test suite**

Run: `go test ./...`
Expected: all packages pass, including the new `internal/answer` tests from Tasks 1-2 and the pre-existing suites (`internal/graph`, `internal/ingestion`, `internal/retrieval`, `internal/sidecar`, `internal/store`) unaffected by this change.

- [ ] **Step 7: Manual verification against a real running stack**

Start the stack (`docker compose up -d` from the repo root, or `make dev`). With a repo already ingested (`repo_id` from a prior `POST /repos`), run:

```bash
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"repo_id":"<real-repo-id>","question":"what does the main function do?","session_id":"test-session-1"}' | python3 -m json.tool
```

Expected: `200 OK`, `tools_used` is `["semantic"]`, `sources` contains real file-grounded citations (no `architecture_overview` entry).

```bash
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"repo_id":"<real-repo-id>","question":"give me a high-level overview of this repository","session_id":"test-session-1"}' | python3 -m json.tool
```

Expected: `200 OK`, `tools_used` is `["semantic", "architecture"]` (or just `["semantic"]` if there happened to be zero semantic hits — either way no error), `sources` still contains no `architecture_overview` entry (filtered out), and the answer text reflects real repository statistics.

Confirm `POST /agent/chat` now returns `404`:
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/agent/chat -d '{}'
```
Expected: `404`.

- [ ] **Step 8: Commit**

```bash
git add api/internal/api/repos.go api/internal/api/agent_chat.go api/internal/api/chat.go api/internal/api/server.go
git rm -r api/internal/agent
git commit -m "refactor: collapse /chat and /agent/chat into one answer.Service-backed route

Deletes the Planner/HybridPlanner/Executor/Tool framework entirely -
see docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md
for the audit that motivated this."
```

---

## Task 4: Delete the sidecar Go client's Classify method

**Files:**
- Delete: `api/internal/sidecar/classify.go`

**Interfaces:**
- None produced or consumed — this is a pure deletion with no remaining callers after Task 3 removed `HybridPlanner` (the only caller of `sidecar.Client.Classify`).

- [ ] **Step 1: Confirm no remaining references**

```bash
grep -rn "\.Classify(\|ClassifyRequest\|ClassifyResponse\|ErrClassificationUnavailable" api --include="*.go"
```
Expected: no output (Task 3 already deleted `hybrid_planner.go`, the only caller).

- [ ] **Step 2: Delete the file**

```bash
rm api/internal/sidecar/classify.go
```

- [ ] **Step 3: Build and test**

Run (from `api/`):
```bash
go build ./...
go test ./internal/sidecar/...
```
Expected: both succeed — `client_test.go` has no `Classify` tests (confirmed during planning), so nothing breaks.

- [ ] **Step 4: Commit**

```bash
git add -u api/internal/sidecar
git commit -m "refactor: remove sidecar.Client.Classify - no longer called

Its only caller (agent.HybridPlanner) was deleted in the prior commit."
```

---

## Task 5: Python sidecar — remove `/classify`

**Files:**
- Delete: `extraction-service/app/routes/classify.py`
- Delete: `extraction-service/app/services/classify.py`
- Delete: `extraction-service/app/models/classify.py`
- Modify: `extraction-service/app/main.py` (remove the classify router import/registration)
- Modify: `extraction-service/app/providers/router.py` (remove `"classify"` from `TASK_PROVIDER_ORDER`)

**Interfaces:**
- None produced — pure deletion. No other route or service imports anything from `app.routes.classify`, `app.services.classify`, or `app.models.classify` (confirmed: `classify` was only ever reached via its own route).

- [ ] **Step 1: Confirm no remaining references before deleting**

```bash
grep -rln "classify" extraction-service/app --include="*.py" | grep -v __pycache__
```
Expected output: exactly `app/main.py`, `app/routes/classify.py`, `app/services/classify.py`, `app/models/classify.py`, `app/providers/router.py` — if anything else appears, read it before proceeding (it may be an unrelated match, e.g. a comment).

- [ ] **Step 2: Remove the files and their wiring**

```bash
rm extraction-service/app/routes/classify.py
rm extraction-service/app/services/classify.py
rm extraction-service/app/models/classify.py
```

In `extraction-service/app/main.py`, remove these two lines:
```python
from app.routes.classify import router as classify_router
```
and
```python
app.include_router(classify_router)
```

In `extraction-service/app/providers/router.py`, change:
```python
TASK_PROVIDER_ORDER: dict[str, list[str]] = {
    "answer" : ["groq", "gemini", "cerebras", "openrouter"],
    "classify": ["groq", "gemini", "cerebras", "openrouter"],
    "default": ["groq", "gemini", "cerebras", "openrouter"],
}
```
to:
```python
TASK_PROVIDER_ORDER: dict[str, list[str]] = {
    "answer" : ["groq", "gemini", "cerebras", "openrouter"],
    "default": ["groq", "gemini", "cerebras", "openrouter"],
}
```

- [ ] **Step 3: Verify the app still imports and starts cleanly**

```bash
cd extraction-service
.venv/bin/python -c "from app.main import app; print('ok:', [r.path for r in app.routes])"
```
Expected: prints `ok: [...]` with `/health`, `/embed/batch`, `/search`, `/chat`, `/metrics` in the list, and no `/classify`, and no import error.

- [ ] **Step 4: Start it for real and hit health**

```bash
.venv/bin/uvicorn app.main:app --host 0.0.0.0 --port 8000 &
sleep 2
curl -s http://localhost:8000/health
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8000/classify -d '{}'
kill %1
```
Expected: `/health` returns `{"status":"ok",...}`; `/classify` returns `404`.

- [ ] **Step 5: Commit**

```bash
cd ..
git add -u extraction-service
git rm extraction-service/app/routes/classify.py extraction-service/app/services/classify.py extraction-service/app/models/classify.py
git commit -m "refactor: remove /classify endpoint - no caller left after agent removal"
```

---

## Task 6: Frontend — point at `/chat`, remove dead tool color

**Files:**
- Modify: `frontend/src/api/chat.ts`
- Modify: `frontend/src/lib/toolColor.ts`

**Interfaces:**
- Consumes: nothing new — `AgentChatRequest`/`AgentChatResponse` types (`frontend/src/api/types.ts`) are unchanged per the Global Constraints.
- Produces: nothing new — `agentChat` keeps its exact name and signature so `Chat.tsx` needs no changes.

- [ ] **Step 1: Update the endpoint path**

In `frontend/src/api/chat.ts`, replace:
```typescript
import { api } from "./client";
import type { AgentChatRequest, AgentChatResponse } from "./types";

// Only the agentic endpoint is wrapped - plain POST /chat has no tools_used/
// sources and nothing in the frontend plan (Bricks 6-10) uses it.
export const agentChat = (req: AgentChatRequest) =>
  api.post<AgentChatResponse>("/agent/chat", req);
```
with:
```typescript
import { api } from "./client";
import type { AgentChatRequest, AgentChatResponse } from "./types";

// POST /chat now runs the single answer.Service retrieval path (see
// docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md) -
// the old separate /agent/chat endpoint no longer exists.
export const agentChat = (req: AgentChatRequest) =>
  api.post<AgentChatResponse>("/chat", req);
```

(`agentChat`'s name is kept unchanged - Chat.tsx imports it by this name and renaming it is out of scope per the Global Constraints.)

- [ ] **Step 2: Remove the dead `graph` tool color entry**

In `frontend/src/lib/toolColor.ts`, replace the whole file with:
```typescript
// Maps the real tool names a chat answer can report in tools_used
// ("semantic" always runs; "architecture" only when the question's
// phrasing calls for an overview - see answer.Service.Answer and
// api/internal/api/agent_chat.go's toolsUsed) to the Field Notes
// per-tool accent tokens, so the badge encodes real information about
// which capability contributed to this answer, not decoration.
// "semantic" reuses --color-accent since it's the retrieval path that
// always runs, not a fourth invented hue.
const TOOL_BADGE_CLASSES: Record<string, string> = {
  semantic: "border-accent/40 text-accent",
  architecture: "border-tool-architecture/40 text-tool-architecture",
};

export function toolBadgeClasses(tool: string): string {
  return TOOL_BADGE_CLASSES[tool] ?? "border-line-faint text-ink-dim";
}
```

- [ ] **Step 3: Type-check and build**

Run (from `frontend/`):
```bash
npx tsc -b
npm run build
```
Expected: both clean, no errors (no other file references the removed `graph` entry — confirmed by grep below).

```bash
grep -rn "toolBadgeClasses\|TOOL_BADGE_CLASSES" frontend/src
```
Expected: only `toolColor.ts` (definition) and `Chat.tsx` (the one call site, unchanged) appear.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/chat.ts frontend/src/lib/toolColor.ts
git commit -m "refactor(frontend): point chat at /chat, drop dead graph tool color

/agent/chat no longer exists; tools_used can only ever contain
'semantic' or 'architecture' now."
```

---

## Task 7: Update CLAUDE.md and do the final end-to-end check

**Files:**
- Modify: `CLAUDE.md` (the "Two answering paths" subsection under Architecture)

**Interfaces:**
- None — documentation only.

- [ ] **Step 1: Rewrite the stale section**

In `CLAUDE.md`, replace the entire "### Two answering paths — know which one you're touching" subsection with:

```markdown
### Answering a question (`POST /chat`)
`answer.Service.Answer` (`api/internal/answer/service.go`) is the single path for every chat question: `retrieval.HybridRetriever.Search` always runs (semantic search that already auto-attaches call-graph edges for each hit via `ExpandSymbol` - there is no separate "graph" step), and `architecture.Service.BuildSummary` is appended only when `answer.WantsArchitectureOverview(query)` matches (a deterministic keyword check, not an LLM call). Both feed into the same `rag.Service.AnswerQuestion`. `AgentChatResponse.sources` and `tools_used` are built from these same merged results in the handler (`api/internal/api/agent_chat.go`) - not a second retrieval pass.

This replaced an earlier `agent` package (`Planner`/`HybridPlanner`/`Executor`/four `Tool` implementations, plus a sidecar `/classify` endpoint) that routed questions to a subset of tools via keyword matching or an LLM classify call. An audit found three of the four tools either always ran anyway, duplicated data `HybridRetriever.Search` already provides automatically, or duplicated the `history` parameter already passed on every call — see `docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md` for the full reasoning if you're wondering why this looks simpler than you might expect from a project describing itself as using an "agent."
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for the single-path answer.Service"
```

- [ ] **Step 3: Final full-stack verification**

```bash
docker compose up --build -d
sleep 5
docker compose ps
```
Expected: `api`, `extraction-service`, `postgres` all show `healthy`; `frontend` shows `Up`.

Repeat the two `curl` checks from Task 3 Step 7 against the freshly rebuilt stack, and open the frontend's Chat page for a previously-ingested repo in a browser to confirm the tool badges (semantic-only vs. semantic+architecture) render correctly with real data — completing the verification gap the original tool-badge feature (Brick 25) had left open (it was verified against synthetic/no-backend data at the time).
