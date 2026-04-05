package tracing

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry configuration
type Config struct {
	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// Environment is the deployment environment (e.g., production, staging, development)
	Environment string

	// OTLPEndpoint is the OTLP collector endpoint
	OTLPEndpoint string

	// OTLPInsecure disables TLS for OTLP connection
	OTLPInsecure bool

	// SamplingRate is the sampling rate (0.0 to 1.0)
	SamplingRate float64

	// EnableBatching enables batch span processing
	EnableBatching bool

	// BatchTimeout is the max time to wait before sending a batch
	BatchTimeout time.Duration

	// MaxExportBatchSize is the max number of spans in a batch
	MaxExportBatchSize int

	// MaxQueueSize is the max number of spans in the queue
	MaxQueueSize int
}

// DefaultConfig returns default OpenTelemetry configuration
func DefaultConfig(serviceName string) Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	env := os.Getenv("OTEL_ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	return Config{
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		Environment:        env,
		OTLPEndpoint:       endpoint,
		OTLPInsecure:       true,
		SamplingRate:       1.0, // 100% sampling in development
		EnableBatching:     true,
		BatchTimeout:       5 * time.Second,
		MaxExportBatchSize: 512,
		MaxQueueSize:       2048,
	}
}

// ProductionConfig returns production-optimized configuration
func ProductionConfig(serviceName string) Config {
	config := DefaultConfig(serviceName)
	config.SamplingRate = 0.1 // 10% sampling in production
	config.Environment = "production"
	return config
}

// TracerProvider wraps the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	config   Config
}

// InitTracer initializes OpenTelemetry with the given configuration
func InitTracer(config Config) (*TracerProvider, error) {
	ctx := context.Background()

	// Create OTLP exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
	}
	if config.OTLPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	client := otlptracehttp.NewClient(opts...)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	}

	// Create span processor
	var spanProcessor sdktrace.SpanProcessor
	if config.EnableBatching {
		spanProcessor = sdktrace.NewBatchSpanProcessor(
			exporter,
			sdktrace.WithBatchTimeout(config.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(config.MaxExportBatchSize),
			sdktrace.WithMaxQueueSize(config.MaxQueueSize),
		)
	} else {
		spanProcessor = sdktrace.NewSimpleSpanProcessor(exporter)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(spanProcessor),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set text map propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{
		provider: tp,
		config:   config,
	}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (t *TracerProvider) Shutdown(ctx context.Context) error {
	return t.provider.Shutdown(ctx)
}

// Tracer returns a tracer for the given name
func (t *TracerProvider) Tracer(name string) trace.Tracer {
	return t.provider.Tracer(name)
}

// GetTracer returns the global tracer for a given name
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// GinMiddleware returns a Gin middleware for OpenTelemetry tracing
func GinMiddleware(serviceName string) gin.HandlerFunc {
	tracer := GetTracer(serviceName)

	return func(c *gin.Context) {
		// Skip health check endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Extract context from headers for distributed tracing
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start span
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		if c.FullPath() == "" {
			spanName = fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPRoute(c.FullPath()),
				semconv.HTTPURL(c.Request.URL.String()),
				semconv.HTTPScheme(c.Request.URL.Scheme),
				semconv.NetHostName(c.Request.Host),
				semconv.UserAgentOriginal(c.Request.UserAgent()),
			),
		)
		defer span.End()

		// Add tenant ID if present
		if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
			span.SetAttributes(attribute.String("tenant.id", tenantID))
		}

		// Add user ID if present
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			span.SetAttributes(attribute.String("user.id", userID))
		}

		// Store span context in gin context
		c.Request = c.Request.WithContext(ctx)
		c.Set("tracing.span", span)

		// Process request
		c.Next()

		// Set response attributes
		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPStatusCode(status))

		// Set span status based on HTTP status code
		if status >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
			if status >= 500 {
				span.SetAttributes(attribute.Bool("error", true))
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Record errors
		if len(c.Errors) > 0 {
			span.SetStatus(codes.Error, c.Errors.String())
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
		}
	}
}

// SpanFromContext returns the current span from the Gin context
func SpanFromContext(c *gin.Context) trace.Span {
	if span, exists := c.Get("tracing.span"); exists {
		return span.(trace.Span)
	}
	return trace.SpanFromContext(c.Request.Context())
}

// StartSpan starts a new child span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer("default").Start(ctx, name, opts...)
}

// StartDBSpan starts a new span for database operations
func StartDBSpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	return GetTracer("database").Start(ctx, fmt.Sprintf("DB %s %s", operation, table),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation(operation),
			semconv.DBSQLTable(table),
		),
	)
}

// StartRedisSpan starts a new span for Redis operations
func StartRedisSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return GetTracer("redis").Start(ctx, fmt.Sprintf("Redis %s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemRedis,
			semconv.DBOperation(operation),
		),
	)
}

// StartHTTPClientSpan starts a new span for HTTP client requests
func StartHTTPClientSpan(ctx context.Context, method, url string) (context.Context, trace.Span) {
	return GetTracer("http-client").Start(ctx, fmt.Sprintf("HTTP %s", method),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPMethod(method),
			semconv.HTTPURL(url),
		),
	)
}

// StartNATSSpan starts a new span for NATS operations
func StartNATSSpan(ctx context.Context, operation, subject string) (context.Context, trace.Span) {
	return GetTracer("nats").Start(ctx, fmt.Sprintf("NATS %s %s", operation, subject),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", subject),
			attribute.String("messaging.operation", operation),
		),
	)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetError marks the span as error and records the error
func SetError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetAttribute sets an attribute on the current span
func SetAttribute(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	switch v := value.(type) {
	case string:
		span.SetAttributes(attribute.String(key, v))
	case int:
		span.SetAttributes(attribute.Int(key, v))
	case int64:
		span.SetAttributes(attribute.Int64(key, v))
	case float64:
		span.SetAttributes(attribute.Float64(key, v))
	case bool:
		span.SetAttributes(attribute.Bool(key, v))
	default:
		span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

// TracingContextKey is the key for storing tracing context
type TracingContextKey struct{}

// WithTraceID adds trace ID to context for logging
func WithTraceID(ctx context.Context) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return context.WithValue(ctx, TracingContextKey{}, span.SpanContext().TraceID().String())
	}
	return ctx
}

// GetTraceID extracts trace ID from context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	if traceID, ok := ctx.Value(TracingContextKey{}).(string); ok {
		return traceID
	}
	return ""
}

// InjectContext injects tracing context into headers for outgoing requests
func InjectContext(ctx context.Context, headers map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
}
