package ingestion

import(
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
)

type Cloner struct {
	logger *slog.Logger
}

func NewCloner(logger *slog.Logger) *Cloner {
	return &Cloner{
		logger: logger, 
	}
}


//CloneRepo function clones(PULLs) a REPO into destPath
//It errors if the destPath EXISTS and is NOT EMPTY
//It uses context to safely handle the context based execution
func(c *Cloner) CloneRepo(ctx context.Context, repoURL, destPath string) error {
	if repoURL == "" {
		return errors.New("repo URL cannot be empty")
	}

	c.logger.Info("starting repository ingestion", "url", repoURL, "dest", destPath)

	//Check destination path exists or not
	exists, err := pathExists(destPath)
	if err != nil {
		return fmt.Errorf("failed to check destination path: %w", err)
	}

	if exists {
		//If destination path is empty
		empty, err := isDirEmpty(destPath)
		if err != nil {
			return fmt.Errorf("failed to check if destination directory is empty: %w", err)
		}

		if !empty {
			return fmt.Errorf("destination directory already exists and is not empty: %s", destPath)
		}
	}else {
		//Create DIR 
		if err := os.MkdirAll(destPath, 0o755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	//Perform clone with Context(for cancellation) and Depth 1 (for Speed)
	_, err = git.PlainCloneContext(ctx, destPath, false, &git.CloneOptions{
		URL: repoURL,
		Depth: 1,
		Progress: os.Stdout,
	})

	if err != nil {
		//Cleanup the Destination Directory if CLONING fails
		_ = os.RemoveAll(destPath)
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	c.logger.Info("repository successfully cloned", "path", destPath)
	return nil
}

//Function to check whether the Path exists or Not
func pathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil 
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err 
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false, err 
	}

	defer f.Close()

	//Try reading one entry
	_, err = f.Readdirnames(1)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrClosed) || fmt.Sprint(err) == "EOF" {
		return true, nil
	}

	return false, err
}