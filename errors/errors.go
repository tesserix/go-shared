package errors

import (
	"fmt"
	"net/http"
)

// AppError represents a custom application error
type AppError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

func (e AppError) Error() string {
	return e.Message
}

// Common error codes
const (
	// Authentication & Authorization
	ErrCodeMissingToken           = "MISSING_TOKEN"
	ErrCodeInvalidTokenFormat     = "INVALID_TOKEN_FORMAT"
	ErrCodeInvalidToken           = "INVALID_TOKEN"
	ErrCodeInvalidClaims          = "INVALID_CLAIMS"
	ErrCodeUnauthorized           = "UNAUTHORIZED"
	ErrCodeForbidden              = "FORBIDDEN"
	ErrCodeInsufficientPermissions = "INSUFFICIENT_PERMISSIONS"
	ErrCodeNoRoles                = "NO_ROLES"
	ErrCodeInvalidRoles           = "INVALID_ROLES"

	// General errors
	ErrCodeInternalServer    = "INTERNAL_SERVER_ERROR"
	ErrCodeBadRequest        = "BAD_REQUEST"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeMethodNotAllowed  = "METHOD_NOT_ALLOWED"
	ErrCodeConflict          = "CONFLICT"
	ErrCodeValidationFailed  = "VALIDATION_FAILED"
	ErrCodeDatabaseError     = "DATABASE_ERROR"
	ErrCodeExternalService   = "EXTERNAL_SERVICE_ERROR"
	// ErrCodeUnavailable means a dependency could not be reached or is not
	// configured — distinct from ErrCodeExternalService, which means a
	// dependency was reached and answered with an error.
	//
	// The distinction is not cosmetic. A caller that cannot tell "this upstream
	// is not wired up" from "this upstream failed" has to render both as broken,
	// which is how a surface shows an error where it should show "not measured"
	// (tesserix-home#198).
	ErrCodeUnavailable = "SERVICE_UNAVAILABLE"

	// Domain-specific errors
	ErrCodeResourceNotFound      = "RESOURCE_NOT_FOUND"
	ErrCodeResourceAlreadyExists = "RESOURCE_ALREADY_EXISTS"
	ErrCodeInvalidData           = "INVALID_DATA"
)

// Constructor functions for common errors

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string) AppError {
	return AppError{
		Code:       ErrCodeUnauthorized,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string) AppError {
	return AppError{
		Code:       ErrCodeForbidden,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewBadRequestError creates a new bad request error
func NewBadRequestError(message string, details map[string]interface{}) AppError {
	return AppError{
		Code:       ErrCodeBadRequest,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(details map[string]interface{}) AppError {
	return AppError{
		Code:       ErrCodeValidationFailed,
		Message:    "Validation failed",
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(resource string) AppError {
	return AppError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(message string, details map[string]interface{}) AppError {
	return AppError{
		Code:       ErrCodeConflict,
		Message:    message,
		StatusCode: http.StatusConflict,
		Details:    details,
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(message string) AppError {
	return AppError{
		Code:       ErrCodeDatabaseError,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewExternalServiceError creates a new external service error.
//
// A nil err is tolerated. This constructor is called on failure paths, often
// from a wrapper that may not have an error to hand, and an error constructor
// that panics turns a handled failure into a crashed request — the one place a
// panic is least affordable.
func NewExternalServiceError(service string, err error) AppError {
	details := map[string]interface{}{"service": service}
	if err != nil {
		details["error"] = err.Error()
	}
	return AppError{
		Code:       ErrCodeExternalService,
		Message:    fmt.Sprintf("External service error: %s", service),
		StatusCode: http.StatusServiceUnavailable,
		Details:    details,
	}
}

// NewUnavailableError creates an error for a dependency that could not be
// reached or is not configured.
//
// Use this rather than NewExternalServiceError when nothing answered at all:
// no connection, no credentials, feature not wired up. Both are 503; only this
// one lets a caller say "not measured" instead of "failed".
//
// The message is shown to callers, so it should describe WHAT is unavailable
// and not why — the underlying cause belongs in the server's log.
func NewUnavailableError(message string) AppError {
	return AppError{
		Code:       ErrCodeUnavailable,
		Message:    message,
		StatusCode: http.StatusServiceUnavailable,
	}
}

// NewInternalServerError creates a new internal server error
func NewInternalServerError(message string) AppError {
	return AppError{
		Code:       ErrCodeInternalServer,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// Auth-specific errors

// NewMissingTokenError creates a new missing token error
func NewMissingTokenError() AppError {
	return AppError{
		Code:       ErrCodeMissingToken,
		Message:    "Authorization header is required",
		StatusCode: http.StatusUnauthorized,
	}
}

// NewInvalidTokenFormatError creates a new invalid token format error
func NewInvalidTokenFormatError() AppError {
	return AppError{
		Code:       ErrCodeInvalidTokenFormat,
		Message:    "Authorization header must be in format: Bearer <token>",
		StatusCode: http.StatusUnauthorized,
	}
}

// NewInvalidTokenError creates a new invalid token error
func NewInvalidTokenError() AppError {
	return AppError{
		Code:       ErrCodeInvalidToken,
		Message:    "Invalid or expired token",
		StatusCode: http.StatusUnauthorized,
	}
}

// NewInvalidClaimsError creates a new invalid claims error
func NewInvalidClaimsError() AppError {
	return AppError{
		Code:       ErrCodeInvalidClaims,
		Message:    "Invalid token claims",
		StatusCode: http.StatusUnauthorized,
	}
}

// NewInsufficientPermissionsError creates a new insufficient permissions error
func NewInsufficientPermissionsError(requiredRole string) AppError {
	return AppError{
		Code:       ErrCodeInsufficientPermissions,
		Message:    fmt.Sprintf("Required role: %s", requiredRole),
		StatusCode: http.StatusForbidden,
	}
}

// NewInsufficientPermissionsAnyError creates a new insufficient permissions error for multiple roles
func NewInsufficientPermissionsAnyError(requiredRoles []string) AppError {
	return AppError{
		Code:       ErrCodeInsufficientPermissions,
		Message:    fmt.Sprintf("Required one of roles: %v", requiredRoles),
		StatusCode: http.StatusForbidden,
	}
}

// NewNoRolesError creates a new no roles error
func NewNoRolesError() AppError {
	return AppError{
		Code:       ErrCodeNoRoles,
		Message:    "User roles not found",
		StatusCode: http.StatusForbidden,
	}
}

// NewInvalidRolesError creates a new invalid roles error
func NewInvalidRolesError() AppError {
	return AppError{
		Code:       ErrCodeInvalidRoles,
		Message:    "Invalid user roles format",
		StatusCode: http.StatusForbidden,
	}
}