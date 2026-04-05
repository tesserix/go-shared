package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SecretType represents the type of secret
type SecretType string

const (
	SecretTypeSSO         SecretType = "sso"
	SecretTypeSCIM        SecretType = "scim"
	SecretTypeAPIKey      SecretType = "api_key"
	SecretTypeCertificate SecretType = "certificate"
)

// SecretProvider represents the SSO provider
type SecretProvider string

const (
	ProviderGoogle    SecretProvider = "google"
	ProviderMicrosoft SecretProvider = "microsoft"
	ProviderOkta      SecretProvider = "okta"
	ProviderSCIM      SecretProvider = "scim"
)

// SecretMetadata contains metadata about a secret
type SecretMetadata struct {
	TenantID     string
	SecretType   SecretType
	Provider     SecretProvider
	SecretName   string
	GCPSecretID  string
	Version      string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
}

// GCPSecretManagerClient provides methods to interact with GCP Secret Manager
type GCPSecretManagerClient struct {
	client    *secretmanager.Client
	projectID string
	mu        sync.RWMutex
	cache     map[string]*cachedSecret
	cacheTTL  time.Duration
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// GCPSecretManagerConfig configuration for the client
type GCPSecretManagerConfig struct {
	ProjectID      string
	CredentialsJSON []byte // Optional: use default credentials if nil
	CacheTTL       time.Duration
}

// NewGCPSecretManagerClient creates a new GCP Secret Manager client
func NewGCPSecretManagerClient(ctx context.Context, config GCPSecretManagerConfig) (*GCPSecretManagerClient, error) {
	var opts []option.ClientOption
	if len(config.CredentialsJSON) > 0 {
		opts = append(opts, option.WithCredentialsJSON(config.CredentialsJSON))
	}

	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	cacheTTL := config.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}

	return &GCPSecretManagerClient{
		client:    client,
		projectID: config.ProjectID,
		cache:     make(map[string]*cachedSecret),
		cacheTTL:  cacheTTL,
	}, nil
}

// Close closes the client connection
func (c *GCPSecretManagerClient) Close() error {
	return c.client.Close()
}

// BuildSecretName generates a unique secret name for a tenant
// Format: {type}-{tenantID}-{provider}-{name}
func BuildSecretName(tenantID string, secretType SecretType, provider SecretProvider, name string) string {
	// Sanitize tenant ID to be GCP-compatible (lowercase, alphanumeric, hyphens)
	sanitizedTenantID := sanitizeForGCP(tenantID)
	return fmt.Sprintf("%s-%s-%s-%s", secretType, sanitizedTenantID, provider, name)
}

// BuildSecretID returns the full GCP secret resource name
func (c *GCPSecretManagerClient) BuildSecretID(secretName string) string {
	return fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)
}

// CreateSecret creates a new secret in GCP Secret Manager
func (c *GCPSecretManagerClient) CreateSecret(ctx context.Context, metadata SecretMetadata, secretValue string) (*SecretMetadata, error) {
	secretName := BuildSecretName(metadata.TenantID, metadata.SecretType, metadata.Provider, metadata.SecretName)

	// Create the secret
	createReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", c.projectID),
		SecretId: secretName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
			Labels: map[string]string{
				"tenant_id":   sanitizeForGCP(metadata.TenantID),
				"secret_type": string(metadata.SecretType),
				"provider":    string(metadata.Provider),
				"managed_by":  "tesseract-hub",
			},
		},
	}

	secret, err := c.client.CreateSecret(ctx, createReq)
	if err != nil {
		// Check if secret already exists
		if status.Code(err) == codes.AlreadyExists {
			return c.UpdateSecret(ctx, metadata, secretValue)
		}
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	// Add the secret version with the actual value
	versionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(secretValue),
		},
	}

	version, err := c.client.AddSecretVersion(ctx, versionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to add secret version: %w", err)
	}

	metadata.GCPSecretID = secret.Name
	metadata.Version = extractVersion(version.Name)
	metadata.CreatedAt = time.Now()

	return &metadata, nil
}

// GetSecret retrieves a secret value from GCP Secret Manager
func (c *GCPSecretManagerClient) GetSecret(ctx context.Context, secretName string) (string, error) {
	// Check cache first
	c.mu.RLock()
	if cached, ok := c.cache[secretName]; ok && time.Now().Before(cached.expiresAt) {
		c.mu.RUnlock()
		return cached.value, nil
	}
	c.mu.RUnlock()

	secretID := c.BuildSecretID(secretName)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", secretID),
	}

	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", fmt.Errorf("secret not found: %s", secretName)
		}
		return "", fmt.Errorf("failed to access secret: %w", err)
	}

	secretValue := string(result.Payload.Data)

	// Update cache
	c.mu.Lock()
	c.cache[secretName] = &cachedSecret{
		value:     secretValue,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return secretValue, nil
}

// GetSecretByRef retrieves a secret using a GCP secret reference (full path)
func (c *GCPSecretManagerClient) GetSecretByRef(ctx context.Context, secretRef string) (string, error) {
	// Extract secret name from ref
	parts := strings.Split(secretRef, "/secrets/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid secret reference format: %s", secretRef)
	}
	secretName := parts[1]

	return c.GetSecret(ctx, secretName)
}

// UpdateSecret updates an existing secret with a new value
func (c *GCPSecretManagerClient) UpdateSecret(ctx context.Context, metadata SecretMetadata, secretValue string) (*SecretMetadata, error) {
	secretName := BuildSecretName(metadata.TenantID, metadata.SecretType, metadata.Provider, metadata.SecretName)
	secretID := c.BuildSecretID(secretName)

	// Add a new version
	versionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretID,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(secretValue),
		},
	}

	version, err := c.client.AddSecretVersion(ctx, versionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to add secret version: %w", err)
	}

	// Invalidate cache
	c.mu.Lock()
	delete(c.cache, secretName)
	c.mu.Unlock()

	metadata.GCPSecretID = secretID
	metadata.Version = extractVersion(version.Name)

	return &metadata, nil
}

// DeleteSecret deletes a secret from GCP Secret Manager
func (c *GCPSecretManagerClient) DeleteSecret(ctx context.Context, secretName string) error {
	secretID := c.BuildSecretID(secretName)

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: secretID,
	}

	if err := c.client.DeleteSecret(ctx, req); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Remove from cache
	c.mu.Lock()
	delete(c.cache, secretName)
	c.mu.Unlock()

	return nil
}

// RotateSecret creates a new version of a secret with a new value
func (c *GCPSecretManagerClient) RotateSecret(ctx context.Context, secretName string, newValue string) (string, error) {
	secretID := c.BuildSecretID(secretName)

	versionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretID,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(newValue),
		},
	}

	version, err := c.client.AddSecretVersion(ctx, versionReq)
	if err != nil {
		return "", fmt.Errorf("failed to rotate secret: %w", err)
	}

	// Invalidate cache
	c.mu.Lock()
	delete(c.cache, secretName)
	c.mu.Unlock()

	return extractVersion(version.Name), nil
}

// GenerateToken generates a cryptographically secure random token
func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ListTenantSecrets lists all secrets for a tenant
func (c *GCPSecretManagerClient) ListTenantSecrets(ctx context.Context, tenantID string) ([]SecretMetadata, error) {
	sanitizedTenantID := sanitizeForGCP(tenantID)

	req := &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", c.projectID),
		Filter: fmt.Sprintf("labels.tenant_id=%s", sanitizedTenantID),
	}

	var secrets []SecretMetadata
	it := c.client.ListSecrets(ctx, req)

	for {
		secret, err := it.Next()
		if err != nil {
			break
		}

		metadata := SecretMetadata{
			TenantID:    tenantID,
			GCPSecretID: secret.Name,
		}

		if labels := secret.Labels; labels != nil {
			if st, ok := labels["secret_type"]; ok {
				metadata.SecretType = SecretType(st)
			}
			if p, ok := labels["provider"]; ok {
				metadata.Provider = SecretProvider(p)
			}
		}

		// Extract secret name from full path
		parts := strings.Split(secret.Name, "/")
		if len(parts) > 0 {
			metadata.SecretName = parts[len(parts)-1]
		}

		secrets = append(secrets, metadata)
	}

	return secrets, nil
}

// DeleteTenantSecrets deletes all secrets for a tenant
func (c *GCPSecretManagerClient) DeleteTenantSecrets(ctx context.Context, tenantID string) error {
	secrets, err := c.ListTenantSecrets(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list tenant secrets: %w", err)
	}

	for _, secret := range secrets {
		if err := c.DeleteSecret(ctx, secret.SecretName); err != nil {
			return fmt.Errorf("failed to delete secret %s: %w", secret.SecretName, err)
		}
	}

	return nil
}

// ClearCache clears the secret cache
func (c *GCPSecretManagerClient) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string]*cachedSecret)
	c.mu.Unlock()
}

// Helper functions

func sanitizeForGCP(s string) string {
	// GCP secret names: lowercase letters, numbers, hyphens, underscores
	// Max length: 255 characters
	result := strings.ToLower(s)
	result = strings.ReplaceAll(result, " ", "-")

	// Keep only allowed characters
	var sb strings.Builder
	for _, c := range result {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			sb.WriteRune(c)
		}
	}
	result = sb.String()

	// Truncate if too long
	if len(result) > 63 { // Keep it shorter for composed names
		result = result[:63]
	}

	return result
}

func extractVersion(versionName string) string {
	// Format: projects/{project}/secrets/{secret}/versions/{version}
	parts := strings.Split(versionName, "/versions/")
	if len(parts) == 2 {
		return parts[1]
	}
	return "latest"
}
