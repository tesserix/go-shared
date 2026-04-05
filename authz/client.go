// Package authz provides OpenFGA authorization checks with multi-store support.
// Each product has its own OpenFGA store. Platform checks (tenant membership)
// use the "platform" store; product checks use the product's store.
package authz

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

// Client wraps the OpenFGA SDK for authorization checks.
type Client struct {
	fga     *client.OpenFgaClient
	storeID string
}

// Config for creating an authz client.
type Config struct {
	// URL of the OpenFGA service (e.g., Cloud Run URL, K8s DNS name, or localhost for dev)
	URL string
	// API key (preshared key auth)
	APIKey string
	// StoreID to use for checks
	StoreID string
	// AuthMode controls how the client authenticates to OpenFGA's host infrastructure.
	// - "serverless" (default): sends OIDC token via X-Serverless-Authorization header
	//   (required when OpenFGA is behind Cloud Run IAM which intercepts Authorization header)
	// - "mesh": no infrastructure-level auth header; relies on Istio mTLS for transport auth
	//   (use on GKE where OpenFGA is inside the service mesh)
	// - "none": no OIDC token at all (local dev)
	// If empty, falls back to OPENFGA_AUTH_MODE env var, then defaults to "serverless".
	AuthMode string
}

// NewClient creates a new authorization client for a specific OpenFGA store.
func NewClient(cfg Config) (*Client, error) {
	fgaCfg := &client.ClientConfiguration{
		ApiUrl:  cfg.URL,
		StoreId: cfg.StoreID,
	}

	// Determine auth mode for the OpenFGA host infrastructure.
	authMode := resolveAuthMode(cfg.AuthMode)

	switch authMode {
	case "preshared":
		// GKE with preshared key auth — send API key as bearer token.
		fgaCfg.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIKey,
			},
		}
		log.Printf("[authz] auth mode %q: no infrastructure auth", authMode)
	case "serverless":
		// Cloud Run IAM intercepts Authorization header, causing conflict with
		// the SDK's preshared-key Authorization. Send OIDC token via
		// X-Serverless-Authorization header instead.
		fgaCfg.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIKey,
			},
		}
		httpClient, err := serverlessAuthHTTPClient(cfg.URL)
		if err != nil {
			log.Printf("[authz] OIDC HTTP client unavailable (local dev?): %v", err)
		} else {
			fgaCfg.HTTPClient = httpClient
		}
	case "mesh":
		// GKE with Istio mTLS — no infrastructure-level auth needed.
		// The service mesh handles transport authentication between pods.
		// No credentials required; the SDK connects without API token auth.
		log.Printf("[authz] mesh mode: relying on Istio mTLS, no OIDC header")
	default:
		// "none" or unknown — no credentials (local dev).
		log.Printf("[authz] auth mode %q: no infrastructure auth", authMode)
	}

	fgaClient, err := client.NewSdkClient(fgaCfg)
	if err != nil {
		return nil, fmt.Errorf("authz: create client: %w", err)
	}

	return &Client{fga: fgaClient, storeID: cfg.StoreID}, nil
}

// resolveAuthMode determines the OpenFGA infrastructure auth mode.
// Priority: explicit config > OPENFGA_AUTH_MODE env var > "serverless" default.
func resolveAuthMode(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if mode := os.Getenv("OPENFGA_AUTH_MODE"); mode != "" {
		return mode
	}
	return "serverless"
}

// serverlessAuthTransport wraps an OIDC token source to add the Google ID token
// in the X-Serverless-Authorization header. This lets the SDK own the standard
// Authorization header (preshared key) while Cloud Run IAM validates the OIDC
// token from the secondary header.
type serverlessAuthTransport struct {
	base     http.RoundTripper
	tokenSrc oauth2.TokenSource
	audience string
}

func (t *serverlessAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.tokenSrc.Token()
	if err != nil {
		return nil, fmt.Errorf("authz: get OIDC token: %w", err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("X-Serverless-Authorization", "Bearer "+token.AccessToken)
	return t.base.RoundTrip(req)
}

// serverlessAuthHTTPClient returns an HTTP client that adds Google OIDC ID tokens
// in the X-Serverless-Authorization header (Cloud Run's secondary auth header).
func serverlessAuthHTTPClient(audience string) (*http.Client, error) {
	ctx := context.Background()
	ts, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &serverlessAuthTransport{
			base:     http.DefaultTransport,
			tokenSrc: ts,
			audience: audience,
		},
	}, nil
}

// Can checks if a user has a relation on an object.
// Example: Can(ctx, "user123", "can_view", "order", "456")
func (c *Client) Can(ctx context.Context, userID, relation, objectType, objectID string) (bool, error) {
	body := client.ClientCheckRequest{
		User:     fmt.Sprintf("user:%s", userID),
		Relation: relation,
		Object:   fmt.Sprintf("%s:%s", objectType, objectID),
	}

	resp, err := c.fga.Check(ctx).Body(body).Execute()
	if err != nil {
		return false, fmt.Errorf("authz: check %s %s %s:%s: %w", userID, relation, objectType, objectID, err)
	}

	return resp.GetAllowed(), nil
}

// Grant creates a relationship tuple (e.g., user:123 is owner of store:456).
func (c *Client) Grant(ctx context.Context, userID, relation, objectType, objectID string) error {
	body := client.ClientWriteRequest{
		Writes: []client.ClientTupleKey{
			{
				User:     fmt.Sprintf("user:%s", userID),
				Relation: relation,
				Object:   fmt.Sprintf("%s:%s", objectType, objectID),
			},
		},
	}

	_, err := c.fga.Write(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("authz: grant %s %s %s:%s: %w", userID, relation, objectType, objectID, err)
	}
	return nil
}

// Revoke removes a relationship tuple.
func (c *Client) Revoke(ctx context.Context, userID, relation, objectType, objectID string) error {
	body := client.ClientWriteRequest{
		Deletes: []client.ClientTupleKeyWithoutCondition{
			{
				User:     fmt.Sprintf("user:%s", userID),
				Relation: relation,
				Object:   fmt.Sprintf("%s:%s", objectType, objectID),
			},
		},
	}

	_, err := c.fga.Write(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("authz: revoke %s %s %s:%s: %w", userID, relation, objectType, objectID, err)
	}
	return nil
}

// ListObjects returns all objects of a type that a user has a relation to.
// Example: ListObjects(ctx, "user123", "can_view", "order") → ["order:1", "order:5"]
func (c *Client) ListObjects(ctx context.Context, userID, relation, objectType string) ([]string, error) {
	body := client.ClientListObjectsRequest{
		User:     fmt.Sprintf("user:%s", userID),
		Relation: relation,
		Type:     objectType,
	}

	resp, err := c.fga.ListObjects(ctx).Body(body).Execute()
	if err != nil {
		return nil, fmt.Errorf("authz: list objects for %s %s %s: %w", userID, relation, objectType, err)
	}

	return resp.GetObjects(), nil
}

// GrantRelation creates a relationship between two objects.
// Example: GrantRelation(ctx, "store:456", "store", "order", "789")
// Means: order:789 has store relation to store:456
func (c *Client) GrantRelation(ctx context.Context, subjectType, subjectID, relation, objectType, objectID string) error {
	body := client.ClientWriteRequest{
		Writes: []client.ClientTupleKey{
			{
				User:     fmt.Sprintf("%s:%s", subjectType, subjectID),
				Relation: relation,
				Object:   fmt.Sprintf("%s:%s", objectType, objectID),
			},
		},
	}

	_, err := c.fga.Write(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("authz: grant relation %s:%s %s %s:%s: %w", subjectType, subjectID, relation, objectType, objectID, err)
	}
	return nil
}

// WriteModel writes an authorization model to the store. Used during deploy/migration.
func (c *Client) WriteModel(ctx context.Context, model openfga.WriteAuthorizationModelRequest) (string, error) {
	resp, err := c.fga.WriteAuthorizationModel(ctx).Body(model).Execute()
	if err != nil {
		return "", fmt.Errorf("authz: write model: %w", err)
	}
	return resp.GetAuthorizationModelId(), nil
}
