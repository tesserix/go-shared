package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// init puts Gin into test mode for the entire package so that router
// creation does not write noise to stdout.
func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRouter returns a fresh Gin engine configured for testing.
func newTestRouter() *gin.Engine {
	return gin.New()
}

// newTestHTTPContext creates a Gin context backed by an httptest recorder.
func newTestHTTPContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

// ---- DefaultConfig ----------------------------------------------------------

func TestDefaultConfig_Namespace(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "tesseract", cfg.Namespace)
}

func TestDefaultConfig_Subsystem(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "http_client", cfg.Subsystem)
}

func TestDefaultConfig_ServiceNameEmpty(t *testing.T) {
	cfg := DefaultConfig()
	assert.Empty(t, cfg.ServiceName, "DefaultConfig should not set a service name")
}

// ---- New / Config -----------------------------------------------------------

func TestNew_ReturnsNonNil(t *testing.T) {
	// Each call to New() registers Prometheus metrics via promauto. Because
	// promauto uses the default registry and panics on duplicate registration,
	// we use a unique namespace per test invocation.
	cfg := Config{
		Namespace:   "test_new_ns",
		Subsystem:   "smoke",
		ServiceName: "test-service",
	}
	m := New(cfg)
	require.NotNil(t, m)
}

func TestNew_ServiceNameStored(t *testing.T) {
	cfg := Config{
		Namespace:   "test_svcname_ns",
		Subsystem:   "svc",
		ServiceName: "order-service",
	}
	m := New(cfg)
	assert.Equal(t, "order-service", m.serviceName)
}

func TestNew_CustomMapsInitialised(t *testing.T) {
	cfg := Config{
		Namespace: "test_maps_ns",
		Subsystem: "maps",
	}
	m := New(cfg)
	assert.NotNil(t, m.customCounters)
	assert.NotNil(t, m.customGauges)
	assert.NotNil(t, m.customHistograms)
}

func TestNew_InternalMetricsNonNil(t *testing.T) {
	cfg := Config{
		Namespace: "test_internal_ns",
		Subsystem: "internal",
	}
	m := New(cfg)
	assert.NotNil(t, m.httpRequestsTotal, "httpRequestsTotal must be initialised")
	assert.NotNil(t, m.httpRequestDuration, "httpRequestDuration must be initialised")
	assert.NotNil(t, m.httpRequestSize, "httpRequestSize must be initialised")
	assert.NotNil(t, m.httpResponseSize, "httpResponseSize must be initialised")
}

// ---- RecordHTTPRequest ------------------------------------------------------

func TestRecordHTTPRequest_DoesNotPanic(t *testing.T) {
	cfg := Config{Namespace: "test_record_ns", Subsystem: "record"}
	m := New(cfg)

	assert.NotPanics(t, func() {
		m.RecordHTTPRequest("GET", "/health", "200", 50*time.Millisecond, 128, 512)
	})
}

func TestRecordHTTPRequest_VariousMethodsAndStatuses(t *testing.T) {
	cfg := Config{Namespace: "test_methods_ns", Subsystem: "methods"}
	m := New(cfg)

	tests := []struct {
		method   string
		path     string
		status   string
		duration time.Duration
		reqSize  int64
		respSize int64
	}{
		{"GET", "/users", "200", 10 * time.Millisecond, 0, 1024},
		{"POST", "/orders", "201", 100 * time.Millisecond, 256, 512},
		{"DELETE", "/items/1", "204", 5 * time.Millisecond, 0, 0},
		{"PUT", "/products/2", "400", 20 * time.Millisecond, 512, 128},
		{"GET", "/missing", "404", 2 * time.Millisecond, 0, 64},
		{"GET", "/crash", "500", 500 * time.Millisecond, 0, 256},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			assert.NotPanics(t, func() {
				m.RecordHTTPRequest(tt.method, tt.path, tt.status, tt.duration, tt.reqSize, tt.respSize)
			})
		})
	}
}

func TestRecordHTTPRequest_ZeroDuration(t *testing.T) {
	cfg := Config{Namespace: "test_zerodur_ns", Subsystem: "zerodur"}
	m := New(cfg)

	assert.NotPanics(t, func() {
		m.RecordHTTPRequest("GET", "/fast", "200", 0, 0, 0)
	})
}

func TestRecordHTTPRequest_LargeSizes(t *testing.T) {
	cfg := Config{Namespace: "test_large_ns", Subsystem: "large"}
	m := New(cfg)

	assert.NotPanics(t, func() {
		m.RecordHTTPRequest("POST", "/upload", "200", time.Second, 1<<20, 1<<10)
	})
}

// ---- Global metrics ---------------------------------------------------------

func TestInitGlobal_ReturnsNonNil(t *testing.T) {
	// Use a unique namespace so promauto does not conflict.
	cfg := Config{Namespace: "test_global_ns", Subsystem: "global"}
	m := InitGlobal(cfg)
	require.NotNil(t, m)
}

func TestGetGlobal_ReturnsInstanceAfterInit(t *testing.T) {
	cfg := Config{Namespace: "test_getglobal_ns", Subsystem: "getglobal"}
	InitGlobal(cfg)
	got := GetGlobal()
	assert.NotNil(t, got)
}

func TestRecordHTTPRequestGlobal_NilGlobalDoesNotPanic(t *testing.T) {
	// Temporarily clear the global so we can verify the nil guard.
	saved := globalMetrics
	globalMetrics = nil
	defer func() { globalMetrics = saved }()

	assert.NotPanics(t, func() {
		RecordHTTPRequestGlobal("GET", "/test", "200", time.Millisecond, 0, 0)
	})
}

func TestRecordHTTPRequestGlobal_WithGlobalInitialised(t *testing.T) {
	cfg := Config{Namespace: "test_globalrec_ns", Subsystem: "globalrec"}
	InitGlobal(cfg)

	assert.NotPanics(t, func() {
		RecordHTTPRequestGlobal("POST", "/events", "201", 15*time.Millisecond, 64, 128)
	})
}

// ---- Middleware --------------------------------------------------------------

func TestMiddleware_ReturnsHandlerFunc(t *testing.T) {
	cfg := Config{Namespace: "test_mw_ns", Subsystem: "mw"}
	m := New(cfg)

	handler := m.Middleware()
	assert.NotNil(t, handler)
}

func TestMiddleware_Executes200Request(t *testing.T) {
	cfg := Config{Namespace: "test_mw200_ns", Subsystem: "mw200"}
	m := New(cfg)

	router := newTestRouter()
	router.Use(m.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_Executes404Request(t *testing.T) {
	cfg := Config{Namespace: "test_mw404_ns", Subsystem: "mw404"}
	m := New(cfg)

	router := newTestRouter()
	router.Use(m.Middleware())
	router.GET("/exists", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Gin returns 404 for unknown routes; the middleware must not panic.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMiddleware_RequestWithBody(t *testing.T) {
	cfg := Config{Namespace: "test_mwbody_ns", Subsystem: "mwbody"}
	m := New(cfg)

	router := newTestRouter()
	router.Use(m.Middleware())
	router.POST("/data", func(c *gin.Context) {
		c.String(http.StatusCreated, "created")
	})

	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/data", nil)
	req.ContentLength = int64(len(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestMiddleware_PreservesNextHandlerResponse(t *testing.T) {
	cfg := Config{Namespace: "test_mwnext_ns", Subsystem: "mwnext"}
	m := New(cfg)

	router := newTestRouter()
	router.Use(m.Middleware())
	router.GET("/value", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"result": 42})
	})

	req := httptest.NewRequest(http.MethodGet, "/value", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "42")
}

// ---- RegisterCounter --------------------------------------------------------

func TestRegisterCounter_ReturnsNonNil(t *testing.T) {
	cfg := Config{Namespace: "test_regcnt_ns", Subsystem: "regcnt"}
	m := New(cfg)

	counter := m.RegisterCounter("my_events_total", "Total events processed", []string{"type"})
	require.NotNil(t, counter)
}

func TestRegisterCounter_IdempotentOnDuplicateName(t *testing.T) {
	cfg := Config{Namespace: "test_idmcnt_ns", Subsystem: "idmcnt"}
	m := New(cfg)

	first := m.RegisterCounter("idm_events_total", "Events", []string{"kind"})
	second := m.RegisterCounter("idm_events_total", "Events", []string{"kind"})

	// Must return the same instance, not register a second metric.
	assert.Equal(t, first, second)
}

func TestRegisterCounter_StoredInMap(t *testing.T) {
	cfg := Config{Namespace: "test_storecnt_ns", Subsystem: "storecnt"}
	m := New(cfg)

	m.RegisterCounter("stored_counter_total", "Stored", []string{"env"})
	got := m.GetCounter("stored_counter_total")
	assert.NotNil(t, got)
}

func TestRegisterCounter_UnknownNameReturnsNil(t *testing.T) {
	cfg := Config{Namespace: "test_nilcnt_ns", Subsystem: "nilcnt"}
	m := New(cfg)

	got := m.GetCounter("nonexistent_counter")
	assert.Nil(t, got)
}

func TestRegisterCounter_CanIncrementReturnedVec(t *testing.T) {
	cfg := Config{Namespace: "test_inccnt_ns", Subsystem: "inccnt"}
	m := New(cfg)

	counter := m.RegisterCounter("inc_counter_total", "Increments", []string{"tag"})
	require.NotNil(t, counter)

	assert.NotPanics(t, func() {
		counter.WithLabelValues("tagA").Inc()
		counter.WithLabelValues("tagB").Add(5)
	})
}

// ---- RegisterGauge ----------------------------------------------------------

func TestRegisterGauge_ReturnsNonNil(t *testing.T) {
	cfg := Config{Namespace: "test_reggauge_ns", Subsystem: "reggauge"}
	m := New(cfg)

	gauge := m.RegisterGauge("active_connections", "Active connections")
	require.NotNil(t, gauge)
}

func TestRegisterGauge_IdempotentOnDuplicateName(t *testing.T) {
	cfg := Config{Namespace: "test_idmgauge_ns", Subsystem: "idmgauge"}
	m := New(cfg)

	first := m.RegisterGauge("idm_connections", "Connections")
	second := m.RegisterGauge("idm_connections", "Connections")
	assert.Equal(t, first, second)
}

func TestRegisterGauge_StoredInMap(t *testing.T) {
	cfg := Config{Namespace: "test_storegauge_ns", Subsystem: "storegauge"}
	m := New(cfg)

	m.RegisterGauge("stored_gauge", "Stored gauge")
	got := m.GetGauge("stored_gauge")
	assert.NotNil(t, got)
}

func TestRegisterGauge_UnknownNameReturnsNil(t *testing.T) {
	cfg := Config{Namespace: "test_nilgauge_ns", Subsystem: "nilgauge"}
	m := New(cfg)

	got := m.GetGauge("nonexistent_gauge")
	assert.Nil(t, got)
}

func TestRegisterGauge_CanSetAndObserve(t *testing.T) {
	cfg := Config{Namespace: "test_setgauge_ns", Subsystem: "setgauge"}
	m := New(cfg)

	gauge := m.RegisterGauge("set_gauge", "Settable gauge")
	require.NotNil(t, gauge)

	assert.NotPanics(t, func() {
		gauge.Set(42.5)
		gauge.Inc()
		gauge.Dec()
		gauge.Add(10)
		gauge.Sub(3)
	})
}

func TestRegisterGauge_ImplementsPrometheusGauge(t *testing.T) {
	cfg := Config{Namespace: "test_iface_ns", Subsystem: "iface"}
	m := New(cfg)

	// RegisterGauge must return a prometheus.Gauge (interface satisfied at compile time).
	var _ prometheus.Gauge = m.RegisterGauge("iface_gauge", "Interface check")
}

// ---- RegisterHistogram -------------------------------------------------------

func TestRegisterHistogram_ReturnsNonNil(t *testing.T) {
	cfg := Config{Namespace: "test_reghist_ns", Subsystem: "reghist"}
	m := New(cfg)

	hist := m.RegisterHistogram("op_duration_seconds", "Operation duration", []string{"op"}, nil)
	require.NotNil(t, hist)
}

func TestRegisterHistogram_IdempotentOnDuplicateName(t *testing.T) {
	cfg := Config{Namespace: "test_idmhist_ns", Subsystem: "idmhist"}
	m := New(cfg)

	first := m.RegisterHistogram("idm_duration_seconds", "Duration", []string{"step"}, nil)
	second := m.RegisterHistogram("idm_duration_seconds", "Duration", []string{"step"}, nil)
	assert.Equal(t, first, second)
}

func TestRegisterHistogram_StoredInMap(t *testing.T) {
	cfg := Config{Namespace: "test_storehist_ns", Subsystem: "storehist"}
	m := New(cfg)

	m.RegisterHistogram("stored_histogram_seconds", "Stored", []string{"env"}, nil)
	got := m.GetHistogram("stored_histogram_seconds")
	assert.NotNil(t, got)
}

func TestRegisterHistogram_UnknownNameReturnsNil(t *testing.T) {
	cfg := Config{Namespace: "test_nilhist_ns", Subsystem: "nilhist"}
	m := New(cfg)

	got := m.GetHistogram("nonexistent_histogram")
	assert.Nil(t, got)
}

func TestRegisterHistogram_UsesDefaultBucketsWhenNilPassed(t *testing.T) {
	cfg := Config{Namespace: "test_defbuck_ns", Subsystem: "defbuck"}
	m := New(cfg)

	// Passing nil buckets must not panic; defaults are applied internally.
	assert.NotPanics(t, func() {
		m.RegisterHistogram("defbuck_duration_seconds", "Default buckets", []string{"tag"}, nil)
	})
}

func TestRegisterHistogram_CustomBuckets(t *testing.T) {
	cfg := Config{Namespace: "test_custbuck_ns", Subsystem: "custbuck"}
	m := New(cfg)

	buckets := []float64{0.001, 0.01, 0.1, 1.0}
	hist := m.RegisterHistogram("custbuck_duration_seconds", "Custom buckets", []string{"route"}, buckets)
	require.NotNil(t, hist)
}

func TestRegisterHistogram_CanObserveValues(t *testing.T) {
	cfg := Config{Namespace: "test_obshist_ns", Subsystem: "obshist"}
	m := New(cfg)

	hist := m.RegisterHistogram("obs_duration_seconds", "Observable", []string{"op"}, nil)
	require.NotNil(t, hist)

	assert.NotPanics(t, func() {
		hist.WithLabelValues("create").Observe(0.05)
		hist.WithLabelValues("update").Observe(0.12)
		hist.WithLabelValues("delete").Observe(0.003)
	})
}

// ---- GetCounter / GetGauge / GetHistogram (already covered above) -----------

func TestGetCounter_AfterRegister(t *testing.T) {
	cfg := Config{Namespace: "test_getcnt2_ns", Subsystem: "getcnt2"}
	m := New(cfg)

	registered := m.RegisterCounter("getcnt2_total", "Counter", []string{"a"})
	retrieved := m.GetCounter("getcnt2_total")
	assert.Equal(t, registered, retrieved)
}

func TestGetGauge_AfterRegister(t *testing.T) {
	cfg := Config{Namespace: "test_getgauge2_ns", Subsystem: "getgauge2"}
	m := New(cfg)

	registered := m.RegisterGauge("getgauge2_metric", "Gauge")
	retrieved := m.GetGauge("getgauge2_metric")
	assert.Equal(t, registered, retrieved)
}

func TestGetHistogram_AfterRegister(t *testing.T) {
	cfg := Config{Namespace: "test_gethist2_ns", Subsystem: "gethist2"}
	m := New(cfg)

	registered := m.RegisterHistogram("gethist2_seconds", "Histogram", []string{"b"}, nil)
	retrieved := m.GetHistogram("gethist2_seconds")
	assert.Equal(t, registered, retrieved)
}
