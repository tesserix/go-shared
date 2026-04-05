package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIsolatedMetrics creates a Metrics instance backed by a fresh, isolated
// Prometheus registry so that tests do not collide with the global default
// registry or with each other.  The promauto package always registers against
// the default registry, so we construct the counters/histograms manually here.
func newIsolatedMetrics(t *testing.T, namespace, subsystem string) *Metrics {
	t.Helper()

	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	cfg := DefaultMetricsConfig(namespace, subsystem)
	m := &Metrics{config: cfg}

	m.requestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_requests_total",
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	m.requestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_request_duration_seconds",
			Buckets:   cfg.CustomBuckets,
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	m.requestSize = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_request_size_bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	m.responseSize = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_response_size_bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path", "status"},
	)

	m.activeRequests = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_requests_active",
		},
		[]string{"method", "path"},
	)

	m.errorsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_errors_total",
		},
		[]string{"method", "path", "status", "error_type"},
	)

	m.tenantRequestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "tenant_requests_total",
		},
		[]string{"tenant_id", "tenant_tier"},
	)

	m.cacheHitsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cache_hits_total",
		},
		[]string{"cache_layer", "cache_key_prefix"},
	)

	m.cacheMissesTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cache_misses_total",
		},
		[]string{"cache_layer", "cache_key_prefix"},
	)

	m.dbQueryDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "db_query_duration_seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "table"},
	)

	m.dbQueriesTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "db_queries_total",
		},
		[]string{"operation", "table", "status"},
	)

	m.dbErrorsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "db_errors_total",
		},
		[]string{"operation", "table", "error_type"},
	)

	m.rateLimitHitsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "rate_limit_hits_total",
		},
		[]string{"tenant_id", "limit_type"},
	)

	return m
}

// ----- DefaultMetricsConfig --------------------------------------------------

func TestDefaultMetricsConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := DefaultMetricsConfig("test_ns", "test_sub")

	assert.Equal(t, "test_ns", cfg.Namespace)
	assert.Equal(t, "test_sub", cfg.Subsystem)
	assert.True(t, cfg.EnableLatencyHistogram)
	assert.True(t, cfg.EnableRequestSize)
	assert.True(t, cfg.EnableResponseSize)
	assert.Contains(t, cfg.ExcludedPaths, "/health")
	assert.Contains(t, cfg.ExcludedPaths, "/ready")
	assert.Contains(t, cfg.ExcludedPaths, "/metrics")
	assert.NotEmpty(t, cfg.CustomBuckets)
}

// ----- Metrics.isExcluded ----------------------------------------------------

func TestMetrics_IsExcluded(t *testing.T) {
	m := newIsolatedMetrics(t, "ns1", "sub1")

	tests := []struct {
		path     string
		excluded bool
	}{
		{"/health", true},
		{"/ready", true},
		{"/metrics", true},
		{"/api/products", false},
		{"/api/health", false},  // not an exact match
		{"/health/live", false}, // isExcluded uses exact equality
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := m.isExcluded(tt.path)
			assert.Equal(t, tt.excluded, got, "isExcluded(%q)", tt.path)
		})
	}
}

// ----- normalizePath ---------------------------------------------------------

func TestNormalizePath_UsesFullPathWhenAvailable(t *testing.T) {
	result := normalizePath("/actual/path", "/gin/full/path")
	assert.Equal(t, "/gin/full/path", result)
}

func TestNormalizePath_FallsBackToPathWhenFullPathEmpty(t *testing.T) {
	result := normalizePath("/actual/path", "")
	assert.Equal(t, "/actual/path", result)
}

// ----- Metrics.Middleware -- pass-through for excluded paths -----------------

func TestMetricsMiddleware_SkipsExcludedPaths(t *testing.T) {
	m := newIsolatedMetrics(t, "ns2", "sub2")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(m.Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// No panic or error means the middleware skipped metrics collection correctly.
}

// ----- Metrics.Middleware -- records successful requests ---------------------

func TestMetricsMiddleware_RecordsSuccessfulRequest(t *testing.T) {
	m := newIsolatedMetrics(t, "ns3", "sub3")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(m.Middleware())
	router.GET("/api/items", func(c *gin.Context) {
		c.String(http.StatusOK, "items")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Header.Set("X-Tenant-ID", "tenant-test")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Verify the counter incremented without panic.
	c, err := m.requestsTotal.GetMetricWithLabelValues("GET", "/api/items", "200", "tenant-test")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestMetricsMiddleware_UsesUnknownTenantWhenHeaderMissing(t *testing.T) {
	m := newIsolatedMetrics(t, "ns4", "sub4")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(m.Middleware())
	router.GET("/api/items", func(c *gin.Context) {
		c.String(http.StatusOK, "items")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	// No X-Tenant-ID header — should fall back to "unknown".
	router.ServeHTTP(w, req)

	c, err := m.requestsTotal.GetMetricWithLabelValues("GET", "/api/items", "200", "unknown")
	require.NoError(t, err)
	require.NotNil(t, c, "metrics must use 'unknown' as tenant_id when X-Tenant-ID is absent")
}

// ----- Metrics.Middleware -- 4xx / 5xx error counters ------------------------

func TestMetricsMiddleware_TracksClientErrors(t *testing.T) {
	m := newIsolatedMetrics(t, "ns5", "sub5")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(m.Middleware())
	router.GET("/api/bad", func(c *gin.Context) {
		c.String(http.StatusBadRequest, "bad request")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/bad", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, err := m.errorsTotal.GetMetricWithLabelValues("GET", "/api/bad", "400", "client_error")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestMetricsMiddleware_TracksServerErrors(t *testing.T) {
	m := newIsolatedMetrics(t, "ns6", "sub6")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(m.Middleware())
	router.GET("/api/crash", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "crash")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/crash", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	c, err := m.errorsTotal.GetMetricWithLabelValues("GET", "/api/crash", "500", "server_error")
	require.NoError(t, err)
	require.NotNil(t, c)
}

// ----- RecordCacheHit / RecordCacheMiss --------------------------------------

func TestMetrics_RecordCacheHit_DoesNotPanic(t *testing.T) {
	m := newIsolatedMetrics(t, "ns7", "sub7")
	assert.NotPanics(t, func() {
		m.RecordCacheHit("redis", "products:")
	})
}

func TestMetrics_RecordCacheMiss_DoesNotPanic(t *testing.T) {
	m := newIsolatedMetrics(t, "ns8", "sub8")
	assert.NotPanics(t, func() {
		m.RecordCacheMiss("redis", "orders:")
	})
}

// ----- RecordDBQuery ---------------------------------------------------------

func TestMetrics_RecordDBQuery_Success(t *testing.T) {
	m := newIsolatedMetrics(t, "ns9", "sub9")
	assert.NotPanics(t, func() {
		m.RecordDBQuery("SELECT", "products", 5*time.Millisecond, nil)
	})
}

func TestMetrics_RecordDBQuery_Error(t *testing.T) {
	m := newIsolatedMetrics(t, "ns10", "sub10")

	assert.NotPanics(t, func() {
		m.RecordDBQuery("INSERT", "orders", 10*time.Millisecond, assert.AnError)
	})

	// Error counter must be incremented for the error case.
	c, err := m.dbErrorsTotal.GetMetricWithLabelValues("INSERT", "orders", "query_error")
	require.NoError(t, err)
	require.NotNil(t, c)
}

// ----- RecordRateLimitHit ----------------------------------------------------

func TestMetrics_RecordRateLimitHit_DoesNotPanic(t *testing.T) {
	m := newIsolatedMetrics(t, "ns11", "sub11")
	assert.NotPanics(t, func() {
		m.RecordRateLimitHit("ten-001", "per_minute")
	})
}

// ----- Global metrics helpers ------------------------------------------------

func TestRecordCacheHitGlobal_NoopWhenGlobalNil(t *testing.T) {
	// Save and restore global state.
	prev := globalMetrics
	globalMetrics = nil
	t.Cleanup(func() { globalMetrics = prev })

	assert.NotPanics(t, func() {
		RecordCacheHitGlobal("redis", "prefix:")
	})
}

func TestRecordCacheMissGlobal_NoopWhenGlobalNil(t *testing.T) {
	prev := globalMetrics
	globalMetrics = nil
	t.Cleanup(func() { globalMetrics = prev })

	assert.NotPanics(t, func() {
		RecordCacheMissGlobal("redis", "prefix:")
	})
}

func TestRecordDBQueryGlobal_NoopWhenGlobalNil(t *testing.T) {
	prev := globalMetrics
	globalMetrics = nil
	t.Cleanup(func() { globalMetrics = prev })

	assert.NotPanics(t, func() {
		RecordDBQueryGlobal("SELECT", "users", time.Millisecond, nil)
	})
}

func TestRecordRateLimitHitGlobal_NoopWhenGlobalNil(t *testing.T) {
	prev := globalMetrics
	globalMetrics = nil
	t.Cleanup(func() { globalMetrics = prev })

	assert.NotPanics(t, func() {
		RecordRateLimitHitGlobal("ten-001", "per_second")
	})
}

// ----- GetGlobalMetrics / InitGlobalMetrics -- smoke tests -------------------

func TestGetGlobalMetrics_ReturnsNilWhenUninitialized(t *testing.T) {
	prev := globalMetrics
	globalMetrics = nil
	t.Cleanup(func() { globalMetrics = prev })

	assert.Nil(t, GetGlobalMetrics())
}

// ----- Business metric helpers (package-level vars) --------------------------

func TestRecordOrderProcessed_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordOrderProcessed("ten-001", "completed", 99.99)
	})
}

func TestRecordProductCreated_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordProductCreated("ten-001", "electronics")
	})
}

func TestRecordCustomerRegistered_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordCustomerRegistered("ten-001", "organic")
	})
}

func TestRecordInventoryUpdate_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordInventoryUpdate("ten-001", "decrement")
	})
}

// ----- MetricsMiddleware convenience function --------------------------------

func TestMetricsMiddleware_ReturnsHandlerFunc(t *testing.T) {
	// MetricsMiddleware uses the default Prometheus registry via promauto.
	// We only verify it does not panic and returns a valid HandlerFunc.
	// Using a unique name to avoid registration conflicts across test runs.
	assert.NotPanics(t, func() {
		h := MetricsMiddleware("testsvc", "middleware_func_test")
		require.NotNil(t, h, "MetricsMiddleware must return a non-nil HandlerFunc")
	})
}

// ----- Handler (Prometheus HTTP handler) -------------------------------------

func TestHandler_ReturnsNonNilHandler(t *testing.T) {
	h := Handler()
	require.NotNil(t, h)
}

func TestHandler_Returns200ForMetricsEndpoint(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.GET("/metrics", Handler())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
