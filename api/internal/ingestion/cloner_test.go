package ingestion 

import(
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestCloneRepo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewCloner(logger)
	ctx := context.Background()

	t.Run("Valid Clone", func(t *testing.T) {
		tmp := t.TempDir()
		target := tmp + "/repo"

		err := c.CloneRepo(ctx, "https://github.com/octocat/Spoon-Knife", target)
		if err != nil {
			t.Fatalf("Expected successful clone, got: %v", err)
		}

		if _, err := os.Stat(target + "/README.md"); os.IsNotExist(err) {
			t.Error("README.md not found in cloned repo")
		}
	})

	t.Run("Fail on non-empty dir", func(t *testing.T) {
		tmp := t.TempDir()
		_ = os.WriteFile(tmp + "/dummy.txt", []byte("data"), 0644)

		err := c.CloneRepo(ctx, "https://github.com/octocat/Spoon-Knife", tmp)
		if err == nil {
			t.Error("Expected error when cloning into non-empty directory, got nil")
		}
	})
}