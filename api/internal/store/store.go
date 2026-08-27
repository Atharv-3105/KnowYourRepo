package store 

import (
	"context"
	"log/slog"
	_ "embed"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schemaSQL string 


type Store struct {
	db *sql.DB
	logger *slog.Logger
}


//function to initialize Postgrs Connection and run the Schema
func NewStore(ctx context.Context, dsn string, logger *slog.Logger) (*Store, error){

	if dsn == "" {
		return nil, fmt.Errorf("database dsn cannot be empty")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	//Ping with context inorder to ensure we don't get hanged during the startup
	if err := db.PingContext(ctx); err != nil{
		return nil, fmt.Errorf("failed to connect db: %w", err)
	}

	store := &Store{
		db: db, 
		logger: logger,
	}

	if err := store.initSchema(ctx); err != nil{
		return nil, err 
	}

	store.logger.Info("database initialized successfully")
	return store, nil 
}

func(s *Store) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil{
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil 
}

func(s *Store) Close() error {
	return s.db.Close()
}

//Dependency Injection of DB
func(s *Store) DB() *sql.DB{
	return s.db
}