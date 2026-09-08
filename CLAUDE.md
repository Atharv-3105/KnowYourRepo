# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

KnowYourRepo is an AI-powered repository intelligence platform: point it at a public GitHub repo, it clones and parses the code (Tree-sitter), builds a symbol/call-graph index, embeds it for semantic search, and answers questions about it via an LLM with citations back to real file/line locations.

Three services, one shared Postgres+pgvector database:
- **`api/`** — Go orchestrator (Gin). Owns ingestion (cloning, parsing, symbol/call-graph extraction), the structural schema, and the retrieval/agent/RAG logic that builds prompts.
- **`extraction-service/`** — Python/FastAPI sidecar. Owns embeddings (Voyage AI `voyage-code-3`), the pgvector-backed vector store, and the multi-provider LLM calls (Groq/Gemini/Cerebras/OpenRouter) for answering.
- **`frontend/`** — React + Vite + TS SPA. Talks only to the Go API.

The two backend services are split by language ecosystem, not by feature: Go does systems/parsing work, Python does ML/LLM work. Nothing in Go calls an LLM or touches an embedding directly — it always goes through `internal/sidecar.Client` to the Python service.

## Commands

### Local dev (three processes, or `make dev` to run all three concurrently)
```bash
make dev              # runs dev-api + dev-extractionservice + dev-frontend via `make -j3`
make dev-api          # cd api && go run ./cmd/server
make dev-extractionservice   # cd extraction-service && .venv/bin/uvicorn app.main:app --reload --port 8000
make dev-frontend     # cd frontend && npm run dev
```
`dev-extractionservice` invokes `uvicorn` directly rather than `python run.py` — for local Windows dev, use `run.py` instead (`extraction-service/.venv/Scripts/python run.py`), since it sets the `SelectorEventLoop` policy psycopg3's async mode needs on Windows *before* uvicorn creates its event loop. This only matters on Windows outside Docker; the container's `Dockerfile` runs `uvicorn` directly on purpose (Linux doesn't need the workaround).

### One-command full stack (Docker)
```bash
docker compose up --build
```
Starts Postgres+pgvector, both backend services, the frontend (built + served via nginx), and Prometheus/Grafana. Frontend on `:5173`, API on `:8080`, sidecar on `:8000`, Postgres only reachable inside the compose network (not host-mapped, to avoid colliding with a local Postgres install).

### Setup
```bash
make setup            # setup-api + setup-extractionservice + setup-frontend
make setup-api                # go mod tidy
make setup-extractionservice  # creates .venv, pip installs requirements.txt
make setup-frontend           # npm install
```

### Build / lint / test
```bash
make build             # go build -o bin/server ./cmd/server (from api/)
make lint               # golangci-lint (api) + ruff check (extraction-service)

# Go tests (from api/)
go test ./...
go test ./internal/graph/...                       # one package
go test ./internal/graph/ -run TestExtractCallGraph # one test

# Frontend
cd frontend && npm run build   # tsc -b && vite build
cd frontend && npm run lint    # oxlint
```
`extraction-service` has `pytest` as a listed dependency but no test files exist yet — there is no Python test suite to run.

Go test coverage exists for `internal/graph`, `internal/ingestion`, `internal/retrieval`, `internal/sidecar`, `internal/store` — check `*_test.go` in those packages for existing patterns before adding new tests elsewhere.

## Architecture

### Ingestion pipeline (`POST /repos` → background job)
`RepoHandler.CreateRepo` dedupes by `repo_url` (re-posting an existing URL refreshes that repo instead of cloning a duplicate), inserts an `ingestion_jobs` row, and hands the job ID to `internal/worker.Pool` (a small buffered-channel worker pool — see `worker/pool.go`) so the HTTP response returns immediately (`202`) while ingestion runs in the background. The pipeline itself:
```
Cloner (go-git, no system git binary) → Walker (file tree) → TreeSitterParser
  → graph.Extractor (symbols + call edges) → chunk/representation
  → sidecar POST /embed/batch → PgVectorStore
```
First-time ingestion uses `Cloner.CloneRepo`; re-ingestion (`POST /repos/:id/sync` or the freshness path below) uses `Cloner.SyncRepo` (fetch + hard reset, not a merge pull — the clone directory is never locally modified). Incremental re-indexing diffs `files.hash` (SHA-256) so unchanged files are skipped entirely; changed files get their stale symbols/call_edges/embeddings purged (`DeleteSymbolsAndCallEdges`, sidecar `DELETE /embed`) before reprocessing.

Cloned repos live on disk at `data/repos/<repoID>/`, referenced via `filepath.Join("..", "data", "repos", repoID)` relative to the API's working directory — the code assumes it's run from `api/` (true for `go run ./cmd/server` and for the Docker image, which sets `WORKDIR /app/api` with `/app/data` as a sibling volume mount specifically to preserve this).

**Freshness-triggered sync**: there's no periodic polling scheduler (deliberately rejected as wasteful — see `answer.WantsReingestion`). Instead, `answer.WantsReingestion(query)` detects phrases like "latest changes" in a chat question and fires `RepoSyncer.SyncIfStale` in the background; the answer still comes from the current index, with `refreshing: true` in the response signaling it might be stale. `answer.RepoSyncer` is an interface implemented by `RepoHandler` specifically to avoid an `answer` → `api` import cycle.

### Answering a question (`POST /chat`)
`answer.Service.Answer` (`api/internal/answer/service.go`) is the single path for every chat question: `retrieval.HybridRetriever.Search` always runs (semantic search that already auto-attaches call-graph edges for each hit via `ExpandSymbol` - there is no separate "graph" step), and `architecture.Service.BuildSummary` is appended only when `answer.WantsArchitectureOverview(query)` matches (a deterministic keyword check, not an LLM call). Both feed into the same `rag.Service.AnswerQuestion`. `AgentChatResponse.sources` and `tools_used` are built from these same merged results in the handler (`api/internal/api/agent_chat.go`) - not a second retrieval pass.

This replaced an earlier `agent` package (`Planner`/`HybridPlanner`/`Executor`/four `Tool` implementations, plus a sidecar `/classify` endpoint) that routed questions to a subset of tools via keyword matching or an LLM classify call. An audit found three of the four tools either always ran anyway, duplicated data `HybridRetriever.Search` already provides automatically, or duplicated the `history` parameter already passed on every call — see `docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md` for the full reasoning if you're wondering why this looks simpler than you might expect from a project describing itself as using an "agent."

### Sidecar (`extraction-service`)
Routes (`app/routes/`): `/health`, `/embed/batch` + `DELETE /embed`, `/search`, `/chat`. `/chat` goes through `app/providers/router.py`'s `LLMRouter`, which holds per-provider state (`RouterProvider`: rate-limit window, consecutive-error count, cooldown) and walks `TASK_PROVIDER_ORDER` (currently `groq → gemini → cerebras → openrouter` for every task) until one succeeds — new providers just implement the `LLMProvider` ABC (`app/providers/base.py`) and get added to that order list, no router changes needed.

`PgVectorStore` (`app/vectorstore/pgvector_store.py`) runs `CREATE EXTENSION IF NOT EXISTS vector` itself on first use — Postgres must be the `pgvector/pgvector` image (or otherwise have the extension files installed), not plain `postgres`, since `CREATE EXTENSION` only enables an already-installed extension, it doesn't install one.

### Shared Postgres, two schema owners
Go's `internal/store` embeds and executes `schema.sql` on every startup (`//go:embed schema.sql`, `initSchema` runs it unconditionally — all `CREATE TABLE IF NOT EXISTS`/`ALTER ... ADD COLUMN IF NOT EXISTS`, safe to rerun) and owns `files`, `symbols`, `edges`, `call_edges`, `repositories`, `ingestion_jobs`. Python's `PgVectorStore` owns the vector table and creates it independently. The two connect with different DSN schemes to the same database — Go uses `postgres://...?sslmode=disable` (`internal/config`), Python uses `postgresql://...` (`app/config.py`, pydantic-settings reading `.env`) — functionally equivalent, just don't assume they're interchangeable strings if you're templating one from the other.

### Frontend
React Router nested routes under `RepoLayout` (`/repos/:repoId/{overview,chat,architecture,symbols,graph}`), TanStack Query for all server state, Tailwind v4 (`@theme` tokens in `src/index.css` — currently the "Field Notes" design system, see `Docs/phase5_frontend/` for the full migration history from the prior "Blueprint" identity). Cytoscape.js + dagre for the call-graph view — it renders to its own canvas, so it cannot read CSS custom properties; its colors/fonts are hardcoded JS literals in `GraphView.tsx` and must be updated by hand whenever design tokens change. Shiki (`@shikijs/core` curated core, not the `shiki` package's convenience API) for syntax highlighting in the code-view panel. `VITE_API_URL` is inlined at Vite **build** time, not read at runtime — the Docker frontend build takes it as a build arg.

### Config
Go: `internal/config.Load()` reads env vars with defaults (`SERVER_HOST/PORT`, `DATABASE_URL`, `EXTRACTOR_URL`, `CORS_ALLOWED_ORIGINS` — comma-separated). Python: `app/config.py`'s pydantic `Settings` reads `extraction-service/.env` (provider API keys, `DATABASE_URL`, provider/model selection per task).

## Documentation

Detailed per-feature build docs (approach, decisions made and why, files touched, failure cases, what was actually tested) live under `Docs/Phase-4/` and `Docs/phase5_frontend/`, one file per "brick." Check the relevant `README.md` index there before assuming a feature's current state — several features (e.g. observability tracing/cost-tracking, a trained intent classifier) are documented as explicitly started-but-incomplete.
