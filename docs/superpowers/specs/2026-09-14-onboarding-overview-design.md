# Turn Overview into an onboarding surface: narrative summary, reading path, concepts

**Date:** 2026-09-14
**Status:** Approved, pending implementation plan

## Why

KnowYourRepo's stated goal is to be a single resource that helps someone understand an unfamiliar codebase faster. Today the Overview page (and Architecture, which feeds it) is purely statistical and structural: file/symbol counts, a directory breakdown, a flat list of detected entrypoints. None of it answers "what does this project actually do," "where should I start reading," or "what non-obvious patterns/concepts does this codebase use." A new user gets data about the repo, not understanding of it - the gap this spec closes.

This is the first of three independent workstreams identified when the broader "improve usability" request was decomposed (the other two - answer-quality/prompt engineering, and general UI/UX polish - are deliberately out of scope here, to be brainstormed separately). It was picked first because it determines what the other two are even polishing or prompting for.

A related finding from this session: README.md is currently invisible to the entire system. The Walker only picks up recognized source-code extensions for Tree-sitter parsing, so README content is never read, chunked, embedded, or surfaced anywhere - including Chat's RAG. Since a README (when present) is usually the best existing "what is this and why" text available, reading it is in scope for this spec.

## Scope

Backend: ingestion pipeline gains a README-capture step and a new narrative/concepts generation step; a new schema table; a new sidecar route; a new deterministic reading-path query. Frontend: `Overview.tsx` gains three new sections. Does not touch: Chat's RAG pipeline (README is *not* embedded for search in this spec - see Explicit non-goals), the Architecture page's existing entrypoint/component detection logic, the Graph or Symbols pages, or any prompt-engineering work on the existing `/chat` path.

## Design

### Data flow

```
Ingestion job (after existing call-graph extraction):
  Walker captures root-level README (if present) -> raw text, in-memory only
    -> overview.Generator builds a prompt from:
         README text (if present) + entrypoints + top-level directory
         structure + capped sample of representative symbol signatures
    -> sidecar.Client.GenerateOverview() -> POST /generate-overview (new)
    -> Python: LLMRouter (existing, same Groq->Gemini->Cerebras->OpenRouter
       chain /chat already uses) -> {narrative_summary, concepts[]}
    -> store.SaveOverview(repoID, narrative_summary, concepts)

Request time (GET /architecture/:repoID, existing endpoint, extended):
  - narrative_summary, concepts: read from repo_overview table (fast, no LLM)
  - reading_path: computed fresh from entrypoints + call_edges (no LLM, no
    storage - always reflects the current index)
```

Generation is ingestion-time and persisted for narrative/concepts (the parts that need LLM judgment and are expensive); the reading path is computed at request time (the part that's structurally derivable from data the product already indexes, and should never go stale between ingestions).

### README capture (`internal/ingestion`)

`Walker` gains a check during its existing walk: a root-level file matching `README.md`/`README.rst`/`README.txt`/`README` (case-insensitive, root directory only - not scanned recursively into subdirectories) is read as raw text and returned alongside the walk's existing file list, not inserted as a `files` row (it produces no symbols, doesn't go through `TreeSitterParser`). Capped at a fixed character limit (e.g. 8,000 chars) before it ever reaches a prompt - a README longer than that gets truncated, not rejected.

### Schema: `repo_overview`

New table, Go-owned (`store` package, `schema.sql`, same `CREATE TABLE IF NOT EXISTS` pattern as every other table there):

```sql
CREATE TABLE IF NOT EXISTS repo_overview (
    repo_id          TEXT PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
    narrative_summary TEXT,
    concepts         JSONB,
    generated_at     TIMESTAMPTZ
);
```

`concepts` is a JSON array of `{term, explanation}` objects. New `internal/store/overview_queries.go` (mirrors the existing `architecture_queries.go` file split): `SaveOverview(ctx, repoID, summary, concepts)` (upsert - re-ingestion/sync overwrites), `GetOverview(ctx, repoID) (*Overview, error)` (returns nil, not an error, when no row exists yet - e.g. generation failed or hasn't run).

### Generation service (`internal/architecture` + sidecar)

New method on the existing `architecture.Service` (not a new package - this is architecture-level understanding, same as entrypoint/component detection, just LLM-generated rather than deterministic): `GenerateOverview(ctx, repoID, readmeText, entrypoints, directoryStructure, symbols) error`. Builds the request, calls `sidecar.Client.GenerateOverview(...)`, on success calls `store.SaveOverview`. Called from the ingestion job after call-graph extraction completes (needs entrypoints to exist first).

New Python route `POST /generate-overview` (`app/routes/overview.py` or similar, following the existing route-file-per-concern pattern): request `{readme_text: str | None, entrypoints: [...], directory_structure: [...], symbols: [...]}`, response `{narrative_summary: str, concepts: [{term, explanation}]}`. Prompt template explicitly handles the no-README case (still generates a summary, just from code signals alone, with correspondingly lower expected quality - not a separate code path, just a conditional section in the same prompt). Uses the existing `LLMRouter.complete()` - no new provider, no new rate-limit surface, reuses everything wired up for `/chat`.

### Reading path (`internal/graph`)

New function, e.g. `BuildReadingPath(entrypoints []Entrypoint, edges []CallEdge) []ReadingStep`, where `ReadingStep` is `{Symbol, FilePath, Reason string}`. Algorithm: BFS outward from every detected entrypoint over `call_edges`, tracking each discovered symbol's distinct-caller count within the traversal as a centrality proxy; sort by (BFS depth ascending, caller count descending); cap at ~10-12 steps; dedupe. `Reason` is templated, not LLM-generated: `"Entrypoint"` for depth 0, `"Called by N functions in the core flow"` for the rest. Zero-entrypoint repos (a real case already on file - `rs/xid` has none) fall back to ranking all symbols repo-wide by caller count with no BFS anchor, same templated reasons minus "Entrypoint". This reuses the same `call_edges` table and query shape the existing bounded-graph endpoint (`GET /graph/:repoID`) already queries - no new indexes needed.

### API surface

`GET /architecture/:repoID` response gains three fields, additive and backward-compatible with the existing frontend consumer:

```json
{
  ...existing fields (entrypoints, components, statistics, languages)...,
  "narrative_summary": "string, empty if generation hasn't run or failed",
  "concepts": [{"term": "...", "explanation": "..."}],
  "reading_path": [{"symbol": "...", "file_path": "...", "reason": "..."}]
}
```

### Frontend (`Overview.tsx`)

Three additions, all consuming the now-extended `getArchitecture` response (no new API call):
- **Narrative summary**: prose paragraph immediately after the repo name/link header - first substantive content on the page, answering "what is this" before any stats. Hidden entirely (not an empty-state box) when `narrative_summary` is empty.
- **Reading path**: replaces the current flat "Entrypoints" section with an ordered list - each item shows symbol, file path, and its `reason`, reusing the existing cross-link pattern already used elsewhere (jump to Graph via `?symbol=`, prefill Chat via `?prefill=` for "ask about this"). Falls back to today's plain entrypoints rendering if `reading_path` is empty (e.g. generation hasn't run yet on an older ingested repo).
- **Concepts**: a new section, same list treatment as entrypoints today, each item with an "ask about this" Chat-prefill link (e.g. `What does "<term>" mean in this codebase?`).

### Error handling

- Ingestion job never fails because overview generation failed - it's called, errors are logged (`slog.Warn`, matching the existing pattern for non-fatal architecture-overview failures in `answer.Service`), and the job proceeds/completes normally with `repo_overview` left empty for that repo.
- All-providers-exhausted (the same failure mode the rate-limiting/cooldown work this session addressed) degrades the same way - no special-casing needed here since it surfaces as a normal `sidecar.Client` error.
- Frontend never shows a broken/loading-forever state for missing overview data - empty fields render as "not generated yet" (narrative/concepts sections hidden) or fall back to existing behavior (reading path -> today's entrypoints list).

## Testing

- Go: unit tests for `BuildReadingPath` covering the BFS/ranking/cap logic and the zero-entrypoint fallback, fixture-based, following existing `internal/graph` test patterns.
- Go: unit tests for `overview_queries.go`'s `SaveOverview`/`GetOverview` against `TEST_DATABASE_URL`, following existing `internal/store` integration-test patterns (including the known cross-package truncation flakiness already documented in CLAUDE.md - nothing new here).
- Python: no test file (matches every other route in this service - CLAUDE.md already documents there's no Python test suite yet).
- Live verification: re-sync the already-ingested JobBot repo (has a README) to populate `repo_overview`, confirm narrative/reading-path/concepts render correctly on Overview, confirm cross-links (Graph jump, Chat prefill) work, and confirm the page still renders correctly for a repo with no `repo_overview` row yet (pre-existing ingested repos before this change ships).

## Explicit non-goals

- **README is not embedded into Chat's RAG pipeline in this spec.** It's read and used only for overview generation. Making it searchable via Chat is a real, easy follow-on improvement but is a change to the retrieval/answer-quality workstream, not this one - keeping it out avoids scope creep across the three decomposed workstreams.
- **No complexity/risk signals** (the fourth onboarding option considered and explicitly not chosen during brainstorming).
- **No sub-directory README aggregation** - root-level only for v1.
- **No UI/UX visual redesign of Overview beyond adding these three sections** - that's the separate, not-yet-brainstormed UI/UX polish workstream.
- **No prompt-engineering work on `/chat` itself** - that's the separate, not-yet-brainstormed answer-quality workstream, even though this spec adds a new LLM-backed generation path of its own.
