package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ValidationErrors
// ---------------------------------------------------------------------------

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   ValidationErrors
		expected string
	}{
		{
			name:     "empty errors",
			errors:   ValidationErrors{},
			expected: "",
		},
		{
			name: "single error",
			errors: ValidationErrors{
				{Field: "email", Message: "Must be a valid email address"},
			},
			expected: "email: Must be a valid email address",
		},
		{
			name: "multiple errors joined with semicolon",
			errors: ValidationErrors{
				{Field: "email", Message: "Must be a valid email address"},
				{Field: "password", Message: "This field is required"},
			},
			expected: "email: Must be a valid email address; password: This field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.errors.Error())
		})
	}
}

func TestValidationErrors_ToMap(t *testing.T) {
	t.Run("empty errors produces empty map", func(t *testing.T) {
		ve := ValidationErrors{}
		m := ve.ToMap()
		assert.NotNil(t, m)
		assert.Empty(t, m)
	})

	t.Run("single error produces correct map entry", func(t *testing.T) {
		ve := ValidationErrors{
			{Field: "email", Value: "bad-email", Message: "Must be a valid email address", Code: "INVALID_EMAIL"},
		}
		m := ve.ToMap()
		require.Contains(t, m, "email")
		entry, ok := m["email"].(map[string]string)
		require.True(t, ok, "map entry should be map[string]string")
		assert.Equal(t, "Must be a valid email address", entry["message"])
		assert.Equal(t, "INVALID_EMAIL", entry["code"])
	})

	t.Run("multiple errors each appear under their field key", func(t *testing.T) {
		ve := ValidationErrors{
			{Field: "email", Message: "Must be a valid email address", Code: "INVALID_EMAIL"},
			{Field: "password", Message: "This field is required", Code: "REQUIRED"},
		}
		m := ve.ToMap()
		assert.Len(t, m, 2)
		assert.Contains(t, m, "email")
		assert.Contains(t, m, "password")
	})

	t.Run("last error wins when field is duplicated", func(t *testing.T) {
		// ToMap uses a plain map so duplicate fields overwrite; document this behaviour.
		ve := ValidationErrors{
			{Field: "name", Message: "first error", Code: "CODE_A"},
			{Field: "name", Message: "second error", Code: "CODE_B"},
		}
		m := ve.ToMap()
		assert.Len(t, m, 1)
		entry := m["name"].(map[string]string)
		assert.Equal(t, "second error", entry["message"])
		assert.Equal(t, "CODE_B", entry["code"])
	})
}

// ---------------------------------------------------------------------------
// Validator — constructor and state management
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	v := New()
	require.NotNil(t, v)
	assert.False(t, v.HasErrors())
	assert.Empty(t, v.GetErrors())
}

func TestValidator_AddError(t *testing.T) {
	v := New()
	v.AddError("field1", "val1", "some message", "SOME_CODE")

	assert.True(t, v.HasErrors())
	errs := v.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, "field1", errs[0].Field)
	assert.Equal(t, "val1", errs[0].Value)
	assert.Equal(t, "some message", errs[0].Message)
	assert.Equal(t, "SOME_CODE", errs[0].Code)
}

func TestValidator_HasErrors(t *testing.T) {
	v := New()
	assert.False(t, v.HasErrors(), "fresh validator should have no errors")

	v.AddError("f", "v", "m", "C")
	assert.True(t, v.HasErrors(), "validator with an error should report HasErrors true")
}

func TestValidator_GetErrors(t *testing.T) {
	v := New()
	assert.Empty(t, v.GetErrors())

	v.AddError("a", "1", "msg a", "CODE_A")
	v.AddError("b", "2", "msg b", "CODE_B")

	errs := v.GetErrors()
	assert.Len(t, errs, 2)
}

func TestValidator_Clear(t *testing.T) {
	v := New()
	v.AddError("f1", "v1", "m1", "C1")
	v.AddError("f2", "v2", "m2", "C2")
	require.True(t, v.HasErrors())

	v.Clear()
	assert.False(t, v.HasErrors())
	assert.Empty(t, v.GetErrors())
}

// ---------------------------------------------------------------------------
// Required
// ---------------------------------------------------------------------------

func TestValidator_Required(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		code        string
	}{
		{"non-empty value passes", "hello", false, ""},
		{"empty string fails", "", true, "REQUIRED"},
		{"whitespace-only string fails", "   ", true, "REQUIRED"},
		{"tab-only string fails", "\t", true, "REQUIRED"},
		{"newline-only string fails", "\n", true, "REQUIRED"},
		{"single space fails", " ", true, "REQUIRED"},
		{"zero string value fails", "0", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Required("field", tt.value)
			assert.Same(t, v, result, "Required should return the same validator for chaining")

			if tt.expectError {
				assert.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MinLength
// ---------------------------------------------------------------------------

func TestValidator_MinLength(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		min         int
		expectError bool
	}{
		{"value equals min passes", "abc", 3, false},
		{"value longer than min passes", "abcd", 3, false},
		{"value shorter than min fails", "ab", 3, true},
		{"empty value is shorter than min and fails", "", 1, true},
		{"min of zero always passes", "anything", 0, false},
		{"empty string with min zero passes", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.MinLength("field", tt.value, tt.min)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "MIN_LENGTH", v.GetErrors()[0].Code)
				assert.Contains(t, v.GetErrors()[0].Message, fmt.Sprintf("%d", tt.min))
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MaxLength
// ---------------------------------------------------------------------------

func TestValidator_MaxLength(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		max         int
		expectError bool
	}{
		{"value equals max passes", "abc", 3, false},
		{"value shorter than max passes", "ab", 3, false},
		{"value longer than max fails", "abcd", 3, true},
		{"empty value always passes", "", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.MaxLength("field", tt.value, tt.max)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "MAX_LENGTH", v.GetErrors()[0].Code)
				assert.Contains(t, v.GetErrors()[0].Message, fmt.Sprintf("%d", tt.max))
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Length
// ---------------------------------------------------------------------------

func TestValidator_Length(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		length      int
		expectError bool
	}{
		{"exact length passes", "abc", 3, false},
		{"shorter than required fails", "ab", 3, true},
		{"longer than required fails", "abcd", 3, true},
		{"empty value with length zero passes", "", 0, false},
		{"empty value with non-zero length fails", "", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Length("field", tt.value, tt.length)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "EXACT_LENGTH", v.GetErrors()[0].Code)
				assert.Contains(t, v.GetErrors()[0].Message, fmt.Sprintf("%d", tt.length))
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Email
// ---------------------------------------------------------------------------

func TestValidator_Email(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid simple email passes", "user@example.com", false},
		{"valid email with subdomain passes", "user@mail.example.com", false},
		{"valid email with plus tag passes", "user+tag@example.com", false},
		{"missing @ fails", "userexample.com", true},
		{"missing domain fails", "user@", true},
		{"missing local part fails", "@example.com", true},
		{"double @ fails", "user@@example.com", true},
		{"empty value passes (optional field)", "", false},
		{"plain text fails", "notanemail", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Email("email", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_EMAIL", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// URL
// ---------------------------------------------------------------------------

func TestValidator_URL(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid https URL passes", "https://example.com", false},
		{"valid http URL passes", "http://example.com", false},
		{"valid URL with path passes", "https://example.com/path/to/resource", false},
		{"valid URL with query passes", "https://example.com?foo=bar", false},
		{"missing scheme fails", "example.com", true},
		{"ftp scheme fails", "ftp://example.com", true},
		{"plain text fails", "not a url", true},
		{"empty value passes (optional field)", "", false},
		{"URL with port passes", "https://example.com:8080/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.URL("website", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_URL", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phone
// ---------------------------------------------------------------------------

func TestValidator_Phone(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"E.164 with plus passes", "+14155551234", false},
		{"digits only passes", "14155551234", false},
		{"formatted with dashes passes", "+1-415-555-1234", false},
		{"formatted with spaces passes", "+1 415 555 1234", false},
		{"Indian number passes", "+919876543210", false},
		{"starts with zero fails (E.164 requires non-zero first digit)", "+0123456789", true},
		{"empty value passes (optional field)", "", false},
		{"single digit fails", "1", true},
		{"too many digits fails", "+123456789012345678", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Phone("phone", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_PHONE", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Min (numeric)
// ---------------------------------------------------------------------------

func TestValidator_Min(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		min         float64
		expectError bool
		code        string
	}{
		{"value equals min passes", "5", 5, false, ""},
		{"value above min passes", "10", 5, false, ""},
		{"value below min fails", "3", 5, true, "MIN_VALUE"},
		{"negative value below positive min fails", "-1", 0, true, "MIN_VALUE"},
		{"float value above min passes", "5.5", 5, false, ""},
		{"float value below min fails", "4.9", 5, true, "MIN_VALUE"},
		{"non-numeric value fails with INVALID_NUMBER", "abc", 5, true, "INVALID_NUMBER"},
		{"empty value passes (optional field)", "", 5, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Min("amount", tt.value, tt.min)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Max (numeric)
// ---------------------------------------------------------------------------

func TestValidator_Max(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		max         float64
		expectError bool
		code        string
	}{
		{"value equals max passes", "5", 5, false, ""},
		{"value below max passes", "3", 5, false, ""},
		{"value above max fails", "6", 5, true, "MAX_VALUE"},
		{"float value above max fails", "5.1", 5, true, "MAX_VALUE"},
		{"non-numeric value fails with INVALID_NUMBER", "xyz", 5, true, "INVALID_NUMBER"},
		{"empty value passes (optional field)", "", 5, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Max("quantity", tt.value, tt.max)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integer
// ---------------------------------------------------------------------------

func TestValidator_Integer(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid positive integer passes", "42", false},
		{"zero passes", "0", false},
		{"negative integer passes", "-7", false},
		{"float fails", "3.14", true},
		{"text fails", "abc", true},
		{"integer with trailing chars fails", "42abc", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Integer("count", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_INTEGER", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Positive
// ---------------------------------------------------------------------------

func TestValidator_Positive(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		code        string
	}{
		{"positive integer passes", "1", false, ""},
		{"positive float passes", "0.001", false, ""},
		{"zero fails", "0", true, "NOT_POSITIVE"},
		{"negative number fails", "-5", true, "NOT_POSITIVE"},
		{"non-numeric fails with INVALID_NUMBER", "abc", true, "INVALID_NUMBER"},
		{"empty value passes (optional field)", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Positive("price", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pattern
// ---------------------------------------------------------------------------

func TestValidator_Pattern(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		pattern     string
		message     string
		expectError bool
		code        string
	}{
		{
			name:        "matching pattern passes",
			value:       "abc123",
			pattern:     `^[a-z0-9]+$`,
			message:     "must be lowercase alphanumeric",
			expectError: false,
		},
		{
			name:        "non-matching pattern fails with PATTERN_MISMATCH",
			value:       "ABC",
			pattern:     `^[a-z]+$`,
			message:     "must be lowercase",
			expectError: true,
			code:        "PATTERN_MISMATCH",
		},
		{
			name:        "invalid regex fails with INVALID_PATTERN",
			value:       "abc",
			pattern:     `[invalid`,
			message:     "irrelevant",
			expectError: true,
			code:        "INVALID_PATTERN",
		},
		{
			name:        "empty value passes (optional field)",
			value:       "",
			pattern:     `^[a-z]+$`,
			message:     "must be lowercase",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Pattern("field", tt.value, tt.pattern, tt.message)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
				if tt.code == "PATTERN_MISMATCH" {
					assert.Equal(t, tt.message, v.GetErrors()[0].Message)
				}
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AlphaNumeric
// ---------------------------------------------------------------------------

func TestValidator_AlphaNumeric(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"letters only passes", "abcXYZ", false},
		{"digits only passes", "12345", false},
		{"mixed letters and digits passes", "abc123XYZ", false},
		{"unicode letters pass", "café", false},
		{"contains hyphen fails", "abc-123", true},
		{"contains underscore fails", "abc_123", true},
		{"contains space fails", "abc 123", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.AlphaNumeric("username", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "NOT_ALPHANUMERIC", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Alpha
// ---------------------------------------------------------------------------

func TestValidator_Alpha(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"letters only passes", "abcXYZ", false},
		{"letters with space pass (space is allowed)", "John Doe", false},
		{"unicode letters pass", "René", false},
		{"contains digit fails", "abc1", true},
		{"contains hyphen fails", "O'Brien", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Alpha("first_name", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "NOT_ALPHA", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Date
// ---------------------------------------------------------------------------

func TestValidator_Date(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid date passes", "2024-01-15", false},
		{"leap day on leap year passes", "2024-02-29", false},
		{"invalid format fails (DD-MM-YYYY)", "15-01-2024", true},
		{"invalid format fails (MM/DD/YYYY)", "01/15/2024", true},
		{"non-existent date fails", "2023-02-29", true},
		{"invalid month fails", "2024-13-01", true},
		{"plain text fails", "not-a-date", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Date("dob", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_DATE", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DateTime
// ---------------------------------------------------------------------------

func TestValidator_DateTime(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid RFC3339 with UTC passes", "2024-01-15T10:30:00Z", false},
		{"valid RFC3339 with offset passes", "2024-01-15T10:30:00+05:30", false},
		{"date only (no time) fails", "2024-01-15", true},
		{"invalid format fails", "2024-01-15 10:30:00", true},
		{"plain text fails", "not-a-datetime", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.DateTime("created_at", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_DATETIME", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DateAfter
// ---------------------------------------------------------------------------

func TestValidator_DateAfter(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		after       string
		expectError bool
		code        string
	}{
		{
			name:        "date after reference passes",
			value:       "2024-06-01",
			after:       "2024-01-01",
			expectError: false,
		},
		{
			name:        "date equal to reference fails",
			value:       "2024-01-01",
			after:       "2024-01-01",
			expectError: true,
			code:        "DATE_NOT_AFTER",
		},
		{
			name:        "date before reference fails",
			value:       "2023-12-31",
			after:       "2024-01-01",
			expectError: true,
			code:        "DATE_NOT_AFTER",
		},
		{
			name:        "invalid value date fails",
			value:       "not-a-date",
			after:       "2024-01-01",
			expectError: true,
			code:        "INVALID_DATE",
		},
		{
			name:        "invalid after date fails",
			value:       "2024-06-01",
			after:       "not-a-date",
			expectError: true,
			code:        "INVALID_COMPARISON_DATE",
		},
		{
			name:        "empty value and empty after both skip validation",
			value:       "",
			after:       "",
			expectError: false,
		},
		{
			name:        "empty value with valid after skips validation",
			value:       "",
			after:       "2024-01-01",
			expectError: false,
		},
		{
			name:        "valid value with empty after skips validation",
			value:       "2024-06-01",
			after:       "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.DateAfter("end_date", tt.value, tt.after)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, tt.code, v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// In
// ---------------------------------------------------------------------------

func TestValidator_In(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		allowed     []string
		expectError bool
	}{
		{"value in list passes", "admin", []string{"admin", "staff", "viewer"}, false},
		{"value not in list fails", "superuser", []string{"admin", "staff", "viewer"}, true},
		{"single-item list match passes", "yes", []string{"yes"}, false},
		{"single-item list no match fails", "no", []string{"yes"}, true},
		{"empty value passes (optional field)", "", []string{"a", "b"}, false},
		{"case-sensitive: lowercase fails when only uppercase present", "admin", []string{"Admin"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.In("role", tt.value, tt.allowed)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "NOT_IN_LIST", v.GetErrors()[0].Code)
				// Error message lists all allowed values
				for _, a := range tt.allowed {
					assert.Contains(t, v.GetErrors()[0].Message, a)
				}
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NotIn
// ---------------------------------------------------------------------------

func TestValidator_NotIn(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		forbidden   []string
		expectError bool
	}{
		{"value not in forbidden list passes", "hello", []string{"admin", "root"}, false},
		{"value in forbidden list fails", "admin", []string{"admin", "root"}, true},
		{"empty value passes (optional field)", "", []string{"admin", "root"}, false},
		{"case-sensitive: lowercase passes when only uppercase forbidden", "admin", []string{"Admin"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.NotIn("username", tt.value, tt.forbidden)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "IN_FORBIDDEN_LIST", v.GetErrors()[0].Code)
				for _, f := range tt.forbidden {
					assert.Contains(t, v.GetErrors()[0].Message, f)
				}
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Equals
// ---------------------------------------------------------------------------

func TestValidator_Equals(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		other       string
		otherField  string
		expectError bool
	}{
		{"identical values pass", "secret", "secret", "password", false},
		{"empty values are equal and pass", "", "", "password", false},
		{"different values fail", "secret", "different", "password", true},
		{"case-sensitive mismatch fails", "Secret", "secret", "password", true},
		{"one empty fails", "abc", "", "password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Equals("password_confirm", tt.value, tt.other, tt.otherField)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "NOT_EQUAL", v.GetErrors()[0].Code)
				assert.Contains(t, v.GetErrors()[0].Message, tt.otherField)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Password
// ---------------------------------------------------------------------------

func TestValidator_Password(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		expectError   bool
		expectedCodes []string
	}{
		{
			name:        "strong password passes",
			value:       "Str0ng!Pass",
			expectError: false,
		},
		{
			name:        "empty value passes (optional — caller should use Required separately)",
			value:       "",
			expectError: false,
		},
		{
			name:          "too short fails",
			value:         "Ab1!",
			expectError:   true,
			expectedCodes: []string{"PASSWORD_TOO_SHORT"},
		},
		{
			name:          "no lowercase fails",
			value:         "STRONG1!PASS",
			expectError:   true,
			expectedCodes: []string{"PASSWORD_NO_LOWERCASE"},
		},
		{
			name:          "no uppercase fails",
			value:         "strong1!pass",
			expectError:   true,
			expectedCodes: []string{"PASSWORD_NO_UPPERCASE"},
		},
		{
			name:          "no digit fails",
			value:         "Strong!Pass",
			expectError:   true,
			expectedCodes: []string{"PASSWORD_NO_DIGIT"},
		},
		{
			name:          "no special character fails",
			value:         "Strong1Pass",
			expectError:   true,
			expectedCodes: []string{"PASSWORD_NO_SPECIAL"},
		},
		{
			name:        "all criteria missing adds multiple errors",
			value:       "a",
			expectError: true,
			// too short, no uppercase, no digit, no special
			expectedCodes: []string{
				"PASSWORD_TOO_SHORT",
				"PASSWORD_NO_UPPERCASE",
				"PASSWORD_NO_DIGIT",
				"PASSWORD_NO_SPECIAL",
			},
		},
		{
			name:        "exactly 8 chars with all criteria passes",
			value:       "Abcde1!x",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Password("password", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				codes := make([]string, 0, len(v.GetErrors()))
				for _, e := range v.GetErrors() {
					codes = append(codes, e.Code)
				}
				for _, expected := range tt.expectedCodes {
					assert.Contains(t, codes, expected, "expected code %q to be present", expected)
				}
				// Password errors use empty string for value (sensitive data)
				for _, e := range v.GetErrors() {
					assert.Equal(t, "", e.Value, "password error value must be empty string to avoid leaking")
				}
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UUID
// ---------------------------------------------------------------------------

func TestValidator_UUID(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"valid v4 UUID passes", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid v1 UUID passes", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", false},
		{"valid v3 UUID passes", "550e8400-e29b-31d4-a716-446655440000", false},
		{"uppercase UUID fails (regex is lowercase only)", "550E8400-E29B-41D4-A716-446655440000", true},
		{"missing hyphens fails", "550e8400e29b41d4a716446655440000", true},
		{"too short fails", "550e8400-e29b-41d4-a716", true},
		{"plain text fails", "not-a-uuid", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.UUID("id", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_UUID", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Slug
// ---------------------------------------------------------------------------

func TestValidator_Slug(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{"simple lowercase slug passes", "my-product", false},
		{"alphanumeric slug passes", "product123", false},
		{"slug with numbers and hyphens passes", "product-123-new", false},
		{"single word passes", "product", false},
		{"uppercase fails", "My-Product", true},
		{"leading hyphen fails", "-product", true},
		{"trailing hyphen fails", "product-", true},
		{"double hyphen fails", "product--name", true},
		{"contains underscore fails", "product_name", true},
		{"contains space fails", "product name", true},
		{"empty value passes (optional field)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			result := v.Slug("slug", tt.value)
			assert.Same(t, v, result)

			if tt.expectError {
				require.True(t, v.HasErrors())
				assert.Equal(t, "INVALID_SLUG", v.GetErrors()[0].Code)
			} else {
				assert.False(t, v.HasErrors())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Method chaining
// ---------------------------------------------------------------------------

func TestValidator_Chaining(t *testing.T) {
	t.Run("chained valid calls accumulate no errors", func(t *testing.T) {
		v := New()
		v.Required("email", "user@example.com").
			Email("email", "user@example.com").
			MaxLength("email", "user@example.com", 255)

		assert.False(t, v.HasErrors())
	})

	t.Run("chained invalid calls accumulate multiple errors", func(t *testing.T) {
		v := New()
		v.Required("email", "").
			Email("email", "not-an-email").
			MaxLength("email", strings.Repeat("a", 300), 255)

		errs := v.GetErrors()
		// Required("email", "") -> 1 error
		// Email("email", "not-an-email") -> 1 error
		// MaxLength(...) -> 1 error
		assert.GreaterOrEqual(t, len(errs), 3)
	})

	t.Run("chained calls return the same validator pointer", func(t *testing.T) {
		v := New()
		v2 := v.Required("f", "v").MinLength("f", "v", 1).MaxLength("f", "v", 10)
		assert.Same(t, v, v2)
	})
}

// ---------------------------------------------------------------------------
// Error accumulation across different fields
// ---------------------------------------------------------------------------

func TestValidator_ErrorAccumulation(t *testing.T) {
	v := New()

	v.Required("first_name", "")
	v.Required("last_name", "")
	v.Email("email", "not-valid")
	v.Min("age", "0", 1)

	errs := v.GetErrors()
	assert.Len(t, errs, 4)

	// Each error targets the correct field
	fields := make(map[string]bool)
	for _, e := range errs {
		fields[e.Field] = true
	}
	assert.True(t, fields["first_name"])
	assert.True(t, fields["last_name"])
	assert.True(t, fields["email"])
	assert.True(t, fields["age"])
}

// ---------------------------------------------------------------------------
// Clear resets state
// ---------------------------------------------------------------------------

func TestValidator_Clear_ResetsToCleanState(t *testing.T) {
	v := New()
	v.Required("field", "")
	v.Email("email", "bad")
	require.True(t, v.HasErrors())
	require.Len(t, v.GetErrors(), 2)

	v.Clear()

	assert.False(t, v.HasErrors())
	assert.Empty(t, v.GetErrors())

	// Validator is fully reusable after Clear
	v.Required("field", "value")
	assert.False(t, v.HasErrors())
}

// ---------------------------------------------------------------------------
// Business validators
// ---------------------------------------------------------------------------

func TestValidateStaffData(t *testing.T) {
	t.Run("valid staff data returns no errors", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "alice@example.com",
		}
		errs := ValidateStaffData(data)
		assert.Empty(t, errs)
	})

	t.Run("valid staff data with optional phone and department passes", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Bob",
			"last_name":  "Jones",
			"email":      "bob@example.com",
			"phone":      "+14155551234",
			"department": "Engineering",
		}
		errs := ValidateStaffData(data)
		assert.Empty(t, errs)
	})

	t.Run("empty first_name adds REQUIRED and skips alpha (alpha skips empty)", func(t *testing.T) {
		data := map[string]string{
			"first_name": "",
			"last_name":  "Smith",
			"email":      "alice@example.com",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
	})

	t.Run("empty last_name adds REQUIRED error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "",
			"email":      "alice@example.com",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
	})

	t.Run("invalid email adds INVALID_EMAIL error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "not-an-email",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "INVALID_EMAIL")
	})

	t.Run("empty email adds REQUIRED error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
	})

	t.Run("first_name with non-alpha chars adds NOT_ALPHA error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice1",
			"last_name":  "Smith",
			"email":      "alice@example.com",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "NOT_ALPHA")
	})

	t.Run("first_name over 50 chars adds MAX_LENGTH error", func(t *testing.T) {
		data := map[string]string{
			"first_name": strings.Repeat("A", 51),
			"last_name":  "Smith",
			"email":      "alice@example.com",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "MAX_LENGTH")
	})

	t.Run("invalid phone adds INVALID_PHONE error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "alice@example.com",
			"phone":      "not-a-phone",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "INVALID_PHONE")
	})

	t.Run("empty phone is skipped (phone is optional)", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "alice@example.com",
			"phone":      "",
		}
		errs := ValidateStaffData(data)
		assert.Empty(t, errs)
	})

	t.Run("department with special chars adds NOT_ALPHANUMERIC error", func(t *testing.T) {
		data := map[string]string{
			"first_name": "Alice",
			"last_name":  "Smith",
			"email":      "alice@example.com",
			"department": "R&D",
		}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "NOT_ALPHANUMERIC")
	})

	t.Run("missing keys produce empty strings and trigger required errors", func(t *testing.T) {
		data := map[string]string{}
		errs := ValidateStaffData(data)
		require.NotEmpty(t, errs)
		// first_name, last_name, email are all required
		fields := errorFields(errs)
		assert.Contains(t, fields, "first_name")
		assert.Contains(t, fields, "last_name")
		assert.Contains(t, fields, "email")
	})

	t.Run("multiple invalid fields accumulate errors for all fields", func(t *testing.T) {
		data := map[string]string{
			"first_name": "",
			"last_name":  "",
			"email":      "bad",
		}
		errs := ValidateStaffData(data)
		assert.GreaterOrEqual(t, len(errs), 3)
	})
}

func TestValidateUserRegistration(t *testing.T) {
	t.Run("valid registration data returns no errors", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		assert.Empty(t, errs)
	})

	t.Run("empty email adds REQUIRED error", func(t *testing.T) {
		data := map[string]string{
			"email":            "",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
	})

	t.Run("invalid email adds INVALID_EMAIL error", func(t *testing.T) {
		data := map[string]string{
			"email":            "bad-email",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "INVALID_EMAIL")
	})

	t.Run("weak password adds password strength errors", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "weak",
			"password_confirm": "weak",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "PASSWORD_TOO_SHORT")
	})

	t.Run("mismatched passwords add NOT_EQUAL error", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "Different1!Pass",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "NOT_EQUAL")
	})

	t.Run("empty password_confirm adds REQUIRED and NOT_EQUAL errors", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "",
			"first_name":       "Jane",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
		assert.Contains(t, codes, "NOT_EQUAL")
	})

	t.Run("non-alpha first_name adds NOT_ALPHA error", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       "Jane1",
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "NOT_ALPHA")
	})

	t.Run("first_name over 50 chars adds MAX_LENGTH error", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       strings.Repeat("A", 51),
			"last_name":        "Doe",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "MAX_LENGTH")
	})

	t.Run("empty last_name adds REQUIRED error", func(t *testing.T) {
		data := map[string]string{
			"email":            "user@example.com",
			"password":         "Str0ng!Pass",
			"password_confirm": "Str0ng!Pass",
			"first_name":       "Jane",
			"last_name":        "",
		}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		codes := errorCodes(errs)
		assert.Contains(t, codes, "REQUIRED")
	})

	t.Run("all fields missing produces errors for all required fields", func(t *testing.T) {
		data := map[string]string{}
		errs := ValidateUserRegistration(data)
		require.NotEmpty(t, errs)
		fields := errorFields(errs)
		assert.Contains(t, fields, "email")
		assert.Contains(t, fields, "password")
		assert.Contains(t, fields, "password_confirm")
		assert.Contains(t, fields, "first_name")
		assert.Contains(t, fields, "last_name")
	})
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkValidator_Email(b *testing.B) {
	v := New()
	for i := 0; i < b.N; i++ {
		v.Clear()
		v.Email("email", "user@example.com")
	}
}

func BenchmarkValidator_Password(b *testing.B) {
	v := New()
	for i := 0; i < b.N; i++ {
		v.Clear()
		v.Password("password", "Str0ng!Pass")
	}
}

func BenchmarkValidateStaffData(b *testing.B) {
	data := map[string]string{
		"first_name": "Alice",
		"last_name":  "Smith",
		"email":      "alice@example.com",
		"phone":      "+14155551234",
		"department": "Engineering",
	}
	for i := 0; i < b.N; i++ {
		ValidateStaffData(data)
	}
}

func BenchmarkValidateUserRegistration(b *testing.B) {
	data := map[string]string{
		"email":            "user@example.com",
		"password":         "Str0ng!Pass",
		"password_confirm": "Str0ng!Pass",
		"first_name":       "Jane",
		"last_name":        "Doe",
	}
	for i := 0; i < b.N; i++ {
		ValidateUserRegistration(data)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// errorCodes returns a slice of all error codes present in the given errors.
func errorCodes(errs ValidationErrors) []string {
	codes := make([]string, 0, len(errs))
	for _, e := range errs {
		codes = append(codes, e.Code)
	}
	return codes
}

// errorFields returns a slice of all field names present in the given errors.
func errorFields(errs ValidationErrors) []string {
	fields := make([]string, 0, len(errs))
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	return fields
}
