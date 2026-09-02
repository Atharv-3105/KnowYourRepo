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