package secrets

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// EnvSecretFetcher fetches secrets from GCP Secret Manager and provides them
// as if they were environment variables. It uses Workload Identity for authentication.
type EnvSecretFetcher struct {
	client    *secretmanager.Client
	projectID string
	prefix    string
}

// NewEnvSecretFetcher creates a new secret fetcher that uses GCP Secret Manager.
// It automatically uses Workload Identity on GKE for authentication.
// If USE_GCP_SECRET_MANAGER is not "true", returns nil (use env vars directly).
func NewEnvSecretFetcher(ctx context.Context) (*EnvSecretFetcher, error) {
	useGCP := os.Getenv("USE_GCP_SECRET_MANAGER")
	if useGCP != "true" {
		log.Println("GCP Secret Manager disabled, using environment variables")
		return nil, nil
	}

	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		// Try to auto-detect from GKE metadata server
		projectID = detectProjectID()
		if projectID == "" {
			return nil, fmt.Errorf("GCP_PROJECT_ID not set and could not be auto-detected")
		}
		log.Printf("Auto-detected GCP project ID: %s", projectID)
	}

	prefix := os.Getenv("GCP_SECRET_PREFIX")
	if prefix == "" {
		prefix = "dev"
	}

	// Create client using default credentials (Workload Identity on GKE)
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	log.Printf("GCP Secret Manager enabled (project: %s, prefix: %s)", projectID, prefix)

	return &EnvSecretFetcher{
		client:    client,
		projectID: projectID,
		prefix:    prefix,
	}, nil
}

// GetSecret fetches a secret from GCP Secret Manager.
// secretName should be just the name part (e.g., "marketplace-postgresql-password")
// The prefix will be prepended automatically (e.g., "dev-marketplace-postgresql-password")
func (f *EnvSecretFetcher) GetSecret(ctx context.Context, secretName string) (string, error) {
	if f == nil {
		return "", fmt.Errorf("secret fetcher is nil")
	}

	fullSecretName := fmt.Sprintf("%s-%s", f.prefix, secretName)
	secretPath := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", f.projectID, fullSecretName)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	}

	result, err := f.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", fullSecretName, err)
	}

	return string(result.Payload.Data), nil
}

// GetSecretOrEnv tries to get a secret from GCP Secret Manager using the secret name
// from the environment variable (e.g., DB_PASSWORD_SECRET_NAME), and falls back to
// the direct environment variable (e.g., DB_PASSWORD) if GCP Secret Manager is not available.
func (f *EnvSecretFetcher) GetSecretOrEnv(ctx context.Context, secretNameEnvVar, fallbackEnvVar, defaultValue string) string {
	// If GCP Secret Manager is enabled
	if f != nil {
		secretName := os.Getenv(secretNameEnvVar)
		if secretName != "" {
			// The secretName already includes the prefix (e.g., "dev-marketplace-postgresql-password")
			// So we need to extract just the part after the prefix
			value, err := f.getSecretDirect(ctx, secretName)
			if err != nil {
				log.Printf("Warning: failed to get secret %s from GCP: %v, falling back to env var", secretName, err)
			} else {
				return value
			}
		}
	}

	// Fall back to direct environment variable
	if value := os.Getenv(fallbackEnvVar); value != "" {
		return value
	}

	return defaultValue
}

// getSecretDirect fetches a secret using the full secret name (already includes prefix)
func (f *EnvSecretFetcher) getSecretDirect(ctx context.Context, fullSecretName string) (string, error) {
	secretPath := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", f.projectID, fullSecretName)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	}

	result, err := f.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", fullSecretName, err)
	}

	return string(result.Payload.Data), nil
}

// Close closes the secret manager client
func (f *EnvSecretFetcher) Close() error {
	if f != nil && f.client != nil {
		return f.client.Close()
	}
	return nil
}

// detectProjectID tries to detect the GCP project ID from the metadata server
func detectProjectID() string {
	// On GKE, the project ID can be fetched from the metadata server
	// This is handled automatically by the Google Cloud SDK when creating clients
	// For now, we'll return empty and let the SDK handle it
	return ""
}

// MustGetEnvSecretFetcher creates a new secret fetcher or panics on error.
// Use this for service initialization where GCP Secret Manager is required.
func MustGetEnvSecretFetcher(ctx context.Context) *EnvSecretFetcher {
	fetcher, err := NewEnvSecretFetcher(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCP Secret Manager client: %v", err)
	}
	return fetcher
}

// LoadDatabasePassword loads the database password from GCP Secret Manager or falls back to env var
func LoadDatabasePassword(ctx context.Context, fetcher *EnvSecretFetcher) string {
	return fetcher.GetSecretOrEnv(ctx, "DB_PASSWORD_SECRET_NAME", "DB_PASSWORD", "password")
}

// LoadJWTSecret loads the JWT secret from GCP Secret Manager or falls back to env var
func LoadJWTSecret(ctx context.Context, fetcher *EnvSecretFetcher) string {
	return fetcher.GetSecretOrEnv(ctx, "JWT_SECRET_NAME", "JWT_SECRET", "default-jwt-secret")
}

// LoadRedisPassword loads the Redis password from GCP Secret Manager or falls back to env var
func LoadRedisPassword(ctx context.Context, fetcher *EnvSecretFetcher) string {
	return fetcher.GetSecretOrEnv(ctx, "REDIS_PASSWORD_SECRET_NAME", "REDIS_PASSWORD", "")
}

// FetchSecretsWithTimeout fetches secrets with a timeout context
func FetchSecretsWithTimeout(timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}
