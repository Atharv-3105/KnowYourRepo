package ingestion

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalker_WalkRepo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := NewWalker(logger)
	ctx := context.Background()

	t.Run("Identify source files and skip ignored", func(t *testing.T) {
		tmpDir := t.TempDir()


		filesToCreate := []string{
			"main.go",
			"utils/helper.py",
			"node_modules/index.js", //should be ignored
			".git/config",
			"ui/app.tsx",
			"README.md",
		}

		for _, f := range filesToCreate{
			fullPath := filepath.Join(tmpDir, f)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
			_ = os.WriteFile(fullPath, []byte("test content"), 0644)
		}

		found, err := w.WalkRepo(ctx, tmpDir)
		if err != nil {
			t.Fatalf("WalkRepo failed: %v", err)
		}

		if len(found) != 3 {
			t.Errorf("Expected 3 files, found %d", len(found))
		}

		for _, f := range found {
			if strings.Contains(f.RelPath, "node_modules") || strings.Contains(f.RelPath, ".git") {
				t.Errorf("Found ignored files: %s", f.RelPath)
			}
		}
	})
}