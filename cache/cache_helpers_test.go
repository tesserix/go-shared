package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Cache key builder functions ---

func TestTenantProductKey_Format(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		productID string
		expected  string
	}{
		{
			name:      "standard IDs",
			tenantID:  "tenant-123",
			productID: "product-456",
			expected:  "product:tenant-123:product-456",
		},
		{
			name:      "UUID-style IDs",
			tenantID:  "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			productID: "11111111-2222-3333-4444-555555555555",
			expected:  "product:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:11111111-2222-3333-4444-555555555555",
		},
		{
			name:      "empty IDs",
			tenantID:  "",
			productID: "",
			expected:  "product::",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantProductKey(tc.tenantID, tc.productID)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTenantProductListKey_Format(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		page     int
		limit    int
		filters  string
		expected string
	}{
		{
			name:     "first page no filters",
			tenantID: "t1",
			page:     1,
			limit:    20,
			filters:  "",
			expected: "products:list:t1:1:20:",
		},
		{
			name:     "with filters",
			tenantID: "t2",
			page:     2,
			limit:    10,
			filters:  "category=shoes&active=true",
			expected: "products:list:t2:2:10:category=shoes&active=true",
		},
		{
			name:     "large page",
			tenantID: "tenant-abc",
			page:     100,
			limit:    50,
			filters:  "brand=nike",
			expected: "products:list:tenant-abc:100:50:brand=nike",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantProductListKey(tc.tenantID, tc.page, tc.limit, tc.filters)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTenantCategoryKey_Format(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		categoryID string
		expected   string
	}{
		{
			name:       "standard IDs",
			tenantID:   "t1",
			categoryID: "cat-99",
			expected:   "category:t1:cat-99",
		},
		{
			name:       "empty category ID",
			tenantID:   "t1",
			categoryID: "",
			expected:   "category:t1:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantCategoryKey(tc.tenantID, tc.categoryID)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTenantCategoryTreeKey_Format(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		expected string
	}{
		{
			name:     "standard tenant",
			tenantID: "tenant-42",
			expected: "categories:tree:tenant-42",
		},
		{
			name:     "empty tenant",
			tenantID: "",
			expected: "categories:tree:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantCategoryTreeKey(tc.tenantID)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTenantCustomerKey_Format(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		customerID string
		expected   string
	}{
		{
			name:       "standard IDs",
			tenantID:   "shop-1",
			customerID: "cust-7",
			expected:   "customer:shop-1:cust-7",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantCustomerKey(tc.tenantID, tc.customerID)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTenantSettingsKey_Format(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		expected string
	}{
		{
			name:     "standard tenant",
			tenantID: "shop-abc",
			expected: "settings:shop-abc",
		},
		{
			name:     "empty tenant",
			tenantID: "",
			expected: "settings:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TenantSettingsKey(tc.tenantID)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestInvalidationPattern_Format(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		entity   string
		expected string
	}{
		{
			name:     "product entity",
			tenantID: "tenant-1",
			entity:   "product",
			expected: "product:*:tenant-1:*",
		},
		{
			name:     "category entity",
			tenantID: "tenant-2",
			entity:   "category",
			expected: "category:*:tenant-2:*",
		},
		{
			name:     "empty entity",
			tenantID: "t1",
			entity:   "",
			expected: ":*:t1:*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := InvalidationPattern(tc.tenantID, tc.entity)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// --- Key builder uniqueness ---

// TestKeyBuilders_UniqueAcrossEntities verifies that different key builders
// produce distinct keys for the same tenant+ID to avoid collision.
func TestKeyBuilders_UniqueAcrossEntities(t *testing.T) {
	tenantID := "shared-tenant"
	id := "shared-id"

	productKey := TenantProductKey(tenantID, id)
	categoryKey := TenantCategoryKey(tenantID, id)
	customerKey := TenantCustomerKey(tenantID, id)

	assert.NotEqual(t, productKey, categoryKey)
	assert.NotEqual(t, productKey, customerKey)
	assert.NotEqual(t, categoryKey, customerKey)
}

// --- TTL constants ---

func TestTTLConstants_Values(t *testing.T) {
	assert.Equal(t, 5*time.Minute, TTLProducts, "TTLProducts should be 5 minutes")
	assert.Equal(t, 30*time.Minute, TTLCategories, "TTLCategories should be 30 minutes")
	assert.Equal(t, 2*time.Minute, TTLCustomers, "TTLCustomers should be 2 minutes")
	assert.Equal(t, 30*time.Second, TTLInventory, "TTLInventory should be 30 seconds")
	assert.Equal(t, 15*time.Minute, TTLSettings, "TTLSettings should be 15 minutes")
	assert.Equal(t, 1*time.Minute, TTLAnalytics, "TTLAnalytics should be 1 minute")
}

// TestTTLConstants_Ordering verifies the relative magnitudes match the
// documented intent (categories cached longer than customers, etc.).
func TestTTLConstants_Ordering(t *testing.T) {
	// Inventory changes most frequently → shortest TTL.
	assert.Less(t, TTLInventory, TTLCustomers)
	assert.Less(t, TTLCustomers, TTLProducts)
	assert.Less(t, TTLProducts, TTLSettings)
	assert.Less(t, TTLSettings, TTLCategories)
}

// --- DefaultCacheConfig ---

func TestDefaultCacheConfig_Values(t *testing.T) {
	cfg := DefaultCacheConfig()

	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, "", cfg.RedisPassword)
	assert.Equal(t, 0, cfg.RedisDB)
	assert.True(t, cfg.L1Enabled)
	assert.Equal(t, 10000, cfg.L1MaxItems)
	assert.Equal(t, 30*time.Second, cfg.L1TTL)
	assert.Equal(t, 5*time.Minute, cfg.DefaultTTL)
	assert.Equal(t, "tesseract:", cfg.KeyPrefix)
}

// --- ErrCacheMiss ---

func TestErrCacheMiss_IsDefined(t *testing.T) {
	assert.NotNil(t, ErrCacheMiss)
	assert.Equal(t, "cache miss", ErrCacheMiss.Error())
}

// --- CacheLayer.prefixKey ---

func TestCacheLayer_PrefixKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{
			name:     "standard prefix",
			prefix:   "tesseract:",
			key:      "product:tenant1:product1",
			expected: "tesseract:product:tenant1:product1",
		},
		{
			name:     "custom prefix",
			prefix:   "myapp:v2:",
			key:      "session:user-123",
			expected: "myapp:v2:session:user-123",
		},
		{
			name:     "empty prefix",
			prefix:   "",
			key:      "bare-key",
			expected: "bare-key",
		},
		{
			name:     "empty key",
			prefix:   "ns:",
			key:      "",
			expected: "ns:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layer := &CacheLayer{
				config: CacheConfig{
					KeyPrefix: tc.prefix,
				},
				stats: &CacheStats{},
			}

			got := layer.prefixKey(tc.key)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestCacheLayer_PrefixKey_DoesNotAlterOriginalKey verifies that the function
// returns a new string without modifying the input.
func TestCacheLayer_PrefixKey_DoesNotAlterOriginalKey(t *testing.T) {
	layer := &CacheLayer{
		config: CacheConfig{KeyPrefix: "prefix:"},
		stats:  &CacheStats{},
	}

	original := "my-key"
	prefixed := layer.prefixKey(original)

	assert.Equal(t, "my-key", original, "original key must not be mutated")
	assert.Equal(t, "prefix:my-key", prefixed)
}

// --- CacheStats ---

func TestCacheStats_ZeroValues(t *testing.T) {
	stats := CacheStats{}
	assert.Equal(t, int64(0), stats.L1Hits)
	assert.Equal(t, int64(0), stats.L1Misses)
	assert.Equal(t, int64(0), stats.L2Hits)
	assert.Equal(t, int64(0), stats.L2Misses)
}

// --- CacheLayer.Stats (without Redis) ---

func TestCacheLayer_Stats_ReturnsZeroInitially(t *testing.T) {
	layer := &CacheLayer{
		stats: &CacheStats{},
	}

	s := layer.Stats()
	assert.Equal(t, int64(0), s.L1Hits)
	assert.Equal(t, int64(0), s.L1Misses)
	assert.Equal(t, int64(0), s.L2Hits)
	assert.Equal(t, int64(0), s.L2Misses)
}

func TestCacheLayer_Stats_ReflectsUpdates(t *testing.T) {
	layer := &CacheLayer{
		stats: &CacheStats{
			L1Hits:   5,
			L1Misses: 3,
			L2Hits:   2,
			L2Misses: 8,
		},
	}

	s := layer.Stats()
	assert.Equal(t, int64(5), s.L1Hits)
	assert.Equal(t, int64(3), s.L1Misses)
	assert.Equal(t, int64(2), s.L2Hits)
	assert.Equal(t, int64(8), s.L2Misses)
}

// --- NewCacheLayerFromClient ---

func TestNewCacheLayerFromClient_WithL1Disabled(t *testing.T) {
	cfg := CacheConfig{
		L1Enabled:  false,
		L1MaxItems: 100,
		L1TTL:      time.Minute,
		KeyPrefix:  "test:",
	}

	layer := NewCacheLayerFromClient(nil, cfg)

	assert.NotNil(t, layer)
	assert.Nil(t, layer.l1Cache, "L1 cache should be nil when L1Enabled is false")
	assert.Equal(t, "test:", layer.config.KeyPrefix)
}

func TestNewCacheLayerFromClient_WithL1Enabled_InitialisesL1Cache(t *testing.T) {
	cfg := CacheConfig{
		L1Enabled:  true,
		L1MaxItems: 500,
		L1TTL:      30 * time.Second,
		KeyPrefix:  "svc:",
	}

	layer := NewCacheLayerFromClient(nil, cfg)

	assert.NotNil(t, layer)
	assert.NotNil(t, layer.l1Cache, "L1 cache should be initialised when L1Enabled is true")
	assert.Equal(t, 500, layer.l1Cache.maxItems)
	assert.Equal(t, 30*time.Second, layer.l1Cache.ttl)
}

func TestNewCacheLayerFromClient_StatsInitialised(t *testing.T) {
	layer := NewCacheLayerFromClient(nil, DefaultCacheConfig())
	assert.NotNil(t, layer.stats)
}

// --- Key format regression tests ---

// TestKeyBuilders_NoSpaces asserts that generated keys never contain spaces,
// which would be invalid as Redis keys in most configurations.
func TestKeyBuilders_NoSpaces(t *testing.T) {
	keys := []string{
		TenantProductKey("tenant-1", "prod-1"),
		TenantProductListKey("tenant-1", 1, 20, "active=true"),
		TenantCategoryKey("tenant-1", "cat-1"),
		TenantCategoryTreeKey("tenant-1"),
		TenantCustomerKey("tenant-1", "cust-1"),
		TenantSettingsKey("tenant-1"),
		InvalidationPattern("tenant-1", "product"),
	}

	for _, key := range keys {
		for i, ch := range key {
			assert.NotEqual(t, ' ', ch, "key %q must not contain a space at position %d", key, i)
		}
	}
}

// TestKeyBuilders_ContainsTenantID asserts that every key builder embeds the
// tenant ID so keys can be scanned or invalidated per-tenant.
func TestKeyBuilders_ContainsTenantID(t *testing.T) {
	tenantID := "acme-corp"

	keysContainingTenant := []string{
		TenantProductKey(tenantID, "p1"),
		TenantProductListKey(tenantID, 1, 10, ""),
		TenantCategoryKey(tenantID, "c1"),
		TenantCategoryTreeKey(tenantID),
		TenantCustomerKey(tenantID, "u1"),
		TenantSettingsKey(tenantID),
	}

	for _, key := range keysContainingTenant {
		assert.Contains(t, key, tenantID, fmt.Sprintf("key %q must contain the tenant ID", key))
	}
}
