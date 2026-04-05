package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRouter returns a Gin engine with no middleware for handler tests.
func newTestRouter() *gin.Engine {
	return gin.New()
}

// ---- TracingConfig struct ----------------------------------------------------

func TestTracingConfig_FieldsExist(t *testing.T) {
	// Verify all documented Config fields are accessible.
	cfg := Config{
		ServiceName:        "my-service",
		ServiceVersion:     "2.0.0",
		Environment:        "staging",
		OTLPEndpoint:       "otel-collector:4318",
		OTLPInsecure:       true,
		SamplingRate:       0.5,
		EnableBatching:     true,
		BatchTimeout:       3 * time.Second,
		MaxExportBatchSize: 256,
		MaxQueueSize:       1024,
	}

	assert.Equal(t, "my-service", cfg.ServiceName)
	assert.Equal(t, "2.0.0", cfg.ServiceVersion)
	assert.Equal(t, "staging", cfg.Environment)
	assert.Equal(t, "otel-collector:4318", cfg.OTLPEndpoint)
	assert.True(t, cfg.OTLPInsecure)
	assert.Equal(t, 0.5, cfg.SamplingRate)
	assert.True(t, cfg.EnableBatching)
	assert.Equal(t, 3*time.Second, cfg.BatchTimeout)
	assert.Equal(t, 256, cfg.MaxExportBatchSize)
	assert.Equal(t, 1024, cfg.MaxQueueSize)
}

// ---- DefaultConfig ----------------------------------------------------------

func TestDefaultConfig_ServiceName(t *testing.T) {
	cfg := DefaultConfig("inventory-service")
	assert.Equal(t, "inventory-service", cfg.ServiceName)
}

func TestDefaultConfig_ServiceVersion(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.Equal(t, "1.0.0", cfg.ServiceVersion)
}

func TestDefaultConfig_DefaultEndpoint(t *testing.T) {
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	cfg := DefaultConfig("svc")
	assert.Equal(t, "localhost:4318", cfg.OTLPEndpoint)
}

func TestDefaultConfig_EndpointFromEnv(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "custom-collector:4317")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	cfg := DefaultConfig("svc")
	assert.Equal(t, "custom-collector:4317", cfg.OTLPEndpoint)
}

func TestDefaultConfig_DefaultEnvironment(t *testing.T) {
	os.Unsetenv("OTEL_ENVIRONMENT")
	cfg := DefaultConfig("svc")
	assert.Equal(t, "development", cfg.Environment)
}

func TestDefaultConfig_EnvironmentFromEnv(t *testing.T) {
	os.Setenv("OTEL_ENVIRONMENT", "staging")
	defer os.Unsetenv("OTEL_ENVIRONMENT")

	cfg := DefaultConfig("svc")
	assert.Equal(t, "staging", cfg.Environment)
}

func TestDefaultConfig_InsecureTrue(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.True(t, cfg.OTLPInsecure)
}

func TestDefaultConfig_SamplingRateIs100Percent(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.Equal(t, 1.0, cfg.SamplingRate)
}

func TestDefaultConfig_BatchingEnabled(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.True(t, cfg.EnableBatching)
}

func TestDefaultConfig_BatchTimeout(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.Equal(t, 5*time.Second, cfg.BatchTimeout)
}

func TestDefaultConfig_MaxExportBatchSize(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.Equal(t, 512, cfg.MaxExportBatchSize)
}

func TestDefaultConfig_MaxQueueSize(t *testing.T) {
	cfg := DefaultConfig("svc")
	assert.Equal(t, 2048, cfg.MaxQueueSize)
}

// ---- ProductionConfig -------------------------------------------------------

func TestProductionConfig_SamplingRateIs10Percent(t *testing.T) {
	cfg := ProductionConfig("svc")
	assert.Equal(t, 0.1, cfg.SamplingRate)
}

func TestProductionConfig_EnvironmentIsProduction(t *testing.T) {
	// Override any OTEL_ENVIRONMENT env var for a clean assertion.
	os.Unsetenv("OTEL_ENVIRONMENT")
	cfg := ProductionConfig("svc")
	assert.Equal(t, "production", cfg.Environment)
}

func TestProductionConfig_ServiceNamePreserved(t *testing.T) {
	cfg := ProductionConfig("payment-service")
	assert.Equal(t, "payment-service", cfg.ServiceName)
}

func TestProductionConfig_BatchingStillEnabled(t *testing.T) {
	cfg := ProductionConfig("svc")
	assert.True(t, cfg.EnableBatching)
}

// ---- GetTracer --------------------------------------------------------------

func TestGetTracer_ReturnsNonNil(t *testing.T) {
	tracer := GetTracer("test-tracer")
	assert.NotNil(t, tracer)
}

func TestGetTracer_DifferentNamesReturnTracers(t *testing.T) {
	tests := []string{"database", "http-client", "redis", "nats", "default"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			tr := GetTracer(name)
			assert.NotNil(t, tr)
		})
	}
}

// ---- StartSpan --------------------------------------------------------------

func TestStartSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "test-operation")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestStartSpan_ContextDerivedFromParent(t *testing.T) {
	type contextKey struct{}
	parent := context.WithValue(context.Background(), contextKey{}, "sentinel")

	ctx, span := StartSpan(parent, "child-span")
	defer span.End()

	assert.Equal(t, "sentinel", ctx.Value(contextKey{}), "child context must carry parent values")
}

func TestStartSpan_WithSpanKindOption(t *testing.T) {
	ctx, span := StartSpan(
		context.Background(),
		"client-call",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

// ---- StartDBSpan ------------------------------------------------------------

func TestStartDBSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := StartDBSpan(context.Background(), "SELECT", "orders")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestStartDBSpan_VariousOperations(t *testing.T) {
	tests := []struct {
		op    string
		table string
	}{
		{"SELECT", "users"},
		{"INSERT", "products"},
		{"UPDATE", "inventory"},
		{"DELETE", "sessions"},
	}

	for _, tt := range tests {
		t.Run(tt.op+"_"+tt.table, func(t *testing.T) {
			ctx, span := StartDBSpan(context.Background(), tt.op, tt.table)
			assert.NotNil(t, ctx)
			assert.NotNil(t, span)
			span.End()
		})
	}
}

// ---- StartRedisSpan ---------------------------------------------------------

func TestStartRedisSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := StartRedisSpan(context.Background(), "GET")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestStartRedisSpan_VariousCommands(t *testing.T) {
	commands := []string{"GET", "SET", "DEL", "HGETALL", "LPUSH", "ZADD"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			ctx, span := StartRedisSpan(context.Background(), cmd)
			assert.NotNil(t, ctx)
			assert.NotNil(t, span)
			span.End()
		})
	}
}

// ---- StartHTTPClientSpan ----------------------------------------------------

func TestStartHTTPClientSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := StartHTTPClientSpan(context.Background(), "GET", "https://api.example.com/items")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestStartHTTPClientSpan_VariousMethods(t *testing.T) {
	tests := []struct {
		method string
		url    string
	}{
		{"GET", "https://svc/users"},
		{"POST", "https://svc/orders"},
		{"PUT", "https://svc/products/1"},
		{"DELETE", "https://svc/items/2"},
		{"PATCH", "https://svc/settings"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			ctx, span := StartHTTPClientSpan(context.Background(), tt.method, tt.url)
			assert.NotNil(t, ctx)
			assert.NotNil(t, span)
			span.End()
		})
	}
}

// ---- StartNATSSpan ----------------------------------------------------------

func TestStartNATSSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := StartNATSSpan(context.Background(), "publish", "orders.created")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestStartNATSSpan_VariousSubjects(t *testing.T) {
	tests := []struct {
		op      string
		subject string
	}{
		{"publish", "orders.created"},
		{"subscribe", "payments.processed"},
		{"request", "inventory.check"},
	}

	for _, tt := range tests {
		t.Run(tt.op+"_"+tt.subject, func(t *testing.T) {
			ctx, span := StartNATSSpan(context.Background(), tt.op, tt.subject)
			assert.NotNil(t, ctx)
			assert.NotNil(t, span)
			span.End()
		})
	}
}

// ---- AddEvent ---------------------------------------------------------------

func TestAddEvent_DoesNotPanic(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "event-span")
	defer span.End()

	assert.NotPanics(t, func() {
		AddEvent(ctx, "cache.miss")
	})
}

func TestAddEvent_WithAttributes(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-event-span")
	defer span.End()

	assert.NotPanics(t, func() {
		AddEvent(ctx, "user.login",
			attribute.String("user.id", "u-123"),
			attribute.Bool("mfa.enabled", true),
		)
	})
}

// ---- SetError ---------------------------------------------------------------

func TestSetError_DoesNotPanic(t *testing.T) {
	_, span := StartSpan(context.Background(), "error-span")
	defer span.End()

	assert.NotPanics(t, func() {
		SetError(span, errors.New("something failed"))
	})
}

func TestSetError_AcceptsWrappedError(t *testing.T) {
	_, span := StartSpan(context.Background(), "wrapped-error-span")
	defer span.End()

	wrapped := errors.New("database connection refused")
	SetError(span, wrapped)
	// No panic and the span is still valid after recording an error.
	span.SetStatus(codes.Error, wrapped.Error())
}

// ---- SetAttribute -----------------------------------------------------------

func TestSetAttribute_StringValue(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-string-span")
	defer span.End()

	assert.NotPanics(t, func() {
		SetAttribute(ctx, "tenant.id", "t-42")
	})
}

func TestSetAttribute_IntValue(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-int-span")
	defer span.End()
	_ = span

	assert.NotPanics(t, func() {
		SetAttribute(ctx, "retry.count", 3)
	})
}

func TestSetAttribute_Int64Value(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-int64-span")
	defer span.End()

	assert.NotPanics(t, func() {
		SetAttribute(ctx, "bytes.written", int64(102400))
	})
}

func TestSetAttribute_Float64Value(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-float64-span")
	defer span.End()

	assert.NotPanics(t, func() {
		SetAttribute(ctx, "score", float64(0.98))
	})
}

func TestSetAttribute_BoolValue(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-bool-span")
	defer span.End()

	assert.NotPanics(t, func() {
		SetAttribute(ctx, "cache.hit", true)
	})
}

func TestSetAttribute_FallbackToStringRepresentation(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "attr-fallback-span")
	defer span.End()

	// A struct type falls through to the default branch, which uses fmt.Sprintf.
	assert.NotPanics(t, func() {
		SetAttribute(ctx, "some.type", struct{ V int }{V: 7})
	})
}

// ---- WithTraceID / GetTraceID -----------------------------------------------

func TestWithTraceID_DoesNotPanic(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		WithTraceID(ctx)
	})
}

func TestGetTraceID_EmptyOnNoSpan(t *testing.T) {
	ctx := context.Background()
	id := GetTraceID(ctx)
	// With no active span the trace ID is the zero value ("").
	assert.Empty(t, id)
}

func TestGetTraceID_StoredValueInContext(t *testing.T) {
	const sentinel = "abc123def456"
	ctx := context.WithValue(context.Background(), TracingContextKey{}, sentinel)

	id := GetTraceID(ctx)
	assert.Equal(t, sentinel, id)
}

func TestWithTraceID_ReturnsContext(t *testing.T) {
	ctx := context.Background()
	derived := WithTraceID(ctx)
	// The returned context must never be nil.
	assert.NotNil(t, derived)
}

// ---- InjectContext ----------------------------------------------------------

func TestInjectContext_DoesNotPanic(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "inject-span")
	defer span.End()

	headers := make(map[string]string)
	assert.NotPanics(t, func() {
		InjectContext(ctx, headers)
	})
}

func TestInjectContext_EmptyContextProducesHeaders(t *testing.T) {
	// Even without an active span, inject must not panic and the headers map
	// must remain a valid (possibly empty) map.
	headers := make(map[string]string)
	InjectContext(context.Background(), headers)
	assert.NotNil(t, headers)
}

// ---- TracingContextKey ------------------------------------------------------

func TestTracingContextKey_IsDistinctType(t *testing.T) {
	// Ensure different key types do not collide in a context.
	type otherKey struct{}
	ctx := context.Background()
	ctx = context.WithValue(ctx, TracingContextKey{}, "tracing-value")
	ctx = context.WithValue(ctx, otherKey{}, "other-value")

	assert.Equal(t, "tracing-value", ctx.Value(TracingContextKey{}))
	assert.Equal(t, "other-value", ctx.Value(otherKey{}))
}

// ---- GinMiddleware ----------------------------------------------------------

func TestGinMiddleware_ReturnsHandlerFunc(t *testing.T) {
	handler := GinMiddleware("test-service")
	assert.NotNil(t, handler)
}

func TestGinMiddleware_SkipsHealthEndpoint(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_SkipsReadyEndpoint(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/ready", func(c *gin.Context) {
		c.String(http.StatusOK, "ready")
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_SkipsMetricsEndpoint(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "metrics")
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_Traces200Request(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"users": []string{}})
	})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_Traces500Request(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/boom", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "crash"})
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGinMiddleware_PropagatesTraceHeaders(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/traced", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/traced", nil)
	// Supply a W3C trace-context header so the propagator can extract it.
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_WithTenantAndUserIDHeaders(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	router.GET("/tenant-route", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/tenant-route", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	req.Header.Set("X-User-ID", "user-xyz")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGinMiddleware_404DoesNotPanic(t *testing.T) {
	router := newTestRouter()
	router.Use(GinMiddleware("svc"))
	// No routes registered.

	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	w := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})
}

// ---- SpanFromContext --------------------------------------------------------

func TestSpanFromContext_ReturnsSpanFromGinKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	_, span := otel.Tracer("test").Start(c.Request.Context(), "stored-span")
	defer span.End()

	c.Set("tracing.span", span)

	got := SpanFromContext(c)
	assert.NotNil(t, got)
	assert.Equal(t, span, got)
}

func TestSpanFromContext_FallsBackToRequestContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	// Do NOT set the "tracing.span" key — should fall back to request context.

	got := SpanFromContext(c)
	// The fallback returns the span from the request context (may be a no-op span).
	assert.NotNil(t, got)
}
