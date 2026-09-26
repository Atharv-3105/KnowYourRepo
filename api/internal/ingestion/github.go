package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrRepoNotFound means GitHub reports the repository doesn't exist or
// isn't publicly visible - cloning it would fail anyway.
var ErrRepoNotFound = errors.New("repository not found or not public")

// ErrRepoTooLarge means the repository exceeds the configured size ceiling.
type ErrRepoTooLarge struct {
	SizeMB  int
	LimitMB int
}

func (e *ErrRepoTooLarge) Error() string {
	return fmt.Sprintf("repository is %dMB, above this deployment's %dMB limit", e.SizeMB, e.LimitMB)
}

// RepoMetadata is the subset of GitHub's repository API response used by
// the pre-clone check.
type RepoMetadata struct {
	SizeKB        int    `json:"size"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// GitHubClient queries the public GitHub REST API. Unauthenticated calls are
// limited to 60/hour per server IP - plenty for a demo; an optional token
// raises that to 5000/hour.
type GitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		baseURL:    "https://api.github.com",
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// WithBaseURL points the client at a different API root (tests).
func (g *GitHubClient) WithBaseURL(url string) *GitHubClient {
	g.baseURL = strings.TrimRight(url, "/")
	return g
}

// RepoMetadata fetches metadata for a canonical https://github.com/owner/repo
// URL. Returns ErrRepoNotFound on 404; any other failure (rate limit,
// network) is returned as a plain error so the caller can choose to fail
// open - the size check is a guardrail, not a hard dependency.
func (g *GitHubClient) RepoMetadata(ctx context.Context, repoURL string) (*RepoMetadata, error) {
	ownerRepo := strings.TrimPrefix(repoURL, "https://github.com/")
	if ownerRepo == repoURL || strings.Count(ownerRepo, "/") != 1 {
		return nil, fmt.Errorf("not a canonical GitHub repo URL: %s", repoURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/repos/"+ownerRepo, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "KnowYourRepo")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github metadata request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrRepoNotFound
	default:
		return nil, fmt.Errorf("github metadata request returned status %d", resp.StatusCode)
	}

	var meta RepoMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode github metadata: %w", err)
	}
	return &meta, nil
}

// CheckRepoSize enforces limitMB against GitHub's reported repository size.
// Fails open (returns nil) when GitHub can't be reached or rate-limits the
// server - the walker's own file-count and file-size caps still bound the
// work in that case. A definite "not found/private" or "too large" is
// returned as an error.
func (g *GitHubClient) CheckRepoSize(ctx context.Context, repoURL string, limitMB int) error {
	if limitMB <= 0 {
		return nil
	}

	meta, err := g.RepoMetadata(ctx, repoURL)
	if errors.Is(err, ErrRepoNotFound) {
		return err
	}
	if err != nil {
		return nil
	}

	sizeMB := meta.SizeKB / 1024
	if sizeMB > limitMB {
		return &ErrRepoTooLarge{SizeMB: sizeMB, LimitMB: limitMB}
	}
	return nil
}
