# Collapse the agent/tool-calling layer into a single retrieval path

**Date:** 2026-09-07
**Status:** Approved, pending implementation plan

## Why

The `agent` package (`Planner`/`HybridPlanner`/`Executor`/four `Tool` implementations) was built to let an LLM-assisted classifier route each chat question to a subset of retrieval "tools" (`semantic`, `graph`, `architecture`, `memory`). Auditing the actual code during this session showed the abstraction isn't earning its cost:

- **`semantic`** is the unconditional default (`Planner.Plan` falls back to it whenever nothing else matches) — there was never a real decision to make here.
- **`graph`** duplicates work `retrieval.HybridRetriever.Search` already does automatically: every semantic search hit already gets `ExpandSymbol` (incoming/outgoing call edges) attached unconditionally, for free. `GraphTool` re-runs the same `GetIncomingCalls`/`GetOutgoingCalls` queries, anchored on a symbol name guessed via regex from the raw question (`extractSymbol`) instead of a symbol semantic search already confirmed relevant — strictly worse grounding for identical underlying queries, and it silently returns zero results when the regex guesses wrong (logged as a normal completion, not a failure).
- **`memory`** just re-wraps the `history` string as a fake retrieval result, but `rag.Service.AnswerQuestion(ctx, query, history, results)` already receives `history` as its own parameter on every call regardless of which tools ran — pure duplication.
- **`architecture`** is the only tool with genuinely distinct data (aggregate repo stats/entrypoints, not per-symbol) that semantic search can't produce.

Net effect: an LLM classify round trip (sidecar `/classify`, 5s timeout, its own failure/fallback path) exists to route into four buckets where three either always run anyway, duplicate an existing parameter, or duplicate a query the retriever already runs for free. This is a retrieval router dressed in agent/tool vocabulary, not agentic tool calling (no loop, no tool sees another tool's output, no re-planning) — and the vocabulary overstates what it does. `Executor.Execute` also swallows per-tool failures by design (log + `continue`), so a failing `SemanticTool` — the one tool doing indispensable work — degrades a request silently rather than surfacing an error.

## Scope

Backend-only architecture change to the answering path. Does not touch: ingestion pipeline, `chat.Store`/session memory, `contextbuilder`/`rag.Service` internals, freshness-sync mechanics (`RepoSyncer`/`SyncIfStale`), or any frontend page other than the one API call `Chat.tsx` makes.

## Design

### Deleted
- `api/internal/agent/` in full: `planner.go`, `hybrid_planner.go`, `executor.go`, `models.go`, `service.go`, `tools/{semantic,graph,architecture,memory}.go`.
- Sidecar `/classify` route, `app/services/classify.py`, `app/models/classify.py`; the `"classify"` task type entry in `app/providers/router.py`'s `TASK_PROVIDER_ORDER`.
- Go `sidecar.Client`'s `Classify` method, `ClassifyRequest` type, `ErrClassificationUnavailable`.
- The plain `POST /chat` route and its old handler, which called `rag.Service` directly with semantic-only results (confirmed dead: `frontend/src/api/chat.ts`'s own comment states nothing in the frontend calls it — only `/agent/chat` is wrapped). Note this frees up the `/chat` path name, which the API surface section below reassigns to the new, simplified logic — `/chat` as a URL survives, but the handler behind it is entirely new.

### New package: `api/internal/answer`
Replaces `agent`. Contains:
- `reingest_intent.go` — `WantsReingestion(query string) bool`, moved verbatim from `agent/reingest_intent.go` (unrelated to tool-calling, just needed a home).
- `overview_intent.go` — `WantsArchitectureOverview(query string) bool`, the existing `architectureKeywords` list + `containsAny` check from `planner.go`, kept as-is (already works correctly as a keyword heuristic), no longer wrapped in a `Planner` type.
- `service.go` — the new orchestration:

```go
type Service struct {
    retriever           *retrieval.HybridRetriever
    architectureService *architecture.Service
    ragService          *rag.Service
    syncer              RepoSyncer
    logger              *slog.Logger
}

func (s *Service) Answer(ctx context.Context, repoID, query, history string) (answer string, refreshing bool, results []retrieval.RetrievalResult, err error) {
    if WantsReingestion(query) {
        refreshing = true
        s.triggerBackgroundSync(repoID) // unchanged from agent.Service today
    }

    results, err = s.retriever.Search(ctx, repoID, query)
    if err != nil {
        return "", refreshing, nil, err
    }

    if WantsArchitectureOverview(query) {
        if overview, ovErr := s.architectureService.BuildSummary(ctx, repoID); ovErr == nil {
            results = append(results, overviewAsResult(overview))
        } else {
            s.logger.Warn("architecture_overview_failed", "repo_id", repoID, "error", ovErr)
            // not fatal - a request with real semantic results shouldn't fail over a missed overview
        }
    }

    answer, err = s.ragService.AnswerQuestion(ctx, query, history, results)
    return answer, refreshing, results, err
}
```
`overviewAsResult` builds the same text block `ArchitectureTool.Execute` builds today (statistics/languages/entrypoints/components), unchanged.

`RepoSyncer` interface and `triggerBackgroundSync` move over unchanged from `agent/service.go`.

### Citations (`buildSources`)
The architecture-overview pseudo-result (`Symbol: "architecture_overview"`, `FilePath: repoID` — not a real file) is excluded from `buildSources()`'s output specifically, so it never renders as a broken clickable citation. It still flows into `results` and therefore into the LLM prompt via `rag.Service.AnswerQuestion` — only the citations list filters it out.

### API surface
- Single endpoint: `POST /chat` (the existing `POST /agent/chat` route is removed; `/chat` now runs `answer.Service.Answer`). `ChatRequest`/`AgentChatResponse` Go types and their JSON field names — including `tools_used` and `sources` — are unchanged, to avoid cosmetic churn against a frontend that already depends on these exact names.
- `tools_used` in the response becomes `["semantic"]` or `["semantic", "architecture"]` (no more `graph`/`memory` values ever appear).

### Frontend
One required change: `frontend/src/api/chat.ts`'s `agentChat` function posts to `/chat` instead of `/agent/chat`. `Chat.tsx` needs no changes (same response shape). `frontend/src/lib/toolColor.ts`'s `graph`/`architecture` tool-graph/tool-architecture... — specifically its `"graph"` entry becomes dead (no response will ever contain it) and should be deleted; `"semantic"` and `"architecture"` entries stay as-is.

### Docs
`CLAUDE.md`'s "Two answering paths" section (written earlier this session, before this redesign) must be rewritten to describe the single path — this is part of this work, not a follow-up.

## Testing

`api/internal/agent` has no existing test files, so nothing is lost by deleting it. New `api/internal/answer` package gets:
- Unit tests for `WantsReingestion` and `WantsArchitectureOverview` (pure functions — trivial table-driven tests, following the existing style in `internal/retrieval/hybrid_test.go`).
- A test for `Service.Answer`'s branching (search-only vs. search+overview vs. reingestion-triggered), using fakes/mocks for `retriever`/`architectureService`/`ragService` the way `internal/store`'s existing tests mock dependencies.

Manual verification: rebuild `api`, run a "what does X do" question (semantic-only) and an "architecture overview" question (semantic+architecture) against a real ingested repo, confirm both produce sensible answers and correct `tools_used`/`sources` in the response; confirm the frontend's Chat page still renders tool badges and citations correctly after the `/chat` endpoint-path change.

## Explicit non-goals

- Not renaming `ChatRequest`/`AgentChatResponse` Go/TS types — cosmetic, doesn't address the actual complexity/cost problem, and would touch more files (frontend types, Chat.tsx) for no functional benefit.
- Not building real multi-step tool calling (an LLM that inspects a tool's output and decides to call another tool). This design deliberately replaces the *appearance* of agentic tool-calling with an honest, simpler retrieval path — it does not attempt to build the real thing, since no concrete use case for it was ever established.
- Not touching the freshness-sync mechanism, ingestion pipeline, or any other page of the frontend.
