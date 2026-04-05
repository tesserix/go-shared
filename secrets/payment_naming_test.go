package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPaymentSecretName(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		tenantID string
		vendorID string
		provider PaymentProvider
		keyName  PaymentKeyName
		expected string
	}{
		{
			name:     "tenant level stripe api key",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "",
			provider: PaymentProviderStripe,
			keyName:  KeyStripeAPIKey,
			expected: "prod-tenant-t_123-stripe-api-key",
		},
		{
			name:     "tenant level stripe webhook secret",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "",
			provider: PaymentProviderStripe,
			keyName:  KeyStripeWebhookSecret,
			expected: "prod-tenant-t_123-stripe-webhook-secret",
		},
		{
			name:     "vendor level stripe api key",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "v_99",
			provider: PaymentProviderStripe,
			keyName:  KeyStripeAPIKey,
			expected: "prod-tenant-t_123-vendor-v_99-stripe-api-key",
		},
		{
			name:     "dev tenant level razorpay key id",
			env:      "dev",
			tenantID: "t_456",
			vendorID: "",
			provider: PaymentProviderRazorpay,
			keyName:  KeyRazorpayKeyID,
			expected: "dev-tenant-t_456-razorpay-key-id",
		},
		{
			name:     "vendor level razorpay key secret",
			env:      "dev",
			tenantID: "t_123",
			vendorID: "v_99",
			provider: PaymentProviderRazorpay,
			keyName:  KeyRazorpayKeySecret,
			expected: "dev-tenant-t_123-vendor-v_99-razorpay-key-secret",
		},
		{
			name:     "sanitizes uppercase",
			env:      "PROD",
			tenantID: "T_ABC",
			vendorID: "",
			provider: PaymentProviderStripe,
			keyName:  KeyStripeAPIKey,
			expected: "prod-tenant-t_abc-stripe-api-key",
		},
		{
			name:     "sanitizes special characters",
			env:      "prod",
			tenantID: "tenant@123!",
			vendorID: "",
			provider: PaymentProviderStripe,
			keyName:  KeyStripeAPIKey,
			expected: "prod-tenant-tenant-123-stripe-api-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPaymentSecretName(tt.env, tt.tenantID, tt.vendorID, tt.provider, tt.keyName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePaymentSecretName(t *testing.T) {
	tests := []struct {
		name        string
		secretName  string
		expected    *PaymentSecretMetadata
		expectError bool
	}{
		{
			name:       "tenant level stripe",
			secretName: "prod-tenant-t_123-stripe-api-key",
			expected: &PaymentSecretMetadata{
				Env:      "prod",
				TenantID: "t_123",
				VendorID: "",
				Provider: PaymentProviderStripe,
				KeyName:  KeyStripeAPIKey,
				Scope:    PaymentScopeTenant,
			},
		},
		{
			name:       "vendor level stripe",
			secretName: "prod-tenant-t_123-vendor-v_99-stripe-api-key",
			expected: &PaymentSecretMetadata{
				Env:      "prod",
				TenantID: "t_123",
				VendorID: "v_99",
				Provider: PaymentProviderStripe,
				KeyName:  KeyStripeAPIKey,
				Scope:    PaymentScopeVendor,
			},
		},
		{
			name:       "dev tenant razorpay",
			secretName: "dev-tenant-t_456-razorpay-key-id",
			expected: &PaymentSecretMetadata{
				Env:      "dev",
				TenantID: "t_456",
				VendorID: "",
				Provider: PaymentProviderRazorpay,
				KeyName:  KeyRazorpayKeyID,
				Scope:    PaymentScopeTenant,
			},
		},
		{
			name:        "invalid format",
			secretName:  "invalid-secret-name",
			expectError: true,
		},
		{
			name:        "empty string",
			secretName:  "",
			expectError: true,
		},
		{
			name:        "missing components",
			secretName:  "prod-tenant-stripe",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePaymentSecretName(tt.secretName)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetPaymentProviderRequiredKeys(t *testing.T) {
	tests := []struct {
		provider PaymentProvider
		expected []PaymentKeyName
	}{
		{
			provider: PaymentProviderStripe,
			expected: []PaymentKeyName{KeyStripeAPIKey},
		},
		{
			provider: PaymentProviderRazorpay,
			expected: []PaymentKeyName{KeyRazorpayKeyID, KeyRazorpayKeySecret},
		},
		{
			provider: PaymentProviderPayPal,
			expected: []PaymentKeyName{KeyPayPalClientID, KeyPayPalClientSecret},
		},
		{
			provider: PaymentProvider("unknown"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := GetPaymentProviderRequiredKeys(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPaymentProviderOptionalKeys(t *testing.T) {
	tests := []struct {
		provider PaymentProvider
		expected []PaymentKeyName
	}{
		{
			provider: PaymentProviderStripe,
			expected: []PaymentKeyName{KeyStripeWebhookSecret, KeyStripeConnectedID},
		},
		{
			provider: PaymentProviderRazorpay,
			expected: []PaymentKeyName{KeyRazorpayWebhook},
		},
		{
			provider: PaymentProvider("unknown"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := GetPaymentProviderOptionalKeys(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVendorLevelSecret(t *testing.T) {
	tests := []struct {
		secretName string
		expected   bool
	}{
		{"prod-tenant-t_123-vendor-v_99-stripe-api-key", true},
		{"prod-tenant-t_123-stripe-api-key", false},
		{"dev-tenant-t_123-vendor-v_1-razorpay-key-id", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.secretName, func(t *testing.T) {
			result := IsVendorLevelSecret(tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTenantLevelSecret(t *testing.T) {
	tests := []struct {
		secretName string
		expected   bool
	}{
		{"prod-tenant-t_123-stripe-api-key", true},
		{"prod-tenant-t_123-vendor-v_99-stripe-api-key", false},
		{"invalid-name", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.secretName, func(t *testing.T) {
			result := IsTenantLevelSecret(tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPaymentSecretLabels(t *testing.T) {
	t.Run("tenant level labels", func(t *testing.T) {
		meta := &PaymentSecretMetadata{
			Env:      "prod",
			TenantID: "t_123",
			VendorID: "",
			Provider: PaymentProviderStripe,
			KeyName:  KeyStripeAPIKey,
			Scope:    PaymentScopeTenant,
		}

		labels := BuildPaymentSecretLabels(meta)

		assert.Equal(t, "prod", labels["environment"])
		assert.Equal(t, "payment", labels["category"])
		assert.Equal(t, "stripe", labels["provider"])
		assert.Equal(t, "t_123", labels["tenant_id"])
		assert.Equal(t, "tenant", labels["scope"])
		assert.Equal(t, "secret-provisioner", labels["managed_by"])
		assert.NotContains(t, labels, "vendor_id")
	})

	t.Run("vendor level labels", func(t *testing.T) {
		meta := &PaymentSecretMetadata{
			Env:      "prod",
			TenantID: "t_123",
			VendorID: "v_99",
			Provider: PaymentProviderRazorpay,
			KeyName:  KeyRazorpayKeyID,
			Scope:    PaymentScopeVendor,
		}

		labels := BuildPaymentSecretLabels(meta)

		assert.Equal(t, "prod", labels["environment"])
		assert.Equal(t, "payment", labels["category"])
		assert.Equal(t, "razorpay", labels["provider"])
		assert.Equal(t, "t_123", labels["tenant_id"])
		assert.Equal(t, "vendor", labels["scope"])
		assert.Equal(t, "v_99", labels["vendor_id"])
		assert.Equal(t, "secret-provisioner", labels["managed_by"])
	})
}

func TestValidatePaymentProvider(t *testing.T) {
	tests := []struct {
		provider PaymentProvider
		valid    bool
	}{
		{PaymentProviderStripe, true},
		{PaymentProviderRazorpay, true},
		{PaymentProviderPayPal, true},
		{PaymentProviderPhonePe, true},
		{PaymentProviderGooglePay, true},
		{PaymentProvider("unknown"), false},
		{PaymentProvider(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := ValidatePaymentProvider(tt.provider)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestValidatePaymentKeyName(t *testing.T) {
	tests := []struct {
		provider PaymentProvider
		keyName  PaymentKeyName
		valid    bool
	}{
		{PaymentProviderStripe, KeyStripeAPIKey, true},
		{PaymentProviderStripe, KeyStripeWebhookSecret, true},
		{PaymentProviderStripe, KeyRazorpayKeyID, false},
		{PaymentProviderRazorpay, KeyRazorpayKeyID, true},
		{PaymentProviderRazorpay, KeyRazorpayKeySecret, true},
		{PaymentProviderRazorpay, KeyStripeAPIKey, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider)+"-"+string(tt.keyName), func(t *testing.T) {
			result := ValidatePaymentKeyName(tt.provider, tt.keyName)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestSanitizeComponent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"UPPERCASE", "uppercase"},
		{"with spaces", "with-spaces"},
		{"special@chars!", "special-chars"},
		{"multiple---dashes", "multiple-dashes"},
		{"-leading-trailing-", "leading-trailing"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeComponent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that building and parsing a secret name are consistent
	testCases := []struct {
		env      string
		tenantID string
		vendorID string
		provider PaymentProvider
		keyName  PaymentKeyName
	}{
		{"prod", "t_123", "", PaymentProviderStripe, KeyStripeAPIKey},
		{"prod", "t_123", "v_99", PaymentProviderStripe, KeyStripeWebhookSecret},
		{"dev", "t_456", "", PaymentProviderRazorpay, KeyRazorpayKeyID},
		{"dev", "t_456", "v_88", PaymentProviderRazorpay, KeyRazorpayKeySecret},
	}

	for _, tc := range testCases {
		name := BuildPaymentSecretName(tc.env, tc.tenantID, tc.vendorID, tc.provider, tc.keyName)
		parsed, err := ParsePaymentSecretName(name)
		require.NoError(t, err)

		assert.Equal(t, tc.env, parsed.Env)
		assert.Equal(t, tc.tenantID, parsed.TenantID)
		assert.Equal(t, tc.vendorID, parsed.VendorID)
		assert.Equal(t, tc.provider, parsed.Provider)
		assert.Equal(t, tc.keyName, parsed.KeyName)
	}
}

func TestBuildPaymentSecretNameWithNames(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		tenantID   string
		tenantName string
		vendorID   string
		vendorName string
		provider   PaymentProvider
		keyName    PaymentKeyName
		expected   string
	}{
		{
			name:       "tenant level with name",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "Acme Corp",
			vendorID:   "",
			vendorName: "",
			provider:   PaymentProviderStripe,
			keyName:    KeyStripeAPIKey,
			expected:   "prod-acme-corp-t_123-stripe-api-key",
		},
		{
			name:       "tenant level without name falls back",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "",
			vendorID:   "",
			vendorName: "",
			provider:   PaymentProviderStripe,
			keyName:    KeyStripeAPIKey,
			expected:   "prod-tenant-t_123-stripe-api-key",
		},
		{
			name:       "vendor level with both names",
			env:        "dev",
			tenantID:   "t_456",
			tenantName: "Test Store",
			vendorID:   "v_99",
			vendorName: "Electronics Shop",
			provider:   PaymentProviderRazorpay,
			keyName:    KeyRazorpayKeyID,
			expected:   "dev-test-store-t_456-electronics-shop-v_99-razorpay-key-id",
		},
		{
			name:       "vendor level with tenant name only",
			env:        "prod",
			tenantID:   "t_789",
			tenantName: "Big Retail",
			vendorID:   "v_55",
			vendorName: "",
			provider:   PaymentProviderStripe,
			keyName:    KeyStripeWebhookSecret,
			expected:   "prod-big-retail-t_789-vendor-v_55-stripe-webhook-secret",
		},
		{
			name:       "sanitizes special characters in names",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "Joe's Shop & More!",
			vendorID:   "",
			vendorName: "",
			provider:   PaymentProviderStripe,
			keyName:    KeyStripeAPIKey,
			expected:   "prod-joe-s-shop-more-t_123-stripe-api-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPaymentSecretNameWithNames(
				tt.env, tt.tenantID, tt.tenantName,
				tt.vendorID, tt.vendorName,
				tt.provider, tt.keyName,
			)
			assert.Equal(t, tt.expected, result)
		})
	}
}
