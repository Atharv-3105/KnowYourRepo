package store

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)


func TestStore_Init(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	//Create a short context for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := NewStore(ctx, dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	defer s.Close()

	var tableName string 
	err = s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type ='table' AND name = 'symbols'").Scan(&tableName)
	if err != nil {
		t.Errorf("Failed to find 'symbols' table: %v", err)
	}
}