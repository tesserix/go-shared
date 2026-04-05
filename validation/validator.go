package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/tesserix/go-shared/middleware"
)

// ValidationError represents a validation error for a specific field
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (ve ValidationErrors) Error() string {
	var messages []string
	for _, err := range ve {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// ToMap converts validation errors to a map for API responses
func (ve ValidationErrors) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	for _, err := range ve {
		result[err.Field] = map[string]string{
			"message": err.Message,
			"code":    err.Code,
		}
	}
	return result
}

// Validator provides validation functionality
type Validator struct {
	errors ValidationErrors
}

// New creates a new validator instance
func New() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() ValidationErrors {
	return v.errors
}

// AddError adds a validation error
func (v *Validator) AddError(field, value, message, code string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

// Clear clears all validation errors
func (v *Validator) Clear() {
	v.errors = make(ValidationErrors, 0)
}

// Basic validation methods

// Required validates that a field is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, value, "This field is required", "REQUIRED")
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.AddError(field, value, fmt.Sprintf("Must be at least %d characters long", min), "MIN_LENGTH")
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.AddError(field, value, fmt.Sprintf("Must be no more than %d characters long", max), "MAX_LENGTH")
	}
	return v
}

// Length validates exact string length
func (v *Validator) Length(field, value string, length int) *Validator {
	if len(value) != length {
		v.AddError(field, value, fmt.Sprintf("Must be exactly %d characters long", length), "EXACT_LENGTH")
	}
	return v
}

// Email validates email format
func (v *Validator) Email(field, value string) *Validator {
	if value != "" {
		if _, err := mail.ParseAddress(value); err != nil {
			v.AddError(field, value, "Must be a valid email address", "INVALID_EMAIL")
		}
	}
	return v
}

// URL validates URL format
func (v *Validator) URL(field, value string) *Validator {
	if value != "" {
		urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
		if !urlRegex.MatchString(value) {
			v.AddError(field, value, "Must be a valid URL", "INVALID_URL")
		}
	}
	return v
}

// Phone validates phone number format
func (v *Validator) Phone(field, value string) *Validator {
	if value != "" {
		phoneRegex := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
		cleanPhone := regexp.MustCompile(`[^\d+]`).ReplaceAllString(value, "")
		if !phoneRegex.MatchString(cleanPhone) {
			v.AddError(field, value, "Must be a valid phone number", "INVALID_PHONE")
		}
	}
	return v
}

// Numeric validation methods

// Min validates minimum numeric value
func (v *Validator) Min(field, value string, min float64) *Validator {
	if value != "" {
		if num, err := strconv.ParseFloat(value, 64); err != nil {
			v.AddError(field, value, "Must be a valid number", "INVALID_NUMBER")
		} else if num < min {
			v.AddError(field, value, fmt.Sprintf("Must be at least %g", min), "MIN_VALUE")
		}
	}
	return v
}

// Max validates maximum numeric value
func (v *Validator) Max(field, value string, max float64) *Validator {
	if value != "" {
		if num, err := strconv.ParseFloat(value, 64); err != nil {
			v.AddError(field, value, "Must be a valid number", "INVALID_NUMBER")
		} else if num > max {
			v.AddError(field, value, fmt.Sprintf("Must be no more than %g", max), "MAX_VALUE")
		}
	}
	return v
}

// Integer validates that value is an integer
func (v *Validator) Integer(field, value string) *Validator {
	if value != "" {
		if _, err := strconv.Atoi(value); err != nil {
			v.AddError(field, value, "Must be a valid integer", "INVALID_INTEGER")
		}
	}
	return v
}

// Positive validates that value is positive
func (v *Validator) Positive(field, value string) *Validator {
	if value != "" {
		if num, err := strconv.ParseFloat(value, 64); err != nil {
			v.AddError(field, value, "Must be a valid number", "INVALID_NUMBER")
		} else if num <= 0 {
			v.AddError(field, value, "Must be a positive number", "NOT_POSITIVE")
		}
	}
	return v
}

// Pattern validation

// Pattern validates against a regular expression
func (v *Validator) Pattern(field, value, pattern, message string) *Validator {
	if value != "" {
		if matched, err := regexp.MatchString(pattern, value); err != nil {
			v.AddError(field, value, "Invalid pattern", "INVALID_PATTERN")
		} else if !matched {
			v.AddError(field, value, message, "PATTERN_MISMATCH")
		}
	}
	return v
}

// AlphaNumeric validates alphanumeric characters only
func (v *Validator) AlphaNumeric(field, value string) *Validator {
	if value != "" {
		for _, r := range value {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				v.AddError(field, value, "Must contain only letters and numbers", "NOT_ALPHANUMERIC")
				break
			}
		}
	}
	return v
}

// Alpha validates alphabetic characters only
func (v *Validator) Alpha(field, value string) *Validator {
	if value != "" {
		for _, r := range value {
			if !unicode.IsLetter(r) && !unicode.IsSpace(r) {
				v.AddError(field, value, "Must contain only letters", "NOT_ALPHA")
				break
			}
		}
	}
	return v
}

// Date validation

// Date validates date format (YYYY-MM-DD)
func (v *Validator) Date(field, value string) *Validator {
	if value != "" {
		if _, err := time.Parse("2006-01-02", value); err != nil {
			v.AddError(field, value, "Must be a valid date (YYYY-MM-DD)", "INVALID_DATE")
		}
	}
	return v
}

// DateTime validates datetime format (RFC3339)
func (v *Validator) DateTime(field, value string) *Validator {
	if value != "" {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			v.AddError(field, value, "Must be a valid datetime (RFC3339)", "INVALID_DATETIME")
		}
	}
	return v
}

// DateAfter validates that date is after specified date
func (v *Validator) DateAfter(field, value, after string) *Validator {
	if value != "" && after != "" {
		valueDate, err1 := time.Parse("2006-01-02", value)
		afterDate, err2 := time.Parse("2006-01-02", after)
		
		if err1 != nil {
			v.AddError(field, value, "Must be a valid date", "INVALID_DATE")
		} else if err2 != nil {
			v.AddError(field, value, "Invalid comparison date", "INVALID_COMPARISON_DATE")
		} else if !valueDate.After(afterDate) {
			v.AddError(field, value, fmt.Sprintf("Must be after %s", after), "DATE_NOT_AFTER")
		}
	}
	return v
}

// Custom validation

// In validates that value is in a list of allowed values
func (v *Validator) In(field, value string, allowed []string) *Validator {
	if value != "" {
		found := false
		for _, allowedValue := range allowed {
			if value == allowedValue {
				found = true
				break
			}
		}
		if !found {
			v.AddError(field, value, fmt.Sprintf("Must be one of: %s", strings.Join(allowed, ", ")), "NOT_IN_LIST")
		}
	}
	return v
}

// NotIn validates that value is not in a list of forbidden values
func (v *Validator) NotIn(field, value string, forbidden []string) *Validator {
	if value != "" {
		for _, forbiddenValue := range forbidden {
			if value == forbiddenValue {
				v.AddError(field, value, fmt.Sprintf("Cannot be one of: %s", strings.Join(forbidden, ", ")), "IN_FORBIDDEN_LIST")
				break
			}
		}
	}
	return v
}

// Equals validates that value equals another value
func (v *Validator) Equals(field, value, other, otherField string) *Validator {
	if value != other {
		v.AddError(field, value, fmt.Sprintf("Must match %s", otherField), "NOT_EQUAL")
	}
	return v
}

// Password validation

// Password validates password strength
func (v *Validator) Password(field, value string) *Validator {
	if value == "" {
		return v
	}

	var hasLower, hasUpper, hasDigit, hasSpecial bool
	
	for _, r := range value {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if len(value) < 8 {
		v.AddError(field, "", "Password must be at least 8 characters long", "PASSWORD_TOO_SHORT")
	}
	if !hasLower {
		v.AddError(field, "", "Password must contain at least one lowercase letter", "PASSWORD_NO_LOWERCASE")
	}
	if !hasUpper {
		v.AddError(field, "", "Password must contain at least one uppercase letter", "PASSWORD_NO_UPPERCASE")
	}
	if !hasDigit {
		v.AddError(field, "", "Password must contain at least one digit", "PASSWORD_NO_DIGIT")
	}
	if !hasSpecial {
		v.AddError(field, "", "Password must contain at least one special character", "PASSWORD_NO_SPECIAL")
	}

	return v
}

// Business validation helpers

// UUID validates UUID format
func (v *Validator) UUID(field, value string) *Validator {
	if value != "" {
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
		if !uuidRegex.MatchString(value) {
			v.AddError(field, value, "Must be a valid UUID", "INVALID_UUID")
		}
	}
	return v
}

// Slug validates URL slug format
func (v *Validator) Slug(field, value string) *Validator {
	if value != "" {
		slugRegex := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
		if !slugRegex.MatchString(value) {
			v.AddError(field, value, "Must be a valid slug (lowercase letters, numbers, and hyphens)", "INVALID_SLUG")
		}
	}
	return v
}

// Middleware and helpers

// GinValidator creates a Gin middleware that handles validation errors
func GinValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are validation errors in the context
		if errors, exists := c.Get("validation_errors"); exists {
			if validationErrors, ok := errors.(ValidationErrors); ok && len(validationErrors) > 0 {
				middleware.ValidationErrorResponse(c, validationErrors.ToMap())
				c.Abort()
				return
			}
		}
	}
}

// SetValidationErrors sets validation errors in Gin context
func SetValidationErrors(c *gin.Context, errors ValidationErrors) {
	c.Set("validation_errors", errors)
}

// ValidateAndRespond validates using the validator and responds with errors if any
func ValidateAndRespond(c *gin.Context, validator *Validator) bool {
	if validator.HasErrors() {
		middleware.ValidationErrorResponse(c, validator.GetErrors().ToMap())
		return false
	}
	return true
}

// Common validation rule sets

// ValidateStaffData validates staff creation/update data
func ValidateStaffData(data map[string]string) ValidationErrors {
	v := New()

	v.Required("first_name", data["first_name"]).
		Alpha("first_name", data["first_name"]).
		MaxLength("first_name", data["first_name"], 50)

	v.Required("last_name", data["last_name"]).
		Alpha("last_name", data["last_name"]).
		MaxLength("last_name", data["last_name"], 50)

	v.Required("email", data["email"]).
		Email("email", data["email"]).
		MaxLength("email", data["email"], 255)

	if phone := data["phone"]; phone != "" {
		v.Phone("phone", phone)
	}

	if department := data["department"]; department != "" {
		v.AlphaNumeric("department", department).
			MaxLength("department", department, 100)
	}

	return v.GetErrors()
}

// ValidateUserRegistration validates user registration data
func ValidateUserRegistration(data map[string]string) ValidationErrors {
	v := New()

	v.Required("email", data["email"]).
		Email("email", data["email"])

	v.Required("password", data["password"]).
		Password("password", data["password"])

	v.Required("password_confirm", data["password_confirm"]).
		Equals("password_confirm", data["password_confirm"], data["password"], "password")

	v.Required("first_name", data["first_name"]).
		Alpha("first_name", data["first_name"]).
		MaxLength("first_name", data["first_name"], 50)

	v.Required("last_name", data["last_name"]).
		Alpha("last_name", data["last_name"]).
		MaxLength("last_name", data["last_name"], 50)

	return v.GetErrors()
}