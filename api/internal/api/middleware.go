package api 

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

const RequestIDHeader = "X-Request-ID"

// CORSMiddleware allows only the configured frontend origins to call this API from a browser,
// and short-circuits preflight OPTIONS requests before they reach any route handler.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {

	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.TrimSpace(o)] = struct{}{}
	}

	return func(c *gin.Context) {

		origin := c.GetHeader("Origin")

		if _, ok := originSet[origin]; ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

//Function ensures every request has a request ID(reusing one supplied by the caller if present)
//The reqID is stored on the request context so it can propagate to the sidecar
func RequestIDMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = generateRequestID()
		}

		ctx := config.WithID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)

		c.Writer.Header().Set(RequestIDHeader, id)

		logger.Info("http_request_started", "request_id", id, "method", c.Request.Method, "path", c.Request.URL.Path)

		c.Next()

		logger.Info("http_request_completed", "request_id", id, "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())
	}
}

func generateRequestID() string {
	//Make a buffer of 8 bytes
	buf := make([]byte, 8)

	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}

	//Return the encoded string of 8 bytes id
	return hex.EncodeToString(buf)
}