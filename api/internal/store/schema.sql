CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    repo_id TEXT NOT NULL,
    path TEXT NOT NULL,
    language TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(repo_id, path)
);

CREATE TABLE IF NOT EXISTS symbols (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL, --function, class, method
    start_line INTEGER,
    end_line  INTEGER,
    FOREIGN KEY(file_id) REFERENCES files(id)
);

CREATE TABLE IF NOT EXISTS edges (
    id BIGSERIAL PRIMARY KEY,
    from_symbol_id BIGINT NOT NULL,
    to_symbol_id  BIGINT NOT NULL,
    type TEXT NOT NULL,
    FOREIGN KEY(from_symbol_id) REFERENCES symbols(id),
    FOREIGN KEY(to_symbol_id) REFERENCES symbols(id)
);

CREATE TABLE IF NOT EXISTS call_edges (
    id BIGSERIAL PRIMARY KEY,
    repo_id TEXT NOT NULL,
    caller_symbol  TEXT NOT NULL,
    caller_file_path TEXT NOT NULL,
    callee_symbol  TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repositories(
    id      TEXT    PRIMARY KEY,
    repo_url  TEXT  NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingestion_jobs(
    id      TEXT     PRIMARY KEY,
    repo_url  TEXT NOT NULL,
    status    TEXT NOT NULL DEFAULT 'pending',    --can be pending, processing, completed, failed
    error_message  TEXT,
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_symbols_file_id ON symbols(file_id);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_symbol_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_symbol_id);

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS last_commit_sha TEXT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ;
ALTER TABLE files ADD COLUMN IF NOT EXISTS hash TEXT;

ALTER TABLE symbols DROP CONSTRAINT IF EXISTS symbols_file_id_fkey;
ALTER TABLE symbols ADD CONSTRAINT symbols_file_id_fkey FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE;

-- call_edges had no indexes at all; the bounded call-graph traversal
-- filters by repo_id and joins repeatedly on caller_symbol/callee_symbol per recursion
-- step, which would otherwise be a full table scan on every hop.
CREATE INDEX IF NOT EXISTS idx_call_edges_repo_caller ON call_edges(repo_id, caller_symbol);
CREATE INDEX IF NOT EXISTS idx_call_edges_repo_callee ON call_edges(repo_id, callee_symbol);

-- Real ingestion sub-stage (cloning/walking/parsing/embedding/done), set by
-- ingestRepository at each actual phase transition - NULL while a job is
-- still queued, not yet picked up by a worker.
ALTER TABLE ingestion_jobs ADD COLUMN IF NOT EXISTS stage TEXT;

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

-- Independent phase-level outcomes, separate from the job's overall status.
-- A repo whose parsing/call-graph extraction completed is genuinely usable
-- (browsable, has a call graph, Chat still works via lexical/graph-fallback
-- retrieval) even if embedding subsequently failed (e.g. an embedding
-- provider rate limit) - the old single status column couldn't represent
-- that and reported a flat "failed" that hid real, usable data. NULL means
-- that phase never ran (e.g. the job never got past cloning/walking).
-- 'skipped' (embed_status only) covers an incremental sync where no files
-- changed, so there was nothing to embed.
ALTER TABLE ingestion_jobs ADD COLUMN IF NOT EXISTS parse_status TEXT;
ALTER TABLE ingestion_jobs ADD COLUMN IF NOT EXISTS embed_status TEXT;