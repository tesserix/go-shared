package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds HTTP client metrics
type Metrics struct {
	serviceName          string
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestSize      *prometheus.HistogramVec
	httpResponseSize     *prometheus.HistogramVec

	// Custom metrics (for backward compatibility)
	customCounters   map[string]*prometheus.CounterVec
	customGauges     map[string]prometheus.Gauge
	customHistograms map[string]*prometheus.HistogramVec
}

// Config holds metrics configuration
type Config struct {
	ServiceName string // Name of the service (optional, used for identification)
	Namespace   string
	Subsystem   string
}

// DefaultConfig returns default metrics configuration
func DefaultConfig() Config {
	return Config{
		Namespace: "tesseract",
		Subsystem: "http_client",
	}
}

// New creates a new Metrics instance
func New(config Config) *Metrics {
	return &Metrics{
		serviceName:      config.ServiceName,
		customCounters:   make(map[string]*prometheus.CounterVec),
		customGauges:     make(map[string]prometheus.Gauge),
		customHistograms: make(map[string]*prometheus.HistogramVec),
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "requests_total",
				Help:      "Total number of HTTP client requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_duration_seconds",
				Help:      "HTTP client request duration in seconds",
				Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path", "status"},
		),
		httpRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_size_bytes",
				Help:      "HTTP client request size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		),
		httpResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "response_size_bytes",
				Help:      "HTTP client response size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path", "status"},
		),
	}
}

// RecordHTTPRequest records HTTP request metrics
func (m *Metrics) RecordHTTPRequest(method, path, status string, duration time.Duration, requestSize, responseSize int64) {
	m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
	m.httpRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	m.httpResponseSize.WithLabelValues(method, path, status).Observe(float64(responseSize))
}

// Global metrics instance
var globalMetrics *Metrics

// InitGlobal initializes global metrics
func InitGlobal(config Config) *Metrics {
	globalMetrics = New(config)
	return globalMetrics
}

// GetGlobal returns the global metrics instance
func GetGlobal() *Metrics {
	return globalMetrics
}

// RecordHTTPRequestGlobal records HTTP request using global metrics
func RecordHTTPRequestGlobal(method, path, status string, duration time.Duration, requestSize, responseSize int64) {
	if globalMetrics != nil {
		globalMetrics.RecordHTTPRequest(method, path, status, duration, requestSize, responseSize)
	}
}

// Middleware returns a Gin middleware that records HTTP request metrics
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start)
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Record request metrics
		m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())

		// Record request/response sizes if available
		reqSize := c.Request.ContentLength
		if reqSize < 0 {
			reqSize = 0
		}
		respSize := int64(c.Writer.Size())
		if respSize < 0 {
			respSize = 0
		}

		m.httpRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
		m.httpResponseSize.WithLabelValues(method, path, status).Observe(float64(respSize))
	}
}

// RegisterCounter registers a new counter metric with the given name, help text, and labels
func (m *Metrics) RegisterCounter(name, help string, labels []string) *prometheus.CounterVec {
	if existing, ok := m.customCounters[name]; ok {
		return existing
	}

	counter := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
	m.customCounters[name] = counter
	return counter
}

// RegisterGauge registers a new gauge metric with the given name and help text
func (m *Metrics) RegisterGauge(name, help string) prometheus.Gauge {
	if existing, ok := m.customGauges[name]; ok {
		return existing
	}

	gauge := promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
	)
	m.customGauges[name] = gauge
	return gauge
}

// RegisterHistogram registers a new histogram metric with the given name, help text, labels, and buckets
func (m *Metrics) RegisterHistogram(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	if existing, ok := m.customHistograms[name]; ok {
		return existing
	}

	if buckets == nil {
		buckets = prometheus.DefBuckets
	}

	histogram := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labels,
	)
	m.customHistograms[name] = histogram
	return histogram
}

// GetCounter returns a registered counter by name
func (m *Metrics) GetCounter(name string) *prometheus.CounterVec {
	return m.customCounters[name]
}

// GetGauge returns a registered gauge by name
func (m *Metrics) GetGauge(name string) prometheus.Gauge {
	return m.customGauges[name]
}

// GetHistogram returns a registered histogram by name
func (m *Metrics) GetHistogram(name string) *prometheus.HistogramVec {
	return m.customHistograms[name]
}
