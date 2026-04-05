package secrets

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Payment-specific errors
var (
	ErrPaymentSecretNotFound      = errors.New("payment secret not found")
	ErrPaymentProviderNotConfigured = errors.New("payment provider not configured")
	ErrPaymentSecretAccessDenied  = errors.New("payment secret access denied")
	ErrInvalidPaymentProvider     = errors.New("invalid payment provider")
	ErrInvalidPaymentKeyName      = errors.New("invalid payment key name")
)

// PaymentSecretClient provides payment-specific secret operations with caching.
// This client is designed for read operations in the payment gateway service.
type PaymentSecretClient struct {
	projectID string
	client    *secretmanager.Client
	cache     *paymentSecretCache
	cacheTTL  time.Duration
	logger    *logrus.Entry
	mu        sync.RWMutex
}

// PaymentSecretClientConfig configuration for the payment secret client
type PaymentSecretClientConfig struct {
	ProjectID string
	CacheTTL  time.Duration   // Default: 10 minutes
	Logger    *logrus.Entry   // Optional: will create default if nil
	Credentials []byte        // Optional: use default credentials if nil
}

// NewPaymentSecretClient creates a new payment secret client.
// This client uses Workload Identity when running in GKE.
func NewPaymentSecretClient(ctx context.Context, config PaymentSecretClientConfig) (*PaymentSecretClient, error) {
	var opts []option.ClientOption
	if len(config.Credentials) > 0 {
		opts = append(opts, option.WithCredentialsJSON(config.Credentials))
	}

	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	cacheTTL := config.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Minute
	}

	logger := config.Logger
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	return &PaymentSecretClient{
		projectID: config.ProjectID,
		client:    client,
		cache:     newPaymentSecretCache(cacheTTL),
		cacheTTL:  cacheTTL,
		logger:    logger,
	}, nil
}

// GetPaymentSecret retrieves a specific payment secret by its full name.
// The result is cached for the configured TTL.
func (c *PaymentSecretClient) GetPaymentSecret(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider PaymentProvider,
	keyName PaymentKeyName,
) (string, error) {
	secretName := BuildPaymentSecretName(env, tenantID, vendorID, provider, keyName)
	return c.getSecretByName(ctx, secretName)
}

// GetPaymentSecretWithFallback implements vendor-first precedence for credential resolution.
//
// Resolution order:
//  1. If vendorID provided → try vendor-level secret
//  2. If vendor secret missing → try tenant-level secret
//  3. If both missing → return ErrPaymentProviderNotConfigured
//
// This method should be used by the payment gateway for all credential lookups.
func (c *PaymentSecretClient) GetPaymentSecretWithFallback(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider PaymentProvider,
	keyName PaymentKeyName,
) (string, error) {
	// Try vendor-level first if vendorID provided
	if vendorID != "" {
		value, err := c.GetPaymentSecret(ctx, env, tenantID, vendorID, provider, keyName)
		if err == nil {
			c.logger.WithFields(logrus.Fields{
				"env":       env,
				"tenant_id": tenantID,
				"vendor_id": vendorID,
				"provider":  provider,
				"key_name":  keyName,
				"scope":     "vendor",
			}).Debug("resolved payment secret at vendor level")
			return value, nil
		}
		if !isNotFoundError(err) {
			return "", err // Real error, not just missing
		}
		// Vendor secret not found, fall through to tenant-level
	}

	// Try tenant-level
	value, err := c.GetPaymentSecret(ctx, env, tenantID, "", provider, keyName)
	if err != nil {
		if isNotFoundError(err) {
			return "", fmt.Errorf("%w: tenant=%s, provider=%s, key=%s",
				ErrPaymentProviderNotConfigured, tenantID, provider, keyName)
		}
		return "", err
	}

	c.logger.WithFields(logrus.Fields{
		"env":       env,
		"tenant_id": tenantID,
		"vendor_id": vendorID,
		"provider":  provider,
		"key_name":  keyName,
		"scope":     "tenant",
	}).Debug("resolved payment secret at tenant level (vendor fallback)")

	return value, nil
}

// GetAllProviderCredentials retrieves all required credentials for a payment provider.
// Returns a map of key names to values.
func (c *PaymentSecretClient) GetAllProviderCredentials(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider PaymentProvider,
) (map[PaymentKeyName]string, error) {
	requiredKeys := GetPaymentProviderRequiredKeys(provider)
	if len(requiredKeys) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPaymentProvider, provider)
	}

	credentials := make(map[PaymentKeyName]string)

	// Get all required keys
	for _, keyName := range requiredKeys {
		value, err := c.GetPaymentSecretWithFallback(ctx, env, tenantID, vendorID, provider, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to get required key %s: %w", keyName, err)
		}
		credentials[keyName] = value
	}

	// Try to get optional keys (don't fail if missing)
	optionalKeys := GetPaymentProviderOptionalKeys(provider)
	for _, keyName := range optionalKeys {
		value, err := c.GetPaymentSecretWithFallback(ctx, env, tenantID, vendorID, provider, keyName)
		if err == nil {
			credentials[keyName] = value
		}
		// Silently ignore missing optional keys
	}

	return credentials, nil
}

// GetDynamicCredentials retrieves credentials using dynamic key names.
// This is useful when integrating new payment gateways without modifying the go-shared package.
// The keyNames should match the keys used when provisioning the secrets.
//
// Usage example:
//
//	creds, err := client.GetDynamicCredentials(ctx, env, tenantID, vendorID, "phonepe", []string{"merchant_id", "salt_key", "salt_index"})
func (c *PaymentSecretClient) GetDynamicCredentials(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider string,
	keyNames []string,
) (map[string]string, error) {
	if len(keyNames) == 0 {
		return nil, fmt.Errorf("no key names provided")
	}

	credentials := make(map[string]string)

	for _, keyName := range keyNames {
		// Convert string provider and keyName to typed values
		// The types are just strings underneath, so this is safe
		typedProvider := PaymentProvider(provider)
		typedKeyName := PaymentKeyName(keyName)

		value, err := c.GetPaymentSecretWithFallback(ctx, env, tenantID, vendorID, typedProvider, typedKeyName)
		if err != nil {
			// Return error for required credentials
			// Callers can decide whether keys are required or optional
			c.logger.WithFields(logrus.Fields{
				"env":       env,
				"tenant_id": tenantID,
				"vendor_id": vendorID,
				"provider":  provider,
				"key_name":  keyName,
			}).Debug("credential not found")
			continue // Skip missing keys rather than failing
		}
		credentials[keyName] = value
	}

	if len(credentials) == 0 {
		return nil, fmt.Errorf("%w: provider=%s has no credentials configured", ErrPaymentProviderNotConfigured, provider)
	}

	c.logger.WithFields(logrus.Fields{
		"env":             env,
		"tenant_id":       tenantID,
		"vendor_id":       vendorID,
		"provider":        provider,
		"keys_found":      len(credentials),
		"keys_requested":  len(keyNames),
	}).Debug("retrieved dynamic credentials")

	return credentials, nil
}

// InvalidateCache removes a specific secret from the cache.
// Call this when you know a secret has been updated.
func (c *PaymentSecretClient) InvalidateCache(env, tenantID, vendorID string, provider PaymentProvider, keyName PaymentKeyName) {
	secretName := BuildPaymentSecretName(env, tenantID, vendorID, provider, keyName)
	c.cache.delete(secretName)
}

// InvalidateAllCache clears the entire cache.
func (c *PaymentSecretClient) InvalidateAllCache() {
	c.cache.clear()
}

// Close closes the underlying GCP client.
func (c *PaymentSecretClient) Close() error {
	return c.client.Close()
}

// getSecretByName retrieves a secret by its full GCP name with caching.
func (c *PaymentSecretClient) getSecretByName(ctx context.Context, secretName string) (string, error) {
	// Check cache first
	if cached, ok := c.cache.get(secretName); ok {
		return cached, nil
	}

	// Fetch from GCP
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", c.projectID, secretName)
	req := &secretmanagerpb.AccessSecretVersionRequest{Name: name}

	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		// Log access attempt (never log values)
		c.logger.WithFields(logrus.Fields{
			"secret_name": secretName,
			"operation":   "read",
			"status":      "failed",
			"error":       err.Error(),
		}).Warn("payment secret access failed")

		return "", wrapGCPError(err, secretName)
	}

	value := string(result.Payload.Data)

	// Cache the result
	c.cache.set(secretName, value)

	// Log successful access (never log values)
	c.logger.WithFields(logrus.Fields{
		"secret_name": secretName,
		"operation":   "read",
		"status":      "success",
	}).Debug("payment secret accessed")

	return value, nil
}

// paymentSecretCache provides thread-safe caching for payment secrets.
type paymentSecretCache struct {
	entries map[string]*paymentCacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type paymentCacheEntry struct {
	value     string
	expiresAt time.Time
}

func newPaymentSecretCache(ttl time.Duration) *paymentSecretCache {
	return &paymentSecretCache{
		entries: make(map[string]*paymentCacheEntry),
		ttl:     ttl,
	}
}

func (c *paymentSecretCache) get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.value, true
}

func (c *paymentSecretCache) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &paymentCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *paymentSecretCache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *paymentSecretCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*paymentCacheEntry)
}

// Helper functions

// isNotFoundError checks if the error indicates a secret was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPaymentSecretNotFound) {
		return true
	}
	if errors.Is(err, ErrPaymentProviderNotConfigured) {
		return true
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.NotFound
	}
	return false
}

// wrapGCPError wraps GCP errors with more specific payment errors.
func wrapGCPError(err error, secretName string) error {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			return fmt.Errorf("%w: %s", ErrPaymentSecretNotFound, secretName)
		case codes.PermissionDenied:
			return fmt.Errorf("%w: %s", ErrPaymentSecretAccessDenied, secretName)
		}
	}
	return fmt.Errorf("failed to access secret %s: %w", secretName, err)
}
