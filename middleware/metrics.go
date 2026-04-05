package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig holds configuration for metrics middleware
type MetricsConfig struct {
	// Namespace is the metric namespace (e.g., "tesseract")
	Namespace string

	// Subsystem is the metric subsystem (e.g., "products_service")
	Subsystem string

	// ExcludedPaths are paths that should not be measured
	ExcludedPaths []string

	// EnableLatencyHistogram enables detailed latency histograms
	EnableLatencyHistogram bool

	// EnableRequestSize enables request size metrics
	EnableRequestSize bool

	// EnableResponseSize enables response size metrics
	EnableResponseSize bool

	// CustomBuckets for latency histogram
	CustomBuckets []float64
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig(namespace, subsystem string) MetricsConfig {
	return MetricsConfig{
		Namespace: namespace,
		Subsystem: subsystem,
		ExcludedPaths: []string{
			"/health",
			"/ready",
			"/metrics",
		},
		EnableLatencyHistogram: true,
		EnableRequestSize:      true,
		EnableResponseSize:     true,
		// Default buckets optimized for API latencies (in seconds)
		CustomBuckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}
}

// Metrics holds all Prometheus metrics
type Metrics struct {
	config MetricsConfig

	// Request metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec

	// Active requests gauge
	activeRequests *prometheus.GaugeVec

	// Error metrics
	errorsTotal *prometheus.CounterVec

	// Business metrics
	tenantRequestsTotal *prometheus.CounterVec

	// Cache metrics
	cacheHitsTotal   *prometheus.CounterVec
	cacheMissesTotal *prometheus.CounterVec

	// Database metrics
	dbQueryDuration *prometheus.HistogramVec
	dbQueriesTotal  *prometheus.CounterVec
	dbErrorsTotal   *prometheus.CounterVec

	// Rate limiting metrics
	rateLimitHitsTotal *prometheus.CounterVec
}

// NewMetrics creates new Prometheus metrics with the given configuration
func NewMetrics(config MetricsConfig) *Metrics {
	m := &Metrics{
		config: config,
	}

	// Request metrics
	m.requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	m.requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   config.CustomBuckets,
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	if config.EnableRequestSize {
		m.requestSize = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "http_request_size_bytes",
				Help:      "HTTP request size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
			},
			[]string{"method", "path"},
		)
	}

	if config.EnableResponseSize {
		m.responseSize = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "http_response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path", "status"},
		)
	}

	m.activeRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_requests_active",
			Help:      "Number of active HTTP requests",
		},
		[]string{"method", "path"},
	)

	m.errorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_errors_total",
			Help:      "Total number of HTTP errors (4xx and 5xx)",
		},
		[]string{"method", "path", "status", "error_type"},
	)

	// Business metrics
	m.tenantRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "tenant_requests_total",
			Help:      "Total requests per tenant",
		},
		[]string{"tenant_id", "tenant_tier"},
	)

	// Cache metrics
	m.cacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "cache_hits_total",
			Help:      "Total cache hits",
		},
		[]string{"cache_layer", "cache_key_prefix"},
	)

	m.cacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "cache_misses_total",
			Help:      "Total cache misses",
		},
		[]string{"cache_layer", "cache_key_prefix"},
	)

	// Database metrics
	m.dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_query_duration_seconds",
			Help:      "Database query latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "table"},
	)

	m.dbQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_queries_total",
			Help:      "Total database queries",
		},
		[]string{"operation", "table", "status"},
	)

	m.dbErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_errors_total",
			Help:      "Total database errors",
		},
		[]string{"operation", "table", "error_type"},
	)

	// Rate limiting metrics
	m.rateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "rate_limit_hits_total",
			Help:      "Total rate limit hits",
		},
		[]string{"tenant_id", "limit_type"},
	)

	return m
}

// isExcluded checks if the path should be excluded from metrics
func (m *Metrics) isExcluded(path string) bool {
	for _, excluded := range m.config.ExcludedPaths {
		if path == excluded {
			return true
		}
	}
	return false
}

// normalizePath normalizes the path for metrics to avoid high cardinality
func normalizePath(path, fullPath string) string {
	if fullPath != "" {
		return fullPath
	}
	return path
}

// Middleware returns a Gin middleware for Prometheus metrics
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.isExcluded(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		method := c.Request.Method
		path := normalizePath(c.Request.URL.Path, c.FullPath())

		// Track active requests
		m.activeRequests.WithLabelValues(method, path).Inc()
		defer m.activeRequests.WithLabelValues(method, path).Dec()

		// Get tenant ID
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "unknown"
		}

		// Track request size
		if m.config.EnableRequestSize && m.requestSize != nil {
			m.requestSize.WithLabelValues(method, path).Observe(float64(c.Request.ContentLength))
		}

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		m.requestsTotal.WithLabelValues(method, path, status, tenantID).Inc()
		m.requestDuration.WithLabelValues(method, path, status, tenantID).Observe(duration)

		// Track response size
		if m.config.EnableResponseSize && m.responseSize != nil {
			m.responseSize.WithLabelValues(method, path, status).Observe(float64(c.Writer.Size()))
		}

		// Track errors
		if c.Writer.Status() >= 400 {
			errorType := "client_error"
			if c.Writer.Status() >= 500 {
				errorType = "server_error"
			}
			m.errorsTotal.WithLabelValues(method, path, status, errorType).Inc()
		}

		// Track tenant requests
		tenantTier := c.GetHeader("X-Tenant-Tier")
		if tenantTier == "" {
			tenantTier = "unknown"
		}
		m.tenantRequestsTotal.WithLabelValues(tenantID, tenantTier).Inc()
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(layer, keyPrefix string) {
	m.cacheHitsTotal.WithLabelValues(layer, keyPrefix).Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(layer, keyPrefix string) {
	m.cacheMissesTotal.WithLabelValues(layer, keyPrefix).Inc()
}

// RecordDBQuery records a database query with its duration
func (m *Metrics) RecordDBQuery(operation, table string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
		m.dbErrorsTotal.WithLabelValues(operation, table, "query_error").Inc()
	}
	m.dbQueriesTotal.WithLabelValues(operation, table, status).Inc()
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordRateLimitHit records a rate limit hit
func (m *Metrics) RecordRateLimitHit(tenantID, limitType string) {
	m.rateLimitHitsTotal.WithLabelValues(tenantID, limitType).Inc()
}

// Handler returns the Prometheus HTTP handler
func Handler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}

// MetricsMiddleware creates a metrics middleware with default configuration
func MetricsMiddleware(namespace, subsystem string) gin.HandlerFunc {
	metrics := NewMetrics(DefaultMetricsConfig(namespace, subsystem))
	return metrics.Middleware()
}

// Global metrics instance for use across the application
var globalMetrics *Metrics

// InitGlobalMetrics initializes global metrics
func InitGlobalMetrics(namespace, subsystem string) *Metrics {
	globalMetrics = NewMetrics(DefaultMetricsConfig(namespace, subsystem))
	return globalMetrics
}

// GetGlobalMetrics returns the global metrics instance
func GetGlobalMetrics() *Metrics {
	return globalMetrics
}

// RecordCacheHitGlobal records a cache hit using global metrics
func RecordCacheHitGlobal(layer, keyPrefix string) {
	if globalMetrics != nil {
		globalMetrics.RecordCacheHit(layer, keyPrefix)
	}
}

// RecordCacheMissGlobal records a cache miss using global metrics
func RecordCacheMissGlobal(layer, keyPrefix string) {
	if globalMetrics != nil {
		globalMetrics.RecordCacheMiss(layer, keyPrefix)
	}
}

// RecordDBQueryGlobal records a DB query using global metrics
func RecordDBQueryGlobal(operation, table string, duration time.Duration, err error) {
	if globalMetrics != nil {
		globalMetrics.RecordDBQuery(operation, table, duration, err)
	}
}

// RecordRateLimitHitGlobal records a rate limit hit using global metrics
func RecordRateLimitHitGlobal(tenantID, limitType string) {
	if globalMetrics != nil {
		globalMetrics.RecordRateLimitHit(tenantID, limitType)
	}
}

// Custom business metrics
var (
	// OrdersProcessed tracks number of orders processed
	OrdersProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "business",
			Name:      "orders_processed_total",
			Help:      "Total orders processed",
		},
		[]string{"tenant_id", "status"},
	)

	// OrderValue tracks order value
	OrderValue = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tesseract",
			Subsystem: "business",
			Name:      "order_value_usd",
			Help:      "Order value in USD",
			Buckets:   []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		},
		[]string{"tenant_id"},
	)

	// ProductsCreated tracks products created
	ProductsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "business",
			Name:      "products_created_total",
			Help:      "Total products created",
		},
		[]string{"tenant_id", "category"},
	)

	// CustomersRegistered tracks customer registrations
	CustomersRegistered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "business",
			Name:      "customers_registered_total",
			Help:      "Total customers registered",
		},
		[]string{"tenant_id", "source"},
	)

	// InventoryUpdates tracks inventory updates
	InventoryUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "business",
			Name:      "inventory_updates_total",
			Help:      "Total inventory updates",
		},
		[]string{"tenant_id", "operation"},
	)
)

// RecordOrderProcessed records an order processed
func RecordOrderProcessed(tenantID, status string, value float64) {
	OrdersProcessed.WithLabelValues(tenantID, status).Inc()
	OrderValue.WithLabelValues(tenantID).Observe(value)
}

// RecordProductCreated records a product created
func RecordProductCreated(tenantID, category string) {
	ProductsCreated.WithLabelValues(tenantID, category).Inc()
}

// RecordCustomerRegistered records a customer registration
func RecordCustomerRegistered(tenantID, source string) {
	CustomersRegistered.WithLabelValues(tenantID, source).Inc()
}

// RecordInventoryUpdate records an inventory update
func RecordInventoryUpdate(tenantID, operation string) {
	InventoryUpdates.WithLabelValues(tenantID, operation).Inc()
}
