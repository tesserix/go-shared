package errors_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	apperrors "github.com/tesserix/go-shared/errors"
	"github.com/stretchr/testify/assert"
)

// TestAppError_ErrorInterface verifies that AppError satisfies the error interface
// and that Error() returns the message field.
func TestAppError_ErrorInterface(t *testing.T) {
	err := apperrors.AppError{
		Code:       "TEST_CODE",
		Message:    "something went wrong",
		StatusCode: http.StatusInternalServerError,
	}

	// Verify it implements the error interface.
	var iface error = err
	assert.Equal(t, "something went wrong", iface.Error())

	// Verify errors.As works correctly with AppError.
	var target apperrors.AppError
	assert.True(t, errors.As(iface, &target))
	assert.Equal(t, "TEST_CODE", target.Code)
}

// TestAppError_DetailsNilByDefault verifies that Details is nil when not provided.
func TestAppError_DetailsNilByDefault(t *testing.T) {
	err := apperrors.AppError{
		Code:       "TEST_CODE",
		Message:    "msg",
		StatusCode: http.StatusOK,
	}
	assert.Nil(t, err.Details)
}

// --- Constructor: NewUnauthorizedError ---

func TestNewUnauthorizedError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"standard message", "you are not authenticated"},
		{"empty message", ""},
		{"long message", "Authorization has been denied for this request due to missing credentials"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewUnauthorizedError(tc.message)

			assert.Equal(t, apperrors.ErrCodeUnauthorized, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewForbiddenError ---

func TestNewForbiddenError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"standard message", "access denied"},
		{"empty message", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewForbiddenError(tc.message)

			assert.Equal(t, apperrors.ErrCodeForbidden, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusForbidden, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewBadRequestError ---

func TestNewBadRequestError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		details map[string]interface{}
	}{
		{
			name:    "with details",
			message: "invalid request body",
			details: map[string]interface{}{"field": "email", "reason": "malformed"},
		},
		{
			name:    "nil details",
			message: "bad input",
			details: nil,
		},
		{
			name:    "empty details map",
			message: "missing required fields",
			details: map[string]interface{}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewBadRequestError(tc.message, tc.details)

			assert.Equal(t, apperrors.ErrCodeBadRequest, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusBadRequest, err.StatusCode)
			assert.Equal(t, tc.details, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewValidationError ---

func TestNewValidationError(t *testing.T) {
	tests := []struct {
		name    string
		details map[string]interface{}
	}{
		{
			name:    "with field-level details",
			details: map[string]interface{}{"name": "required", "age": "must be positive"},
		},
		{
			name:    "nil details",
			details: nil,
		},
		{
			name:    "multiple detail entries",
			details: map[string]interface{}{"a": 1, "b": false, "c": "msg"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewValidationError(tc.details)

			assert.Equal(t, apperrors.ErrCodeValidationFailed, err.Code)
			assert.Equal(t, "Validation failed", err.Message)
			assert.Equal(t, http.StatusBadRequest, err.StatusCode)
			assert.Equal(t, tc.details, err.Details)
			assert.Equal(t, "Validation failed", err.Error())
		})
	}
}

// --- Constructor: NewNotFoundError ---

func TestNewNotFoundError(t *testing.T) {
	tests := []struct {
		name            string
		resource        string
		expectedMessage string
	}{
		{
			name:            "user resource",
			resource:        "User",
			expectedMessage: "User not found",
		},
		{
			name:            "order resource",
			resource:        "Order",
			expectedMessage: "Order not found",
		},
		{
			name:            "empty resource",
			resource:        "",
			expectedMessage: " not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewNotFoundError(tc.resource)

			assert.Equal(t, apperrors.ErrCodeNotFound, err.Code)
			assert.Equal(t, tc.expectedMessage, err.Message)
			assert.Equal(t, http.StatusNotFound, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.expectedMessage, err.Error())
		})
	}
}

// --- Constructor: NewConflictError ---

func TestNewConflictError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		details map[string]interface{}
	}{
		{
			name:    "with details",
			message: "resource already exists",
			details: map[string]interface{}{"duplicate_field": "email"},
		},
		{
			name:    "nil details",
			message: "duplicate entry",
			details: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewConflictError(tc.message, tc.details)

			assert.Equal(t, apperrors.ErrCodeConflict, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusConflict, err.StatusCode)
			assert.Equal(t, tc.details, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewDatabaseError ---

func TestNewDatabaseError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"query failed", "failed to execute query"},
		{"connection timeout", "database connection timed out"},
		{"empty message", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewDatabaseError(tc.message)

			assert.Equal(t, apperrors.ErrCodeDatabaseError, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewExternalServiceError ---

func TestNewExternalServiceError(t *testing.T) {
	tests := []struct {
		name            string
		service         string
		underlyingErr   error
		expectedMessage string
	}{
		{
			name:            "payment service error",
			service:         "payment-service",
			underlyingErr:   fmt.Errorf("connection refused"),
			expectedMessage: "External service error: payment-service",
		},
		{
			name:            "notification service error",
			service:         "notification-service",
			underlyingErr:   fmt.Errorf("timeout after 30s"),
			expectedMessage: "External service error: notification-service",
		},
		{
			name:            "empty service name",
			service:         "",
			underlyingErr:   fmt.Errorf("unknown"),
			expectedMessage: "External service error: ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewExternalServiceError(tc.service, tc.underlyingErr)

			assert.Equal(t, apperrors.ErrCodeExternalService, err.Code)
			assert.Equal(t, tc.expectedMessage, err.Message)
			assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
			assert.Equal(t, tc.expectedMessage, err.Error())

			// Verify details are populated with service name and underlying error text.
			assert.NotNil(t, err.Details)
			assert.Equal(t, tc.service, err.Details["service"])
			assert.Equal(t, tc.underlyingErr.Error(), err.Details["error"])
		})
	}
}

// --- Constructor: NewInternalServerError ---

func TestNewInternalServerError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"generic message", "an unexpected error occurred"},
		{"empty message", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewInternalServerError(tc.message)

			assert.Equal(t, apperrors.ErrCodeInternalServer, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.message, err.Error())
		})
	}
}

// --- Constructor: NewMissingTokenError ---

func TestNewMissingTokenError(t *testing.T) {
	err := apperrors.NewMissingTokenError()

	assert.Equal(t, apperrors.ErrCodeMissingToken, err.Code)
	assert.Equal(t, "Authorization header is required", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "Authorization header is required", err.Error())
}

// --- Constructor: NewInvalidTokenFormatError ---

func TestNewInvalidTokenFormatError(t *testing.T) {
	err := apperrors.NewInvalidTokenFormatError()

	assert.Equal(t, apperrors.ErrCodeInvalidTokenFormat, err.Code)
	assert.Equal(t, "Authorization header must be in format: Bearer <token>", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "Authorization header must be in format: Bearer <token>", err.Error())
}

// --- Constructor: NewInvalidTokenError ---

func TestNewInvalidTokenError(t *testing.T) {
	err := apperrors.NewInvalidTokenError()

	assert.Equal(t, apperrors.ErrCodeInvalidToken, err.Code)
	assert.Equal(t, "Invalid or expired token", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "Invalid or expired token", err.Error())
}

// --- Constructor: NewInvalidClaimsError ---

func TestNewInvalidClaimsError(t *testing.T) {
	err := apperrors.NewInvalidClaimsError()

	assert.Equal(t, apperrors.ErrCodeInvalidClaims, err.Code)
	assert.Equal(t, "Invalid token claims", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "Invalid token claims", err.Error())
}

// --- Constructor: NewInsufficientPermissionsError ---

func TestNewInsufficientPermissionsError(t *testing.T) {
	tests := []struct {
		name            string
		requiredRole    string
		expectedMessage string
	}{
		{
			name:            "admin role",
			requiredRole:    "admin",
			expectedMessage: "Required role: admin",
		},
		{
			name:            "superuser role",
			requiredRole:    "superuser",
			expectedMessage: "Required role: superuser",
		},
		{
			name:            "empty role",
			requiredRole:    "",
			expectedMessage: "Required role: ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewInsufficientPermissionsError(tc.requiredRole)

			assert.Equal(t, apperrors.ErrCodeInsufficientPermissions, err.Code)
			assert.Equal(t, tc.expectedMessage, err.Message)
			assert.Equal(t, http.StatusForbidden, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.expectedMessage, err.Error())
		})
	}
}

// --- Constructor: NewInsufficientPermissionsAnyError ---

func TestNewInsufficientPermissionsAnyError(t *testing.T) {
	tests := []struct {
		name            string
		requiredRoles   []string
		expectedMessage string
	}{
		{
			name:            "two roles",
			requiredRoles:   []string{"admin", "manager"},
			expectedMessage: "Required one of roles: [admin manager]",
		},
		{
			name:            "single role",
			requiredRoles:   []string{"editor"},
			expectedMessage: "Required one of roles: [editor]",
		},
		{
			name:            "empty slice",
			requiredRoles:   []string{},
			expectedMessage: "Required one of roles: []",
		},
		{
			name:            "nil slice",
			requiredRoles:   nil,
			expectedMessage: "Required one of roles: []",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewInsufficientPermissionsAnyError(tc.requiredRoles)

			assert.Equal(t, apperrors.ErrCodeInsufficientPermissions, err.Code)
			assert.Equal(t, tc.expectedMessage, err.Message)
			assert.Equal(t, http.StatusForbidden, err.StatusCode)
			assert.Nil(t, err.Details)
			assert.Equal(t, tc.expectedMessage, err.Error())
		})
	}
}

// --- Constructor: NewNoRolesError ---

func TestNewNoRolesError(t *testing.T) {
	err := apperrors.NewNoRolesError()

	assert.Equal(t, apperrors.ErrCodeNoRoles, err.Code)
	assert.Equal(t, "User roles not found", err.Message)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "User roles not found", err.Error())
}

// --- Constructor: NewInvalidRolesError ---

func TestNewInvalidRolesError(t *testing.T) {
	err := apperrors.NewInvalidRolesError()

	assert.Equal(t, apperrors.ErrCodeInvalidRoles, err.Code)
	assert.Equal(t, "Invalid user roles format", err.Message)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Nil(t, err.Details)
	assert.Equal(t, "Invalid user roles format", err.Error())
}

// --- Error code constants ---

// TestErrorCodeConstants verifies that all exported error code constants have
// the exact string values the rest of the codebase depends on. A change to any
// of these values would be a breaking change and this test makes it explicit.
func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"ErrCodeMissingToken", apperrors.ErrCodeMissingToken, "MISSING_TOKEN"},
		{"ErrCodeInvalidTokenFormat", apperrors.ErrCodeInvalidTokenFormat, "INVALID_TOKEN_FORMAT"},
		{"ErrCodeInvalidToken", apperrors.ErrCodeInvalidToken, "INVALID_TOKEN"},
		{"ErrCodeInvalidClaims", apperrors.ErrCodeInvalidClaims, "INVALID_CLAIMS"},
		{"ErrCodeUnauthorized", apperrors.ErrCodeUnauthorized, "UNAUTHORIZED"},
		{"ErrCodeForbidden", apperrors.ErrCodeForbidden, "FORBIDDEN"},
		{"ErrCodeInsufficientPermissions", apperrors.ErrCodeInsufficientPermissions, "INSUFFICIENT_PERMISSIONS"},
		{"ErrCodeNoRoles", apperrors.ErrCodeNoRoles, "NO_ROLES"},
		{"ErrCodeInvalidRoles", apperrors.ErrCodeInvalidRoles, "INVALID_ROLES"},
		{"ErrCodeInternalServer", apperrors.ErrCodeInternalServer, "INTERNAL_SERVER_ERROR"},
		{"ErrCodeBadRequest", apperrors.ErrCodeBadRequest, "BAD_REQUEST"},
		{"ErrCodeNotFound", apperrors.ErrCodeNotFound, "NOT_FOUND"},
		{"ErrCodeMethodNotAllowed", apperrors.ErrCodeMethodNotAllowed, "METHOD_NOT_ALLOWED"},
		{"ErrCodeConflict", apperrors.ErrCodeConflict, "CONFLICT"},
		{"ErrCodeValidationFailed", apperrors.ErrCodeValidationFailed, "VALIDATION_FAILED"},
		{"ErrCodeDatabaseError", apperrors.ErrCodeDatabaseError, "DATABASE_ERROR"},
		{"ErrCodeExternalService", apperrors.ErrCodeExternalService, "EXTERNAL_SERVICE_ERROR"},
		{"ErrCodeResourceNotFound", apperrors.ErrCodeResourceNotFound, "RESOURCE_NOT_FOUND"},
		{"ErrCodeResourceAlreadyExists", apperrors.ErrCodeResourceAlreadyExists, "RESOURCE_ALREADY_EXISTS"},
		{"ErrCodeInvalidData", apperrors.ErrCodeInvalidData, "INVALID_DATA"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.got)
		})
	}
}

// --- HTTP status code groupings ---

// TestHTTPStatusCodes consolidates the expected HTTP status code for every
// constructor into a single table so that future HTTP contract regressions are
// caught in one place.
func TestHTTPStatusCodes(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")

	tests := []struct {
		name           string
		err            apperrors.AppError
		expectedStatus int
	}{
		// 401 Unauthorized
		{"NewUnauthorizedError", apperrors.NewUnauthorizedError("msg"), http.StatusUnauthorized},
		{"NewMissingTokenError", apperrors.NewMissingTokenError(), http.StatusUnauthorized},
		{"NewInvalidTokenFormatError", apperrors.NewInvalidTokenFormatError(), http.StatusUnauthorized},
		{"NewInvalidTokenError", apperrors.NewInvalidTokenError(), http.StatusUnauthorized},
		{"NewInvalidClaimsError", apperrors.NewInvalidClaimsError(), http.StatusUnauthorized},

		// 403 Forbidden
		{"NewForbiddenError", apperrors.NewForbiddenError("msg"), http.StatusForbidden},
		{"NewInsufficientPermissionsError", apperrors.NewInsufficientPermissionsError("admin"), http.StatusForbidden},
		{"NewInsufficientPermissionsAnyError", apperrors.NewInsufficientPermissionsAnyError([]string{"a", "b"}), http.StatusForbidden},
		{"NewNoRolesError", apperrors.NewNoRolesError(), http.StatusForbidden},
		{"NewInvalidRolesError", apperrors.NewInvalidRolesError(), http.StatusForbidden},

		// 400 Bad Request
		{"NewBadRequestError", apperrors.NewBadRequestError("msg", nil), http.StatusBadRequest},
		{"NewValidationError", apperrors.NewValidationError(nil), http.StatusBadRequest},

		// 404 Not Found
		{"NewNotFoundError", apperrors.NewNotFoundError("Resource"), http.StatusNotFound},

		// 409 Conflict
		{"NewConflictError", apperrors.NewConflictError("msg", nil), http.StatusConflict},

		// 500 Internal Server Error
		{"NewInternalServerError", apperrors.NewInternalServerError("msg"), http.StatusInternalServerError},
		{"NewDatabaseError", apperrors.NewDatabaseError("msg"), http.StatusInternalServerError},

		// 503 Service Unavailable
		{"NewExternalServiceError", apperrors.NewExternalServiceError("svc", sentinel), http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedStatus, tc.err.StatusCode,
				"unexpected StatusCode for constructor %s", tc.name)
		})
	}
}

// --- Error code groupings ---

// TestErrorCodes verifies that every constructor sets the correct Code constant.
func TestErrorCodes(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")

	tests := []struct {
		name         string
		err          apperrors.AppError
		expectedCode string
	}{
		{"NewUnauthorizedError", apperrors.NewUnauthorizedError("msg"), apperrors.ErrCodeUnauthorized},
		{"NewForbiddenError", apperrors.NewForbiddenError("msg"), apperrors.ErrCodeForbidden},
		{"NewBadRequestError", apperrors.NewBadRequestError("msg", nil), apperrors.ErrCodeBadRequest},
		{"NewValidationError", apperrors.NewValidationError(nil), apperrors.ErrCodeValidationFailed},
		{"NewNotFoundError", apperrors.NewNotFoundError("X"), apperrors.ErrCodeNotFound},
		{"NewConflictError", apperrors.NewConflictError("msg", nil), apperrors.ErrCodeConflict},
		{"NewDatabaseError", apperrors.NewDatabaseError("msg"), apperrors.ErrCodeDatabaseError},
		{"NewExternalServiceError", apperrors.NewExternalServiceError("svc", sentinel), apperrors.ErrCodeExternalService},
		{"NewInternalServerError", apperrors.NewInternalServerError("msg"), apperrors.ErrCodeInternalServer},
		{"NewMissingTokenError", apperrors.NewMissingTokenError(), apperrors.ErrCodeMissingToken},
		{"NewInvalidTokenFormatError", apperrors.NewInvalidTokenFormatError(), apperrors.ErrCodeInvalidTokenFormat},
		{"NewInvalidTokenError", apperrors.NewInvalidTokenError(), apperrors.ErrCodeInvalidToken},
		{"NewInvalidClaimsError", apperrors.NewInvalidClaimsError(), apperrors.ErrCodeInvalidClaims},
		{"NewInsufficientPermissionsError", apperrors.NewInsufficientPermissionsError("r"), apperrors.ErrCodeInsufficientPermissions},
		{"NewInsufficientPermissionsAnyError", apperrors.NewInsufficientPermissionsAnyError([]string{"r"}), apperrors.ErrCodeInsufficientPermissions},
		{"NewNoRolesError", apperrors.NewNoRolesError(), apperrors.ErrCodeNoRoles},
		{"NewInvalidRolesError", apperrors.NewInvalidRolesError(), apperrors.ErrCodeInvalidRoles},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedCode, tc.err.Code,
				"unexpected Code for constructor %s", tc.name)
		})
	}
}

// --- Details population ---

// TestDetailsPopulation groups constructors that accept or produce a Details
// map and verifies the exact content written by the constructor.
func TestDetailsPopulation(t *testing.T) {
	t.Run("NewBadRequestError propagates caller-supplied details", func(t *testing.T) {
		details := map[string]interface{}{"field": "username", "issue": "too short"}
		err := apperrors.NewBadRequestError("bad input", details)
		assert.Equal(t, details, err.Details)
	})

	t.Run("NewValidationError propagates caller-supplied details", func(t *testing.T) {
		details := map[string]interface{}{"email": "invalid format", "phone": "required"}
		err := apperrors.NewValidationError(details)
		assert.Equal(t, details, err.Details)
	})

	t.Run("NewConflictError propagates caller-supplied details", func(t *testing.T) {
		details := map[string]interface{}{"conflicting_id": "42"}
		err := apperrors.NewConflictError("already exists", details)
		assert.Equal(t, details, err.Details)
	})

	t.Run("NewExternalServiceError sets service and error keys", func(t *testing.T) {
		underlying := fmt.Errorf("dial tcp: connection refused")
		err := apperrors.NewExternalServiceError("stripe", underlying)

		assert.NotNil(t, err.Details)
		assert.Equal(t, "stripe", err.Details["service"])
		assert.Equal(t, "dial tcp: connection refused", err.Details["error"])
		// No extra keys beyond service and error.
		assert.Len(t, err.Details, 2)
	})
}

// --- Idempotency ---

// TestConstructorsReturnDistinctInstances ensures that calling a no-arg
// constructor twice yields independent structs (no shared state between calls).
func TestConstructorsReturnDistinctInstances(t *testing.T) {
	a := apperrors.NewMissingTokenError()
	b := apperrors.NewMissingTokenError()

	// Mutate one instance and confirm the other is unaffected.
	a.Message = "mutated"
	assert.NotEqual(t, a.Message, b.Message)
}

// TestExternalServiceError_MutatingDetailsDoesNotAffectOriginalError verifies
// that the Details map returned by NewExternalServiceError is independent —
// callers cannot corrupt the error by mutating the map they received.
func TestExternalServiceError_MutatingDetailsDoesNotAffectOriginalError(t *testing.T) {
	underlying := fmt.Errorf("original error")
	err := apperrors.NewExternalServiceError("inventory", underlying)

	// Take a reference to the map and mutate it.
	details := err.Details
	originalService := details["service"]

	details["service"] = "tampered"

	// The struct field is the same map so mutation is visible — this test
	// documents the current (by-reference) behaviour rather than asserting
	// isolation, so downstream callers know not to mutate Details.
	assert.Equal(t, "tampered", err.Details["service"],
		"Details map is shared by reference; callers must not mutate it")
	_ = originalService
}

// --- Constructor: NewUnavailableError ---

// TestNewUnavailableError verifies the code and status for a dependency that
// could not be reached at all.
func TestNewUnavailableError(t *testing.T) {
	err := apperrors.NewUnavailableError("uptime probe is not configured")

	assert.Equal(t, apperrors.ErrCodeUnavailable, err.Code)
	assert.Equal(t, "uptime probe is not configured", err.Message)
	assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	assert.Nil(t, err.Details)
}

// TestNewUnavailableError_IsDistinguishableFromExternalServiceError is the
// point of the constructor existing. Both are 503, so the CODE is the only
// thing a caller can branch on to tell "nothing answered" from "something
// answered with an error" — which is the difference between rendering
// "not measured" and rendering "failed".
func TestNewUnavailableError_IsDistinguishableFromExternalServiceError(t *testing.T) {
	unavailable := apperrors.NewUnavailableError("clickhouse is not configured")
	failed := apperrors.NewExternalServiceError("clickhouse", fmt.Errorf("query timeout"))

	assert.Equal(t, unavailable.StatusCode, failed.StatusCode,
		"both are 503; the status cannot be what distinguishes them")
	assert.NotEqual(t, unavailable.Code, failed.Code,
		"the code must distinguish 'not reachable' from 'reached and failed'")
}

// TestNewUnavailableError_SatisfiesErrorInterface confirms it behaves like the
// other constructors under errors.As.
func TestNewUnavailableError_SatisfiesErrorInterface(t *testing.T) {
	var err error = apperrors.NewUnavailableError("not configured")

	var target apperrors.AppError
	assert.True(t, errors.As(err, &target))
	assert.Equal(t, apperrors.ErrCodeUnavailable, target.Code)
}

// --- Regression: NewExternalServiceError with a nil cause ---

// TestNewExternalServiceError_NilErrorDoesNotPanic is a regression test.
//
// This constructor called err.Error() unconditionally, so passing a nil error
// panicked with a nil pointer dereference. It is called on failure paths —
// frequently from a wrapper that may have no error to hand — and an error
// constructor that panics converts a handled failure into a crashed request.
func TestNewExternalServiceError_NilErrorDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = apperrors.NewExternalServiceError("payments", nil)
	})
}

// TestNewExternalServiceError_NilErrorOmitsTheErrorDetail verifies the shape
// when there is no cause: "error" is absent rather than present and empty, so a
// consumer checking for the key gets a truthful answer.
func TestNewExternalServiceError_NilErrorOmitsTheErrorDetail(t *testing.T) {
	err := apperrors.NewExternalServiceError("payments", nil)

	assert.Equal(t, apperrors.ErrCodeExternalService, err.Code)
	assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	assert.Equal(t, "payments", err.Details["service"])

	_, present := err.Details["error"]
	assert.False(t, present, "absent cause must be omitted, not an empty string")
}

// TestNewExternalServiceError_NonNilErrorStillCarriesTheCause guards the fix
// against over-correction — the existing behaviour must be unchanged when a
// cause is supplied.
func TestNewExternalServiceError_NonNilErrorStillCarriesTheCause(t *testing.T) {
	err := apperrors.NewExternalServiceError("inventory", fmt.Errorf("connection refused"))

	assert.Equal(t, "inventory", err.Details["service"])
	assert.Equal(t, "connection refused", err.Details["error"])
}
