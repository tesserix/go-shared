package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tesserix/go-shared/errors"
)

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success   bool          `json:"success"`
	Data      interface{}   `json:"data,omitempty"`
	Error     *ErrorDetails `json:"error,omitempty"`
	Meta      *MetaData     `json:"meta,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	RequestID string        `json:"request_id,omitempty"`
}

// ErrorDetails contains error information
type ErrorDetails struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// MetaData contains pagination and other metadata
type MetaData struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// SuccessResponse sends a success response
func SuccessResponse(c *gin.Context, data interface{}) {
	requestID, _ := c.Get("request_id")

	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	if requestID != nil {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusOK, response)
}

// SuccessResponseWithMeta sends a success response with metadata
func SuccessResponseWithMeta(c *gin.Context, data interface{}, meta *MetaData) {
	requestID, _ := c.Get("request_id")

	response := StandardResponse{
		Success:   true,
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now().UTC(),
	}

	if requestID != nil {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusOK, response)
}

// CreatedResponse sends a created response
func CreatedResponse(c *gin.Context, data interface{}) {
	requestID, _ := c.Get("request_id")

	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	if requestID != nil {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusCreated, response)
}

// NoContentResponse sends a no content response
func NoContentResponse(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, appErr errors.AppError) {
	requestID, _ := c.Get("request_id")

	response := StandardResponse{
		Success: false,
		Error: &ErrorDetails{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
		Timestamp: time.Now().UTC(),
	}

	if requestID != nil {
		response.RequestID = requestID.(string)
	}

	c.JSON(appErr.StatusCode, response)
}

// ValidationErrorResponse sends a validation error response
func ValidationErrorResponse(c *gin.Context, validationErrors map[string]interface{}) {
	appErr := errors.NewValidationError(validationErrors)
	ErrorResponse(c, appErr)
}

// UnauthorizedResponse sends an unauthorized response
func UnauthorizedResponse(c *gin.Context, message string) {
	appErr := errors.NewUnauthorizedError(message)
	ErrorResponse(c, appErr)
}

// ForbiddenResponse sends a forbidden response
func ForbiddenResponse(c *gin.Context, message string) {
	appErr := errors.NewForbiddenError(message)
	ErrorResponse(c, appErr)
}

// NotFoundResponse sends a not found response
func NotFoundResponse(c *gin.Context, resource string) {
	appErr := errors.NewNotFoundError(resource)
	ErrorResponse(c, appErr)
}

// BadRequestResponse sends a bad request response
func BadRequestResponse(c *gin.Context, message string, details map[string]interface{}) {
	appErr := errors.NewBadRequestError(message, details)
	ErrorResponse(c, appErr)
}

// ConflictResponse sends a conflict response
func ConflictResponse(c *gin.Context, message string, details map[string]interface{}) {
	appErr := errors.NewConflictError(message, details)
	ErrorResponse(c, appErr)
}

// InternalServerErrorResponse sends an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	appErr := errors.NewInternalServerError(message)
	ErrorResponse(c, appErr)
}

// PaginatedResponse creates metadata for paginated responses
func PaginatedResponse(page, perPage int, total int64) *MetaData {
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	return &MetaData{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}
