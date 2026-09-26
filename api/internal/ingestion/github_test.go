package ingestion

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeGitHub(t *testing.T, status int, body string) *GitHubClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewGitHubClient("").WithBaseURL(server.URL)
}

func TestCheckRepoSize_UnderLimitPasses(t *testing.T) {
	g := fakeGitHub(t, http.StatusOK, `{"size": 10240}`) // 10MB
	if err := g.CheckRepoSize(context.Background(), "https://github.com/owner/repo", 150); err != nil {
		t.Errorf("expected a 10MB repo to pass a 150MB limit, got %v", err)
	}
}

func TestCheckRepoSize_OverLimitRejected(t *testing.T) {
	g := fakeGitHub(t, http.StatusOK, `{"size": 512000}`) // 500MB
	err := g.CheckRepoSize(context.Background(), "https://github.com/owner/repo", 150)

	var tooLarge *ErrRepoTooLarge
	if !errors.As(err, &tooLarge) {
		t.Fatalf("expected ErrRepoTooLarge, got %v", err)
	}
	if tooLarge.SizeMB != 500 || tooLarge.LimitMB != 150 {
		t.Errorf("unexpected sizes in error: %+v", tooLarge)
	}
}

func TestCheckRepoSize_NotFoundRejected(t *testing.T) {
	g := fakeGitHub(t, http.StatusNotFound, `{"message": "Not Found"}`)
	if err := g.CheckRepoSize(context.Background(), "https://github.com/owner/repo", 150); !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

// GitHub's unauthenticated rate limit (60/hour/IP) is easy to hit on a
// shared demo server - that must not block ingestion outright.
func TestCheckRepoSize_FailsOpenOnRateLimit(t *testing.T) {
	g := fakeGitHub(t, http.StatusForbidden, `{"message": "API rate limit exceeded"}`)
	if err := g.CheckRepoSize(context.Background(), "https://github.com/owner/repo", 150); err != nil {
		t.Errorf("expected fail-open on a rate-limit response, got %v", err)
	}
}
