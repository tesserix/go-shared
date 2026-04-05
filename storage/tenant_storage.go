// Package storage provides multi-tenant blob storage on GCS.
// Files are stored at: gs://{bucket}/{app}/{tenant_id}/{category}/{filename}
// App-level isolation via IAM Conditions; tenant isolation in application code.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
)

// TenantStorage scopes GCS operations to a specific app.
type TenantStorage struct {
	client  *gcs.Client
	bucket  string
	appName string
}

// NewTenantStorage creates a storage client scoped to an app prefix.
// appName is the top-level prefix (e.g., "platform", "marketplace").
func NewTenantStorage(ctx context.Context, bucket, appName string) (*TenantStorage, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: create client: %w", err)
	}
	return &TenantStorage{
		client:  client,
		bucket:  bucket,
		appName: appName,
	}, nil
}

// Upload stores a file for a specific tenant.
// Returns the full object path (e.g., "marketplace/tenant123/products/image.jpg").
func (s *TenantStorage) Upload(ctx context.Context, tenantID, category, filename string, data io.Reader, contentType string) (string, error) {
	if err := s.validateTenantID(tenantID); err != nil {
		return "", err
	}

	path := s.objectPath(tenantID, category, filename)
	obj := s.client.Bucket(s.bucket).Object(path)

	w := obj.NewWriter(ctx)
	w.ContentType = contentType
	w.CacheControl = "public, max-age=86400"

	if _, err := io.Copy(w, data); err != nil {
		w.Close()
		return "", fmt.Errorf("storage: upload %s: %w", path, err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("storage: close writer %s: %w", path, err)
	}

	return path, nil
}

// Download retrieves a file. Caller must close the returned reader.
func (s *TenantStorage) Download(ctx context.Context, tenantID, path string) (io.ReadCloser, error) {
	if !s.isOwnedByTenant(tenantID, path) {
		return nil, fmt.Errorf("storage: path %s does not belong to tenant %s", path, tenantID)
	}

	reader, err := s.client.Bucket(s.bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: download %s: %w", path, err)
	}
	return reader, nil
}

// SignedURL generates a time-limited signed URL for a tenant's file.
func (s *TenantStorage) SignedURL(ctx context.Context, tenantID, path string, expiry time.Duration) (string, error) {
	if !s.isOwnedByTenant(tenantID, path) {
		return "", fmt.Errorf("storage: path %s does not belong to tenant %s", path, tenantID)
	}

	url, err := s.client.Bucket(s.bucket).SignedURL(path, &gcs.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	})
	if err != nil {
		return "", fmt.Errorf("storage: sign url %s: %w", path, err)
	}
	return url, nil
}

// Delete removes a tenant's file.
func (s *TenantStorage) Delete(ctx context.Context, tenantID, path string) error {
	if !s.isOwnedByTenant(tenantID, path) {
		return fmt.Errorf("storage: path %s does not belong to tenant %s", path, tenantID)
	}

	if err := s.client.Bucket(s.bucket).Object(path).Delete(ctx); err != nil {
		return fmt.Errorf("storage: delete %s: %w", path, err)
	}
	return nil
}

// List returns all objects for a tenant under a category prefix.
func (s *TenantStorage) List(ctx context.Context, tenantID, category string) ([]string, error) {
	prefix := fmt.Sprintf("%s/%s/%s/", s.appName, tenantID, category)
	it := s.client.Bucket(s.bucket).Objects(ctx, &gcs.Query{Prefix: prefix})

	var paths []string
	for {
		attrs, err := it.Next()
		if err != nil {
			break
		}
		paths = append(paths, attrs.Name)
	}
	return paths, nil
}

func (s *TenantStorage) objectPath(tenantID, category, filename string) string {
	return fmt.Sprintf("%s/%s/%s/%s", s.appName, tenantID, category, filename)
}

func (s *TenantStorage) isOwnedByTenant(tenantID, path string) bool {
	prefix := fmt.Sprintf("%s/%s/", s.appName, tenantID)
	return strings.HasPrefix(path, prefix)
}

func (s *TenantStorage) validateTenantID(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("storage: tenant ID cannot be empty")
	}
	if strings.Contains(tenantID, "/") || strings.Contains(tenantID, "..") {
		return fmt.Errorf("storage: invalid tenant ID %q", tenantID)
	}
	return nil
}

// Close releases resources.
func (s *TenantStorage) Close() error {
	return s.client.Close()
}
