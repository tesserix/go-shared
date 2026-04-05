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

// Shipping-specific errors
var (
	ErrShippingSecretNotFound      = errors.New("shipping secret not found")
	ErrShippingCarrierNotConfigured = errors.New("shipping carrier not configured")
	ErrShippingSecretAccessDenied  = errors.New("shipping secret access denied")
	ErrInvalidShippingProvider     = errors.New("invalid shipping provider")
	ErrInvalidShippingKeyName      = errors.New("invalid shipping key name")
)

// ShippingSecretClient provides shipping-specific secret operations with caching.
// This client is designed for read operations in the shipping service.
type ShippingSecretClient struct {
	projectID string
	client    *secretmanager.Client
	cache     *shippingSecretCache
	cacheTTL  time.Duration
	logger    *logrus.Entry
	mu        sync.RWMutex
}

// ShippingSecretClientConfig configuration for the shipping secret client
type ShippingSecretClientConfig struct {
	ProjectID   string
	CacheTTL    time.Duration // Default: 10 minutes
	Logger      *logrus.Entry // Optional: will create default if nil
	Credentials []byte        // Optional: use default credentials if nil
}

// NewShippingSecretClient creates a new shipping secret client.
// This client uses Workload Identity when running in GKE.
func NewShippingSecretClient(ctx context.Context, config ShippingSecretClientConfig) (*ShippingSecretClient, error) {
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

	return &ShippingSecretClient{
		projectID: config.ProjectID,
		client:    client,
		cache:     newShippingSecretCache(cacheTTL),
		cacheTTL:  cacheTTL,
		logger:    logger,
	}, nil
}

// GetShippingSecret retrieves a specific shipping secret by its full name.
// The result is cached for the configured TTL.
func (c *ShippingSecretClient) GetShippingSecret(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider ShippingProvider,
	keyName ShippingKeyName,
) (string, error) {
	secretName := BuildShippingSecretName(env, tenantID, vendorID, provider, keyName)
	return c.getSecretByName(ctx, secretName)
}

// GetShippingSecretWithFallback implements vendor-first precedence for credential resolution.
//
// Resolution order:
//  1. If vendorID provided -> try vendor-level secret
//  2. If vendor secret missing -> try tenant-level secret
//  3. If both missing -> return ErrShippingCarrierNotConfigured
//
// This method should be used by the shipping service for all credential lookups.
func (c *ShippingSecretClient) GetShippingSecretWithFallback(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider ShippingProvider,
	keyName ShippingKeyName,
) (string, error) {
	// Try vendor-level first if vendorID provided
	if vendorID != "" {
		value, err := c.GetShippingSecret(ctx, env, tenantID, vendorID, provider, keyName)
		if err == nil {
			c.logger.WithFields(logrus.Fields{
				"env":       env,
				"tenant_id": tenantID,
				"vendor_id": vendorID,
				"provider":  provider,
				"key_name":  keyName,
				"scope":     "vendor",
			}).Debug("resolved shipping secret at vendor level")
			return value, nil
		}
		if !isShippingNotFoundError(err) {
			return "", err // Real error, not just missing
		}
		// Vendor secret not found, fall through to tenant-level
	}

	// Try tenant-level
	value, err := c.GetShippingSecret(ctx, env, tenantID, "", provider, keyName)
	if err != nil {
		if isShippingNotFoundError(err) {
			return "", fmt.Errorf("%w: tenant=%s, provider=%s, key=%s",
				ErrShippingCarrierNotConfigured, tenantID, provider, keyName)
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
	}).Debug("resolved shipping secret at tenant level (vendor fallback)")

	return value, nil
}

// GetAllCarrierCredentials retrieves all required credentials for a shipping carrier.
// Returns a map of key names to values.
func (c *ShippingSecretClient) GetAllCarrierCredentials(
	ctx context.Context,
	env, tenantID, vendorID string,
	provider ShippingProvider,
) (map[ShippingKeyName]string, error) {
	requiredKeys := GetShippingProviderRequiredKeys(provider)
	if len(requiredKeys) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidShippingProvider, provider)
	}

	credentials := make(map[ShippingKeyName]string)

	// Get all required keys
	for _, keyName := range requiredKeys {
		value, err := c.GetShippingSecretWithFallback(ctx, env, tenantID, vendorID, provider, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to get required key %s: %w", keyName, err)
		}
		credentials[keyName] = value
	}

	// Try to get optional keys (don't fail if missing)
	optionalKeys := GetShippingProviderOptionalKeys(provider)
	for _, keyName := range optionalKeys {
		value, err := c.GetShippingSecretWithFallback(ctx, env, tenantID, vendorID, provider, keyName)
		if err == nil {
			credentials[keyName] = value
		}
		// Silently ignore missing optional keys
	}

	return credentials, nil
}

// GetDynamicCredentials retrieves credentials using dynamic key names.
// This is useful when integrating new shipping carriers without modifying the go-shared package.
// The keyNames should match the keys used when provisioning the secrets.
//
// Usage example:
//
//	creds, err := client.GetDynamicCredentials(ctx, env, tenantID, vendorID, "fedex", []string{"api_key", "secret_key", "account_number"})
func (c *ShippingSecretClient) GetDynamicCredentials(
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
		typedProvider := ShippingProvider(provider)
		typedKeyName := ShippingKeyName(keyName)

		value, err := c.GetShippingSecretWithFallback(ctx, env, tenantID, vendorID, typedProvider, typedKeyName)
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
		return nil, fmt.Errorf("%w: provider=%s has no credentials configured", ErrShippingCarrierNotConfigured, provider)
	}

	c.logger.WithFields(logrus.Fields{
		"env":            env,
		"tenant_id":      tenantID,
		"vendor_id":      vendorID,
		"provider":       provider,
		"keys_found":     len(credentials),
		"keys_requested": len(keyNames),
	}).Debug("retrieved dynamic credentials")

	return credentials, nil
}

// InvalidateCache removes a specific secret from the cache.
// Call this when you know a secret has been updated.
func (c *ShippingSecretClient) InvalidateCache(env, tenantID, vendorID string, provider ShippingProvider, keyName ShippingKeyName) {
	secretName := BuildShippingSecretName(env, tenantID, vendorID, provider, keyName)
	c.cache.delete(secretName)
}

// InvalidateAllCache clears the entire cache.
func (c *ShippingSecretClient) InvalidateAllCache() {
	c.cache.clear()
}

// Close closes the underlying GCP client.
func (c *ShippingSecretClient) Close() error {
	return c.client.Close()
}

// getSecretByName retrieves a secret by its full GCP name with caching.
func (c *ShippingSecretClient) getSecretByName(ctx context.Context, secretName string) (string, error) {
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
		}).Warn("shipping secret access failed")

		return "", wrapShippingGCPError(err, secretName)
	}

	value := string(result.Payload.Data)

	// Cache the result
	c.cache.set(secretName, value)

	// Log successful access (never log values)
	c.logger.WithFields(logrus.Fields{
		"secret_name": secretName,
		"operation":   "read",
		"status":      "success",
	}).Debug("shipping secret accessed")

	return value, nil
}

// shippingSecretCache provides thread-safe caching for shipping secrets.
type shippingSecretCache struct {
	entries map[string]*shippingCacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type shippingCacheEntry struct {
	value     string
	expiresAt time.Time
}

func newShippingSecretCache(ttl time.Duration) *shippingSecretCache {
	return &shippingSecretCache{
		entries: make(map[string]*shippingCacheEntry),
		ttl:     ttl,
	}
}

func (c *shippingSecretCache) get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.value, true
}

func (c *shippingSecretCache) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &shippingCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *shippingSecretCache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *shippingSecretCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*shippingCacheEntry)
}

// Helper functions

// isShippingNotFoundError checks if the error indicates a secret was not found.
func isShippingNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrShippingSecretNotFound) {
		return true
	}
	if errors.Is(err, ErrShippingCarrierNotConfigured) {
		return true
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.NotFound
	}
	return false
}

// wrapShippingGCPError wraps GCP errors with more specific shipping errors.
func wrapShippingGCPError(err error, secretName string) error {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			return fmt.Errorf("%w: %s", ErrShippingSecretNotFound, secretName)
		case codes.PermissionDenied:
			return fmt.Errorf("%w: %s", ErrShippingSecretAccessDenied, secretName)
		}
	}
	return fmt.Errorf("failed to access secret %s: %w", secretName, err)
}
