package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newTestL1Cache is a helper that creates an L1Cache with a configurable TTL
// and max-items for use in tests.
func newTestL1Cache(maxItems int, ttl time.Duration) *L1Cache {
	return &L1Cache{
		items:    make(map[string]*l1CacheItem),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
		ttl:      ttl,
	}
}

// --- Get ---

func TestL1Cache_Get_ExistingKey_ReturnsValue(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("key1", []byte("hello"))

	value, found := cache.Get("key1")

	assert.True(t, found)
	assert.Equal(t, []byte("hello"), value)
}

func TestL1Cache_Get_MissingKey_ReturnsFalse(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)

	value, found := cache.Get("nonexistent")

	assert.False(t, found)
	assert.Nil(t, value)
}

func TestL1Cache_Get_ExpiredKey_ReturnsFalse(t *testing.T) {
	// Use a TTL in the past so the item is immediately expired.
	cache := newTestL1Cache(10, time.Nanosecond)
	cache.Set("expiring", []byte("data"))

	// Sleep just long enough for the TTL to elapse.
	time.Sleep(5 * time.Millisecond)

	value, found := cache.Get("expiring")

	assert.False(t, found)
	assert.Nil(t, value)
}

func TestL1Cache_Get_NotExpiredKey_ReturnsValue(t *testing.T) {
	cache := newTestL1Cache(10, time.Hour)
	cache.Set("fresh", []byte("still valid"))

	value, found := cache.Get("fresh")

	assert.True(t, found)
	assert.Equal(t, []byte("still valid"), value)
}

// --- Set ---

func TestL1Cache_Set_StoresValue(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("k", []byte("v"))

	value, found := cache.Get("k")
	assert.True(t, found)
	assert.Equal(t, []byte("v"), value)
}

func TestL1Cache_Set_OverwritesExistingValue(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("k", []byte("original"))
	cache.Set("k", []byte("updated"))

	value, found := cache.Get("k")
	assert.True(t, found)
	assert.Equal(t, []byte("updated"), value)
}

func TestL1Cache_Set_UpdateExistingKey_DoesNotDuplicateOrderEntry(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("k", []byte("first"))
	cache.Set("k", []byte("second"))

	// The order slice must contain exactly one entry for "k".
	count := 0
	for _, key := range cache.order {
		if key == "k" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate keys must not appear in the order slice")
}

func TestL1Cache_Set_EvictsOldestWhenAtCapacity(t *testing.T) {
	maxItems := 3
	cache := newTestL1Cache(maxItems, time.Minute)

	cache.Set("a", []byte("1"))
	cache.Set("b", []byte("2"))
	cache.Set("c", []byte("3"))

	// Adding a fourth item must evict "a" (the oldest).
	cache.Set("d", []byte("4"))

	_, foundA := cache.Get("a")
	assert.False(t, foundA, "oldest item 'a' should have been evicted")

	_, foundB := cache.Get("b")
	assert.True(t, foundB)

	_, foundD := cache.Get("d")
	assert.True(t, foundD)
}

func TestL1Cache_Set_CapacityMaintained(t *testing.T) {
	maxItems := 3
	cache := newTestL1Cache(maxItems, time.Minute)

	for i := 0; i < 10; i++ {
		cache.Set(string(rune('a'+i)), []byte{byte(i)})
	}

	// After 10 inserts the in-memory item count must not exceed maxItems.
	assert.LessOrEqual(t, len(cache.items), maxItems)
	assert.LessOrEqual(t, len(cache.order), maxItems)
}

func TestL1Cache_Set_OrderSliceGrows(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)

	cache.Set("x", []byte("1"))
	assert.Len(t, cache.order, 1)

	cache.Set("y", []byte("2"))
	assert.Len(t, cache.order, 2)
}

// --- Delete ---

func TestL1Cache_Delete_RemovesKey(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("del", []byte("value"))

	cache.Delete("del")

	_, found := cache.Get("del")
	assert.False(t, found)
}

func TestL1Cache_Delete_RemovesFromOrderSlice(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("alpha", []byte("1"))
	cache.Set("beta", []byte("2"))
	cache.Set("gamma", []byte("3"))

	cache.Delete("beta")

	for _, key := range cache.order {
		assert.NotEqual(t, "beta", key, "deleted key must not remain in order slice")
	}
	assert.Len(t, cache.order, 2)
}

func TestL1Cache_Delete_NonExistentKey_IsNoOp(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)
	cache.Set("existing", []byte("v"))

	// Deleting a key that was never set should not panic or alter existing keys.
	assert.NotPanics(t, func() {
		cache.Delete("nonexistent")
	})

	_, found := cache.Get("existing")
	assert.True(t, found, "unrelated key should remain after deleting a nonexistent key")
}

func TestL1Cache_Delete_EmptyCache_IsNoOp(t *testing.T) {
	cache := newTestL1Cache(10, time.Minute)

	assert.NotPanics(t, func() {
		cache.Delete("any")
	})
}

// --- Concurrency ---

func TestL1Cache_ConcurrentSetAndGet(t *testing.T) {
	cache := newTestL1Cache(100, time.Minute)

	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < 500; i++ {
			cache.Set("shared-key", []byte("value"))
		}
		close(done)
	}()

	// Reader goroutine (runs concurrently with writer)
	for i := 0; i < 500; i++ {
		cache.Get("shared-key")
	}

	<-done
}

func TestL1Cache_ConcurrentSetAndDelete(t *testing.T) {
	cache := newTestL1Cache(100, time.Minute)
	cache.Set("to-delete", []byte("v"))

	done := make(chan struct{})

	go func() {
		for i := 0; i < 200; i++ {
			cache.Set("to-delete", []byte("updated"))
		}
		close(done)
	}()

	for i := 0; i < 200; i++ {
		cache.Delete("to-delete")
	}

	<-done
}
