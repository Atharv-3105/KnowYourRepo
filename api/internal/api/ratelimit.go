package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// visitorLimitIdleTimeout controls how long a caller's bucket is kept
// around after its last request before the cleanup loop evicts it - keeps
// the map bounded for a long-running process seen by many distinct IPs
// over time, without needing every caller to keep hitting the route to
// stay "known".
const visitorLimitIdleTimeout = 10 * time.Minute

// visitor pairs one caller's token bucket with when it was last used, so
// the cleanup loop can tell which entries are safe to forget.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// KeyLimiter is a per-key token-bucket rate limiter - one bucket per
// caller, keyed by client IP in practice (see middleware below). A token
// bucket, not a rolling window: it allows a small legitimate burst (a user
// firing off a couple of quick chat messages, or retrying a failed repo
// POST once) while still bounding the sustained rate, and golang.org/x/time/rate
// is a well-tested standard implementation rather than something hand-rolled.
// Safe for concurrent use.
type KeyLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	r        rate.Limit
	burst    int
}

// NewKeyLimiter builds a limiter allowing r events per second (construct
// with rate.Every(interval) for "N per duration" limits) with the given
// burst capacity, per distinct key.
func NewKeyLimiter(r rate.Limit, burst int) *KeyLimiter {
	kl := &KeyLimiter{
		visitors: make(map[string]*visitor),
		r:        r,
		burst:    burst,
	}
	go kl.cleanupLoop()
	return kl
}

func (kl *KeyLimiter) getLimiter(key string) *rate.Limiter {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	v, exists := kl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(kl.r, kl.burst)
		kl.visitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupLoop evicts visitors idle longer than visitorLimitIdleTimeout.
// Runs for the lifetime of the process - KeyLimiter instances are meant to
// be created once at server startup, not per-request.
func (kl *KeyLimiter) cleanupLoop() {
	for {
		time.Sleep(time.Minute)

		kl.mu.Lock()
		for key, v := range kl.visitors {
			if time.Since(v.lastSeen) > visitorLimitIdleTimeout {
				delete(kl.visitors, key)
			}
		}
		kl.mu.Unlock()
	}
}

// Middleware returns Gin middleware that rejects any request exceeding
// this limiter's rate for the caller's client IP with 429. Intended to be
// attached to specific routes (chat, repo creation) that front real LLM/
// embedding provider cost or heavy background work - not registered
// globally, since most routes (health, listing, browsing an already-
// ingested repo) don't need this protection.
func (kl *KeyLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := kl.getLimiter(c.ClientIP())

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please slow down and try again shortly",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
