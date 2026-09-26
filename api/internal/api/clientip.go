package api

import (
	"net"
	"net/http"
	"strings"
)

// ResolveClientIP returns the address used to key per-IP rate limiting.
//
// Gin's own ClientIP() with "trust all proxies" (its default, and what this
// server used before) walks X-Forwarded-For and ends up returning the
// leftmost entry - which the client controls. A caller could send a fresh
// fake "X-Forwarded-For: 1.2.3.4" with every request and get a brand-new
// rate-limit bucket each time, bypassing the limiter entirely.
//
// Instead this uses the standard hop-count model: with trustedHops = N,
// exactly N proxies we control sit in front of the app, each appending the
// address it received the connection from, so the real client is the Nth
// entry from the right. Everything further left is client-supplied and
// ignored. trustedHops = 0 (direct exposure, local dev) ignores forwarding
// headers entirely and uses the TCP peer address.
func ResolveClientIP(r *http.Request, trustedHops int, header string) string {
	remote := remoteHost(r.RemoteAddr)

	if trustedHops <= 0 || header == "" {
		return remote
	}

	value := r.Header.Get(header)
	if value == "" {
		return remote
	}

	entries := strings.Split(value, ",")

	// A single-value header set by the edge itself (e.g. Cloudflare's
	// CF-Connecting-IP) - nothing to count hops through.
	if !strings.EqualFold(header, "X-Forwarded-For") {
		if ip := net.ParseIP(strings.TrimSpace(entries[len(entries)-1])); ip != nil {
			return ip.String()
		}
		return remote
	}

	if len(entries) < trustedHops {
		return remote
	}

	candidate := strings.TrimSpace(entries[len(entries)-trustedHops])
	if ip := net.ParseIP(candidate); ip != nil {
		return ip.String()
	}
	return remote
}

func remoteHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
