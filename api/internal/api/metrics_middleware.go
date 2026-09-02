package api

import (
	"strconv"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/metrics"
	"github.com/gin-gonic/gin"
)

// MetricsMiddleware records request count and latency for every request.
// Kept as its own middleware, separate from RequestIDMiddleware, even
// though both wrap the whole request lifecycle - metrics and
// request-ID/logging are different concerns that happen to share the
// same before/after shape, and keeping them separate means either can be
// disabled or changed without touching the other.
func MetricsMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}

		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
