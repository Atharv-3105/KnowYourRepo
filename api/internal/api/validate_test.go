package api

import "testing"

func TestNormalizeGitHubURL_AcceptsCanonicalForms(t *testing.T) {
	cases := map[string]string{
		"https://github.com/Atharv-3105/JobBot":           "https://github.com/Atharv-3105/JobBot",
		"https://github.com/Atharv-3105/JobBot/":          "https://github.com/Atharv-3105/JobBot",
		"https://github.com/Atharv-3105/JobBot.git":       "https://github.com/Atharv-3105/JobBot",
		"  https://github.com/octocat/Hello-World  ":      "https://github.com/octocat/Hello-World",
		"https://www.github.com/gin-gonic/gin":            "https://github.com/gin-gonic/gin",
		"http://github.com/rs/xid":                        "https://github.com/rs/xid",
		"https://github.com/some-org/repo_name.with.dots": "https://github.com/some-org/repo_name.with.dots",
		"https://github.com/some-org/.github":             "https://github.com/some-org/.github",
	}

	for in, want := range cases {
		got, err := NormalizeGitHubURL(in)
		if err != nil {
			t.Errorf("NormalizeGitHubURL(%q) returned error %v, want %q", in, err, want)
			continue
		}
		if got != want {
			t.Errorf("NormalizeGitHubURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every one of these would otherwise be handed straight to go-git's clone:
// local filesystem paths, other schemes, other hosts (including internal
// network addresses - an SSRF vector on a hosted deployment), embedded
// credentials, and malformed owner/repo paths.
func TestNormalizeGitHubURL_RejectsUnsafeOrNonGitHubInput(t *testing.T) {
	bad := []string{
		"",
		"not a url",
		"file:///etc/passwd",
		"/etc/passwd",
		"C:\\Windows\\win.ini",
		"git@github.com:owner/repo.git",
		"ssh://git@github.com/owner/repo",
		"https://gitlab.com/owner/repo",
		"https://github.com.evil.com/owner/repo",
		"https://evil.com/github.com/owner/repo",
		"http://169.254.169.254/latest/meta-data",
		"http://localhost:8080/repos",
		"https://user:token@github.com/owner/repo",
		"https://github.com/owner",
		"https://github.com/owner/repo/tree/main",
		"https://github.com/owner/repo?x=1",
		"https://github.com/owner/repo#readme",
		"https://github.com/../etc/passwd",
		"https://github.com/owner/..",
		"https://github.com/-bad/repo",
		"https://github.com/owner/re po",
	}

	for _, in := range bad {
		if got, err := NormalizeGitHubURL(in); err == nil {
			t.Errorf("NormalizeGitHubURL(%q) = %q, expected an error", in, got)
		}
	}
}
