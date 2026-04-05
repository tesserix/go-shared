package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already present in headers
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate new UUID if not present
			requestID = uuid.New().String()
		}

		// Set request ID in context
		c.Set("request_id", requestID)
		c.Set("trace_id", requestID) // Also set as trace_id for error handling

		// Add to response headers
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
