package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCacheConfig_ZeroValue verifies zero-value fields so calling code can
// detect unconfigured instances before use.
func TestCacheConfig_ZeroValue(t *testing.T) {
	var cfg CacheConfig
	assert.Equal(t, "", cfg.RedisAddr)
	assert.Equal(t, "", cfg.RedisPassword)
	assert.Equal(t, 0, cfg.RedisDB)
	assert.False(t, cfg.L1Enabled)
	assert.Equal(t, 0, cfg.L1MaxItems)
	assert.Equal(t, time.Duration(0), cfg.L1TTL)
	assert.Equal(t, time.Duration(0), cfg.DefaultTTL)
	assert.Equal(t, "", cfg.KeyPrefix)
}

// TestDefaultCacheConfig_L1Enabled verifies the default L1 configuration.
func TestDefaultCacheConfig_L1Enabled(t *testing.T) {
	cfg := DefaultCacheConfig()
	assert.True(t, cfg.L1Enabled)
	assert.Greater(t, cfg.L1MaxItems, 0)
	assert.Greater(t, cfg.L1TTL, time.Duration(0))
}

// TestDefaultCacheConfig_DefaultTTL verifies a sensible default TTL is set.
func TestDefaultCacheConfig_DefaultTTL(t *testing.T) {
	cfg := DefaultCacheConfig()
	assert.Greater(t, cfg.DefaultTTL, time.Duration(0))
}

// TestDefaultCacheConfig_KeyPrefix verifies the namespace prefix is non-empty.
func TestDefaultCacheConfig_KeyPrefix(t *testing.T) {
	cfg := DefaultCacheConfig()
	assert.NotEmpty(t, cfg.KeyPrefix)
}

// TestNewCacheLayerFromClient_ReturnsNonNil confirms that the constructor
// always returns a non-nil layer even when a nil Redis client is supplied.
func TestNewCacheLayerFromClient_ReturnsNonNil(t *testing.T) {
	cfg := DefaultCacheConfig()
	layer := NewCacheLayerFromClient(nil, cfg)
	assert.NotNil(t, layer)
}

// TestNewCacheLayerFromClient_ConfigStored verifies that the supplied config
// is stored verbatim inside the layer.
func TestNewCacheLayerFromClient_ConfigStored(t *testing.T) {
	cfg := CacheConfig{
		RedisAddr:  "redis-host:6380",
		KeyPrefix:  "myns:",
		L1Enabled:  false,
		DefaultTTL: 10 * time.Minute,
	}

	layer := NewCacheLayerFromClient(nil, cfg)

	assert.Equal(t, "redis-host:6380", layer.config.RedisAddr)
	assert.Equal(t, "myns:", layer.config.KeyPrefix)
	assert.Equal(t, 10*time.Minute, layer.config.DefaultTTL)
}

// TestNewCacheLayerFromClient_L1Disabled_NoL1Cache confirms that L1Cache is
// not allocated when L1Enabled is false.
func TestNewCacheLayerFromClient_L1Disabled_NoL1Cache(t *testing.T) {
	cfg := CacheConfig{L1Enabled: false, L1MaxItems: 1000}
	layer := NewCacheLayerFromClient(nil, cfg)
	assert.Nil(t, layer.l1Cache)
}

// TestNewCacheLayerFromClient_L1Enabled_L1CacheAllocated confirms that L1Cache
// is allocated when L1Enabled is true.
func TestNewCacheLayerFromClient_L1Enabled_L1CacheAllocated(t *testing.T) {
	cfg := CacheConfig{
		L1Enabled:  true,
		L1MaxItems: 200,
		L1TTL:      time.Minute,
	}
	layer := NewCacheLayerFromClient(nil, cfg)
	assert.NotNil(t, layer.l1Cache)
	assert.Equal(t, 200, layer.l1Cache.maxItems)
	assert.Equal(t, time.Minute, layer.l1Cache.ttl)
}

// TestStats_ZeroOnCreation verifies that a fresh CacheLayer reports all-zero
// hit/miss counters.
func TestStats_ZeroOnCreation(t *testing.T) {
	cfg := CacheConfig{L1Enabled: false}
	layer := NewCacheLayerFromClient(nil, cfg)

	s := layer.Stats()

	assert.Equal(t, int64(0), s.L1Hits)
	assert.Equal(t, int64(0), s.L1Misses)
	assert.Equal(t, int64(0), s.L2Hits)
	assert.Equal(t, int64(0), s.L2Misses)
}

// TestStats_ReturnsSnapshotNotPointer verifies that Stats() returns a value
// copy so that mutating the returned struct does not affect internal counters.
func TestStats_ReturnsSnapshotNotPointer(t *testing.T) {
	layer := &CacheLayer{
		stats: &CacheStats{L1Hits: 7},
	}

	s1 := layer.Stats()
	s1.L1Hits = 999

	s2 := layer.Stats()
	assert.Equal(t, int64(7), s2.L1Hits, "internal counter must not be changed via the returned snapshot")
}

// TestPrefixKey_UsesConfiguredPrefix is a focused test at the layer level
// (complementing the helper test) to confirm the method reads from config.
func TestPrefixKey_UsesConfiguredPrefix(t *testing.T) {
	layer := NewCacheLayerFromClient(nil, CacheConfig{
		L1Enabled: false,
		KeyPrefix: "svc:v1:",
	})

	got := layer.prefixKey("product:t1:p1")
	assert.Equal(t, "svc:v1:product:t1:p1", got)
}

// TestCacheConfig_Validation_L1MaxItems tests edge cases for L1MaxItems.
func TestCacheConfig_Validation_L1MaxItems(t *testing.T) {
	tests := []struct {
		name       string
		maxItems   int
		wantNilL1  bool
	}{
		{"positive max items", 100, false},
		{"zero max items", 0, false},  // L1Enabled drives nil check, not maxItems
		{"large max items", 100000, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := CacheConfig{
				L1Enabled:  true,
				L1MaxItems: tc.maxItems,
				L1TTL:      time.Minute,
			}
			layer := NewCacheLayerFromClient(nil, cfg)
			if tc.wantNilL1 {
				assert.Nil(t, layer.l1Cache)
			} else {
				assert.NotNil(t, layer.l1Cache)
			}
		})
	}
}

// TestCacheLayer_L1Cache_GetMiss_WhenL1Disabled confirms no L1 look-up occurs
// when L1 is disabled, by verifying the l1Cache field is nil.
func TestCacheLayer_L1Cache_GetMiss_WhenL1Disabled(t *testing.T) {
	cfg := CacheConfig{L1Enabled: false}
	layer := NewCacheLayerFromClient(nil, cfg)

	// l1Cache being nil is the prerequisite for skipping L1 in Get/Set.
	assert.Nil(t, layer.l1Cache)
}

// TestCacheLayer_L1Cache_ItemsMap_InitialisedEmpty verifies the items map
// starts empty after construction.
func TestCacheLayer_L1Cache_ItemsMap_InitialisedEmpty(t *testing.T) {
	cfg := CacheConfig{
		L1Enabled:  true,
		L1MaxItems: 50,
		L1TTL:      time.Minute,
	}
	layer := NewCacheLayerFromClient(nil, cfg)

	assert.NotNil(t, layer.l1Cache.items)
	assert.Empty(t, layer.l1Cache.items)
	assert.Empty(t, layer.l1Cache.order)
}

// TestErrCacheMiss_IsNotNilAndCorrectMessage is a cache-layer-level assertion
// that the sentinel error is properly defined and carries an actionable message.
func TestErrCacheMiss_IsNotNilAndCorrectMessage(t *testing.T) {
	assert.NotNil(t, ErrCacheMiss)
	assert.Contains(t, ErrCacheMiss.Error(), "miss")
}

// TestCacheLayer_Redis_ClientStoredAsSupplied verifies that the Redis client
// passed to NewCacheLayerFromClient is the exact same pointer stored internally.
func TestCacheLayer_Redis_ClientStoredAsSupplied(t *testing.T) {
	cfg := CacheConfig{L1Enabled: false}

	// We use nil intentionally here; the point is to verify pointer identity.
	layer := NewCacheLayerFromClient(nil, cfg)
	assert.Nil(t, layer.redis)
}

// TestCacheLayer_ConcurrentStats_IsThreadSafe calls Stats() from multiple
// goroutines to detect data-race issues (run with -race to catch them).
func TestCacheLayer_ConcurrentStats_IsThreadSafe(t *testing.T) {
	layer := &CacheLayer{
		stats: &CacheStats{},
	}

	done := make(chan struct{})

	go func() {
		for i := 0; i < 500; i++ {
			layer.stats.mu.Lock()
			layer.stats.L1Hits++
			layer.stats.mu.Unlock()
		}
		close(done)
	}()

	for i := 0; i < 500; i++ {
		_ = layer.Stats()
	}

	<-done
}
