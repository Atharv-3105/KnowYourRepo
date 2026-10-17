<div align="center">

# KnowYourRepo

**Know your repo before you touch it.**

Point it at a public GitHub repository. It clones and parses the code, builds a symbol and call-graph index, embeds it for semantic search, and answers questions with citations back to real files and line numbers.

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Gin](https://img.shields.io/badge/Gin-API-008ECF?logo=gin&logoColor=white)](https://gin-gonic.com/)
[![Python](https://img.shields.io/badge/Python-3.12-3776AB?logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-sidecar-009688?logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-6-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-8-646CFF?logo=vite&logoColor=white)](https://vite.dev/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind-v4-06B6D4?logo=tailwindcss&logoColor=white)](https://tailwindcss.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-pgvector-4169E1?logo=postgresql&logoColor=white)](https://github.com/pgvector/pgvector)
[![Tree-sitter](https://img.shields.io/badge/Tree--sitter-AST_parsing-4C9A2A)](https://tree-sitter.github.io/)
[![OpenRouter](https://img.shields.io/badge/OpenRouter-free_embeddings-6467F2)](https://openrouter.ai/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![Prometheus](https://img.shields.io/badge/Prometheus-metrics-E6522C?logo=prometheus&logoColor=white)](https://prometheus.io/)
[![Grafana](https://img.shields.io/badge/Grafana-dashboards-F46800?logo=grafana&logoColor=white)](https://grafana.com/)
[![Cost](https://img.shields.io/badge/runs_on-free_tiers_only-brightgreen)](#free-tier-design)

</div>

---

## Demo

<div align="center">

![KnowYourRepo demo: ingesting gorilla/mux, then exploring the overview, architecture, symbols, call graph and a cited chat answer](Docs/demo/knowyourrepo-demo.gif)

</div>

Recorded against the Docker Compose stack with real services (no mocks). The run shows:

1. **Ingest.** Paste `https://github.com/gorilla/mux`. The job moves through *cloning, walking, parsing, generating embeddings* and finishes in a few seconds.
2. **Overview.** A generated summary, repo stats (17 files, 261 symbols, 2,239 call edges) and a reading path with inline code previews.
3. **Architecture.** 31 detected components (structs and interfaces such as `Router` and `RouteMatch`). Opening one shows the exact source lines.
4. **Symbols.** The searchable symbol index across functions, structs, interfaces and types.
5. **Call graph.** The call graph rooted at `ServeHTTP`, in 2D and 3D.
6. **Chat.** "How does the router match an incoming request to a route?" returns a Markdown answer built from semantic and lexical retrieval, with cited sources. Clicking `Match` opens `route.go:377-379`.

---

## Table of contents

- [Demo](#demo)
- [What it does](#what-it-does)
- [Architecture](#architecture)
- [How it works](#how-it-works)
  - [Ingestion pipeline](#ingestion-pipeline)
  - [Answering a question](#answering-a-question)
  - [Resumable embedding](#resumable-embedding)
- [Tech stack](#tech-stack)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [API reference](#api-reference)
- [Free-tier design](#free-tier-design)
- [Security and robustness](#security-and-robustness)
- [Observability](#observability)
- [Testing](#testing)
- [Project layout](#project-layout)
- [Documentation](#documentation)
- [Known limitations](#known-limitations)

---

## What it does

| Feature | Description |
| --- | --- |
| **Repository ingestion** | Paste a GitHub URL. The repo is cloned (pure Go, no `git` binary), walked, parsed with Tree-sitter, and indexed in the background with live progress. |
| **Symbol index** | Every function, method and class is extracted and searchable, with file and line locations. |
| **Call graph** | Static caller/callee edges, shown as a bounded, symbol-anchored graph (2D and 3D). |
| **Hybrid retrieval** | Semantic vector search plus lexical identifier search, each hit expanded with its call-graph neighbours. |
| **Chat with citations** | Ask questions in plain language. Answers cite the exact files and line ranges they were built from, and clicking a citation opens the code. |
| **Generated overview** | A narrative summary, key concepts and a suggested reading path for the repo, with inline code previews. |
| **Architecture view** | Entry points, modules and dependencies detected from the code. |
| **Incremental re-indexing** | Only files whose content hash changed are re-parsed and re-embedded. |
| **Command palette** | Instant symbol lookup as you type, with semantic search for natural-language queries. |

Supported languages: Go, Python, JavaScript, TypeScript, C/C++ and Java.

---

## Architecture

Three services share one Postgres database. The split is by language ecosystem rather than by feature: Go does the systems and parsing work, Python does the ML and LLM work. Nothing in Go calls an LLM or touches an embedding directly; it always goes through the sidecar client.

```mermaid
flowchart TB
    subgraph Browser["Browser"]
        SPA["React SPA<br/>React 19, TanStack Query, Tailwind v4<br/>Cytoscape + three.js, Shiki"]
    end

    subgraph Go["api/ - Go orchestrator (Gin)"]
        direction TB
        MW["Middleware<br/>CORS, request ID, security headers,<br/>body limit, per-IP rate limits, metrics"]
        H["HTTP handlers<br/>repos, chat, search, graph,<br/>symbols, files, architecture"]
        WP["Worker pool<br/>bounded queue, graceful shutdown,<br/>resume on restart"]
        ING["Ingestion<br/>go-git clone, walker,<br/>Tree-sitter parser"]
        EXT["Graph extractor<br/>symbols and call edges"]
        CH["Chunker<br/>function-level, overlapping"]
        ANS["Answer service<br/>intent routing"]
        RET["Hybrid retriever<br/>semantic + lexical + call-graph expansion"]
        CTX["Context builder and prompt builder"]
        SC["Sidecar client<br/>X-Internal-Key, request ID propagation"]
        MW --> H
        H --> WP
        WP --> ING --> EXT --> CH
        H --> ANS --> RET --> CTX
        CH --> SC
        CTX --> SC
        RET --> SC
    end

    subgraph Py["extraction-service/ - Python sidecar (FastAPI)"]
        direction TB
        AUTH["Shared-secret auth<br/>X-Internal-Key"]
        EMB["OpenRouterEmbedProvider<br/>batching, 20 RPM limiter,<br/>daily budget, query LRU cache"]
        VS["PgVectorStore<br/>cosine HNSW, per-row model tag"]
        ROUTER["LLM router<br/>Groq, Gemini, Cerebras, OpenRouter<br/>cooldowns and rate-limit windows"]
        AUTH --> EMB
        AUTH --> ROUTER
        EMB --> VS
    end

    subgraph DB["PostgreSQL + pgvector"]
        direction LR
        T1[("files, symbols, edges,<br/>call_edges, repositories,<br/>ingestion_jobs, repo_overview")]
        T2[("embeddings<br/>vector(1024)")]
    end

    subgraph Ext["External services (free tiers)"]
        GH["GitHub<br/>clone + REST size check"]
        ORE["OpenRouter<br/>/embeddings"]
        LLMS["Groq / Gemini /<br/>Cerebras / OpenRouter chat"]
    end

    subgraph Obs["Observability"]
        PROM["Prometheus"] --> GRAF["Grafana"]
    end

    SPA -- "REST / JSON" --> MW
    ING -- "clone, fetch, reset" --> GH
    ING -. "repo size check" .-> GH
    SC -- "HTTP + shared secret" --> AUTH
    EMB -- "batched requests" --> ORE
    ROUTER -- "completions" --> LLMS
    Go -- "structural data" --> T1
    VS -- "vectors" --> T2
    PROM -. "scrape /metrics" .-> Go
    PROM -. "scrape /metrics" .-> Py
```

---

## How it works

### Ingestion pipeline

`POST /repos` validates and normalises the URL, then either returns an existing repo (`cached: true`), reports an ingestion already in progress, or queues a job. The response returns immediately with `202` while a worker runs the pipeline.

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant FE as React SPA
    participant API as Go API
    participant W as Worker pool
    participant GH as GitHub
    participant SC as Python sidecar
    participant OR as OpenRouter
    participant PG as Postgres + pgvector

    U->>FE: Paste GitHub URL
    FE->>API: POST /repos
    API->>API: Normalise URL, check for existing repo or active job
    API->>PG: Insert ingestion_jobs row
    API-->>FE: 202 Accepted (job id)
    API->>W: Enqueue job
    loop Poll
        FE->>API: GET /repos/jobs/:id
        API-->>FE: stage + parse_status + embed_status
    end

    W->>GH: Repo size check (REST)
    W->>GH: Clone (go-git)
    W->>W: Walk files (skip minified, oversize, vendored)
    W->>W: Hash each file (SHA-256) and diff against the index
    W->>W: Tree-sitter parse, extract symbols and call edges
    W->>PG: Store files, symbols, call_edges
    Note over W,PG: parse_status = completed.<br/>The repo is already browsable.
    W->>SC: GET /embed/info (current model)
    W->>SC: POST /embed/batch (up to 128 chunks per call)
    SC->>OR: POST /embeddings (one request per batch)
    OR-->>SC: vectors
    SC->>PG: Upsert vectors tagged with model
    W->>PG: Mark fully embedded files with the model name
    W->>SC: POST /overview (narrative, concepts, reading path)
    Note over W,PG: embed_status = completed, failed or skipped,<br/>tracked independently of parsing.
```

Parsing and embedding are tracked as separate outcomes. A repo whose embedding hit a quota is still fully usable (browsing, call graph, lexical search, chat), and the UI says so plainly.

### Answering a question

There is a single path for every chat question. There is no router that picks among "tools": semantic search always runs and already attaches call-graph edges to each hit.

```mermaid
flowchart LR
    Q["POST /chat<br/>question + history"] --> V["Validate<br/>length, repo exists,<br/>session id"]
    V --> F{"Asks about<br/>latest changes?"}
    F -- yes --> SY["Background sync check<br/>answer still uses current index"]
    F -- no --> S
    SY --> S["Semantic search<br/>embed query, cosine top-k"]
    S -->|"quota or provider error"| LX
    S --> GX["Call-graph expansion<br/>callers and callees per hit"]
    GX --> LX["Lexical search<br/>exact identifiers, ranked"]
    LX --> M["Merge and de-duplicate<br/>by symbol + file"]
    M --> A{"Wants architecture<br/>overview?"}
    A -- yes --> AO["Append architecture summary<br/>and generated overview"]
    A -- no --> P
    AO --> P["Prompt builder<br/>context + trimmed history"]
    P --> R["Sidecar /chat<br/>LLM router"]
    R --> OUT["Answer + sources<br/>file, symbol, line range"]
```

If semantic search fails for any reason, including an exhausted embedding quota, retrieval degrades to lexical and call-graph grounding instead of failing the question.

### Resumable embedding

Free embedding models are limited per request, not per token, so a large repo can outlast one day's quota. Embedding is therefore resumable:

- `files.embedded_model` records which model fully embedded each file (`NULL` means not yet).
- Each ingestion asks the sidecar which model is current and re-processes any file whose `embedded_model` differs, even if its content hash is unchanged.
- Files whose chunks all made it in are marked done. The rest resume on the next sync.
- Switching `OPENROUTER_EMBED_MODEL` is safe: the sidecar purges vectors from other models on startup (and drops the table if the dimension changed), and the API re-embeds everything.
- A sync is also triggered when files are pending embedding, unless the sidecar reports the quota as exhausted.

---

## Tech stack

| Layer | Technology |
| --- | --- |
| **API** | Go 1.26, Gin, pgx, go-git, smacker/go-tree-sitter, Prometheus client, `x/time/rate` |
| **Sidecar** | Python 3.12, FastAPI, psycopg 3 (async pool), pgvector, httpx, aiolimiter, Groq / Google GenAI SDKs |
| **Frontend** | React 19, TypeScript 6, Vite 8, React Router 7, TanStack Query 5, Tailwind CSS v4, Cytoscape.js + dagre, three.js, Shiki, Motion |
| **Data** | PostgreSQL 16 with the pgvector extension (HNSW, cosine) |
| **Embeddings** | OpenRouter free embedding models (default `nvidia/nemotron-3-embed-1b:free`, truncated to 1024 dimensions) |
| **LLMs** | Groq, Gemini, Cerebras and OpenRouter, tried in that order with per-provider cooldowns |
| **Ops** | Docker Compose, Prometheus, Grafana |

---

## Quick start

### Option 1: Docker (everything in one command)

You need Docker with Compose, and an [OpenRouter API key](https://openrouter.ai/keys) (free, no payment method) plus at least one chat-LLM key (for example [Groq](https://console.groq.com/keys), also free).

```bash
git clone https://github.com/Atharv-3105/KnowYourRepo.git
cd KnowYourRepo

cp .env.example .env                                    # compose-level settings
cp extraction-service/.env.example extraction-service/.env
# edit extraction-service/.env and set OPENROUTER_API_KEY and GROQ_API_KEY
# set the same random value for SIDECAR_API_KEY (root .env); compose passes it to both services

docker compose up --build
```

| Service | URL |
| --- | --- |
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| Sidecar | http://localhost:8000 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 |

### Option 2: Local development

Prerequisites: Go 1.26+, Python 3.12+, Node 22+, and Postgres with the `pgvector` extension (for example `docker run -e POSTGRES_PASSWORD=postgres -p 5432:5432 pgvector/pgvector:pg16`).

```bash
make setup                       # go mod tidy, python venv + pip install, npm install
cp api/.env.example api/.env
cp extraction-service/.env.example extraction-service/.env
# fill in OPENROUTER_API_KEY and a chat-LLM key in extraction-service/.env
```

Run the three processes (or `make dev` to run them together):

```bash
cd api && go run ./cmd/server                              # :8080
cd extraction-service && .venv/bin/uvicorn app.main:app --reload --port 8000
cd frontend && npm run dev                                 # :5173
```

On Windows, start the sidecar with `python run.py` instead of calling `uvicorn` directly. It sets the event-loop policy that psycopg 3's async mode needs, and it must run before uvicorn creates its loop.

---

## Configuration

Real environment variables always override `.env` files. Full, commented examples live in [`api/.env.example`](api/.env.example), [`extraction-service/.env.example`](extraction-service/.env.example) and [`.env.example`](.env.example).

**API (`api/`)**

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | local Postgres | Postgres connection string |
| `EXTRACTOR_URL` | `http://localhost:8000` | Sidecar base URL |
| `SIDECAR_API_KEY` | empty | Shared secret sent to the sidecar. Required on any public deployment |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated browser origins |
| `TRUSTED_PROXY_HOPS`, `CLIENT_IP_HEADER` | `0`, `X-Forwarded-For` | How the real client IP is resolved for rate limiting |
| `GITHUB_TOKEN` | empty | Optional. Raises the GitHub API limit from 60 to 5000 requests per hour |
| `MAX_REPO_SIZE_MB`, `MAX_REPO_FILES`, `MAX_FILE_BYTES` | 150, 2000, 512 KB | Ingestion size guardrails |
| `MAX_EMBED_CHUNKS`, `EMBED_BATCH_ITEMS` | 1024, 128 | Per-run embedding budget and chunks per sidecar call |
| `INGESTION_WORKERS` | 1 | Concurrent ingestion jobs |
| `MAX_QUESTION_CHARS`, `MAX_CHAT_SESSIONS`, `CHAT_SESSION_TTL_MINUTES` | 2000, 1000, 1440 | Chat bounds |

**Sidecar (`extraction-service/`)**

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENROUTER_API_KEY` | | Embeddings (and the OpenRouter chat fallback) |
| `OPENROUTER_EMBED_MODEL` | `nvidia/nemotron-3-embed-1b:free` | Embedding model |
| `EMBED_DIMENSION` | 1024 | Stored vector size (Matryoshka truncation) |
| `OPENROUTER_EMBED_RPM`, `OPENROUTER_EMBED_DAILY_LIMIT` | 20, 50 | Client-side limiter and daily request budget |
| `GROQ_API_KEY`, `GEMINI_API_KEY`, `CEREBRAS_API_KEY` | empty | Chat providers. A provider is only used if its key is set |
| `INTERNAL_API_KEY` | empty | Must equal the API's `SIDECAR_API_KEY` |

---

## API reference

All errors are a flat `{"error": "..."}` with a `X-Request-ID` header. Internal failures return a generic message plus a request ID; details stay in server logs.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Readiness check (pings the database) |
| `GET` | `/metrics` | Prometheus metrics |
| `POST` | `/repos` | Ingest a repository. `202` queued, `200` with `cached: true` if already indexed |
| `GET` | `/repos` | List ingested repositories |
| `GET` | `/repos/jobs/:id` | Ingestion status: stage, parse status, embed status |
| `POST` | `/repos/:id/sync` | Check the remote and re-ingest if it changed |
| `POST` | `/chat` | Ask a question. Returns the answer, cited sources and tools used |
| `POST` | `/search` | Semantic search for the command palette (falls back to lexical) |
| `GET` | `/graph/:repoID` | Bounded call graph around a symbol |
| `GET` | `/symbols/:repoID` | Searchable symbol index |
| `GET` | `/files/:repoID?path=&start_line=&end_line=` | Source from the on-disk clone. `503` + `Retry-After` while a missing clone is restored |
| `GET` | `/architecture/:repoID` | Detected entry points, modules and generated overview |

The sidecar's endpoints (`/embed/info`, `/embed/batch`, `DELETE /embed`, `/search`, `/chat`, `/overview`) are internal and require `X-Internal-Key`.

---

## Free-tier design

The project is built to run on free tiers with no payment method. The constraints that shaped it:

- **Embeddings are limited per request, not per token.** OpenRouter's free models allow 20 requests per minute and 50 per day across all free models on an account without credits. So the sidecar sends up to 128 chunks per request, caches query embeddings, throttles to the per-minute limit, and tracks a daily budget that fails fast instead of making doomed calls.
- **A quota-limited day is not a failure.** Ingestion keeps the work that succeeded, marks only fully embedded files, and resumes later. Chat and search degrade to lexical and call-graph retrieval.
- **Ephemeral disks.** Cloned working copies can disappear on restart. The index lives in Postgres; a missing clone is restored on demand and unchanged files are skipped by hash, so restoring costs no embedding quota.
- **Small instances.** Repo size, file count, file size and embed chunk count are all capped, minified and vendored files are skipped, chat history is bounded and expires, and request bodies are size-limited.
- **LLM budget.** The overview is regenerated only when a sync actually changed something, and the LLM router tracks per-provider rate-limit windows and cooldowns across Groq, Gemini, Cerebras and OpenRouter.
- **Frontend.** The entry bundle is about 450 KB; the graph and code highlighter load on first use.

---

## Security and robustness

- Repository URLs are validated against a strict GitHub owner/repo allow-list, so the clone path cannot be turned into a request to arbitrary hosts.
- File serving rejects absolute paths and checks that the resolved path stays inside the repo's own directory.
- The sidecar requires a shared secret on every route except `/health` and `/metrics`, has no CORS, and returns generic error bodies.
- Per-IP token-bucket rate limits on chat, search and ingestion. Client IPs are resolved by hop count from the right of `X-Forwarded-For`, because the leftmost entry is attacker-controlled.
- Security headers (`nosniff`, `X-Frame-Options: DENY`, `no-referrer`), request body limits, and server timeouts.
- Graceful shutdown: in-flight requests finish, running jobs get a deadline and are then recorded as pending, and unfinished jobs resume on the next start.
- Duplicate submissions of a repo that is already being ingested are collapsed instead of racing on the same clone directory.
- The API and sidecar containers run as a non-root user.

---

## Observability

- **Request correlation.** A request ID is generated at the edge, propagated to the sidecar, logged on both sides and echoed back in the `X-Request-ID` response header.
- **Metrics.** The API exports request counts and latency, agent-step outcomes and embedded/failed/capped chunk counts. The sidecar exports HTTP metrics, LLM router requests and tokens per provider, and embedding requests by outcome (success, cache hit, rate limited, quota exhausted).
- **Dashboards.** Prometheus and Grafana come up with `docker compose`, with provisioning under [`observability/`](observability/).
- **Logs.** Structured JSON when `LOG_FORMAT=json`.

---

## Testing

```bash
# Go (from api/). The store, retrieval and answer packages need Postgres.
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/knowyourrepo?sslmode=disable"
go test ./... -p 1

# Python sidecar (from extraction-service/). Uses a fake OpenRouter, so no quota is spent.
python -m pytest

# Frontend (from frontend/)
npm run lint && npm run build
```

Several Go packages share one test database and truncate tables between tests, so use `-p 1` to run packages sequentially.

---

## Project layout

```
KnowYourRepo/
├── api/                      Go orchestrator
│   ├── cmd/server/           entrypoint, graceful shutdown
│   └── internal/
│       ├── api/              handlers, middleware, rate limiting, validation
│       ├── ingestion/        clone, walk, parse, GitHub size check
│       ├── graph/            symbol and call-edge extraction
│       ├── chunk/            function-level chunking with overlap
│       ├── retrieval/        hybrid (semantic + lexical + call-graph) retriever
│       ├── answer/           single-path question answering, intent checks
│       ├── rag/ chat/        prompt building, bounded chat store
│       ├── architecture/     architecture summary and generated overview
│       ├── sidecar/          typed client for the Python service
│       ├── store/            Postgres access and embedded schema.sql
│       ├── worker/           bounded job pool
│       └── metrics/ config/
├── extraction-service/       Python sidecar
│   └── app/
│       ├── providers/        OpenRouter embeddings, LLM providers, router
│       ├── vectorstore/      pgvector store
│       ├── routes/ services/ embed, search, chat, overview
│       └── auth.py           shared-secret middleware
├── frontend/                 React SPA
│   └── src/ pages/ components/ api/ lib/
├── observability/            Prometheus config, Grafana provisioning
├── Docs/                     per-feature build notes
└── docker-compose.yml
```

---

## Documentation

Per-feature build notes (approach, decisions and why, failure cases, what was tested) live in [`Docs/Phase-4/`](Docs/Phase-4/) and [`Docs/phase5_frontend/`](Docs/phase5_frontend/). [`CLAUDE.md`](CLAUDE.md) is a compact map of the architecture and commands for contributors and coding assistants.

---

## Known limitations

- Free OpenRouter embeddings allow 50 requests per day without account credits (1000 per day after buying $10 of credits). A full ingestion uses several of them and each new semantic question uses one, so heavy use exhausts the quota. Retrieval then falls back to lexical search until it resets.
- Only public GitHub repositories are supported.
- Call edges are static and name-based, so dynamic dispatch and reflection are not resolved.
- There is no authentication or per-user data: repositories and indexes are shared by everyone using a deployment.
- Parsing covers Go, Python, JavaScript, TypeScript, C/C++ and Java.
