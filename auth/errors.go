package auth

import (
	"errors"
	"fmt"
)

// Configuration errors
var (
	ErrNoIssuersConfigured = errors.New("auth: no issuers configured")
)

// Token validation errors
var (
	ErrTokenMissing       = errors.New("auth: authorization token is required")
	ErrTokenMalformed     = errors.New("auth: token is malformed")
	ErrTokenInvalidFormat = errors.New("auth: authorization header must be 'Bearer {token}'")
	ErrTokenExpired       = errors.New("auth: token has expired")
	ErrTokenNotYetValid   = errors.New("auth: token is not yet valid")
	ErrTokenInvalidIssuer = errors.New("auth: token issuer is not allowed")
	ErrTokenInvalidSig    = errors.New("auth: token signature is invalid")
	ErrTokenInvalidClaims = errors.New("auth: token claims are invalid")
)

// Authorization errors
var (
	ErrNoRolesInToken         = errors.New("auth: no roles found in token")
	ErrInsufficientPermission = errors.New("auth: insufficient permissions")
	ErrTenantMismatch         = errors.New("auth: tenant ID mismatch")
)

// TokenValidationError provides detailed error information
type TokenValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Issuer  string `json:"issuer,omitempty"`
	Cause   error  `json:"-"`
}

func (e *TokenValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *TokenValidationError) Unwrap() error {
	return e.Cause
}

// Error constructors for consistent error creation

func NewTokenMissingError() *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_MISSING",
		Message: "Authorization token is required",
		Cause:   ErrTokenMissing,
	}
}

func NewTokenMalformedError(cause error) *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_MALFORMED",
		Message: "Token format is invalid",
		Cause:   cause,
	}
}

func NewTokenInvalidFormatError() *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_INVALID_FORMAT",
		Message: "Authorization header must be 'Bearer {token}'",
		Cause:   ErrTokenInvalidFormat,
	}
}

func NewTokenExpiredError() *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_EXPIRED",
		Message: "Token has expired",
		Cause:   ErrTokenExpired,
	}
}

func NewTokenNotYetValidError() *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_NOT_YET_VALID",
		Message: "Token is not yet valid",
		Cause:   ErrTokenNotYetValid,
	}
}

func NewTokenInvalidIssuerError(issuer string) *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_INVALID_ISSUER",
		Message: fmt.Sprintf("Issuer '%s' is not allowed", issuer),
		Issuer:  issuer,
		Cause:   ErrTokenInvalidIssuer,
	}
}

func NewTokenInvalidSignatureError(cause error) *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_INVALID_SIGNATURE",
		Message: "Token signature verification failed",
		Cause:   cause,
	}
}

func NewTokenInvalidClaimsError(cause error) *TokenValidationError {
	return &TokenValidationError{
		Code:    "TOKEN_INVALID_CLAIMS",
		Message: "Token claims are invalid or missing required fields",
		Cause:   cause,
	}
}


func NewInsufficientPermissionError(required string) *TokenValidationError {
	return &TokenValidationError{
		Code:    "INSUFFICIENT_PERMISSION",
		Message: fmt.Sprintf("Required permission: %s", required),
		Cause:   ErrInsufficientPermission,
	}
}

func NewTenantMismatchError(tokenTenant, requestTenant string) *TokenValidationError {
	return &TokenValidationError{
		Code:    "TENANT_MISMATCH",
		Message: "Token tenant does not match request tenant",
		Cause:   ErrTenantMismatch,
	}
}

// IsTokenExpiredError checks if error is due to token expiration
func IsTokenExpiredError(err error) bool {
	var tokenErr *TokenValidationError
	if errors.As(err, &tokenErr) {
		return tokenErr.Code == "TOKEN_EXPIRED"
	}
	return errors.Is(err, ErrTokenExpired)
}

// IsAuthError checks if error is an authentication error
func IsAuthError(err error) bool {
	var tokenErr *TokenValidationError
	return errors.As(err, &tokenErr)
}

// GetErrorCode extracts error code from TokenValidationError
func GetErrorCode(err error) string {
	var tokenErr *TokenValidationError
	if errors.As(err, &tokenErr) {
		return tokenErr.Code
	}
	return "UNKNOWN_ERROR"
}
