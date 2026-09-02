package ingestion

import(
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/config"
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

// SyncRepo updates an already-cloned repository in place: fetch the latest
// from origin, then hard-reset the worktree to origin's current HEAD
// commit. We never write into this directory ourselves, so there's no
// local state to merge - a hard reset is simpler than a true "pull" and
// can't produce conflicts.
//
// The target commit is resolved via RemoteHeadSHA (a direct remote query)
// rather than by reading refs/remotes/origin/HEAD locally - go-git's
// programmatic clone/fetch, especially with a shallow Depth, doesn't
// reliably set up that symbolic ref the way the real git CLI does.
func (c *Cloner) SyncRepo(ctx context.Context, repoURL, destPath string) error {

	repo, err := git.PlainOpen(destPath)
	if err != nil {
		return fmt.Errorf("failed to open existing repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Force:      true,
		Depth:      1,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch latest: %w", err)
	}

	remoteSHA, err := RemoteHeadSHA(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to resolve remote head: %w", err)
	}

	err = worktree.Reset(&git.ResetOptions{
		Commit: plumbing.NewHash(remoteSHA),
		Mode:   git.HardReset,
	})

	if err != nil {
		return fmt.Errorf("failed to reset to remote HEAD: %w", err)
	}

	c.logger.Info("repository synced", "path", destPath, "commit", remoteSHA)
	return nil
}

// HeadCommitSHA returns the current HEAD commit hash of a cloned repo,
// recorded after a clone or sync so we know what state we last ingested.
func (c *Cloner) HeadCommitSHA(destPath string) (string, error) {

	repo, err := git.PlainOpen(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HEAD: %w", err)
	}

	return head.Hash().String(), nil
}

// RemoteHeadSHA checks a remote repository's default-branch HEAD commit
// without cloning or fetching any content - a single lightweight network
// call, used by the sync scheduler to cheaply decide whether a repo needs
// re-ingesting at all before touching disk.
func RemoteHeadSHA(ctx context.Context, repoURL string) (string, error) {

	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs: %w", err)
	}

	for _, ref := range refs {

		if ref.Name() != plumbing.HEAD {
			continue
		}

		if ref.Type() == plumbing.HashReference {
			return ref.Hash().String(), nil
		}

		// HEAD came back as a symbolic ref (points at e.g. refs/heads/main
		// by name, not a hash) - resolve it by finding that target ref in
		// the same listing.
		target := ref.Target()

		for _, r := range refs {
			if r.Name() == target {
				return r.Hash().String(), nil
			}
		}

		return "", fmt.Errorf("could not resolve symbolic HEAD target %s for %s", target, repoURL)
	}

	return "", fmt.Errorf("remote HEAD not found for %s", repoURL)
}