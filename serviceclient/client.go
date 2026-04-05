// Package serviceclient provides an HTTP client for service-to-service
// communication with automatic auth injection.
//
// Auth modes (SERVICE_AUTH_MODE env var):
//   - "oidc" (default): Google OIDC tokens via metadata server.
//     Works on Cloud Run (implicit SA) and GKE (Workload Identity).
//   - "mesh": no auth header — relies on Istio mTLS for transport auth (GKE).
//   - "key": shared INTERNAL_SERVICE_KEY header (local dev).
package serviceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

// Client calls another Cloud Run service with automatic auth.
type Client struct {
	baseURL    string
	httpClient *http.Client
	tokenSrc   tokenSource
	headers    map[string]string
}

type tokenSource interface {
	Token(ctx context.Context, audience string) (string, error)
}

// NewClient creates a service client for the given base URL.
// Auth method is determined by SERVICE_AUTH_MODE env var:
//   - "oidc" (default): Google OIDC tokens from metadata server
//   - "mesh": no auth header; relies on Istio mTLS for service identity (GKE)
//   - "key": shared INTERNAL_SERVICE_KEY header (for local dev)
func NewClient(baseURL string) (*Client, error) {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	mode := os.Getenv("SERVICE_AUTH_MODE")
	switch mode {
	case "key":
		c.tokenSrc = &keyTokenSource{key: os.Getenv("INTERNAL_SERVICE_KEY")}
	case "mesh":
		c.tokenSrc = &meshTokenSource{}
	default:
		c.tokenSrc = &oidcTokenSource{}
	}

	return c, nil
}

// WithHeaders returns a copy of the client with additional default headers.
// These headers are sent on every request (useful for X-Tenant-ID, X-API-Key, etc.).
func (c *Client) WithHeaders(headers map[string]string) *Client {
	clone := *c
	clone.headers = make(map[string]string, len(c.headers)+len(headers))
	for k, v := range c.headers {
		clone.headers[k] = v
	}
	for k, v := range headers {
		clone.headers[k] = v
	}
	return &clone
}

// Get performs an authenticated GET request.
func (c *Client) Get(ctx context.Context, path string, result any) error {
	return c.do(ctx, http.MethodGet, path, nil, result)
}

// Post performs an authenticated POST request with JSON body.
func (c *Client) Post(ctx context.Context, path string, body any, result any) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}

// Put performs an authenticated PUT request with JSON body.
func (c *Client) Put(ctx context.Context, path string, body any, result any) error {
	return c.do(ctx, http.MethodPut, path, body, result)
}

// Delete performs an authenticated DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result any) error {
	return c.do(ctx, http.MethodDelete, path, nil, result)
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("serviceclient: marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("serviceclient: create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Inject default headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Inject auth token (skipped in mesh mode where Istio mTLS handles auth)
	token, err := c.tokenSrc.Token(ctx, c.baseURL)
	if err != nil {
		return fmt.Errorf("serviceclient: get token: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("serviceclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("serviceclient: %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("serviceclient: decode response: %w", err)
		}
	}

	return nil
}

// oidcTokenSource fetches OIDC tokens from the GCP metadata server.
// Works on both Cloud Run and GKE (with Workload Identity).
// Token sources are cached per audience to avoid re-creating HTTP clients
// and metadata server connections on every call.
type oidcTokenSource struct {
	cache sync.Map // audience string → oauth2.TokenSource
}

func (s *oidcTokenSource) Token(ctx context.Context, audience string) (string, error) {
	ts, err := s.getOrCreate(ctx, audience)
	if err != nil {
		return "", fmt.Errorf("create token source for %s: %w", audience, err)
	}
	t, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("fetch token for %s: %w", audience, err)
	}
	return t.AccessToken, nil
}

func (s *oidcTokenSource) getOrCreate(_ context.Context, audience string) (oauth2.TokenSource, error) {
	if v, ok := s.cache.Load(audience); ok {
		return v.(oauth2.TokenSource), nil
	}
	// Use background context — the token source outlives any single request.
	ts, err := idtoken.NewTokenSource(context.Background(), audience)
	if err != nil {
		return nil, err
	}
	actual, _ := s.cache.LoadOrStore(audience, ts)
	return actual.(oauth2.TokenSource), nil
}

// meshTokenSource is a no-op token source for GKE with Istio mTLS.
// Istio handles transport authentication; no app-level auth header needed
// for service-to-service calls within the mesh.
type meshTokenSource struct{}

func (s *meshTokenSource) Token(_ context.Context, _ string) (string, error) {
	return "", nil
}

// keyTokenSource uses a shared key (local dev / GKE with service key).
type keyTokenSource struct {
	key string
}

func (s *keyTokenSource) Token(_ context.Context, _ string) (string, error) {
	return s.key, nil
}
