package authz

import (
	"context"
	"fmt"
	"sync"
)

// MultiStoreClient manages multiple OpenFGA stores (one per product).
// Platform checks go to the "platform" store.
// Product-specific checks go to the product's store.
type MultiStoreClient struct {
	mu      sync.RWMutex
	clients map[string]*Client
	url     string
	apiKey  string
}

// NewMultiStoreClient creates a client manager for multiple OpenFGA stores.
func NewMultiStoreClient(url, apiKey string) *MultiStoreClient {
	return &MultiStoreClient{
		clients: make(map[string]*Client),
		url:     url,
		apiKey:  apiKey,
	}
}

// RegisterStore adds a store by name and ID.
func (m *MultiStoreClient) RegisterStore(name, storeID string) error {
	client, err := NewClient(Config{
		URL:     m.url,
		APIKey:  m.apiKey,
		StoreID: storeID,
	})
	if err != nil {
		return fmt.Errorf("authz: register store %s: %w", name, err)
	}
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()
	return nil
}

// Store returns the client for a named store.
func (m *MultiStoreClient) Store(name string) (*Client, error) {
	m.mu.RLock()
	c, ok := m.clients[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("authz: store %q not registered", name)
	}
	return c, nil
}

// Platform returns the platform store client.
func (m *MultiStoreClient) Platform() (*Client, error) {
	return m.Store("platform")
}

// CanInStore checks authorization in a specific store.
func (m *MultiStoreClient) CanInStore(ctx context.Context, storeName, userID, relation, objectType, objectID string) (bool, error) {
	c, err := m.Store(storeName)
	if err != nil {
		return false, err
	}
	return c.Can(ctx, userID, relation, objectType, objectID)
}

// IsTenantMember checks if a user is a member of a tenant (platform store).
func (m *MultiStoreClient) IsTenantMember(ctx context.Context, userID, tenantID string) (bool, error) {
	return m.CanInStore(ctx, "platform", userID, "member", "tenant", tenantID)
}
