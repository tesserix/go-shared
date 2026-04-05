package secrets

import (
	"fmt"
	"strings"
)

// ShippingProvider represents supported shipping carriers
type ShippingProvider string

const (
	ShippingProviderShiprocket ShippingProvider = "shiprocket"
	ShippingProviderDelhivery  ShippingProvider = "delhivery"
	ShippingProviderBluedart   ShippingProvider = "bluedart"
	ShippingProviderDTDC       ShippingProvider = "dtdc"
	ShippingProviderShippo     ShippingProvider = "shippo"
	ShippingProviderShipEngine ShippingProvider = "shipengine"
	ShippingProviderFedEx      ShippingProvider = "fedex"
	ShippingProviderUPS        ShippingProvider = "ups"
	ShippingProviderDHL        ShippingProvider = "dhl"
)

// ShippingKeyName represents supported key names per shipping provider
type ShippingKeyName string

const (
	ShippingKeyAPIKey         ShippingKeyName = "api-key"
	ShippingKeyAPISecret      ShippingKeyName = "api-secret"
	ShippingKeyAPIEmail       ShippingKeyName = "api-email"
	ShippingKeyAPIPassword    ShippingKeyName = "api-password"
	ShippingKeyAPIToken       ShippingKeyName = "api-token"
	ShippingKeyWebhookSecret  ShippingKeyName = "webhook-secret"
	ShippingKeyPickupLocation ShippingKeyName = "pickup-location"
	ShippingKeyAccountNumber  ShippingKeyName = "account-number"
	ShippingKeyLicenseKey     ShippingKeyName = "license-key"
	ShippingKeyClientID       ShippingKeyName = "client-id"
	ShippingKeyClientSecret   ShippingKeyName = "client-secret"
)

// ShippingSecretScope represents the scope level for a shipping secret
type ShippingSecretScope string

const (
	ShippingScopeTenant ShippingSecretScope = "tenant"
	ShippingScopeVendor ShippingSecretScope = "vendor"
)

// ShippingSecretMetadata holds parsed secret name components
type ShippingSecretMetadata struct {
	Env      string
	TenantID string
	VendorID string // Empty for tenant-level
	Provider ShippingProvider
	KeyName  ShippingKeyName
	Scope    ShippingSecretScope
}

var (
	// Known shipping provider names for parsing (order matters - longer names first)
	knownShippingProviders = []string{
		"shipengine", "shiprocket", "delhivery", "bluedart", "dtdc", "shippo",
		"fedex", "ups", "dhl",
	}
)

// BuildShippingSecretName constructs a GCP Secret Manager secret name for shipping credentials.
//
// Naming convention:
//   - Tenant-level: {env}-tenant-{tenantId}-shipping-{provider}-{keyName}
//   - Vendor-level: {env}-tenant-{tenantId}-vendor-{vendorId}-shipping-{provider}-{keyName}
//
// Examples:
//   - prod-tenant-t_123-shipping-shiprocket-api-email
//   - dev-tenant-t_123-vendor-v_99-shipping-delhivery-api-token
func BuildShippingSecretName(env, tenantID, vendorID string, provider ShippingProvider, keyName ShippingKeyName) string {
	env = sanitizeComponent(env)
	tenantID = sanitizeComponent(tenantID)
	providerStr := sanitizeComponent(string(provider))
	keyNameStr := sanitizeComponent(string(keyName))

	if vendorID == "" {
		// Tenant-level: {env}-tenant-{tenantId}-shipping-{provider}-{keyName}
		return fmt.Sprintf("%s-tenant-%s-shipping-%s-%s", env, tenantID, providerStr, keyNameStr)
	}

	// Vendor-level: {env}-tenant-{tenantId}-vendor-{vendorId}-shipping-{provider}-{keyName}
	vendorID = sanitizeComponent(vendorID)
	return fmt.Sprintf("%s-tenant-%s-vendor-%s-shipping-%s-%s", env, tenantID, vendorID, providerStr, keyNameStr)
}

// BuildShippingSecretNameWithNames constructs a GCP Secret Manager secret name with human-readable names.
// This creates more readable secret names by including tenant/vendor names alongside IDs for uniqueness.
//
// Naming convention:
//   - Tenant-level: {env}-{tenantSlug}-{tenantId}-shipping-{provider}-{keyName}
//   - Vendor-level: {env}-{tenantSlug}-{tenantId}-{vendorSlug}-{vendorId}-shipping-{provider}-{keyName}
//
// Examples:
//   - prod-acme-corp-t-123-shipping-shiprocket-api-email
//   - dev-acme-corp-t-123-electronics-v-99-shipping-delhivery-api-token
//
// Note: Names are slugified (lowercase, special chars replaced with dashes).
// IDs ensure uniqueness even if names change.
func BuildShippingSecretNameWithNames(env, tenantID, tenantName, vendorID, vendorName string, provider ShippingProvider, keyName ShippingKeyName) string {
	env = sanitizeComponent(env)
	tenantID = sanitizeComponent(tenantID)
	tenantSlug := sanitizeComponent(tenantName)
	providerStr := sanitizeComponent(string(provider))
	keyNameStr := sanitizeComponent(string(keyName))

	// If no tenant name provided, fall back to ID-only naming
	if tenantSlug == "" {
		return BuildShippingSecretName(env, tenantID, vendorID, provider, keyName)
	}

	if vendorID == "" {
		// Tenant-level: {env}-{tenantSlug}-{tenantId}-shipping-{provider}-{keyName}
		return fmt.Sprintf("%s-%s-%s-shipping-%s-%s", env, tenantSlug, tenantID, providerStr, keyNameStr)
	}

	// Vendor-level: {env}-{tenantSlug}-{tenantId}-{vendorSlug}-{vendorId}-shipping-{provider}-{keyName}
	vendorID = sanitizeComponent(vendorID)
	vendorSlug := sanitizeComponent(vendorName)

	if vendorSlug == "" {
		// Vendor name not provided, use ID only for vendor part
		return fmt.Sprintf("%s-%s-%s-vendor-%s-shipping-%s-%s", env, tenantSlug, tenantID, vendorID, providerStr, keyNameStr)
	}

	return fmt.Sprintf("%s-%s-%s-%s-%s-shipping-%s-%s", env, tenantSlug, tenantID, vendorSlug, vendorID, providerStr, keyNameStr)
}

// ParseShippingSecretName extracts metadata from a shipping secret name.
// Returns an error if the name doesn't match the expected format.
func ParseShippingSecretName(secretName string) (*ShippingSecretMetadata, error) {
	if secretName == "" {
		return nil, fmt.Errorf("invalid shipping secret name format: empty string")
	}

	// Find the "-shipping-" segment followed by a known provider
	shippingIdx := strings.Index(secretName, "-shipping-")
	if shippingIdx == -1 {
		return nil, fmt.Errorf("invalid shipping secret name format: %s (no '-shipping-' segment found)", secretName)
	}

	// The part after "-shipping-" should start with a known provider
	afterShipping := secretName[shippingIdx+len("-shipping-"):]

	var foundProvider string
	for _, p := range knownShippingProviders {
		if strings.HasPrefix(afterShipping, p+"-") {
			foundProvider = p
			break
		}
	}

	if foundProvider == "" {
		return nil, fmt.Errorf("invalid shipping secret name format: %s (no known shipping provider found)", secretName)
	}

	// Extract the prefix (env-tenant-{tenantId} or env-tenant-{tenantId}-vendor-{vendorId})
	prefix := secretName[:shippingIdx]
	suffix := afterShipping[len(foundProvider)+1:] // Skip "provider-"

	// Parse the prefix to extract env, tenantID, and optionally vendorID
	var env, tenantID, vendorID string
	var scope ShippingSecretScope

	if strings.Contains(prefix, "-vendor-") {
		// Vendor-level: env-tenant-{tenantId}-vendor-{vendorId}
		vendorPartIdx := strings.Index(prefix, "-vendor-")
		if vendorPartIdx == -1 {
			return nil, fmt.Errorf("invalid shipping secret name format: %s", secretName)
		}

		tenantPart := prefix[:vendorPartIdx]   // env-tenant-{tenantId}
		vendorID = prefix[vendorPartIdx+8:]    // {vendorId} (skip "-vendor-")

		// Parse tenant part: env-tenant-{tenantId}
		tenantIdx := strings.Index(tenantPart, "-tenant-")
		if tenantIdx == -1 {
			return nil, fmt.Errorf("invalid shipping secret name format: %s", secretName)
		}
		env = tenantPart[:tenantIdx]
		tenantID = tenantPart[tenantIdx+8:]    // Skip "-tenant-"
		scope = ShippingScopeVendor
	} else {
		// Tenant-level: env-tenant-{tenantId}
		tenantIdx := strings.Index(prefix, "-tenant-")
		if tenantIdx == -1 {
			return nil, fmt.Errorf("invalid shipping secret name format: %s", secretName)
		}
		env = prefix[:tenantIdx]
		tenantID = prefix[tenantIdx+8:]        // Skip "-tenant-"
		scope = ShippingScopeTenant
	}

	// Validate extracted components
	if env == "" || tenantID == "" || suffix == "" {
		return nil, fmt.Errorf("invalid shipping secret name format: %s", secretName)
	}

	return &ShippingSecretMetadata{
		Env:      env,
		TenantID: tenantID,
		VendorID: vendorID,
		Provider: ShippingProvider(foundProvider),
		KeyName:  ShippingKeyName(suffix),
		Scope:    scope,
	}, nil
}

// GetShippingProviderRequiredKeys returns the required key names for a shipping provider.
// These keys must be configured for the provider to work.
func GetShippingProviderRequiredKeys(provider ShippingProvider) []ShippingKeyName {
	switch provider {
	case ShippingProviderShiprocket:
		return []ShippingKeyName{ShippingKeyAPIEmail, ShippingKeyAPIPassword}
	case ShippingProviderDelhivery:
		return []ShippingKeyName{ShippingKeyAPIToken}
	case ShippingProviderBluedart:
		return []ShippingKeyName{ShippingKeyAPIKey, ShippingKeyLicenseKey}
	case ShippingProviderDTDC:
		return []ShippingKeyName{ShippingKeyAPIKey}
	case ShippingProviderFedEx:
		return []ShippingKeyName{ShippingKeyClientID, ShippingKeyClientSecret}
	case ShippingProviderUPS:
		return []ShippingKeyName{ShippingKeyClientID, ShippingKeyClientSecret}
	case ShippingProviderDHL:
		return []ShippingKeyName{ShippingKeyAPIKey, ShippingKeyAPISecret}
	case ShippingProviderShippo:
		return []ShippingKeyName{ShippingKeyAPIKey}
	case ShippingProviderShipEngine:
		return []ShippingKeyName{ShippingKeyAPIKey}
	default:
		return nil
	}
}

// GetShippingProviderOptionalKeys returns optional key names for a shipping provider.
// These keys enhance functionality but are not required.
func GetShippingProviderOptionalKeys(provider ShippingProvider) []ShippingKeyName {
	switch provider {
	case ShippingProviderShiprocket:
		return []ShippingKeyName{ShippingKeyWebhookSecret}
	case ShippingProviderDelhivery:
		return []ShippingKeyName{ShippingKeyPickupLocation, ShippingKeyWebhookSecret}
	case ShippingProviderFedEx:
		return []ShippingKeyName{ShippingKeyAccountNumber}
	case ShippingProviderUPS:
		return []ShippingKeyName{ShippingKeyAccountNumber}
	case ShippingProviderDHL:
		return []ShippingKeyName{ShippingKeyAccountNumber}
	default:
		return nil
	}
}

// GetAllShippingProviderKeys returns all supported key names for a shipping provider.
func GetAllShippingProviderKeys(provider ShippingProvider) []ShippingKeyName {
	required := GetShippingProviderRequiredKeys(provider)
	optional := GetShippingProviderOptionalKeys(provider)
	return append(required, optional...)
}

// BuildShippingSecretLabels returns GCP Secret Manager labels for a shipping secret.
// These labels are useful for filtering and auditing.
func BuildShippingSecretLabels(meta *ShippingSecretMetadata) map[string]string {
	labels := map[string]string{
		"environment": sanitizeLabelValue(meta.Env),
		"category":    "shipping",
		"provider":    sanitizeLabelValue(string(meta.Provider)),
		"tenant_id":   sanitizeLabelValue(meta.TenantID),
		"managed_by":  "secret-provisioner",
	}

	if meta.VendorID != "" {
		labels["scope"] = "vendor"
		labels["vendor_id"] = sanitizeLabelValue(meta.VendorID)
	} else {
		labels["scope"] = "tenant"
	}

	return labels
}

// ValidateShippingProvider checks if the given shipping provider is supported.
func ValidateShippingProvider(provider ShippingProvider) bool {
	switch provider {
	case ShippingProviderShiprocket, ShippingProviderDelhivery, ShippingProviderBluedart,
		ShippingProviderDTDC, ShippingProviderShippo, ShippingProviderShipEngine,
		ShippingProviderFedEx, ShippingProviderUPS, ShippingProviderDHL:
		return true
	default:
		return false
	}
}

// ValidateShippingKeyName checks if the given key name is valid for the shipping provider.
func ValidateShippingKeyName(provider ShippingProvider, keyName ShippingKeyName) bool {
	allKeys := GetAllShippingProviderKeys(provider)
	for _, k := range allKeys {
		if k == keyName {
			return true
		}
	}
	return false
}

// GetSupportedShippingProviders returns a list of all supported shipping providers.
func GetSupportedShippingProviders() []ShippingProvider {
	return []ShippingProvider{
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
}
