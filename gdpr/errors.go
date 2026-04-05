package gdpr

import (
	"errors"
	"fmt"
	"net/http"
)

// Standard GDPR errors
var (
	// User errors
	ErrInvalidUserID = errors.New("invalid user ID")

	// Consent errors
	ErrInvalidPurpose          = errors.New("invalid consent purpose")
	ErrConsentNotFound         = errors.New("consent record not found")
	ErrConsentAlreadyWithdrawn = errors.New("consent has already been withdrawn")
	ErrConsentExpired          = errors.New("consent has expired")

	// Deletion errors
	ErrInvalidRequestID      = errors.New("invalid request ID")
	ErrRequestNotFound       = errors.New("request not found")
	ErrDeletionRequestExists = errors.New("a pending deletion request already exists for this user")
	ErrInvalidRequestStatus  = errors.New("invalid request status for this operation")
	ErrDeletionNotDue        = errors.New("deletion is scheduled for a future date")
	ErrCannotCancelRequest   = errors.New("cannot cancel request in current status")
	ErrDeletionNotFound      = errors.New("deletion record not found")
	ErrInvalidDeletionID     = errors.New("invalid deletion ID")

	// Retention errors
	ErrInvalidDataCategory    = errors.New("invalid data category")
	ErrInvalidRetentionPeriod = errors.New("retention period must be greater than zero")
	ErrInvalidPolicyID        = errors.New("invalid policy ID")
	ErrPolicyNotFound         = errors.New("retention policy not found")

	// Export errors
	ErrExportFailed = errors.New("data export failed")
	ErrNoDataFound  = errors.New("no data found for user")
)

// GDPRError represents a GDPR-specific error with additional context
type GDPRError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Err        error                  `json:"-"`
}

// Error implements the error interface
func (e *GDPRError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *GDPRError) Unwrap() error {
	return e.Err
}

// WithDetails adds details to the error
func (e *GDPRError) WithDetails(details map[string]interface{}) *GDPRError {
	e.Details = details
	return e
}

// GDPR error codes
const (
	ErrCodeGDPRInvalidRequest     = "GDPR_INVALID_REQUEST"
	ErrCodeGDPRUnauthorized       = "GDPR_UNAUTHORIZED"
	ErrCodeGDPRNotFound           = "GDPR_NOT_FOUND"
	ErrCodeGDPRConsentRequired    = "GDPR_CONSENT_REQUIRED"
	ErrCodeGDPRConsentExpired     = "GDPR_CONSENT_EXPIRED"
	ErrCodeGDPRDeletionPending    = "GDPR_DELETION_PENDING"
	ErrCodeGDPRDeletionFailed     = "GDPR_DELETION_FAILED"
	ErrCodeGDPRExportFailed       = "GDPR_EXPORT_FAILED"
	ErrCodeGDPRPolicyViolation    = "GDPR_POLICY_VIOLATION"
	ErrCodeGDPRRetentionViolation = "GDPR_RETENTION_VIOLATION"
	ErrCodeGDPRInternalError      = "GDPR_INTERNAL_ERROR"
)

// Error constructors

// NewGDPRError creates a new GDPR error
func NewGDPRError(code string, message string, statusCode int) *GDPRError {
	return &GDPRError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRInvalidRequest,
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRUnauthorized,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

// NewConsentRequiredError creates a consent required error
func NewConsentRequiredError(purpose ConsentPurpose) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRConsentRequired,
		Message:    fmt.Sprintf("consent required for purpose: %s", purpose),
		StatusCode: http.StatusForbidden,
		Details: map[string]interface{}{
			"purpose": purpose,
		},
	}
}

// NewConsentExpiredError creates a consent expired error
func NewConsentExpiredError(purpose ConsentPurpose) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRConsentExpired,
		Message:    fmt.Sprintf("consent has expired for purpose: %s", purpose),
		StatusCode: http.StatusForbidden,
		Details: map[string]interface{}{
			"purpose": purpose,
		},
	}
}

// NewDeletionPendingError creates a deletion pending error
func NewDeletionPendingError(userID string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRDeletionPending,
		Message:    "a deletion request is already pending for this user",
		StatusCode: http.StatusConflict,
		Details: map[string]interface{}{
			"user_id": userID,
		},
	}
}

// NewDeletionFailedError creates a deletion failed error
func NewDeletionFailedError(err error) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRDeletionFailed,
		Message:    "data deletion failed",
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

// NewExportFailedError creates an export failed error
func NewExportFailedError(err error) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRExportFailed,
		Message:    "data export failed",
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

// NewPolicyViolationError creates a policy violation error
func NewPolicyViolationError(policy string, violation string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRPolicyViolation,
		Message:    fmt.Sprintf("GDPR policy violation: %s", violation),
		StatusCode: http.StatusForbidden,
		Details: map[string]interface{}{
			"policy":    policy,
			"violation": violation,
		},
	}
}

// NewRetentionViolationError creates a retention violation error
func NewRetentionViolationError(category DataCategory, message string) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRRetentionViolation,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Details: map[string]interface{}{
			"data_category": category,
		},
	}
}

// NewInternalError creates an internal error
func NewInternalError(err error) *GDPRError {
	return &GDPRError{
		Code:       ErrCodeGDPRInternalError,
		Message:    "internal GDPR processing error",
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

// IsGDPRError checks if an error is a GDPRError
func IsGDPRError(err error) bool {
	var gdprErr *GDPRError
	return errors.As(err, &gdprErr)
}

// GetGDPRError extracts a GDPRError from an error chain
func GetGDPRError(err error) *GDPRError {
	var gdprErr *GDPRError
	if errors.As(err, &gdprErr) {
		return gdprErr
	}
	return nil
}

// WrapError wraps a standard error in a GDPRError
func WrapError(err error, code string, message string) *GDPRError {
	return &GDPRError{
		Code:       code,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}
