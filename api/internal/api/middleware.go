package api 

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"github.com/gin-gonic/gin"
	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

const RequestIDHeader = "X-Request-ID"

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