package secrets

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ShippingProvider constants
// ---------------------------------------------------------------------------

func TestShippingProviderConstants(t *testing.T) {
	tests := []struct {
		name     string
		provider ShippingProvider
		expected string
	}{
		{"Shiprocket", ShippingProviderShiprocket, "shiprocket"},
		{"Delhivery", ShippingProviderDelhivery, "delhivery"},
		{"Bluedart", ShippingProviderBluedart, "bluedart"},
		{"DTDC", ShippingProviderDTDC, "dtdc"},
		{"Shippo", ShippingProviderShippo, "shippo"},
		{"ShipEngine", ShippingProviderShipEngine, "shipengine"},
		{"FedEx", ShippingProviderFedEx, "fedex"},
		{"UPS", ShippingProviderUPS, "ups"},
		{"DHL", ShippingProviderDHL, "dhl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, ShippingProvider(tt.expected), tt.provider)
		})
	}
}

func TestShippingKeyNameConstants(t *testing.T) {
	tests := []struct {
		name     string
		keyName  ShippingKeyName
		expected string
	}{
		{"APIKey", ShippingKeyAPIKey, "api-key"},
		{"APISecret", ShippingKeyAPISecret, "api-secret"},
		{"APIEmail", ShippingKeyAPIEmail, "api-email"},
		{"APIPassword", ShippingKeyAPIPassword, "api-password"},
		{"APIToken", ShippingKeyAPIToken, "api-token"},
		{"WebhookSecret", ShippingKeyWebhookSecret, "webhook-secret"},
		{"PickupLocation", ShippingKeyPickupLocation, "pickup-location"},
		{"AccountNumber", ShippingKeyAccountNumber, "account-number"},
		{"LicenseKey", ShippingKeyLicenseKey, "license-key"},
		{"ClientID", ShippingKeyClientID, "client-id"},
		{"ClientSecret", ShippingKeyClientSecret, "client-secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, ShippingKeyName(tt.expected), tt.keyName)
		})
	}
}

func TestShippingScopeConstants(t *testing.T) {
	assert.Equal(t, ShippingSecretScope("tenant"), ShippingScopeTenant)
	assert.Equal(t, ShippingSecretScope("vendor"), ShippingScopeVendor)
}

// ---------------------------------------------------------------------------
// GetSupportedShippingProviders
// ---------------------------------------------------------------------------

func TestGetSupportedShippingProviders(t *testing.T) {
	providers := GetSupportedShippingProviders()

	require.Len(t, providers, 9)

	expected := []ShippingProvider{
		ShippingProviderShiprocket,
		ShippingProviderDelhivery,
		ShippingProviderBluedart,
		ShippingProviderDTDC,
		ShippingProviderShippo,
		ShippingProviderShipEngine,
		ShippingProviderFedEx,
		ShippingProviderUPS,
		ShippingProviderDHL,
	}

	assert.Equal(t, expected, providers)
}

// ---------------------------------------------------------------------------
// BuildShippingSecretName (ShippingSecretName equivalent)
// ---------------------------------------------------------------------------

func TestBuildShippingSecretName(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		tenantID string
		vendorID string
		provider ShippingProvider
		keyName  ShippingKeyName
		expected string
	}{
		{
			name:     "tenant level shiprocket api email",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "",
			provider: ShippingProviderShiprocket,
			keyName:  ShippingKeyAPIEmail,
			expected: "prod-tenant-t_123-shipping-shiprocket-api-email",
		},
		{
			name:     "tenant level delhivery api token",
			env:      "dev",
			tenantID: "t_456",
			vendorID: "",
			provider: ShippingProviderDelhivery,
			keyName:  ShippingKeyAPIToken,
			expected: "dev-tenant-t_456-shipping-delhivery-api-token",
		},
		{
			name:     "vendor level delhivery api token",
			env:      "dev",
			tenantID: "t_123",
			vendorID: "v_99",
			provider: ShippingProviderDelhivery,
			keyName:  ShippingKeyAPIToken,
			expected: "dev-tenant-t_123-vendor-v_99-shipping-delhivery-api-token",
		},
		{
			name:     "tenant level fedex client id",
			env:      "prod",
			tenantID: "t_789",
			vendorID: "",
			provider: ShippingProviderFedEx,
			keyName:  ShippingKeyClientID,
			expected: "prod-tenant-t_789-shipping-fedex-client-id",
		},
		{
			name:     "vendor level fedex client secret",
			env:      "prod",
			tenantID: "t_789",
			vendorID: "v_10",
			provider: ShippingProviderFedEx,
			keyName:  ShippingKeyClientSecret,
			expected: "prod-tenant-t_789-vendor-v_10-shipping-fedex-client-secret",
		},
		{
			name:     "tenant level ups account number",
			env:      "staging",
			tenantID: "t_100",
			vendorID: "",
			provider: ShippingProviderUPS,
			keyName:  ShippingKeyAccountNumber,
			expected: "staging-tenant-t_100-shipping-ups-account-number",
		},
		{
			name:     "tenant level dhl api key",
			env:      "prod",
			tenantID: "t_200",
			vendorID: "",
			provider: ShippingProviderDHL,
			keyName:  ShippingKeyAPIKey,
			expected: "prod-tenant-t_200-shipping-dhl-api-key",
		},
		{
			name:     "tenant level shippo api key",
			env:      "prod",
			tenantID: "t_300",
			vendorID: "",
			provider: ShippingProviderShippo,
			keyName:  ShippingKeyAPIKey,
			expected: "prod-tenant-t_300-shipping-shippo-api-key",
		},
		{
			name:     "tenant level shipengine api key",
			env:      "prod",
			tenantID: "t_400",
			vendorID: "",
			provider: ShippingProviderShipEngine,
			keyName:  ShippingKeyAPIKey,
			expected: "prod-tenant-t_400-shipping-shipengine-api-key",
		},
		{
			name:     "tenant level bluedart license key",
			env:      "prod",
			tenantID: "t_500",
			vendorID: "",
			provider: ShippingProviderBluedart,
			keyName:  ShippingKeyLicenseKey,
			expected: "prod-tenant-t_500-shipping-bluedart-license-key",
		},
		{
			name:     "tenant level dtdc api key",
			env:      "prod",
			tenantID: "t_600",
			vendorID: "",
			provider: ShippingProviderDTDC,
			keyName:  ShippingKeyAPIKey,
			expected: "prod-tenant-t_600-shipping-dtdc-api-key",
		},
		{
			name:     "sanitizes uppercase env",
			env:      "PROD",
			tenantID: "T_ABC",
			vendorID: "",
			provider: ShippingProviderShiprocket,
			keyName:  ShippingKeyAPIEmail,
			expected: "prod-tenant-t_abc-shipping-shiprocket-api-email",
		},
		{
			name:     "sanitizes special characters in tenantID",
			env:      "prod",
			tenantID: "tenant@123!",
			vendorID: "",
			provider: ShippingProviderShiprocket,
			keyName:  ShippingKeyAPIEmail,
			expected: "prod-tenant-tenant-123-shipping-shiprocket-api-email",
		},
		{
			name:     "sanitizes special characters in vendorID",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "vendor@99!",
			provider: ShippingProviderDelhivery,
			keyName:  ShippingKeyAPIToken,
			expected: "prod-tenant-t_123-vendor-vendor-99-shipping-delhivery-api-token",
		},
		{
			name:     "webhook secret key",
			env:      "prod",
			tenantID: "t_123",
			vendorID: "",
			provider: ShippingProviderShiprocket,
			keyName:  ShippingKeyWebhookSecret,
			expected: "prod-tenant-t_123-shipping-shiprocket-webhook-secret",
		},
		{
			name:     "pickup location key",
			env:      "dev",
			tenantID: "t_123",
			vendorID: "v_1",
			provider: ShippingProviderDelhivery,
			keyName:  ShippingKeyPickupLocation,
			expected: "dev-tenant-t_123-vendor-v_1-shipping-delhivery-pickup-location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildShippingSecretName(tt.env, tt.tenantID, tt.vendorID, tt.provider, tt.keyName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// BuildShippingSecretNameWithNames (ShippingSecretKey / named variant)
// ---------------------------------------------------------------------------

func TestBuildShippingSecretNameWithNames(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		tenantID   string
		tenantName string
		vendorID   string
		vendorName string
		provider   ShippingProvider
		keyName    ShippingKeyName
		expected   string
	}{
		{
			name:       "tenant level with name",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "Acme Corp",
			vendorID:   "",
			vendorName: "",
			provider:   ShippingProviderShiprocket,
			keyName:    ShippingKeyAPIEmail,
			expected:   "prod-acme-corp-t_123-shipping-shiprocket-api-email",
		},
		{
			name:       "tenant level without name falls back to ID-only format",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "",
			vendorID:   "",
			vendorName: "",
			provider:   ShippingProviderShiprocket,
			keyName:    ShippingKeyAPIEmail,
			expected:   "prod-tenant-t_123-shipping-shiprocket-api-email",
		},
		{
			name:       "vendor level with both names",
			env:        "dev",
			tenantID:   "t_456",
			tenantName: "Test Store",
			vendorID:   "v_99",
			vendorName: "Electronics Shop",
			provider:   ShippingProviderDelhivery,
			keyName:    ShippingKeyAPIToken,
			expected:   "dev-test-store-t_456-electronics-shop-v_99-shipping-delhivery-api-token",
		},
		{
			name:       "vendor level with tenant name only",
			env:        "prod",
			tenantID:   "t_789",
			tenantName: "Big Retail",
			vendorID:   "v_55",
			vendorName: "",
			provider:   ShippingProviderFedEx,
			keyName:    ShippingKeyClientID,
			expected:   "prod-big-retail-t_789-vendor-v_55-shipping-fedex-client-id",
		},
		{
			name:       "sanitizes special characters in tenant name",
			env:        "prod",
			tenantID:   "t_123",
			tenantName: "Joe's Shop & More!",
			vendorID:   "",
			vendorName: "",
			provider:   ShippingProviderShiprocket,
			keyName:    ShippingKeyAPIEmail,
			expected:   "prod-joe-s-shop-more-t_123-shipping-shiprocket-api-email",
		},
		{
			name:       "vendor level without names falls back to ID-only format",
			env:        "dev",
			tenantID:   "t_111",
			tenantName: "",
			vendorID:   "v_22",
			vendorName: "",
			provider:   ShippingProviderUPS,
			keyName:    ShippingKeyClientSecret,
			expected:   "dev-tenant-t_111-vendor-v_22-shipping-ups-client-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildShippingSecretNameWithNames(
				tt.env, tt.tenantID, tt.tenantName,
				tt.vendorID, tt.vendorName,
				tt.provider, tt.keyName,
			)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// ParseShippingSecretName
// ---------------------------------------------------------------------------

func TestParseShippingSecretName(t *testing.T) {
	tests := []struct {
		name        string
		secretName  string
		expected    *ShippingSecretMetadata
		expectError bool
	}{
		{
			name:       "tenant level shiprocket api email",
			secretName: "prod-tenant-t_123-shipping-shiprocket-api-email",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_123",
				VendorID: "",
				Provider: ShippingProviderShiprocket,
				KeyName:  ShippingKeyAPIEmail,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level delhivery api token",
			secretName: "dev-tenant-t_456-shipping-delhivery-api-token",
			expected: &ShippingSecretMetadata{
				Env:      "dev",
				TenantID: "t_456",
				VendorID: "",
				Provider: ShippingProviderDelhivery,
				KeyName:  ShippingKeyAPIToken,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "vendor level delhivery api token",
			secretName: "dev-tenant-t_123-vendor-v_99-shipping-delhivery-api-token",
			expected: &ShippingSecretMetadata{
				Env:      "dev",
				TenantID: "t_123",
				VendorID: "v_99",
				Provider: ShippingProviderDelhivery,
				KeyName:  ShippingKeyAPIToken,
				Scope:    ShippingScopeVendor,
			},
		},
		{
			name:       "tenant level fedex client id",
			secretName: "prod-tenant-t_789-shipping-fedex-client-id",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_789",
				VendorID: "",
				Provider: ShippingProviderFedEx,
				KeyName:  ShippingKeyClientID,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "vendor level fedex client secret",
			secretName: "prod-tenant-t_789-vendor-v_10-shipping-fedex-client-secret",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_789",
				VendorID: "v_10",
				Provider: ShippingProviderFedEx,
				KeyName:  ShippingKeyClientSecret,
				Scope:    ShippingScopeVendor,
			},
		},
		{
			name:       "tenant level ups account number",
			secretName: "staging-tenant-t_100-shipping-ups-account-number",
			expected: &ShippingSecretMetadata{
				Env:      "staging",
				TenantID: "t_100",
				VendorID: "",
				Provider: ShippingProviderUPS,
				KeyName:  ShippingKeyAccountNumber,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level dhl api key",
			secretName: "prod-tenant-t_200-shipping-dhl-api-key",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_200",
				VendorID: "",
				Provider: ShippingProviderDHL,
				KeyName:  ShippingKeyAPIKey,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level shippo api key",
			secretName: "prod-tenant-t_300-shipping-shippo-api-key",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_300",
				VendorID: "",
				Provider: ShippingProviderShippo,
				KeyName:  ShippingKeyAPIKey,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level shipengine api key",
			secretName: "prod-tenant-t_400-shipping-shipengine-api-key",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_400",
				VendorID: "",
				Provider: ShippingProviderShipEngine,
				KeyName:  ShippingKeyAPIKey,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level bluedart license key",
			secretName: "prod-tenant-t_500-shipping-bluedart-license-key",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_500",
				VendorID: "",
				Provider: ShippingProviderBluedart,
				KeyName:  ShippingKeyLicenseKey,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "tenant level dtdc api key",
			secretName: "prod-tenant-t_600-shipping-dtdc-api-key",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_600",
				VendorID: "",
				Provider: ShippingProviderDTDC,
				KeyName:  ShippingKeyAPIKey,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "webhook secret key",
			secretName: "prod-tenant-t_123-shipping-shiprocket-webhook-secret",
			expected: &ShippingSecretMetadata{
				Env:      "prod",
				TenantID: "t_123",
				VendorID: "",
				Provider: ShippingProviderShiprocket,
				KeyName:  ShippingKeyWebhookSecret,
				Scope:    ShippingScopeTenant,
			},
		},
		{
			name:       "vendor level pickup location",
			secretName: "dev-tenant-t_123-vendor-v_1-shipping-delhivery-pickup-location",
			expected: &ShippingSecretMetadata{
				Env:      "dev",
				TenantID: "t_123",
				VendorID: "v_1",
				Provider: ShippingProviderDelhivery,
				KeyName:  ShippingKeyPickupLocation,
				Scope:    ShippingScopeVendor,
			},
		},
		{
			name:        "empty string returns error",
			secretName:  "",
			expectError: true,
		},
		{
			name:        "no shipping segment returns error",
			secretName:  "invalid-secret-name",
			expectError: true,
		},
		{
			name:        "shipping segment present but no known provider",
			secretName:  "prod-tenant-t_123-shipping-unknowncarrier-api-key",
			expectError: true,
		},
		{
			name:        "missing tenant segment returns error",
			secretName:  "prod-shipping-shiprocket-api-key",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseShippingSecretName(tt.secretName)
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

// ---------------------------------------------------------------------------
// ValidateShippingProvider
// ---------------------------------------------------------------------------

func TestValidateShippingProvider(t *testing.T) {
	tests := []struct {
		provider ShippingProvider
		valid    bool
	}{
		{ShippingProviderShiprocket, true},
		{ShippingProviderDelhivery, true},
		{ShippingProviderBluedart, true},
		{ShippingProviderDTDC, true},
		{ShippingProviderShippo, true},
		{ShippingProviderShipEngine, true},
		{ShippingProviderFedEx, true},
		{ShippingProviderUPS, true},
		{ShippingProviderDHL, true},
		{ShippingProvider("unknown"), false},
		{ShippingProvider(""), false},
		{ShippingProvider("FEDEX"), false},  // case-sensitive
		{ShippingProvider("Shiprocket"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := ValidateShippingProvider(tt.provider)
			assert.Equal(t, tt.valid, result)
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateShippingKeyName
// ---------------------------------------------------------------------------

func TestValidateShippingKeyName(t *testing.T) {
	tests := []struct {
		name     string
		provider ShippingProvider
		keyName  ShippingKeyName
		valid    bool
	}{
		// Shiprocket required keys
		{"shiprocket api-email required", ShippingProviderShiprocket, ShippingKeyAPIEmail, true},
		{"shiprocket api-password required", ShippingProviderShiprocket, ShippingKeyAPIPassword, true},
		// Shiprocket optional keys
		{"shiprocket webhook-secret optional", ShippingProviderShiprocket, ShippingKeyWebhookSecret, true},
		// Shiprocket invalid key
		{"shiprocket api-key not valid", ShippingProviderShiprocket, ShippingKeyAPIKey, false},
		{"shiprocket api-token not valid", ShippingProviderShiprocket, ShippingKeyAPIToken, false},

		// Delhivery required keys
		{"delhivery api-token required", ShippingProviderDelhivery, ShippingKeyAPIToken, true},
		// Delhivery optional keys
		{"delhivery pickup-location optional", ShippingProviderDelhivery, ShippingKeyPickupLocation, true},
		{"delhivery webhook-secret optional", ShippingProviderDelhivery, ShippingKeyWebhookSecret, true},
		// Delhivery invalid key
		{"delhivery api-key not valid", ShippingProviderDelhivery, ShippingKeyAPIKey, false},

		// Bluedart required keys
		{"bluedart api-key required", ShippingProviderBluedart, ShippingKeyAPIKey, true},
		{"bluedart license-key required", ShippingProviderBluedart, ShippingKeyLicenseKey, true},
		// Bluedart invalid key
		{"bluedart api-token not valid", ShippingProviderBluedart, ShippingKeyAPIToken, false},

		// DTDC required keys
		{"dtdc api-key required", ShippingProviderDTDC, ShippingKeyAPIKey, true},
		// DTDC invalid key
		{"dtdc api-token not valid", ShippingProviderDTDC, ShippingKeyAPIToken, false},

		// FedEx required keys
		{"fedex client-id required", ShippingProviderFedEx, ShippingKeyClientID, true},
		{"fedex client-secret required", ShippingProviderFedEx, ShippingKeyClientSecret, true},
		// FedEx optional keys
		{"fedex account-number optional", ShippingProviderFedEx, ShippingKeyAccountNumber, true},
		// FedEx invalid key
		{"fedex api-key not valid", ShippingProviderFedEx, ShippingKeyAPIKey, false},

		// UPS required keys
		{"ups client-id required", ShippingProviderUPS, ShippingKeyClientID, true},
		{"ups client-secret required", ShippingProviderUPS, ShippingKeyClientSecret, true},
		// UPS optional keys
		{"ups account-number optional", ShippingProviderUPS, ShippingKeyAccountNumber, true},
		// UPS invalid key
		{"ups api-key not valid", ShippingProviderUPS, ShippingKeyAPIKey, false},

		// DHL required keys
		{"dhl api-key required", ShippingProviderDHL, ShippingKeyAPIKey, true},
		{"dhl api-secret required", ShippingProviderDHL, ShippingKeyAPISecret, true},
		// DHL optional keys
		{"dhl account-number optional", ShippingProviderDHL, ShippingKeyAccountNumber, true},
		// DHL invalid key
		{"dhl api-token not valid", ShippingProviderDHL, ShippingKeyAPIToken, false},

		// Shippo required keys
		{"shippo api-key required", ShippingProviderShippo, ShippingKeyAPIKey, true},
		// Shippo invalid key
		{"shippo api-token not valid", ShippingProviderShippo, ShippingKeyAPIToken, false},

		// ShipEngine required keys
		{"shipengine api-key required", ShippingProviderShipEngine, ShippingKeyAPIKey, true},
		// ShipEngine invalid key
		{"shipengine api-token not valid", ShippingProviderShipEngine, ShippingKeyAPIToken, false},

		// Unknown provider returns false for all keys
		{"unknown provider any key", ShippingProvider("unknown"), ShippingKeyAPIKey, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateShippingKeyName(tt.provider, tt.keyName)
			assert.Equal(t, tt.valid, result)
		})
	}
}

// ---------------------------------------------------------------------------
// GetShippingProviderRequiredKeys
// ---------------------------------------------------------------------------

func TestGetShippingProviderRequiredKeys(t *testing.T) {
	tests := []struct {
		provider ShippingProvider
		expected []ShippingKeyName
	}{
		{
			provider: ShippingProviderShiprocket,
			expected: []ShippingKeyName{ShippingKeyAPIEmail, ShippingKeyAPIPassword},
		},
		{
			provider: ShippingProviderDelhivery,
			expected: []ShippingKeyName{ShippingKeyAPIToken},
		},
		{
			provider: ShippingProviderBluedart,
			expected: []ShippingKeyName{ShippingKeyAPIKey, ShippingKeyLicenseKey},
		},
		{
			provider: ShippingProviderDTDC,
			expected: []ShippingKeyName{ShippingKeyAPIKey},
		},
		{
			provider: ShippingProviderFedEx,
			expected: []ShippingKeyName{ShippingKeyClientID, ShippingKeyClientSecret},
		},
		{
			provider: ShippingProviderUPS,
			expected: []ShippingKeyName{ShippingKeyClientID, ShippingKeyClientSecret},
		},
		{
			provider: ShippingProviderDHL,
			expected: []ShippingKeyName{ShippingKeyAPIKey, ShippingKeyAPISecret},
		},
		{
			provider: ShippingProviderShippo,
			expected: []ShippingKeyName{ShippingKeyAPIKey},
		},
		{
			provider: ShippingProviderShipEngine,
			expected: []ShippingKeyName{ShippingKeyAPIKey},
		},
		{
			provider: ShippingProvider("unknown"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := GetShippingProviderRequiredKeys(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// GetShippingProviderOptionalKeys
// ---------------------------------------------------------------------------

func TestGetShippingProviderOptionalKeys(t *testing.T) {
	tests := []struct {
		provider ShippingProvider
		expected []ShippingKeyName
	}{
		{
			provider: ShippingProviderShiprocket,
			expected: []ShippingKeyName{ShippingKeyWebhookSecret},
		},
		{
			provider: ShippingProviderDelhivery,
			expected: []ShippingKeyName{ShippingKeyPickupLocation, ShippingKeyWebhookSecret},
		},
		{
			provider: ShippingProviderFedEx,
			expected: []ShippingKeyName{ShippingKeyAccountNumber},
		},
		{
			provider: ShippingProviderUPS,
			expected: []ShippingKeyName{ShippingKeyAccountNumber},
		},
		{
			provider: ShippingProviderDHL,
			expected: []ShippingKeyName{ShippingKeyAccountNumber},
		},
		// Providers with no optional keys
		{
			provider: ShippingProviderBluedart,
			expected: nil,
		},
		{
			provider: ShippingProviderDTDC,
			expected: nil,
		},
		{
			provider: ShippingProviderShippo,
			expected: nil,
		},
		{
			provider: ShippingProviderShipEngine,
			expected: nil,
		},
		{
			provider: ShippingProvider("unknown"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := GetShippingProviderOptionalKeys(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// GetAllShippingProviderKeys
// ---------------------------------------------------------------------------

func TestGetAllShippingProviderKeys(t *testing.T) {
	tests := []struct {
		provider ShippingProvider
		expected []ShippingKeyName
	}{
		{
			provider: ShippingProviderShiprocket,
			expected: []ShippingKeyName{
				ShippingKeyAPIEmail,
				ShippingKeyAPIPassword,
				ShippingKeyWebhookSecret,
			},
		},
		{
			provider: ShippingProviderDelhivery,
			expected: []ShippingKeyName{
				ShippingKeyAPIToken,
				ShippingKeyPickupLocation,
				ShippingKeyWebhookSecret,
			},
		},
		{
			provider: ShippingProviderBluedart,
			expected: []ShippingKeyName{
				ShippingKeyAPIKey,
				ShippingKeyLicenseKey,
			},
		},
		{
			provider: ShippingProviderDTDC,
			expected: []ShippingKeyName{
				ShippingKeyAPIKey,
			},
		},
		{
			provider: ShippingProviderFedEx,
			expected: []ShippingKeyName{
				ShippingKeyClientID,
				ShippingKeyClientSecret,
				ShippingKeyAccountNumber,
			},
		},
		{
			provider: ShippingProviderUPS,
			expected: []ShippingKeyName{
				ShippingKeyClientID,
				ShippingKeyClientSecret,
				ShippingKeyAccountNumber,
			},
		},
		{
			provider: ShippingProviderDHL,
			expected: []ShippingKeyName{
				ShippingKeyAPIKey,
				ShippingKeyAPISecret,
				ShippingKeyAccountNumber,
			},
		},
		{
			provider: ShippingProvider("unknown"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := GetAllShippingProviderKeys(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// BuildShippingSecretLabels
// ---------------------------------------------------------------------------

func TestBuildShippingSecretLabels(t *testing.T) {
	t.Run("tenant level labels", func(t *testing.T) {
		meta := &ShippingSecretMetadata{
			Env:      "prod",
			TenantID: "t_123",
			VendorID: "",
			Provider: ShippingProviderShiprocket,
			KeyName:  ShippingKeyAPIEmail,
			Scope:    ShippingScopeTenant,
		}

		labels := BuildShippingSecretLabels(meta)

		assert.Equal(t, "prod", labels["environment"])
		assert.Equal(t, "shipping", labels["category"])
		assert.Equal(t, "shiprocket", labels["provider"])
		assert.Equal(t, "t_123", labels["tenant_id"])
		assert.Equal(t, "tenant", labels["scope"])
		assert.Equal(t, "secret-provisioner", labels["managed_by"])
		assert.NotContains(t, labels, "vendor_id")
	})

	t.Run("vendor level labels", func(t *testing.T) {
		meta := &ShippingSecretMetadata{
			Env:      "prod",
			TenantID: "t_123",
			VendorID: "v_99",
			Provider: ShippingProviderDelhivery,
			KeyName:  ShippingKeyAPIToken,
			Scope:    ShippingScopeVendor,
		}

		labels := BuildShippingSecretLabels(meta)

		assert.Equal(t, "prod", labels["environment"])
		assert.Equal(t, "shipping", labels["category"])
		assert.Equal(t, "delhivery", labels["provider"])
		assert.Equal(t, "t_123", labels["tenant_id"])
		assert.Equal(t, "vendor", labels["scope"])
		assert.Equal(t, "v_99", labels["vendor_id"])
		assert.Equal(t, "secret-provisioner", labels["managed_by"])
	})

	t.Run("dev environment labels", func(t *testing.T) {
		meta := &ShippingSecretMetadata{
			Env:      "dev",
			TenantID: "t_456",
			VendorID: "",
			Provider: ShippingProviderFedEx,
			KeyName:  ShippingKeyClientID,
			Scope:    ShippingScopeTenant,
		}

		labels := BuildShippingSecretLabels(meta)

		assert.Equal(t, "dev", labels["environment"])
		assert.Equal(t, "fedex", labels["provider"])
		assert.Equal(t, "t_456", labels["tenant_id"])
		assert.Equal(t, "tenant", labels["scope"])
	})
}

// ---------------------------------------------------------------------------
// Round-trip: BuildShippingSecretName <-> ParseShippingSecretName
// ---------------------------------------------------------------------------

func TestShippingSecretNameRoundTrip(t *testing.T) {
	testCases := []struct {
		env      string
		tenantID string
		vendorID string
		provider ShippingProvider
		keyName  ShippingKeyName
	}{
		{"prod", "t_123", "", ShippingProviderShiprocket, ShippingKeyAPIEmail},
		{"prod", "t_123", "v_99", ShippingProviderShiprocket, ShippingKeyAPIPassword},
		{"dev", "t_456", "", ShippingProviderDelhivery, ShippingKeyAPIToken},
		{"dev", "t_456", "v_88", ShippingProviderDelhivery, ShippingKeyPickupLocation},
		{"prod", "t_789", "", ShippingProviderFedEx, ShippingKeyClientID},
		{"prod", "t_789", "v_10", ShippingProviderFedEx, ShippingKeyClientSecret},
		{"staging", "t_100", "", ShippingProviderUPS, ShippingKeyAccountNumber},
		{"prod", "t_200", "", ShippingProviderDHL, ShippingKeyAPIKey},
		{"prod", "t_300", "", ShippingProviderShippo, ShippingKeyAPIKey},
		{"prod", "t_400", "", ShippingProviderShipEngine, ShippingKeyAPIKey},
		{"prod", "t_500", "", ShippingProviderBluedart, ShippingKeyLicenseKey},
		{"prod", "t_600", "", ShippingProviderDTDC, ShippingKeyAPIKey},
	}

	for _, tc := range testCases {
		name := BuildShippingSecretName(tc.env, tc.tenantID, tc.vendorID, tc.provider, tc.keyName)
		parsed, err := ParseShippingSecretName(name)
		require.NoError(t, err, "failed to parse secret name: %s", name)

		assert.Equal(t, tc.env, parsed.Env, "env mismatch for %s", name)
		assert.Equal(t, tc.tenantID, parsed.TenantID, "tenantID mismatch for %s", name)
		assert.Equal(t, tc.vendorID, parsed.VendorID, "vendorID mismatch for %s", name)
		assert.Equal(t, tc.provider, parsed.Provider, "provider mismatch for %s", name)
		assert.Equal(t, tc.keyName, parsed.KeyName, "keyName mismatch for %s", name)

		if tc.vendorID == "" {
			assert.Equal(t, ShippingScopeTenant, parsed.Scope, "scope mismatch for %s", name)
		} else {
			assert.Equal(t, ShippingScopeVendor, parsed.Scope, "scope mismatch for %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// shippingSecretCache (internal cache functions)
// ---------------------------------------------------------------------------

func TestShippingSecretCache(t *testing.T) {
	t.Run("set and get returns cached value", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)
		cache.set("key1", "value1")

		val, ok := cache.get("key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", val)
	})

	t.Run("get missing key returns false", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)

		val, ok := cache.get("nonexistent")
		assert.False(t, ok)
		assert.Empty(t, val)
	})

	t.Run("expired entry returns false", func(t *testing.T) {
		cache := newShippingSecretCache(1 * time.Millisecond)
		cache.set("key1", "value1")

		// Wait for the TTL to expire
		time.Sleep(10 * time.Millisecond)

		val, ok := cache.get("key1")
		assert.False(t, ok)
		assert.Empty(t, val)
	})

	t.Run("delete removes entry", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)
		cache.set("key1", "value1")
		cache.set("key2", "value2")

		cache.delete("key1")

		_, ok := cache.get("key1")
		assert.False(t, ok)

		// key2 should still be present
		val, ok := cache.get("key2")
		assert.True(t, ok)
		assert.Equal(t, "value2", val)
	})

	t.Run("clear removes all entries", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)
		cache.set("key1", "value1")
		cache.set("key2", "value2")
		cache.set("key3", "value3")

		cache.clear()

		_, ok1 := cache.get("key1")
		_, ok2 := cache.get("key2")
		_, ok3 := cache.get("key3")

		assert.False(t, ok1)
		assert.False(t, ok2)
		assert.False(t, ok3)
	})

	t.Run("overwrite existing entry", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)
		cache.set("key1", "original")
		cache.set("key1", "updated")

		val, ok := cache.get("key1")
		assert.True(t, ok)
		assert.Equal(t, "updated", val)
	})

	t.Run("delete on empty cache does not panic", func(t *testing.T) {
		cache := newShippingSecretCache(10 * time.Minute)
		// Should not panic
		cache.delete("nonexistent")
	})
}

// ---------------------------------------------------------------------------
// GetSecretOrEnv with USE_GCP_SECRET_MANAGER=false (env-based path)
// ---------------------------------------------------------------------------

func TestGetSecretOrEnvWithoutGCP(t *testing.T) {
	tests := []struct {
		name             string
		secretNameEnvVar string
		secretNameValue  string
		fallbackEnvVar   string
		fallbackValue    string
		defaultValue     string
		expected         string
	}{
		{
			name:           "returns fallback env var value",
			fallbackEnvVar: "TEST_SHIPPING_SECRET_FALLBACK",
			fallbackValue:  "env-value",
			defaultValue:   "",
			expected:       "env-value",
		},
		{
			name:         "returns default when no env var set",
			fallbackEnvVar: "TEST_SHIPPING_SECRET_FALLBACK_EMPTY",
			fallbackValue:  "",
			defaultValue:   "default-value",
			expected:       "default-value",
		},
		{
			name:           "fallback env var takes precedence over default",
			fallbackEnvVar: "TEST_SHIPPING_SECRET_FALLBACK_PRIO",
			fallbackValue:  "env-wins",
			defaultValue:   "default-loses",
			expected:       "env-wins",
		},
		{
			name:         "returns empty string when nothing configured",
			fallbackEnvVar: "TEST_SHIPPING_SECRET_FALLBACK_NONE",
			fallbackValue:  "",
			defaultValue:   "",
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure GCP is disabled
			t.Setenv("USE_GCP_SECRET_MANAGER", "false")

			if tt.fallbackValue != "" {
				t.Setenv(tt.fallbackEnvVar, tt.fallbackValue)
			} else {
				os.Unsetenv(tt.fallbackEnvVar)
			}

			secretNameEnvVar := "TEST_SHIPPING_SECRET_NAME_VAR"
			os.Unsetenv(secretNameEnvVar)

			result := GetSecretOrEnv(secretNameEnvVar, tt.fallbackEnvVar, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// GetDBPassword with env fallback
// ---------------------------------------------------------------------------

func TestGetDBPasswordEnvFallback(t *testing.T) {
	t.Run("returns DB_PASSWORD from environment", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		t.Setenv("DB_PASSWORD", "super-secret-db-pass")
		os.Unsetenv("DB_PASSWORD_SECRET_NAME")

		result := GetDBPassword()
		assert.Equal(t, "super-secret-db-pass", result)
	})

	t.Run("returns empty string when DB_PASSWORD not set", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_PASSWORD_SECRET_NAME")

		result := GetDBPassword()
		assert.Equal(t, "", result)
	})
}

// ---------------------------------------------------------------------------
// GetJWTSecret with env fallback
// ---------------------------------------------------------------------------

func TestGetJWTSecretEnvFallback(t *testing.T) {
	t.Run("returns JWT_SECRET from environment", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		t.Setenv("JWT_SECRET", "my-jwt-signing-key")
		os.Unsetenv("JWT_SECRET_NAME")

		result := GetJWTSecret()
		assert.Equal(t, "my-jwt-signing-key", result)
	})

	t.Run("returns empty string when JWT_SECRET not set", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_SECRET_NAME")

		result := GetJWTSecret()
		assert.Equal(t, "", result)
	})
}

// ---------------------------------------------------------------------------
// GetRedisPassword with env fallback
// ---------------------------------------------------------------------------

func TestGetRedisPasswordEnvFallback(t *testing.T) {
	t.Run("returns REDIS_PASSWORD from environment", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		t.Setenv("REDIS_PASSWORD", "redis-pass-123")
		os.Unsetenv("REDIS_PASSWORD_SECRET_NAME")

		result := GetRedisPassword()
		assert.Equal(t, "redis-pass-123", result)
	})

	t.Run("returns empty string when REDIS_PASSWORD not set", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_PASSWORD_SECRET_NAME")

		result := GetRedisPassword()
		assert.Equal(t, "", result)
	})
}

// ---------------------------------------------------------------------------
// EnvSecretFetcher.GetSecretOrEnv with nil fetcher (GCP disabled)
// ---------------------------------------------------------------------------

func TestEnvSecretFetcherGetSecretOrEnvNilFetcher(t *testing.T) {
	t.Run("nil fetcher falls through to env var", func(t *testing.T) {
		t.Setenv("TEST_FALLBACK_SHIPPING", "fallback-secret-value")
		os.Unsetenv("TEST_SECRET_NAME_SHIPPING")

		var fetcher *EnvSecretFetcher // nil fetcher = GCP disabled
		ctx := context.Background()

		result := fetcher.GetSecretOrEnv(ctx, "TEST_SECRET_NAME_SHIPPING", "TEST_FALLBACK_SHIPPING", "default")
		assert.Equal(t, "fallback-secret-value", result)
	})

	t.Run("nil fetcher returns default when env var not set", func(t *testing.T) {
		os.Unsetenv("TEST_FALLBACK_SHIPPING_EMPTY")
		os.Unsetenv("TEST_SECRET_NAME_SHIPPING_EMPTY")

		var fetcher *EnvSecretFetcher
		ctx := context.Background()

		result := fetcher.GetSecretOrEnv(ctx, "TEST_SECRET_NAME_SHIPPING_EMPTY", "TEST_FALLBACK_SHIPPING_EMPTY", "my-default")
		assert.Equal(t, "my-default", result)
	})
}

// ---------------------------------------------------------------------------
// LoadDatabasePassword, LoadJWTSecret, LoadRedisPassword with nil fetcher
// ---------------------------------------------------------------------------

func TestLoadSecretsWithNilFetcher(t *testing.T) {
	ctx := context.Background()
	var fetcher *EnvSecretFetcher

	t.Run("LoadDatabasePassword uses DB_PASSWORD env var", func(t *testing.T) {
		t.Setenv("DB_PASSWORD", "load-db-pass")
		os.Unsetenv("DB_PASSWORD_SECRET_NAME")

		result := LoadDatabasePassword(ctx, fetcher)
		assert.Equal(t, "load-db-pass", result)
	})

	t.Run("LoadDatabasePassword returns default when env not set", func(t *testing.T) {
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_PASSWORD_SECRET_NAME")

		result := LoadDatabasePassword(ctx, fetcher)
		assert.Equal(t, "password", result) // default value in the function
	})

	t.Run("LoadJWTSecret uses JWT_SECRET env var", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "load-jwt-secret")
		os.Unsetenv("JWT_SECRET_NAME")

		result := LoadJWTSecret(ctx, fetcher)
		assert.Equal(t, "load-jwt-secret", result)
	})

	t.Run("LoadJWTSecret returns default when env not set", func(t *testing.T) {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_SECRET_NAME")

		result := LoadJWTSecret(ctx, fetcher)
		assert.Equal(t, "default-jwt-secret", result) // default value in the function
	})

	t.Run("LoadRedisPassword uses REDIS_PASSWORD env var", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "load-redis-pass")
		os.Unsetenv("REDIS_PASSWORD_SECRET_NAME")

		result := LoadRedisPassword(ctx, fetcher)
		assert.Equal(t, "load-redis-pass", result)
	})

	t.Run("LoadRedisPassword returns empty string when env not set", func(t *testing.T) {
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_PASSWORD_SECRET_NAME")

		result := LoadRedisPassword(ctx, fetcher)
		assert.Equal(t, "", result) // default value in the function is ""
	})
}

// ---------------------------------------------------------------------------
// FetchSecretsWithTimeout
// ---------------------------------------------------------------------------

func TestFetchSecretsWithTimeout(t *testing.T) {
	t.Run("executes function within timeout", func(t *testing.T) {
		called := false
		err := FetchSecretsWithTimeout(5*time.Second, func(ctx context.Context) error {
			called = true
			return nil
		})

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("propagates error from function", func(t *testing.T) {
		err := FetchSecretsWithTimeout(5*time.Second, func(ctx context.Context) error {
			return assert.AnError
		})

		assert.Error(t, err)
	})

	t.Run("context deadline is set", func(t *testing.T) {
		var deadlineSet bool
		err := FetchSecretsWithTimeout(5*time.Second, func(ctx context.Context) error {
			_, ok := ctx.Deadline()
			deadlineSet = ok
			return nil
		})

		require.NoError(t, err)
		assert.True(t, deadlineSet, "expected context to have a deadline")
	})
}

// ---------------------------------------------------------------------------
// NewEnvSecretFetcher with GCP disabled
// ---------------------------------------------------------------------------

func TestNewEnvSecretFetcherGCPDisabled(t *testing.T) {
	t.Run("returns nil fetcher when USE_GCP_SECRET_MANAGER is false", func(t *testing.T) {
		t.Setenv("USE_GCP_SECRET_MANAGER", "false")

		ctx := context.Background()
		fetcher, err := NewEnvSecretFetcher(ctx)

		require.NoError(t, err)
		assert.Nil(t, fetcher)
	})

	t.Run("returns nil fetcher when USE_GCP_SECRET_MANAGER is not set", func(t *testing.T) {
		os.Unsetenv("USE_GCP_SECRET_MANAGER")

		ctx := context.Background()
		fetcher, err := NewEnvSecretFetcher(ctx)

		require.NoError(t, err)
		assert.Nil(t, fetcher)
	})
}

// ---------------------------------------------------------------------------
// EnvSecretFetcher.Close with nil fetcher
// ---------------------------------------------------------------------------

func TestEnvSecretFetcherCloseNil(t *testing.T) {
	var fetcher *EnvSecretFetcher
	err := fetcher.Close()
	assert.NoError(t, err, "Close on nil fetcher should not return an error")
}

// ---------------------------------------------------------------------------
// isShippingNotFoundError
// ---------------------------------------------------------------------------

func TestIsShippingNotFoundError(t *testing.T) {
	t.Run("nil error returns false", func(t *testing.T) {
		assert.False(t, isShippingNotFoundError(nil))
	})

	t.Run("ErrShippingSecretNotFound returns true", func(t *testing.T) {
		assert.True(t, isShippingNotFoundError(ErrShippingSecretNotFound))
	})

	t.Run("ErrShippingCarrierNotConfigured returns true", func(t *testing.T) {
		assert.True(t, isShippingNotFoundError(ErrShippingCarrierNotConfigured))
	})

	t.Run("unrelated error returns false", func(t *testing.T) {
		assert.False(t, isShippingNotFoundError(assert.AnError))
	})
}
