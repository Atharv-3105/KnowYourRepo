package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

// respondInternalError logs the real error server-side and returns a
// generic message plus the request ID to the client. Raw error strings
// from the database driver, the sidecar, or an upstream LLM/embedding
// provider can carry DSNs, internal hostnames, SQL, or provider responses -
// none of which belong in a public API response. The request ID lets a
// user report a failure that can be matched to the full log line.
func respondInternalError(c *gin.Context, logger *slog.Logger, event string, err error, attrs ...any) {
	requestID := config.FromContext(c.Request.Context())

	logArgs := append([]any{"request_id", requestID, "error", err}, attrs...)
	logger.Error(event, logArgs...)

	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      "internal server error",
		"request_id": requestID,
	})
}

// BodyLimitMiddleware caps request body size. Every JSON body this API
// accepts is tiny (a URL, a question, a query) - without a cap, a single
// multi-megabyte POST would be read fully into memory by the JSON binder
// on a small free-tier instance.
func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// SecurityHeadersMiddleware sets conservative headers appropriate for a
// JSON-only API (nothing here is ever meant to be rendered or framed).
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
