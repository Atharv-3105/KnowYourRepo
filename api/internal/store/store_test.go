package store

import (
	"context"
	"testing"
	"time"
)

func TestStore_Init(t *testing.T) {
	s := newTestStore(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var tableName string
	err := s.db.QueryRowContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'symbols'
	`).Scan(&tableName)

	if err != nil {
		t.Errorf("Failed to find 'symbols' table: %v", err)
	}
}