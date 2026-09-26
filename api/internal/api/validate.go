package api

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidRepoURL is returned for anything that isn't a plain, public
// https://github.com/<owner>/<repo> URL.
var ErrInvalidRepoURL = errors.New("repo_url must be a GitHub repository URL like https://github.com/owner/repo")

// GitHub's own rules: owner names are alphanumeric plus single hyphens and
// can't start with a hyphen; repo names allow alphanumerics, '-', '_' and '.'.
var (
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	githubRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// NormalizeGitHubURL validates a user-supplied repository URL and returns
// its canonical form (https scheme, github.com host, no ".git" suffix, no
// trailing slash). The raw URL is otherwise handed straight to go-git's
// clone, so anything that isn't a public GitHub repo is rejected outright:
// local paths and file:// URLs (would read the server's own filesystem),
// other schemes and hosts (including internal addresses like
// 169.254.169.254 - an SSRF vector once this runs on a hosted platform),
// embedded credentials, and extra path segments/query/fragment. Case is
// preserved for display - GitHub treats owner/repo case-insensitively, so
// callers comparing URLs for deduplication must compare case-insensitively.
func NormalizeGitHubURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidRepoURL
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidRepoURL
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return "", ErrInvalidRepoURL
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", ErrInvalidRepoURL
	}

	if u.Port() != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", ErrInvalidRepoURL
	}

	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", ErrInvalidRepoURL
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	if !githubOwnerPattern.MatchString(owner) || !githubRepoPattern.MatchString(repo) {
		return "", ErrInvalidRepoURL
	}
	// "." and ".." pass the repo charset but are path traversal, not repos
	// (a leading dot is otherwise legal - e.g. an org's ".github" repo).
	if repo == "." || repo == ".." {
		return "", ErrInvalidRepoURL
	}

	return "https://github.com/" + owner + "/" + repo, nil
}
