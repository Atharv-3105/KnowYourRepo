package store

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// testDSN returns the Postgres connection string used by the store
// package's tests. Override with TEST_DATABASE_URL to point at a
// dedicated test database instead of your local dev one.
func testDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:password@localhost:5432/knowyourrepo?sslmode=disable"
}

// newTestStore opens a fresh Store against the test database and
// registers cleanup that truncates every table (so each test starts from
// an empty state) and closes the connection, regardless of pass/fail.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := NewStore(ctx, testDSN(), logger)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	t.Cleanup(func() {
		truncateAll(t, s)
		s.Close()
	})

	return s
}

func truncateAll(t *testing.T, s *Store) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
		TRUNCATE TABLE call_edges, edges, symbols, files, repositories RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("failed to truncate tables during test cleanup: %v", err)
	}
}