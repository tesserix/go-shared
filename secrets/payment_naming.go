package secrets

import (
	"fmt"
	"regexp"
	"strings"
)

// PaymentProvider represents supported payment providers
type PaymentProvider string

const (
	// Production-ready providers
	PaymentProviderStripe   PaymentProvider = "stripe"
	PaymentProviderRazorpay PaymentProvider = "razorpay"

	// Coming Soon providers (placeholders)
	PaymentProviderPayPal    PaymentProvider = "paypal"
	PaymentProviderPhonePe   PaymentProvider = "phonepe"
	PaymentProviderAfterpay  PaymentProvider = "afterpay"
	PaymentProviderZipPay    PaymentProvider = "zippay"
	PaymentProviderGooglePay PaymentProvider = "googlepay"
	PaymentProviderApplePay  PaymentProvider = "applepay"
	PaymentProviderPaytm     PaymentProvider = "paytm"
	PaymentProviderUPIDirect PaymentProvider = "upi-direct"
)

// PaymentProviderStatus represents the availability status of a provider
type PaymentProviderStatus string

const (
	ProviderStatusReady      PaymentProviderStatus = "ready"
	ProviderStatusComingSoon PaymentProviderStatus = "coming_soon"
	ProviderStatusBeta       PaymentProviderStatus = "beta"
)

// PaymentProviderInfo holds metadata about a payment provider
type PaymentProviderInfo struct {
	Provider    PaymentProvider
	Name        string
	Status      PaymentProviderStatus
	Regions     []string
	Description string
}

// PaymentKeyName represents supported key names per provider
type PaymentKeyName string

const (
	// Stripe keys
	KeyStripeAPIKey        PaymentKeyName = "api-key"
	KeyStripeWebhookSecret PaymentKeyName = "webhook-secret"
	KeyStripeConnectedID   PaymentKeyName = "connected-account-id"

	// Razorpay keys
	KeyRazorpayKeyID       PaymentKeyName = "key-id"
	KeyRazorpayKeySecret   PaymentKeyName = "key-secret"
	KeyRazorpayWebhook     PaymentKeyName = "webhook-secret"

	// PayPal keys
	KeyPayPalClientID     PaymentKeyName = "client-id"
	KeyPayPalClientSecret PaymentKeyName = "client-secret"
	KeyPayPalWebhookID    PaymentKeyName = "webhook-id"
)

// PaymentSecretScope represents the scope level for a payment secret
type PaymentSecretScope string

const (
	PaymentScopeTenant PaymentSecretScope = "tenant"
	PaymentScopeVendor PaymentSecretScope = "vendor"
)

// PaymentSecretMetadata holds parsed secret name components
type PaymentSecretMetadata struct {
	Env       string
	TenantID  string
	VendorID  string // Empty for tenant-level
	Provider  PaymentProvider
	KeyName   PaymentKeyName
	Scope     PaymentSecretScope
}

var (
	// Regex for validating/sanitizing secret name components
	// GCP allows: [a-zA-Z0-9_-], max 255 chars
	invalidCharsRegex = regexp.MustCompile(`[^a-z0-9_-]`)

	// Known provider names for parsing (order matters - longer names first)
	knownProviders = []string{
		"upi-direct", "googlepay", "applepay", "razorpay", "afterpay", "phonepe",
		"zippay", "stripe", "paypal", "paytm",
	}
)

// PaymentSecretNameOptions provides optional naming configuration for secrets.
type PaymentSecretNameOptions struct {
	TenantName string // Optional: human-readable tenant name for the secret
	VendorName string // Optional: human-readable vendor name for the secret
}

// BuildPaymentSecretName constructs a GCP Secret Manager secret name for payment credentials.
//
// Naming convention:
//   - Tenant-level: {env}-tenant-{tenantId}-{provider}-{keyName}
//   - Vendor-level: {env}-tenant-{tenantId}-vendor-{vendorId}-{provider}-{keyName}
//
// Examples:
//   - prod-tenant-t_123-stripe-api-key
//   - dev-tenant-t_123-vendor-v_99-razorpay-key-id
func BuildPaymentSecretName(env, tenantID, vendorID string, provider PaymentProvider, keyName PaymentKeyName) string {
	env = sanitizeComponent(env)
	tenantID = sanitizeComponent(tenantID)
	providerStr := sanitizeComponent(string(provider))
	keyNameStr := sanitizeComponent(string(keyName))

	if vendorID == "" {
		// Tenant-level: {env}-tenant-{tenantId}-{provider}-{keyName}
		return fmt.Sprintf("%s-tenant-%s-%s-%s", env, tenantID, providerStr, keyNameStr)
	}

	// Vendor-level: {env}-tenant-{tenantId}-vendor-{vendorId}-{provider}-{keyName}
	vendorID = sanitizeComponent(vendorID)
	return fmt.Sprintf("%s-tenant-%s-vendor-%s-%s-%s", env, tenantID, vendorID, providerStr, keyNameStr)
}

// BuildPaymentSecretNameWithNames constructs a GCP Secret Manager secret name with human-readable names.
// This creates more readable secret names by including tenant/vendor names alongside IDs for uniqueness.
//
// Naming convention:
//   - Tenant-level: {env}-{tenantSlug}-{tenantId}-{provider}-{keyName}
//   - Vendor-level: {env}-{tenantSlug}-{tenantId}-{vendorSlug}-{vendorId}-{provider}-{keyName}
//
// Examples:
//   - prod-acme-corp-t-123-stripe-api-key
//   - dev-acme-corp-t-123-electronics-v-99-razorpay-key-id
//
// Note: Names are slugified (lowercase, special chars replaced with dashes).
// IDs ensure uniqueness even if names change.
func BuildPaymentSecretNameWithNames(env, tenantID, tenantName, vendorID, vendorName string, provider PaymentProvider, keyName PaymentKeyName) string {
	env = sanitizeComponent(env)
	tenantID = sanitizeComponent(tenantID)
	tenantSlug := sanitizeComponent(tenantName)
	providerStr := sanitizeComponent(string(provider))
	keyNameStr := sanitizeComponent(string(keyName))

	// If no tenant name provided, fall back to ID-only naming
	if tenantSlug == "" {
		return BuildPaymentSecretName(env, tenantID, vendorID, provider, keyName)
	}

	if vendorID == "" {
		// Tenant-level: {env}-{tenantSlug}-{tenantId}-{provider}-{keyName}
		return fmt.Sprintf("%s-%s-%s-%s-%s", env, tenantSlug, tenantID, providerStr, keyNameStr)
	}

	// Vendor-level: {env}-{tenantSlug}-{tenantId}-{vendorSlug}-{vendorId}-{provider}-{keyName}
	vendorID = sanitizeComponent(vendorID)
	vendorSlug := sanitizeComponent(vendorName)

	if vendorSlug == "" {
		// Vendor name not provided, use ID only for vendor part
		return fmt.Sprintf("%s-%s-%s-vendor-%s-%s-%s", env, tenantSlug, tenantID, vendorID, providerStr, keyNameStr)
	}

	return fmt.Sprintf("%s-%s-%s-%s-%s-%s-%s", env, tenantSlug, tenantID, vendorSlug, vendorID, providerStr, keyNameStr)
}

// ParsePaymentSecretName extracts metadata from a payment secret name.
// Returns an error if the name doesn't match the expected format.
func ParsePaymentSecretName(secretName string) (*PaymentSecretMetadata, error) {
	if secretName == "" {
		return nil, fmt.Errorf("invalid payment secret name format: empty string")
	}

	// Find the provider in the secret name
	var foundProvider string
	var providerIdx int = -1

	for _, p := range knownProviders {
		idx := strings.Index(secretName, "-"+p+"-")
		if idx != -1 {
			foundProvider = p
			providerIdx = idx
			break
		}
	}

	if providerIdx == -1 {
		return nil, fmt.Errorf("invalid payment secret name format: %s (no known provider found)", secretName)
	}

	// Extract the prefix (env-tenant-{tenantId} or env-tenant-{tenantId}-vendor-{vendorId})
	prefix := secretName[:providerIdx]
	suffix := secretName[providerIdx+len(foundProvider)+2:] // Skip "-provider-"

	// Parse the prefix to extract env, tenantID, and optionally vendorID
	var env, tenantID, vendorID string
	var scope PaymentSecretScope

	if strings.Contains(prefix, "-vendor-") {
		// Vendor-level: env-tenant-{tenantId}-vendor-{vendorId}
		vendorIdx := strings.Index(prefix, "-vendor-")
		if vendorIdx == -1 {
			return nil, fmt.Errorf("invalid payment secret name format: %s", secretName)
		}

		tenantPart := prefix[:vendorIdx]          // env-tenant-{tenantId}
		vendorID = prefix[vendorIdx+8:]            // {vendorId} (skip "-vendor-")

		// Parse tenant part: env-tenant-{tenantId}
		tenantIdx := strings.Index(tenantPart, "-tenant-")
		if tenantIdx == -1 {
			return nil, fmt.Errorf("invalid payment secret name format: %s", secretName)
		}
		env = tenantPart[:tenantIdx]
		tenantID = tenantPart[tenantIdx+8:]        // Skip "-tenant-"
		scope = PaymentScopeVendor
	} else {
		// Tenant-level: env-tenant-{tenantId}
		tenantIdx := strings.Index(prefix, "-tenant-")
		if tenantIdx == -1 {
			return nil, fmt.Errorf("invalid payment secret name format: %s", secretName)
		}
		env = prefix[:tenantIdx]
		tenantID = prefix[tenantIdx+8:]            // Skip "-tenant-"
		scope = PaymentScopeTenant
	}

	// Validate extracted components
	if env == "" || tenantID == "" || suffix == "" {
		return nil, fmt.Errorf("invalid payment secret name format: %s", secretName)
	}

	return &PaymentSecretMetadata{
		Env:      env,
		TenantID: tenantID,
		VendorID: vendorID,
		Provider: PaymentProvider(foundProvider),
		KeyName:  PaymentKeyName(suffix),
		Scope:    scope,
	}, nil
}

// GetPaymentProviderRequiredKeys returns the required key names for a payment provider.
// These keys must be configured for the provider to work.
func GetPaymentProviderRequiredKeys(provider PaymentProvider) []PaymentKeyName {
	switch provider {
	// Production-ready providers
	case PaymentProviderStripe:
		return []PaymentKeyName{KeyStripeAPIKey}
	case PaymentProviderRazorpay:
		return []PaymentKeyName{KeyRazorpayKeyID, KeyRazorpayKeySecret}
	// Coming Soon providers (placeholders)
	case PaymentProviderPayPal:
		return []PaymentKeyName{KeyPayPalClientID, KeyPayPalClientSecret}
	case PaymentProviderPhonePe:
		return []PaymentKeyName{"merchant-id", "salt-key"}
	case PaymentProviderAfterpay:
		return []PaymentKeyName{"merchant-id", "secret-key"}
	case PaymentProviderZipPay:
		return []PaymentKeyName{"merchant-id", "api-key"}
	case PaymentProviderGooglePay:
		return []PaymentKeyName{"merchant-id"}
	case PaymentProviderApplePay:
		return []PaymentKeyName{"merchant-id", "merchant-certificate"}
	case PaymentProviderPaytm:
		return []PaymentKeyName{"merchant-id", "merchant-key"}
	case PaymentProviderUPIDirect:
		return []PaymentKeyName{"vpa", "merchant-key"}
	default:
		return nil
	}
}

// GetPaymentProviderOptionalKeys returns optional key names for a payment provider.
// These keys enhance functionality but are not required.
func GetPaymentProviderOptionalKeys(provider PaymentProvider) []PaymentKeyName {
	switch provider {
	case PaymentProviderStripe:
		return []PaymentKeyName{KeyStripeWebhookSecret, KeyStripeConnectedID}
	case PaymentProviderRazorpay:
		return []PaymentKeyName{KeyRazorpayWebhook}
	case PaymentProviderPayPal:
		return []PaymentKeyName{KeyPayPalWebhookID}
	default:
		return nil
	}
}

// GetAllPaymentProviderKeys returns all supported key names for a payment provider.
func GetAllPaymentProviderKeys(provider PaymentProvider) []PaymentKeyName {
	required := GetPaymentProviderRequiredKeys(provider)
	optional := GetPaymentProviderOptionalKeys(provider)
	return append(required, optional...)
}

// IsVendorLevelSecret checks if a secret name represents vendor-level credentials.
func IsVendorLevelSecret(secretName string) bool {
	return strings.Contains(secretName, "-vendor-")
}

// IsTenantLevelSecret checks if a secret name represents tenant-level credentials.
func IsTenantLevelSecret(secretName string) bool {
	return strings.Contains(secretName, "-tenant-") && !strings.Contains(secretName, "-vendor-")
}

// BuildPaymentSecretLabels returns GCP Secret Manager labels for a payment secret.
// These labels are useful for filtering and auditing.
func BuildPaymentSecretLabels(meta *PaymentSecretMetadata) map[string]string {
	labels := map[string]string{
		"environment": sanitizeLabelValue(meta.Env),
		"category":    "payment",
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

// ValidatePaymentProvider checks if the given provider is supported.
func ValidatePaymentProvider(provider PaymentProvider) bool {
	switch provider {
	case PaymentProviderStripe, PaymentProviderRazorpay, PaymentProviderPayPal,
		PaymentProviderPhonePe, PaymentProviderAfterpay, PaymentProviderZipPay,
		PaymentProviderGooglePay, PaymentProviderApplePay, PaymentProviderPaytm,
		PaymentProviderUPIDirect:
		return true
	default:
		return false
	}
}

// ValidatePaymentKeyName checks if the given key name is valid for the provider.
func ValidatePaymentKeyName(provider PaymentProvider, keyName PaymentKeyName) bool {
	allKeys := GetAllPaymentProviderKeys(provider)
	for _, k := range allKeys {
		if k == keyName {
			return true
		}
	}
	return false
}

// sanitizeComponent sanitizes a component for use in GCP secret names.
// GCP Secret Manager allows: [a-zA-Z0-9_-], max 255 chars.
func sanitizeComponent(s string) string {
	s = strings.ToLower(s)
	s = invalidCharsRegex.ReplaceAllString(s, "-")
	// Remove consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	// Trim leading/trailing dashes
	s = strings.Trim(s, "-")
	// Limit length to keep total secret name under 255 chars
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// sanitizeLabelValue sanitizes a value for use in GCP labels.
// Labels have stricter requirements: lowercase, max 63 chars, alphanumeric and dashes.
func sanitizeLabelValue(s string) string {
	s = strings.ToLower(s)
	s = invalidCharsRegex.ReplaceAllString(s, "-")
	// Remove consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	// Trim leading/trailing dashes
	s = strings.Trim(s, "-")
	// GCP label values max 63 chars
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

// GetSupportedPaymentProviders returns a list of all supported payment providers.
func GetSupportedPaymentProviders() []PaymentProvider {
	return []PaymentProvider{
		// Production-ready
		PaymentProviderStripe,
		PaymentProviderRazorpay,
		// Coming Soon
		PaymentProviderPayPal,
		PaymentProviderPhonePe,
		PaymentProviderAfterpay,
		PaymentProviderZipPay,
		PaymentProviderGooglePay,
		PaymentProviderApplePay,
		PaymentProviderPaytm,
		PaymentProviderUPIDirect,
	}
}

// GetProductionReadyProviders returns providers that are fully integrated and tested.
func GetProductionReadyProviders() []PaymentProvider {
	return []PaymentProvider{
		PaymentProviderStripe,
		PaymentProviderRazorpay,
	}
}

// GetComingSoonProviders returns providers that are planned but not yet available.
func GetComingSoonProviders() []PaymentProvider {
	return []PaymentProvider{
		PaymentProviderPayPal,
		PaymentProviderPhonePe,
		PaymentProviderAfterpay,
		PaymentProviderZipPay,
		PaymentProviderGooglePay,
		PaymentProviderApplePay,
		PaymentProviderPaytm,
		PaymentProviderUPIDirect,
	}
}

// GetPaymentProviderStatus returns the availability status of a provider.
func GetPaymentProviderStatus(provider PaymentProvider) PaymentProviderStatus {
	switch provider {
	case PaymentProviderStripe, PaymentProviderRazorpay:
		return ProviderStatusReady
	default:
		return ProviderStatusComingSoon
	}
}

// IsProviderReady checks if a provider is production-ready.
func IsProviderReady(provider PaymentProvider) bool {
	return GetPaymentProviderStatus(provider) == ProviderStatusReady
}

// GetAllPaymentProviderInfo returns detailed information about all providers.
func GetAllPaymentProviderInfo() []PaymentProviderInfo {
	return []PaymentProviderInfo{
		// Production-ready providers
		{
			Provider:    PaymentProviderStripe,
			Name:        "Stripe",
			Status:      ProviderStatusReady,
			Regions:     []string{"US", "GB", "AU", "CA", "EU"},
			Description: "Global payment processing platform",
		},
		{
			Provider:    PaymentProviderRazorpay,
			Name:        "Razorpay",
			Status:      ProviderStatusReady,
			Regions:     []string{"IN"},
			Description: "India's leading payment gateway",
		},
		// Coming Soon providers
		{
			Provider:    PaymentProviderPayPal,
			Name:        "PayPal",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"US", "GB", "AU"},
			Description: "Global digital payments platform",
		},
		{
			Provider:    PaymentProviderPhonePe,
			Name:        "PhonePe",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"IN"},
			Description: "UPI-based payments in India",
		},
		{
			Provider:    PaymentProviderAfterpay,
			Name:        "Afterpay",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"AU", "NZ", "US"},
			Description: "Buy now, pay later service",
		},
		{
			Provider:    PaymentProviderZipPay,
			Name:        "Zip Pay",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"AU", "NZ", "US"},
			Description: "Buy now, pay later service",
		},
		{
			Provider:    PaymentProviderGooglePay,
			Name:        "Google Pay",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"US", "GB", "AU"},
			Description: "Google's digital wallet platform",
		},
		{
			Provider:    PaymentProviderApplePay,
			Name:        "Apple Pay",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"US", "GB", "AU"},
			Description: "Apple's contactless payment system",
		},
		{
			Provider:    PaymentProviderPaytm,
			Name:        "Paytm",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"IN"},
			Description: "India's digital payment platform",
		},
		{
			Provider:    PaymentProviderUPIDirect,
			Name:        "UPI Direct",
			Status:      ProviderStatusComingSoon,
			Regions:     []string{"IN"},
			Description: "Direct UPI integration for India",
		},
	}
}
