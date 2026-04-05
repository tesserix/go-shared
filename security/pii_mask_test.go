package security

import (
	"testing"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard email",
			input:    "john.doe@example.com",
			expected: "jo***@example.com",
		},
		{
			name:     "short local part",
			input:    "jo@example.com",
			expected: "**@example.com",
		},
		{
			name:     "very short local part",
			input:    "j@example.com",
			expected: RedactedEmail,
		},
		{
			name:     "empty email",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid email without @",
			input:    "notanemail",
			expected: RedactedEmail,
		},
		{
			name:     "email with subdomain",
			input:    "user@mail.example.com",
			expected: "us***@mail.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskEmail(tt.input)
			if result != tt.expected {
				t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard phone",
			input:    "+1-555-123-4567",
			expected: "***4567",
		},
		{
			name:     "phone without country code",
			input:    "5551234567",
			expected: "***4567",
		},
		{
			name:     "phone with spaces",
			input:    "555 123 4567",
			expected: "***4567",
		},
		{
			name:     "empty phone",
			input:    "",
			expected: "",
		},
		{
			name:     "short phone",
			input:    "123",
			expected: RedactedPhone,
		},
		{
			name:     "Indian phone",
			input:    "+91 98765 43210",
			expected: "***3210",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskPhone(tt.input)
			if result != tt.expected {
				t.Errorf("MaskPhone(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full name",
			input:    "John Doe",
			expected: "J*** D***",
		},
		{
			name:     "single name",
			input:    "John",
			expected: "J***",
		},
		{
			name:     "three part name",
			input:    "John Michael Doe",
			expected: "J*** M*** D***",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskName(tt.input)
			if result != tt.expected {
				t.Errorf("MaskName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskCard(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "visa card",
			input:    "4111111111111111",
			expected: "****1111",
		},
		{
			name:     "card with spaces",
			input:    "4111 1111 1111 1111",
			expected: "****1111",
		},
		{
			name:     "card with dashes",
			input:    "4111-1111-1111-1111",
			expected: "****1111",
		},
		{
			name:     "empty card",
			input:    "",
			expected: "",
		},
		{
			name:     "short number",
			input:    "123",
			expected: RedactedCard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskCard(tt.input)
			if result != tt.expected {
				t.Errorf("MaskCard(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskPAN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid PAN",
			input:    "ABCDE1234F",
			expected: "ABCD***34F",
		},
		{
			name:     "lowercase PAN",
			input:    "abcde1234f",
			expected: "ABCD***34F",
		},
		{
			name:     "empty PAN",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid length",
			input:    "ABC123",
			expected: RedactedPAN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskPAN(tt.input)
			if result != tt.expected {
				t.Errorf("MaskPAN(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskGSTIN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid GSTIN",
			input:    "29ABCDE1234F1Z5",
			expected: "29****4F1Z5",
		},
		{
			name:     "empty GSTIN",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid length",
			input:    "29ABC",
			expected: RedactedGSTIN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskGSTIN(tt.input)
			if result != tt.expected {
				t.Errorf("MaskGSTIN(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskVerificationCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "6 digit code",
			input:    "123456",
			expected: RedactedCode,
		},
		{
			name:     "4 digit code",
			input:    "1234",
			expected: RedactedCode,
		},
		{
			name:     "empty code",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskVerificationCode(tt.input)
			if result != tt.expected {
				t.Errorf("MaskVerificationCode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeLogMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		notContains string
	}{
		{
			name:        "message with email",
			input:       "User john.doe@example.com logged in",
			contains:    "jo***@example.com",
			notContains: "john.doe@example.com",
		},
		{
			name:        "message with PAN",
			input:       "PAN verified: ABCDE1234F",
			contains:    "ABCD***34F",
			notContains: "ABCDE1234F",
		},
		{
			name:        "plain message",
			input:       "User logged in successfully",
			contains:    "User logged in successfully",
			notContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLogMessage(tt.input)
			if tt.contains != "" && !containsString(result, tt.contains) {
				t.Errorf("SanitizeLogMessage(%q) should contain %q, got %q", tt.input, tt.contains, result)
			}
			if tt.notContains != "" && containsString(result, tt.notContains) {
				t.Errorf("SanitizeLogMessage(%q) should not contain %q, got %q", tt.input, tt.notContains, result)
			}
		})
	}
}

func TestSecureLogFields(t *testing.T) {
	input := map[string]interface{}{
		"email":      "john@example.com",
		"phone":      "+1-555-123-4567",
		"name":       "John Doe",
		"order_id":   "12345",
		"password":   "secret123",
		"code":       "123456",
	}

	result := SecureLogFields(input)

	// Check email is masked
	if email, ok := result["email"].(string); ok {
		if email == "john@example.com" {
			t.Error("Email should be masked")
		}
		if email != "jo***@example.com" {
			t.Errorf("Email mask incorrect: got %q", email)
		}
	}

	// Check phone is masked
	if phone, ok := result["phone"].(string); ok {
		if phone == "+1-555-123-4567" {
			t.Error("Phone should be masked")
		}
	}

	// Check name is masked
	if name, ok := result["name"].(string); ok {
		if name == "John Doe" {
			t.Error("Name should be masked")
		}
	}

	// Check order_id is NOT masked (not PII)
	if orderID, ok := result["order_id"].(string); ok {
		if orderID != "12345" {
			t.Error("Non-PII fields should not be modified")
		}
	}

	// Check password is redacted
	if password, ok := result["password"].(string); ok {
		if password != "[REDACTED]" {
			t.Errorf("Password should be [REDACTED], got %q", password)
		}
	}

	// Check code is redacted
	if code, ok := result["code"].(string); ok {
		if code != RedactedCode {
			t.Errorf("Code should be %q, got %q", RedactedCode, code)
		}
	}
}

func TestMaskIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 address",
			input:    "192.168.1.100",
			expected: "192.168.***.***",
		},
		{
			name:     "public IPv4",
			input:    "8.8.8.8",
			expected: "8.8.***.***",
		},
		{
			name:     "localhost",
			input:    "127.0.0.1",
			expected: "127.0.***.***",
		},
		{
			name:     "IPv6 address",
			input:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: "2001:0db8:****",
		},
		{
			name:     "short IPv6",
			input:    "::1",
			expected: "::****",
		},
		{
			name:     "empty IP",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid format",
			input:    "not-an-ip",
			expected: "[IP_REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskIPAddress(tt.input)
			if result != tt.expected {
				t.Errorf("MaskIPAddress(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskUserID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UUID format",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-****",
		},
		{
			name:     "short ID",
			input:    "abc123",
			expected: "abc****",
		},
		{
			name:     "empty ID",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskUserID(tt.input)
			if result != tt.expected {
				t.Errorf("MaskUserID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeQueryString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "token parameter",
			input:    "token=abc123xyz&page=1",
			expected: "token=[REDACTED]&page=1",
		},
		{
			name:     "access_token parameter",
			input:    "access_token=secret123&user=john",
			expected: "access_token=[REDACTED]&user=john",
		},
		{
			name:     "password parameter",
			input:    "username=admin&password=secret123",
			expected: "username=admin&password=[REDACTED]",
		},
		{
			name:     "api_key parameter",
			input:    "api_key=sk_live_abc123&format=json",
			expected: "api_key=[REDACTED]&format=json",
		},
		{
			name:     "multiple sensitive params",
			input:    "token=abc&secret=xyz&api_key=123",
			expected: "token=[REDACTED]&secret=[REDACTED]&api_key=[REDACTED]",
		},
		{
			name:     "code parameter",
			input:    "code=123456&state=xyz",
			expected: "code=[REDACTED]&state=xyz",
		},
		{
			name:     "session parameter",
			input:    "session=sess_abc123&id=1",
			expected: "session=[REDACTED]&id=1",
		},
		{
			name:     "empty query string",
			input:    "",
			expected: "",
		},
		{
			name:     "no sensitive params",
			input:    "page=1&limit=10&sort=asc",
			expected: "page=1&limit=10&sort=asc",
		},
		{
			name:     "email in value gets masked",
			input:    "search=john.doe@example.com&page=1",
			expected: "search=jo***@example.com&page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeQueryString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeQueryString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeLogMessageCardNumbers(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		notContains string
		contains    string
	}{
		{
			name:        "16 digit card number",
			input:       "Payment processed for card 4111111111111111",
			notContains: "4111111111111111",
			contains:    "****1111", // MaskCard outputs ****+last4
		},
		{
			name:        "19 digit card number",
			input:       "Processing card 6011111111111111117",
			notContains: "6011111111111111117",
			contains:    "****1117",
		},
		// Note: 13-15 digit numbers may be caught by phone pattern first
		// Cards with spaces are NOT masked by SanitizeLogMessage
		// to avoid false positives. Use MaskCard() directly for formatted cards.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLogMessage(tt.input)
			if tt.notContains != "" && containsString(result, tt.notContains) {
				t.Errorf("SanitizeLogMessage(%q) should not contain %q, got %q", tt.input, tt.notContains, result)
			}
			if tt.contains != "" && !containsString(result, tt.contains) {
				t.Errorf("SanitizeLogMessage(%q) should contain %q, got %q", tt.input, tt.contains, result)
			}
		})
	}
}

func TestSanitizeLogMessageOTP(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		notContains string
		contains    string
	}{
		{
			name:        "OTP code pattern",
			input:       "Sending otp: 123456 to user",
			notContains: "123456",
			contains:    RedactedCode,
		},
		{
			name:        "verification code pattern",
			input:       "verification code=654321 sent",
			notContains: "654321",
			contains:    RedactedCode,
		},
		{
			name:        "code colon pattern",
			input:       "User entered code: 987654",
			notContains: "987654",
			contains:    RedactedCode,
		},
		{
			name:        "PIN pattern",
			input:       "Setting PIN: 1234 for account",
			notContains: "1234",
			contains:    RedactedCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLogMessage(tt.input)
			if tt.notContains != "" && containsString(result, tt.notContains) {
				t.Errorf("SanitizeLogMessage(%q) should not contain %q, got %q", tt.input, tt.notContains, result)
			}
			if tt.contains != "" && !containsString(result, tt.contains) {
				t.Errorf("SanitizeLogMessage(%q) should contain %q, got %q", tt.input, tt.contains, result)
			}
		})
	}
}

func TestSecureLogFieldsExtended(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		field      string
		shouldMask bool
		expected   string
	}{
		{
			name:       "SSN field",
			input:      map[string]interface{}{"ssn": "123-45-6789"},
			field:      "ssn",
			shouldMask: true,
			expected:   RedactedSSN,
		},
		{
			name:       "date of birth field",
			input:      map[string]interface{}{"dob": "1990-01-01"},
			field:      "dob",
			shouldMask: true,
			expected:   RedactedDOB,
		},
		{
			name:       "bank account field",
			input:      map[string]interface{}{"bank_account": "1234567890"},
			field:      "bank_account",
			shouldMask: true,
			expected:   RedactedAccount,
		},
		{
			name:       "IP address field",
			input:      map[string]interface{}{"ip_address": "192.168.1.1"},
			field:      "ip_address",
			shouldMask: true,
			expected:   "192.168.***.***",
		},
		{
			name:       "client IP field",
			input:      map[string]interface{}{"client_ip": "10.0.0.1"},
			field:      "client_ip",
			shouldMask: true,
			expected:   "10.0.***.***",
		},
		{
			name:       "token field",
			input:      map[string]interface{}{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			field:      "token",
			shouldMask: true,
			expected:   RedactedToken,
		},
		{
			name:       "access_token field",
			input:      map[string]interface{}{"access_token": "abc123xyz"},
			field:      "access_token",
			shouldMask: true,
			expected:   RedactedToken,
		},
		{
			name:       "card_number field",
			input:      map[string]interface{}{"card_number": "4111111111111111"},
			field:      "card_number",
			shouldMask: true,
			expected:   "****1111",
		},
		{
			name:       "user_email field",
			input:      map[string]interface{}{"user_email": "test@example.com"},
			field:      "user_email",
			shouldMask: true,
			expected:   "te***@example.com",
		},
		{
			name:       "customer_name field",
			input:      map[string]interface{}{"customer_name": "Jane Smith"},
			field:      "customer_name",
			shouldMask: true,
			expected:   "J*** S***",
		},
		{
			name:       "mobile_number field",
			input:      map[string]interface{}{"mobile_number": "+919876543210"},
			field:      "mobile_number",
			shouldMask: true,
			expected:   "***3210",
		},
		{
			name:       "address field",
			input:      map[string]interface{}{"address": "123 Main St, City"},
			field:      "address",
			shouldMask: true,
			expected:   RedactedAddress,
		},
		{
			name:       "shipping_address field",
			input:      map[string]interface{}{"shipping_address": "456 Oak Ave"},
			field:      "shipping_address",
			shouldMask: true,
			expected:   RedactedAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureLogFields(tt.input)
			value, ok := result[tt.field].(string)
			if !ok {
				t.Errorf("Expected string value for field %q", tt.field)
				return
			}
			if tt.shouldMask && value != tt.expected {
				t.Errorf("SecureLogFields field %q = %q, want %q", tt.field, value, tt.expected)
			}
		})
	}
}

func TestSanitizeLogMessageSSN(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		notContains string
		contains    string
	}{
		{
			name:        "SSN with dashes",
			input:       "SSN: 123-45-6789 for user",
			notContains: "123-45-6789",
			contains:    RedactedSSN,
		},
		{
			name:        "SSN with spaces",
			input:       "Social: 123 45 6789",
			notContains: "123 45 6789",
			contains:    RedactedSSN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLogMessage(tt.input)
			if tt.notContains != "" && containsString(result, tt.notContains) {
				t.Errorf("SanitizeLogMessage(%q) should not contain %q, got %q", tt.input, tt.notContains, result)
			}
			if tt.contains != "" && !containsString(result, tt.contains) {
				t.Errorf("SanitizeLogMessage(%q) should contain %q, got %q", tt.input, tt.contains, result)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
