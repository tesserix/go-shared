package middleware

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/tesserix/go-shared/errors"
)

// ErrorHandler is a middleware that handles errors in a consistent way
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			handleError(c, err.Err)
		}
	}
}

// handleError processes the error and sends appropriate response
func handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(errors.AppError); ok {
		logError(c, err, appErr)
		ErrorResponse(c, appErr)
		return
	}

	// Default to internal server error
	appErr := errors.NewInternalServerError("An unexpected error occurred")
	logError(c, err, appErr)
	ErrorResponse(c, appErr)
}

// logError logs the error details
func logError(c *gin.Context, err error, appErr errors.AppError) {
	requestID, _ := c.Get("request_id")

	log.Printf("[ERROR] RequestID: %v, Code: %s, Message: %s, Path: %s, Method: %s, Error: %v",
		requestID,
		appErr.Code,
		appErr.Message,
		c.Request.URL.Path,
		c.Request.Method,
		err,
	)
}

// Recovery middleware with standardized error response
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			appErr := errors.NewInternalServerError(fmt.Sprintf("Panic recovered: %s", err))
			logError(c, fmt.Errorf("%s", err), appErr)
			ErrorResponse(c, appErr)
		} else {
			appErr := errors.NewInternalServerError("Internal server error")
			logError(c, fmt.Errorf("%v", recovered), appErr)
			ErrorResponse(c, appErr)
		}
	})
}
