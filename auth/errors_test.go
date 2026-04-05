package auth

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----------------------------------------------------------------------------
// Sentinel error identity
// ----------------------------------------------------------------------------

// TestSentinelErrors_Identity verifies that all exported sentinel errors are
// distinct values and are not accidentally aliased to one another.
func TestSentinelErrors_Identity(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNoIssuersConfigured", ErrNoIssuersConfigured},
		{"ErrTokenMissing", ErrTokenMissing},
		{"ErrTokenMalformed", ErrTokenMalformed},
		{"ErrTokenInvalidFormat", ErrTokenInvalidFormat},
		{"ErrTokenExpired", ErrTokenExpired},
		{"ErrTokenNotYetValid", ErrTokenNotYetValid},
		{"ErrTokenInvalidIssuer", ErrTokenInvalidIssuer},
		{"ErrTokenInvalidSig", ErrTokenInvalidSig},
		{"ErrTokenInvalidClaims", ErrTokenInvalidClaims},
		{"ErrNoRolesInToken", ErrNoRolesInToken},
		{"ErrInsufficientPermission", ErrInsufficientPermission},
		{"ErrTenantMismatch", ErrTenantMismatch},
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			assert.False(t, errors.Is(a.err, b.err),
				"%s must not match %s via errors.Is", a.name, b.name)
		}
	}
}

// ----------------------------------------------------------------------------
// TokenValidationError — Error() method
// ----------------------------------------------------------------------------

func TestTokenValidationError_Error_WithoutCause(t *testing.T) {
	e := &TokenValidationError{
		Code:    "MY_CODE",
		Message: "something went wrong",
	}
	assert.Equal(t, "MY_CODE: something went wrong", e.Error())
}

func TestTokenValidationError_Error_WithCause(t *testing.T) {
	cause := errors.New("underlying io error")
	e := &TokenValidationError{
		Code:    "MY_CODE",
		Message: "something went wrong",
		Cause:   cause,
	}
	got := e.Error()
	assert.Contains(t, got, "MY_CODE")
	assert.Contains(t, got, "something went wrong")
	assert.Contains(t, got, "underlying io error")
}

func TestTokenValidationError_Unwrap(t *testing.T) {
	cause := errors.New("the root cause")
	e := &TokenValidationError{
		Code:  "X",
		Cause: cause,
	}

	// errors.Is traverses Unwrap chains.
	assert.True(t, errors.Is(e, cause),
		"errors.Is must find the cause through Unwrap")
}

func TestTokenValidationError_Unwrap_NilCause(t *testing.T) {
	e := &TokenValidationError{Code: "X"}
	assert.Nil(t, e.Unwrap(), "Unwrap must return nil when Cause is nil")
}

// ----------------------------------------------------------------------------
// Constructor — field values
// ----------------------------------------------------------------------------

func TestNewTokenMissingError(t *testing.T) {
	err := NewTokenMissingError()
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_MISSING", err.Code)
	assert.Equal(t, "Authorization token is required", err.Message)
	assert.True(t, errors.Is(err, ErrTokenMissing))
}

func TestNewTokenMalformedError(t *testing.T) {
	cause := errors.New("bad base64")
	err := NewTokenMalformedError(cause)
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_MALFORMED", err.Code)
	assert.True(t, errors.Is(err, cause))
}

func TestNewTokenInvalidFormatError(t *testing.T) {
	err := NewTokenInvalidFormatError()
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_INVALID_FORMAT", err.Code)
	assert.True(t, errors.Is(err, ErrTokenInvalidFormat))
}

func TestNewTokenExpiredError(t *testing.T) {
	err := NewTokenExpiredError()
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_EXPIRED", err.Code)
	assert.True(t, errors.Is(err, ErrTokenExpired))
}

func TestNewTokenNotYetValidError(t *testing.T) {
	err := NewTokenNotYetValidError()
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_NOT_YET_VALID", err.Code)
	assert.True(t, errors.Is(err, ErrTokenNotYetValid))
}

func TestNewTokenInvalidIssuerError(t *testing.T) {
	const issuer = "https://evil.example.com"
	err := NewTokenInvalidIssuerError(issuer)
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_INVALID_ISSUER", err.Code)
	assert.Equal(t, issuer, err.Issuer)
	assert.Contains(t, err.Message, issuer)
	assert.True(t, errors.Is(err, ErrTokenInvalidIssuer))
}

func TestNewTokenInvalidSignatureError(t *testing.T) {
	cause := errors.New("rsa verification failed")
	err := NewTokenInvalidSignatureError(cause)
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_INVALID_SIGNATURE", err.Code)
	assert.True(t, errors.Is(err, cause))
}

func TestNewTokenInvalidClaimsError(t *testing.T) {
	cause := errors.New("missing sub claim")
	err := NewTokenInvalidClaimsError(cause)
	require.NotNil(t, err)
	assert.Equal(t, "TOKEN_INVALID_CLAIMS", err.Code)
	assert.True(t, errors.Is(err, cause))
}

func TestNewInsufficientPermissionError(t *testing.T) {
	const required = "admin:write"
	err := NewInsufficientPermissionError(required)
	require.NotNil(t, err)
	assert.Equal(t, "INSUFFICIENT_PERMISSION", err.Code)
	assert.Contains(t, err.Message, required)
	assert.True(t, errors.Is(err, ErrInsufficientPermission))
}

func TestNewTenantMismatchError(t *testing.T) {
	err := NewTenantMismatchError("tenant-a", "tenant-b")
	require.NotNil(t, err)
	assert.Equal(t, "TENANT_MISMATCH", err.Code)
	assert.True(t, errors.Is(err, ErrTenantMismatch))
}

// ----------------------------------------------------------------------------
// IsTokenExpiredError
// ----------------------------------------------------------------------------

func TestIsTokenExpiredError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "TokenValidationError with TOKEN_EXPIRED code",
			err:  NewTokenExpiredError(),
			want: true,
		},
		{
			name: "raw ErrTokenExpired sentinel",
			err:  ErrTokenExpired,
			want: true,
		},
		{
			name: "ErrTokenExpired wrapped with fmt.Errorf %w",
			err:  fmt.Errorf("outer: %w", ErrTokenExpired),
			want: true,
		},
		{
			name: "different TokenValidationError (not expired)",
			err:  NewTokenMissingError(),
			want: false,
		},
		{
			name: "unrelated plain error",
			err:  errors.New("some other error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsTokenExpiredError(tc.err))
		})
	}
}

// ----------------------------------------------------------------------------
// IsAuthError
// ----------------------------------------------------------------------------

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "TokenValidationError pointer",
			err:  NewTokenExpiredError(),
			want: true,
		},
		{
			name: "TokenValidationError wrapped by fmt.Errorf",
			err:  fmt.Errorf("wrap: %w", NewTokenMissingError()),
			want: true,
		},
		{
			name: "sentinel error (not TokenValidationError)",
			err:  ErrTokenExpired,
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("plain"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsAuthError(tc.err))
		})
	}
}

// ----------------------------------------------------------------------------
// GetErrorCode
// ----------------------------------------------------------------------------

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{
			name:     "TOKEN_MISSING",
			err:      NewTokenMissingError(),
			wantCode: "TOKEN_MISSING",
		},
		{
			name:     "TOKEN_EXPIRED",
			err:      NewTokenExpiredError(),
			wantCode: "TOKEN_EXPIRED",
		},
		{
			name:     "TOKEN_INVALID_ISSUER",
			err:      NewTokenInvalidIssuerError("bad-issuer"),
			wantCode: "TOKEN_INVALID_ISSUER",
		},
		{
			name:     "TokenValidationError wrapped by fmt.Errorf",
			err:      fmt.Errorf("outer: %w", NewTokenExpiredError()),
			wantCode: "TOKEN_EXPIRED",
		},
		{
			name:     "plain error returns UNKNOWN_ERROR",
			err:      errors.New("boom"),
			wantCode: "UNKNOWN_ERROR",
		},
		{
			name:     "nil returns UNKNOWN_ERROR",
			err:      nil,
			wantCode: "UNKNOWN_ERROR",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantCode, GetErrorCode(tc.err))
		})
	}
}

// ----------------------------------------------------------------------------
// errors.As / errors.Is through wrapping chains
// ----------------------------------------------------------------------------

// TestErrorsAs_ThroughWrapper ensures that wrapping a *TokenValidationError with
// fmt.Errorf still allows errors.As to extract it.
func TestErrorsAs_ThroughWrapper(t *testing.T) {
	original := NewTokenExpiredError()
	wrapped := fmt.Errorf("middleware: %w", original)

	var target *TokenValidationError
	require.True(t, errors.As(wrapped, &target))
	assert.Equal(t, "TOKEN_EXPIRED", target.Code)
}

// TestTokenValidationError_IssuerField confirms that issuer is only populated by
// constructors that carry issuer information.
func TestTokenValidationError_IssuerField(t *testing.T) {
	noIssuer := NewTokenExpiredError()
	assert.Empty(t, noIssuer.Issuer, "TOKEN_EXPIRED error must not set Issuer")

	withIssuer := NewTokenInvalidIssuerError("https://example.com")
	assert.Equal(t, "https://example.com", withIssuer.Issuer)
}
