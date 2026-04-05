package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheConfig holds configuration for the cache layer
type CacheConfig struct {
	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// L1 (in-memory) cache configuration
	L1Enabled  bool
	L1MaxItems int
	L1TTL      time.Duration

	// Default TTL for cached items
	DefaultTTL time.Duration

	// Key prefix for namespacing
	KeyPrefix string
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		L1Enabled:     true,
		L1MaxItems:    10000,
		L1TTL:         30 * time.Second,
		DefaultTTL:    5 * time.Minute,
		KeyPrefix:     "tesseract:",
	}
}

// CacheLayer provides a multi-level caching implementation
// L1: In-memory LRU cache for hot data (sub-millisecond access)
// L2: Redis for distributed caching across service instances
type CacheLayer struct {
	redis    *redis.Client
	l1Cache  *L1Cache
	config   CacheConfig
	mu       sync.RWMutex
	stats    *CacheStats
}

// CacheStats tracks cache hit/miss statistics
type CacheStats struct {
	L1Hits   int64
	L1Misses int64
	L2Hits   int64
	L2Misses int64
	mu       sync.Mutex
}

// L1Cache is a simple in-memory LRU cache
type L1Cache struct {
	items    map[string]*l1CacheItem
	order    []string
	maxItems int
	ttl      time.Duration
	mu       sync.RWMutex
}

type l1CacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewCacheLayer creates a new multi-level cache
func NewCacheLayer(config CacheConfig) (*CacheLayer, error) {
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.RedisAddr,
		Password:     config.RedisPassword,
		DB:           config.RedisDB,
		PoolSize:     50,
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cache := &CacheLayer{
		redis:  rdb,
		config: config,
		stats:  &CacheStats{},
	}

	// Initialize L1 cache if enabled
	if config.L1Enabled {
		cache.l1Cache = &L1Cache{
			items:    make(map[string]*l1CacheItem),
			order:    make([]string, 0, config.L1MaxItems),
			maxItems: config.L1MaxItems,
			ttl:      config.L1TTL,
		}

		// Start background cleanup for expired L1 items
		go cache.l1CleanupLoop()
	}

	return cache, nil
}

// NewCacheLayerFromClient creates a cache layer with an existing Redis client
func NewCacheLayerFromClient(client *redis.Client, config CacheConfig) *CacheLayer {
	cache := &CacheLayer{
		redis:  client,
		config: config,
		stats:  &CacheStats{},
	}

	if config.L1Enabled {
		cache.l1Cache = &L1Cache{
			items:    make(map[string]*l1CacheItem),
			order:    make([]string, 0, config.L1MaxItems),
			maxItems: config.L1MaxItems,
			ttl:      config.L1TTL,
		}
		go cache.l1CleanupLoop()
	}

	return cache
}

// prefixKey adds the configured prefix to a key
func (c *CacheLayer) prefixKey(key string) string {
	return c.config.KeyPrefix + key
}

// Get retrieves a value from the cache (L1 first, then L2)
func (c *CacheLayer) Get(ctx context.Context, key string) ([]byte, error) {
	prefixedKey := c.prefixKey(key)

	// Try L1 cache first
	if c.l1Cache != nil {
		if value, found := c.l1Cache.Get(prefixedKey); found {
			c.stats.mu.Lock()
			c.stats.L1Hits++
			c.stats.mu.Unlock()
			return value, nil
		}
		c.stats.mu.Lock()
		c.stats.L1Misses++
		c.stats.mu.Unlock()
	}

	// Try L2 (Redis) cache
	value, err := c.redis.Get(ctx, prefixedKey).Bytes()
	if err == redis.Nil {
		c.stats.mu.Lock()
		c.stats.L2Misses++
		c.stats.mu.Unlock()
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	c.stats.mu.Lock()
	c.stats.L2Hits++
	c.stats.mu.Unlock()

	// Populate L1 cache with the value
	if c.l1Cache != nil {
		c.l1Cache.Set(prefixedKey, value)
	}

	return value, nil
}

// GetJSON retrieves and unmarshals a JSON value from cache
func (c *CacheLayer) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Set stores a value in both L1 and L2 cache
func (c *CacheLayer) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	prefixedKey := c.prefixKey(key)

	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}

	// Set in L2 (Redis) first
	if err := c.redis.Set(ctx, prefixedKey, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}

	// Set in L1 cache
	if c.l1Cache != nil {
		c.l1Cache.Set(prefixedKey, value)
	}

	return nil
}

// SetJSON marshals and stores a value in cache
func (c *CacheLayer) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}
	return c.Set(ctx, key, data, ttl)
}

// Delete removes a value from both L1 and L2 cache
func (c *CacheLayer) Delete(ctx context.Context, keys ...string) error {
	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	// Delete from L1
	if c.l1Cache != nil {
		for _, key := range prefixedKeys {
			c.l1Cache.Delete(key)
		}
	}

	// Delete from L2 (Redis)
	if err := c.redis.Del(ctx, prefixedKeys...).Err(); err != nil {
		return fmt.Errorf("redis delete error: %w", err)
	}

	return nil
}

// DeletePattern removes all keys matching a pattern
func (c *CacheLayer) DeletePattern(ctx context.Context, pattern string) error {
	prefixedPattern := c.prefixKey(pattern)

	// Use SCAN to find matching keys (safer than KEYS for production)
	var cursor uint64
	var keysToDelete []string

	for {
		keys, nextCursor, err := c.redis.Scan(ctx, cursor, prefixedPattern, 100).Result()
		if err != nil {
			return fmt.Errorf("redis scan error: %w", err)
		}

		keysToDelete = append(keysToDelete, keys...)
		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	if len(keysToDelete) == 0 {
		return nil
	}

	// Delete from L1
	if c.l1Cache != nil {
		for _, key := range keysToDelete {
			c.l1Cache.Delete(key)
		}
	}

	// Delete from L2 (Redis) in batches
	const batchSize = 1000
	for i := 0; i < len(keysToDelete); i += batchSize {
		end := i + batchSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}
		if err := c.redis.Del(ctx, keysToDelete[i:end]...).Err(); err != nil {
			return fmt.Errorf("redis delete batch error: %w", err)
		}
	}

	return nil
}

// GetOrSet attempts to get a value, and if not found, calls the loader function
// and caches the result. This is atomic per key to prevent thundering herd.
func (c *CacheLayer) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	// Try to get from cache first
	value, err := c.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	// Cache miss - load the value
	value, err = loader()
	if err != nil {
		return nil, err
	}

	// Store in cache (ignore errors as the value was loaded successfully)
	_ = c.Set(ctx, key, value, ttl)

	return value, nil
}

// GetOrSetJSON is like GetOrSet but for JSON values
func (c *CacheLayer) GetOrSetJSON(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func() (interface{}, error)) error {
	value, err := c.GetOrSet(ctx, key, ttl, func() ([]byte, error) {
		data, err := loader()
		if err != nil {
			return nil, err
		}
		return json.Marshal(data)
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(value, dest)
}

// Stats returns current cache statistics
func (c *CacheLayer) Stats() CacheStats {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()
	return CacheStats{
		L1Hits:   c.stats.L1Hits,
		L1Misses: c.stats.L1Misses,
		L2Hits:   c.stats.L2Hits,
		L2Misses: c.stats.L2Misses,
	}
}

// Close closes the cache layer and releases resources
func (c *CacheLayer) Close() error {
	return c.redis.Close()
}

// L1 Cache methods

func (l *L1Cache) Get(key string) ([]byte, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	item, found := l.items[key]
	if !found {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.value, true
}

func (l *L1Cache) Set(key string, value []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if key already exists
	if _, exists := l.items[key]; !exists {
		// Evict oldest item if at capacity
		if len(l.order) >= l.maxItems {
			oldestKey := l.order[0]
			l.order = l.order[1:]
			delete(l.items, oldestKey)
		}
		l.order = append(l.order, key)
	}

	l.items[key] = &l1CacheItem{
		value:     value,
		expiresAt: time.Now().Add(l.ttl),
	}
}

func (l *L1Cache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.items, key)

	// Remove from order slice
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
}

// l1CleanupLoop periodically removes expired items from L1 cache
func (c *CacheLayer) l1CleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if c.l1Cache == nil {
			return
		}

		c.l1Cache.mu.Lock()
		now := time.Now()
		keysToDelete := make([]string, 0)

		for key, item := range c.l1Cache.items {
			if now.After(item.expiresAt) {
				keysToDelete = append(keysToDelete, key)
			}
		}

		for _, key := range keysToDelete {
			delete(c.l1Cache.items, key)
		}
		c.l1Cache.mu.Unlock()
	}
}

// CacheMiss error for when a key is not found
var ErrCacheMiss = fmt.Errorf("cache miss")

// Cache key builders for common patterns

// TenantProductKey generates a cache key for a product
func TenantProductKey(tenantID, productID string) string {
	return fmt.Sprintf("product:%s:%s", tenantID, productID)
}

// TenantProductListKey generates a cache key for a product list
func TenantProductListKey(tenantID string, page, limit int, filters string) string {
	return fmt.Sprintf("products:list:%s:%d:%d:%s", tenantID, page, limit, filters)
}

// TenantCategoryKey generates a cache key for a category
func TenantCategoryKey(tenantID, categoryID string) string {
	return fmt.Sprintf("category:%s:%s", tenantID, categoryID)
}

// TenantCategoryTreeKey generates a cache key for category tree
func TenantCategoryTreeKey(tenantID string) string {
	return fmt.Sprintf("categories:tree:%s", tenantID)
}

// TenantCustomerKey generates a cache key for a customer
func TenantCustomerKey(tenantID, customerID string) string {
	return fmt.Sprintf("customer:%s:%s", tenantID, customerID)
}

// TenantSettingsKey generates a cache key for tenant settings
func TenantSettingsKey(tenantID string) string {
	return fmt.Sprintf("settings:%s", tenantID)
}

// InvalidationPattern generates a wildcard pattern for cache invalidation
func InvalidationPattern(tenantID, entity string) string {
	return fmt.Sprintf("%s:*:%s:*", entity, tenantID)
}

// TTL constants for different data types
const (
	TTLProducts   = 5 * time.Minute   // Products change moderately
	TTLCategories = 30 * time.Minute  // Categories rarely change
	TTLCustomers  = 2 * time.Minute   // Customer data is sensitive
	TTLInventory  = 30 * time.Second  // Inventory changes frequently
	TTLSettings   = 15 * time.Minute  // Settings rarely change
	TTLAnalytics  = 1 * time.Minute   // Analytics data updates frequently
)
