package api

import (
	"net/http"
	"testing"
)

func newReq(remoteAddr string, headers map[string]string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// With zero trusted hops (direct exposure / local dev), forwarding headers
// are attacker-controlled and must be ignored entirely.
func TestResolveClientIP_ZeroHopsIgnoresHeaders(t *testing.T) {
	r := newReq("203.0.113.9:5555", map[string]string{"X-Forwarded-For": "1.2.3.4"})

	if got := ResolveClientIP(r, 0, "X-Forwarded-For"); got != "203.0.113.9" {
		t.Errorf("expected RemoteAddr host, got %q", got)
	}
}

// Behind exactly one proxy (e.g. a PaaS router), the proxy appends the real
// client address as the rightmost X-Forwarded-For entry. Anything to the
// left of it was supplied by the client and is spoofable - so a client
// sending its own fake "X-Forwarded-For: 1.2.3.4" must NOT be able to pick
// its rate-limit bucket.
func TestResolveClientIP_OneHopTakesRightmostEntry(t *testing.T) {
	r := newReq("10.0.0.2:443", map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.7"})

	if got := ResolveClientIP(r, 1, "X-Forwarded-For"); got != "198.51.100.7" {
		t.Errorf("expected the proxy-appended rightmost entry, got %q", got)
	}
}

func TestResolveClientIP_TwoHops(t *testing.T) {
	r := newReq("10.0.0.2:443", map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.7, 10.0.0.9"})

	if got := ResolveClientIP(r, 2, "X-Forwarded-For"); got != "198.51.100.7" {
		t.Errorf("expected second-from-right entry, got %q", got)
	}
}

// Fewer entries than configured hops means the header wasn't set the way
// the deployment claims - fall back to the connection address rather than
// trusting a client-supplied value.
func TestResolveClientIP_TooFewEntriesFallsBack(t *testing.T) {
	r := newReq("10.0.0.2:443", map[string]string{"X-Forwarded-For": "198.51.100.7"})

	if got := ResolveClientIP(r, 2, "X-Forwarded-For"); got != "10.0.0.2" {
		t.Errorf("expected RemoteAddr fallback, got %q", got)
	}
}

// A single-value header set by the edge (e.g. Cloudflare's
// CF-Connecting-IP) is used as-is when configured.
func TestResolveClientIP_SingleValueHeader(t *testing.T) {
	r := newReq("10.0.0.2:443", map[string]string{"CF-Connecting-IP": "198.51.100.7"})

	if got := ResolveClientIP(r, 1, "CF-Connecting-IP"); got != "198.51.100.7" {
		t.Errorf("expected CF-Connecting-IP value, got %q", got)
	}
}

func TestResolveClientIP_GarbageEntryFallsBack(t *testing.T) {
	r := newReq("10.0.0.2:443", map[string]string{"X-Forwarded-For": "not-an-ip"})

	if got := ResolveClientIP(r, 1, "X-Forwarded-For"); got != "10.0.0.2" {
		t.Errorf("expected RemoteAddr fallback for an unparseable entry, got %q", got)
	}
}
