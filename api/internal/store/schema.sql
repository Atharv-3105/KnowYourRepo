CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL, --function, class, method
    start_line INTEGER,
    end_line  INTEGER,
    FOREIGN KEY(file_id) REFERENCES files(id)
);

CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_symbol_id INTEGER NOT NULL,
    to_symbol_id  INTEGER NOT NULL,
    type TEXT NOT NULL,
    FOREIGN KEY(from_symbol_id) REFERENCES symbols(id),
    FOREIGN KEY(to_symbol_id) REFERENCES symbols(id)
);

CREATE TABLE IF NOT EXISTS call_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    caller_symbol  TEXT NOT NULL,
    callee_symbol  TEXT NOT NULL,

    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_symbols_file_id ON symbols(file_id);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_symbol_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_symbol_id);