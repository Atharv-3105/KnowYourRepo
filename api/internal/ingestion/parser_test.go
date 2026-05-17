package ingestion 

import (
	"log/slog"
	"context"
	"path/filepath"
	"testing"
	"os"
)


func TestParser_ParseFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level:slog.LevelDebug}))
	p := NewParser(logger)
	ctx := context.Background()

	t.Run("parse valid Go file", func(t *testing.T){
		content := []byte("package main\nfunc main() {}")
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.go")
		_ = os.WriteFile(tmpFile, content, 0644)

		res, err := p.ParseFile(ctx, tmpFile, "go")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if res.Root.Type() != "source_file" {
			t.Errorf("Expected root type source_file, got %s", res.Root.Type())
		}
	})

	t.Run("Context cancellation during parse", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.go")
		_ = os.WriteFile(tmpFile, []byte("package main"), 0644)

		_, err := p.ParseFile(ctx, tmpFile, "go")
		if err == nil {
			t.Error("Expected context cancellation error, got nil")
		}
	})
}
