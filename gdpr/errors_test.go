package gdpr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStandardErrorVariables verifies that all sentinel error variables are
// defined and distinct.
func TestStandardErrorVariables(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidUserID", ErrInvalidUserID},
		{"ErrInvalidPurpose", ErrInvalidPurpose},
		{"ErrConsentNotFound", ErrConsentNotFound},
		{"ErrConsentAlreadyWithdrawn", ErrConsentAlreadyWithdrawn},
		{"ErrConsentExpired", ErrConsentExpired},
		{"ErrInvalidRequestID", ErrInvalidRequestID},
		{"ErrRequestNotFound", ErrRequestNotFound},
		{"ErrDeletionRequestExists", ErrDeletionRequestExists},
		{"ErrInvalidRequestStatus", ErrInvalidRequestStatus},
		{"ErrDeletionNotDue", ErrDeletionNotDue},
		{"ErrCannotCancelRequest", ErrCannotCancelRequest},
		{"ErrDeletionNotFound", ErrDeletionNotFound},
		{"ErrInvalidDeletionID", ErrInvalidDeletionID},
		{"ErrInvalidDataCategory", ErrInvalidDataCategory},
		{"ErrInvalidRetentionPeriod", ErrInvalidRetentionPeriod},
		{"ErrInvalidPolicyID", ErrInvalidPolicyID},
		{"ErrPolicyNotFound", ErrPolicyNotFound},
		{"ErrExportFailed", ErrExportFailed},
		{"ErrNoDataFound", ErrNoDataFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.err, "sentinel error %s must not be nil", tc.name)
			assert.NotEmpty(t, tc.err.Error(), "sentinel error %s must have a non-empty message", tc.name)
		})
	}
}

// TestStandardErrorDistinctness verifies that every sentinel error is distinct
// so that callers can rely on errors.Is comparisons.
func TestStandardErrorDistinctness(t *testing.T) {
	allErrors := []error{
		ErrInvalidUserID,
		ErrInvalidPurpose,
		ErrConsentNotFound,
		ErrConsentAlreadyWithdrawn,
		ErrConsentExpired,
		ErrInvalidRequestID,
		ErrRequestNotFound,
		ErrDeletionRequestExists,
		ErrInvalidRequestStatus,
		ErrDeletionNotDue,
		ErrCannotCancelRequest,
		ErrDeletionNotFound,
		ErrInvalidDeletionID,
		ErrInvalidDataCategory,
		ErrInvalidRetentionPeriod,
		ErrInvalidPolicyID,
		ErrPolicyNotFound,
		ErrExportFailed,
		ErrNoDataFound,
	}

	for i := 0; i < len(allErrors); i++ {
		for j := i + 1; j < len(allErrors); j++ {
			assert.NotEqual(t, allErrors[i], allErrors[j],
				"errors at index %d and %d must be distinct", i, j)
		}
	}
}

// TestGDPRErrorCodes verifies that all error code constants are defined and
// have distinct non-empty string values.
func TestGDPRErrorCodes(t *testing.T) {
	codes := []struct {
		name  string
		value string
	}{
		{"ErrCodeGDPRInvalidRequest", ErrCodeGDPRInvalidRequest},
		{"ErrCodeGDPRUnauthorized", ErrCodeGDPRUnauthorized},
		{"ErrCodeGDPRNotFound", ErrCodeGDPRNotFound},
		{"ErrCodeGDPRConsentRequired", ErrCodeGDPRConsentRequired},
		{"ErrCodeGDPRConsentExpired", ErrCodeGDPRConsentExpired},
		{"ErrCodeGDPRDeletionPending", ErrCodeGDPRDeletionPending},
		{"ErrCodeGDPRDeletionFailed", ErrCodeGDPRDeletionFailed},
		{"ErrCodeGDPRExportFailed", ErrCodeGDPRExportFailed},
		{"ErrCodeGDPRPolicyViolation", ErrCodeGDPRPolicyViolation},
		{"ErrCodeGDPRRetentionViolation", ErrCodeGDPRRetentionViolation},
		{"ErrCodeGDPRInternalError", ErrCodeGDPRInternalError},
	}

	seen := make(map[string]bool)
	for _, c := range codes {
		t.Run(c.name, func(t *testing.T) {
			assert.NotEmpty(t, c.value)
			assert.False(t, seen[c.value], "error code %q must be unique", c.value)
			seen[c.value] = true
		})
	}
}

// TestGDPRError_ErrorMethod tests the Error() method formatting with and
// without an underlying wrapped error.
func TestGDPRError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name        string
		gdprErr     *GDPRError
		wantContain string
	}{
		{
			name: "without wrapped error",
			gdprErr: &GDPRError{
				Code:       ErrCodeGDPRNotFound,
				Message:    "resource not found",
				StatusCode: http.StatusNotFound,
			},
			wantContain: "resource not found",
		},
		{
			name: "with wrapped error",
			gdprErr: &GDPRError{
				Code:       ErrCodeGDPRInternalError,
				Message:    "processing failed",
				StatusCode: http.StatusInternalServerError,
				Err:        errors.New("database connection refused"),
			},
			wantContain: "processing failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.gdprErr.Error(), tc.wantContain)
		})
	}
}

// TestGDPRError_ErrorWithWrappedContainsUnderlyingMessage verifies that when
// a wrapped error is present its message appears in the full error string.
func TestGDPRError_ErrorWithWrappedContainsUnderlyingMessage(t *testing.T) {
	underlying := errors.New("disk full")
	gdprErr := &GDPRError{
		Message: "export failed",
		Err:     underlying,
	}
	assert.Contains(t, gdprErr.Error(), "export failed")
	assert.Contains(t, gdprErr.Error(), "disk full")
}

// TestGDPRError_Unwrap verifies that Unwrap returns the underlying error so
// that errors.Is and errors.As chains work correctly.
func TestGDPRError_Unwrap(t *testing.T) {
	underlying := errors.New("root cause")
	gdprErr := &GDPRError{
		Message: "outer",
		Err:     underlying,
	}

	assert.True(t, errors.Is(gdprErr, underlying), "errors.Is must traverse Unwrap chain")
	assert.Equal(t, underlying, gdprErr.Unwrap())
}

// TestGDPRError_UnwrapNil verifies that Unwrap returns nil when there is no
// underlying error.
func TestGDPRError_UnwrapNil(t *testing.T) {
	gdprErr := &GDPRError{Message: "standalone error"}
	assert.Nil(t, gdprErr.Unwrap())
}

// TestGDPRError_WithDetails verifies the WithDetails chaining method.
func TestGDPRError_WithDetails(t *testing.T) {
	gdprErr := &GDPRError{Code: ErrCodeGDPRInvalidRequest, Message: "bad request"}
	details := map[string]interface{}{"field": "email", "reason": "invalid format"}

	result := gdprErr.WithDetails(details)

	// WithDetails must return the same pointer for chaining.
	assert.Same(t, gdprErr, result)
	assert.Equal(t, "email", gdprErr.Details["field"])
	assert.Equal(t, "invalid format", gdprErr.Details["reason"])
}

// TestNewGDPRError verifies that NewGDPRError correctly sets all fields.
func TestNewGDPRError(t *testing.T) {
	err := NewGDPRError(ErrCodeGDPRNotFound, "item not found", http.StatusNotFound)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRNotFound, err.Code)
	assert.Equal(t, "item not found", err.Message)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
}

// TestNewInvalidRequestError verifies the invalid request constructor.
func TestNewInvalidRequestError(t *testing.T) {
	err := NewInvalidRequestError("user_id is required")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRInvalidRequest, err.Code)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Contains(t, err.Message, "user_id is required")
}

// TestNewUnauthorizedError verifies the unauthorized constructor.
func TestNewUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("not authenticated")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRUnauthorized, err.Code)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
}

// TestNewNotFoundError verifies the not found constructor includes the resource
// name in the message.
func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("ConsentRecord")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRNotFound, err.Code)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Contains(t, err.Message, "ConsentRecord")
	assert.Contains(t, err.Message, "not found")
}

// TestNewConsentRequiredError verifies the consent required constructor
// embeds the purpose in both message and details.
func TestNewConsentRequiredError(t *testing.T) {
	err := NewConsentRequiredError(ConsentPurposeMarketing)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRConsentRequired, err.Code)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Contains(t, err.Message, string(ConsentPurposeMarketing))
	assert.Equal(t, ConsentPurposeMarketing, err.Details["purpose"])
}

// TestNewConsentExpiredError verifies the consent expired constructor.
func TestNewConsentExpiredError(t *testing.T) {
	err := NewConsentExpiredError(ConsentPurposeAnalytics)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRConsentExpired, err.Code)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Contains(t, err.Message, string(ConsentPurposeAnalytics))
	assert.Equal(t, ConsentPurposeAnalytics, err.Details["purpose"])
}

// TestNewDeletionPendingError verifies the deletion pending constructor
// embeds the user ID in the details.
func TestNewDeletionPendingError(t *testing.T) {
	err := NewDeletionPendingError("user-123")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRDeletionPending, err.Code)
	assert.Equal(t, http.StatusConflict, err.StatusCode)
	assert.Equal(t, "user-123", err.Details["user_id"])
}

// TestNewDeletionFailedError verifies the deletion failed constructor wraps
// the underlying error and uses 500 status.
func TestNewDeletionFailedError(t *testing.T) {
	underlying := errors.New("foreign key constraint")
	err := NewDeletionFailedError(underlying)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRDeletionFailed, err.Code)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, underlying, err.Err)
}

// TestNewExportFailedError verifies the export failed constructor.
func TestNewExportFailedError(t *testing.T) {
	underlying := errors.New("zip writer closed")
	err := NewExportFailedError(underlying)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRExportFailed, err.Code)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, underlying, err.Err)
}

// TestNewPolicyViolationError verifies the policy violation constructor
// includes both policy and violation in details.
func TestNewPolicyViolationError(t *testing.T) {
	err := NewPolicyViolationError("retention-policy-A", "data retained beyond limit")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRPolicyViolation, err.Code)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Equal(t, "retention-policy-A", err.Details["policy"])
	assert.Equal(t, "data retained beyond limit", err.Details["violation"])
	assert.Contains(t, err.Message, "data retained beyond limit")
}

// TestNewRetentionViolationError verifies the retention violation constructor
// includes the data category in details.
func TestNewRetentionViolationError(t *testing.T) {
	err := NewRetentionViolationError(DataCategoryFinancial, "data older than allowed period")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRRetentionViolation, err.Code)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, DataCategoryFinancial, err.Details["data_category"])
}

// TestNewInternalError verifies the internal error constructor wraps the cause.
func TestNewInternalError(t *testing.T) {
	cause := errors.New("goroutine panic")
	err := NewInternalError(cause)
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeGDPRInternalError, err.Code)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, cause, err.Err)
}

// TestIsGDPRError verifies the type guard helper.
func TestIsGDPRError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{"GDPRError value", NewNotFoundError("record"), true},
		// NewInternalError itself is a *GDPRError, so IsGDPRError returns true.
		{"GDPRError wrapping another GDPRError", NewInternalError(NewNotFoundError("record")), true},
		{"standard error", errors.New("plain error"), false},
		{"nil", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsGDPRError(tc.err))
		})
	}
}

// TestGetGDPRError_Extracts verifies extraction from a GDPRError.
func TestGetGDPRError_Extracts(t *testing.T) {
	original := NewNotFoundError("ConsentRecord")
	extracted := GetGDPRError(original)
	assert.Equal(t, original, extracted)
}

// TestGetGDPRError_ReturnsNilForStandardError verifies nil is returned for
// non-GDPR errors.
func TestGetGDPRError_ReturnsNilForStandardError(t *testing.T) {
	assert.Nil(t, GetGDPRError(errors.New("plain")))
	assert.Nil(t, GetGDPRError(nil))
}

// TestWrapError verifies that WrapError produces a GDPRError with the supplied
// code, message, and wrapped error.
func TestWrapError(t *testing.T) {
	cause := errors.New("root")
	wrapped := WrapError(cause, ErrCodeGDPRInternalError, "wrapping message")
	require.NotNil(t, wrapped)
	assert.Equal(t, ErrCodeGDPRInternalError, wrapped.Code)
	assert.Equal(t, "wrapping message", wrapped.Message)
	assert.Equal(t, cause, wrapped.Err)
	assert.Equal(t, http.StatusInternalServerError, wrapped.StatusCode)
	// errors.Is must traverse through Unwrap.
	assert.True(t, errors.Is(wrapped, cause))
}
