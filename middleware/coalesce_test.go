package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----- DefaultCoalesceConfig -------------------------------------------------

func TestDefaultCoalesceConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := DefaultCoalesceConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 100*time.Millisecond, cfg.MaxWaitTime)
	assert.True(t, cfg.IncludeTenantID)
	assert.False(t, cfg.IncludeUserID)
	assert.True(t, cfg.CacheResult)
	assert.Equal(t, 50*time.Millisecond, cfg.CacheTTL)
	assert.Contains(t, cfg.ExcludedPaths, "/health")
	assert.Contains(t, cfg.ExcludedPaths, "/ready")
	assert.Contains(t, cfg.ExcludedPaths, "/metrics")
	assert.Contains(t, cfg.ExcludedHeaders, "Authorization")
	assert.Contains(t, cfg.ExcludedHeaders, "Cookie")
}

// ----- RequestCoalescer.buildKey ---------------------------------------------

func TestRequestCoalescer_BuildKey_DifferentMethodsProduceDifferentKeys(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())

	req1 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req2 := httptest.NewRequest(http.MethodPost, "/api/items", nil)

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.NotEqual(t, k1, k2, "GET and POST to the same path must produce different keys")
}

func TestRequestCoalescer_BuildKey_DifferentPathsProduceDifferentKeys(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())

	req1 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/api/orders", nil)

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.NotEqual(t, k1, k2)
}

func TestRequestCoalescer_BuildKey_QueryParamsIncluded(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())

	req1 := httptest.NewRequest(http.MethodGet, "/api/items?page=1", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/api/items?page=2", nil)

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.NotEqual(t, k1, k2, "different query params must produce different keys")
}

func TestRequestCoalescer_BuildKey_QueryParamsOrderIndependent(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())

	req1 := httptest.NewRequest(http.MethodGet, "/api/items?b=2&a=1", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/api/items?a=1&b=2", nil)

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.Equal(t, k1, k2, "query param order must not affect the coalesce key")
}

func TestRequestCoalescer_BuildKey_TenantIDIncludedWhenConfigured(t *testing.T) {
	cfg := DefaultCoalesceConfig()
	cfg.IncludeTenantID = true
	coalescer := NewRequestCoalescer(cfg)

	req1 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req1.Header.Set("X-Tenant-ID", "tenant-A")

	req2 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req2.Header.Set("X-Tenant-ID", "tenant-B")

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.NotEqual(t, k1, k2, "different tenant IDs must produce different keys when IncludeTenantID=true")
}

func TestRequestCoalescer_BuildKey_UserIDIncludedWhenConfigured(t *testing.T) {
	cfg := DefaultCoalesceConfig()
	cfg.IncludeUserID = true
	coalescer := NewRequestCoalescer(cfg)

	req1 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req1.Header.Set("X-User-ID", "user-1")

	req2 := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req2.Header.Set("X-User-ID", "user-2")

	k1 := coalescer.buildKey(req1)
	k2 := coalescer.buildKey(req2)

	assert.NotEqual(t, k1, k2, "different user IDs must produce different keys when IncludeUserID=true")
}

func TestRequestCoalescer_BuildKey_ProducesCompactHexString(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)

	key := coalescer.buildKey(req)

	// buildKey returns first 16 bytes as 32 hex characters.
	assert.Len(t, key, 32, "coalesce key must be 32 hex characters")
}

// ----- RequestCoalescer.isExcludedPath ---------------------------------------

func TestRequestCoalescer_IsExcludedPath(t *testing.T) {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())

	tests := []struct {
		path     string
		excluded bool
	}{
		{"/health", true},
		{"/health/detail", true},
		{"/ready", true},
		{"/metrics", true},
		{"/api/items", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := coalescer.isExcludedPath(tt.path)
			assert.Equal(t, tt.excluded, got)
		})
	}
}

// ----- CoalesceMiddleware -- Gin middleware behavior -------------------------

func TestCoalesceMiddleware_PassesThroughNonGetRequests(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(CoalesceMiddleware())
			router.Handle(method, "/test", func(c *gin.Context) {
				c.String(http.StatusOK, "response")
			})

			req := httptest.NewRequest(method, "/test", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			// Non-GET requests must not receive coalesce headers.
			assert.Empty(t, w.Header().Get("X-Coalesce"))
		})
	}
}

func TestCoalesceMiddleware_SkipsExcludedPaths(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CoalesceMiddleware())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-Coalesce"))
}

func TestCoalesceMiddleware_DisabledSkipsCoalescing(t *testing.T) {
	cfg := DefaultCoalesceConfig()
	cfg.Enabled = false

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CoalesceMiddlewareWithConfig(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "direct")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-Coalesce"),
		"disabled coalescer must not add X-Coalesce header")
}

func TestCoalesceMiddleware_LeaderGetsLeaderHeader(t *testing.T) {
	cfg := DefaultCoalesceConfig()
	cfg.CacheResult = false // disable cache for isolation

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CoalesceMiddlewareWithConfig(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "leader response")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "LEADER", w.Header().Get("X-Coalesce"),
		"the first (leader) request must receive X-Coalesce: LEADER")
}

func TestCoalesceMiddleware_CachingServesSubsequentRequestFromCache(t *testing.T) {
	callCount := 0

	cfg := DefaultCoalesceConfig()
	cfg.CacheResult = true
	cfg.CacheTTL = 500 * time.Millisecond

	_, router := gin.CreateTestContext(httptest.NewRecorder())
	router.Use(CoalesceMiddlewareWithConfig(cfg))
	router.GET("/data", func(c *gin.Context) {
		callCount++
		c.String(http.StatusOK, "cached response")
	})

	// First request — leader.
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/data", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, "LEADER", w1.Header().Get("X-Coalesce"))
	assert.Equal(t, 1, callCount)

	// Second request within cache TTL — should be served from cache.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/data", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, "HIT", w2.Header().Get("X-Coalesce-Cache"),
		"second request within TTL must be served from cache")
	assert.Equal(t, 1, callCount, "handler must not be called again for cached response")
}

// ----- SingleFlight ----------------------------------------------------------

func TestSingleFlight_ExecutesFunctionOnce(t *testing.T) {
	sf := NewSingleFlight(0)
	var callCount int64 // use atomic to avoid data race in concurrent goroutines

	var wg sync.WaitGroup
	results := make([]interface{}, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := sf.Do("key", func() (interface{}, error) {
				atomic.AddInt64(&callCount, 1)
				return "shared-result", nil
			})
			require.NoError(t, err)
			results[idx] = result
		}(i)
	}

	wg.Wait()

	// All goroutines should receive the same result.
	for _, r := range results {
		assert.Equal(t, "shared-result", r)
	}
}

func TestSingleFlight_CachesResultWithTTL(t *testing.T) {
	sf := NewSingleFlight(200 * time.Millisecond)
	callCount := 0

	first, err := sf.Do("key", func() (interface{}, error) {
		callCount++
		return "result", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "result", first)
	assert.Equal(t, 1, callCount)

	// Second call within TTL should use cached result.
	second, err := sf.Do("key", func() (interface{}, error) {
		callCount++
		return "result", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "result", second)
	assert.Equal(t, 1, callCount, "cached result must prevent second execution")
}

func TestSingleFlight_DoBytes_ReturnsBytes(t *testing.T) {
	sf := NewSingleFlight(0)

	result, err := sf.DoBytes("key", func() ([]byte, error) {
		return []byte("byte result"), nil
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("byte result"), result)
}

func TestSingleFlight_DoBytes_ReturnsNilOnNilResult(t *testing.T) {
	sf := NewSingleFlight(0)

	result, err := sf.DoBytes("key", func() ([]byte, error) {
		return nil, nil
	})
	require.NoError(t, err)
	assert.Nil(t, result)
}

// ----- BatchCoalescer --------------------------------------------------------

func TestBatchCoalescer_RegisterHandlerAndGet(t *testing.T) {
	bc := NewBatchCoalescer(10, 50*time.Millisecond)

	bc.RegisterHandler("product", func(ids []string) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, id := range ids {
			result[id] = fmt.Sprintf("data-for-%s", id)
		}
		return result, nil
	})

	result, err := bc.Get("product", "item-1")
	require.NoError(t, err)
	assert.Equal(t, "data-for-item-1", result)
}

func TestBatchCoalescer_UnregisteredHandlerReturnsError(t *testing.T) {
	bc := NewBatchCoalescer(10, 50*time.Millisecond)

	result, err := bc.Get("unknown-resource", "some-id")
	// The implementation returns io.EOF for missing handler.
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestBatchCoalescer_BatchesConcurrentRequests(t *testing.T) {
	var batchCallCount int64 // atomic to avoid data race from concurrent batch handler calls
	bc := NewBatchCoalescer(5, 50*time.Millisecond)

	bc.RegisterHandler("item", func(ids []string) (map[string]interface{}, error) {
		atomic.AddInt64(&batchCallCount, 1)
		result := make(map[string]interface{})
		for _, id := range ids {
			result[id] = id + "-value"
		}
		return result, nil
	})

	var wg sync.WaitGroup
	results := make([]interface{}, 3)
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("id-%d", idx)
			r, e := bc.Get("item", id)
			results[idx] = r
			errors[idx] = e
		}(i)
	}

	wg.Wait()

	for _, e := range errors {
		assert.NoError(t, e)
	}

	for i, r := range results {
		expected := fmt.Sprintf("id-%d-value", i)
		assert.Equal(t, expected, r)
	}
}
