package ingestion

import (
	"context"
	"io"
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

func TestReadReadme_FindsRootReadme(t *testing.T) {
	dir := t.TempDir()
	content := "# My Project\n\nThis project does things."
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if got != content {
		t.Errorf("expected README content %q, got %q", content, got)
	}
}

func TestReadReadme_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	content := "lowercase readme"
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if got != content {
		t.Errorf("expected README content %q, got %q", content, got)
	}
}

func TestReadReadme_NoReadmePresent(t *testing.T) {
	dir := t.TempDir()

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("expected no error when no README is present, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string when no README is present, got: %q", got)
	}
}

func TestReadReadme_TruncatesLongReadme(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("x", maxReadmeChars+500)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture README: %v", err)
	}

	w := NewWalker(slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := w.ReadReadme(dir)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if len(got) != maxReadmeChars {
		t.Errorf("expected truncation to %d chars, got %d", maxReadmeChars, len(got))
	}
}