package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func newTestRouter(kl *KeyLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/limited", kl.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func doRequest(router *gin.Engine, remoteAddr string) int {
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func TestKeyLimiter_AllowsUpToBurstThenRejects(t *testing.T) {
	// burst 2, effectively no refill within the test's lifetime
	kl := NewKeyLimiter(rate.Every(time.Hour), 2)
	router := newTestRouter(kl)

	if code := doRequest(router, "1.2.3.4:5555"); code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", code)
	}
	if code := doRequest(router, "1.2.3.4:5555"); code != http.StatusOK {
		t.Fatalf("request 2 (within burst): expected 200, got %d", code)
	}
	if code := doRequest(router, "1.2.3.4:5555"); code != http.StatusTooManyRequests {
		t.Fatalf("request 3 (over burst): expected 429, got %d", code)
	}
}

func TestKeyLimiter_TracksCallersIndependently(t *testing.T) {
	kl := NewKeyLimiter(rate.Every(time.Hour), 1)
	router := newTestRouter(kl)

	if code := doRequest(router, "1.1.1.1:1111"); code != http.StatusOK {
		t.Fatalf("caller A request 1: expected 200, got %d", code)
	}
	if code := doRequest(router, "1.1.1.1:1111"); code != http.StatusTooManyRequests {
		t.Fatalf("caller A request 2 (over burst): expected 429, got %d", code)
	}
	// A different caller has its own bucket, unaffected by A's usage.
	if code := doRequest(router, "2.2.2.2:2222"); code != http.StatusOK {
		t.Fatalf("caller B request 1: expected 200 (independent bucket), got %d", code)
	}
}

func TestKeyLimiter_RefillsOverTime(t *testing.T) {
	// 100 events/sec refill rate - effectively instant refill for the test.
	kl := NewKeyLimiter(rate.Limit(100), 1)
	router := newTestRouter(kl)

	if code := doRequest(router, "3.3.3.3:3333"); code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", code)
	}
	if code := doRequest(router, "3.3.3.3:3333"); code != http.StatusTooManyRequests {
		t.Fatalf("request 2 (immediately after, over burst): expected 429, got %d", code)
	}

	time.Sleep(30 * time.Millisecond) // several refill intervals at 100/sec

	if code := doRequest(router, "3.3.3.3:3333"); code != http.StatusOK {
		t.Fatalf("request 3 (after refill): expected 200, got %d", code)
	}
}
