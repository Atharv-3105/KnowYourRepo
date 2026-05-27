package store 

import (
	"context"
	"log/slog"
	_ "embed"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string 


type Store struct {
	db *sql.DB
	logger *slog.Logger
}


//function to initialize SQLite DB and run the Schema
func NewStore(ctx context.Context, dbPath string, logger *slog.Logger) (*Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("db path cannot be empty")
	}

	//WAL mode for better concurrency and setting foreign keys for data integrity
	var dsn string
	if dbPath == ":memory:" {
		dsn = "file::memory:?cache=shared&_foreign_keys=on&_journal_mode=WAL"
	} else {
		dsn = fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL", dbPath)
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	//Ping with context inorder to ensure we don't get hanged during the startup
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect db: %w", err)
	}

	store := &Store{
		db: db,
		logger: logger,
	}

	if err := store.initSchema(ctx); err != nil {
		return nil, err 
	}

	store.logger.Info("database initialized successfully", "path", dbPath)
	return store, nil
}

func(s *Store) initSchema(ctx context.Context) error {

	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w",err)
	}
	return nil
}

func(s *Store) Close() error{
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}