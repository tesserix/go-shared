package logger

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tesserix/go-shared/security"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Config holds logger configuration
type Config struct {
	Level       LogLevel
	Environment string
	ServiceName string
	Version     string
	Output      io.Writer
	Format      string // "json" or "text"
}

// DefaultConfig returns a default logger configuration
func DefaultConfig(serviceName string) Config {
	return Config{
		Level:       LevelInfo,
		Environment: "development",
		ServiceName: serviceName,
		Version:     "1.0.0",
		Output:      os.Stdout,
		Format:      "json",
	}
}

// Logger wraps slog.Logger with additional context
type Logger struct {
	*slog.Logger
	config Config
}

// New creates a new logger instance
func New(config Config) *Logger {
	var level slog.Level
	switch config.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add service information to all logs
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(time.Now().UTC().Format(time.RFC3339)),
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if config.Format == "text" {
		handler = slog.NewTextHandler(config.Output, opts)
	} else {
		handler = slog.NewJSONHandler(config.Output, opts)
	}

	// Add default attributes
	logger := slog.New(handler).With(
		"service", config.ServiceName,
		"version", config.Version,
		"environment", config.Environment,
	)

	return &Logger{
		Logger: logger,
		config: config,
	}
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs := []slog.Attr{}

	// Extract request ID from context
	if requestID := getRequestID(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	// Extract user ID from context
	if userID := getUserID(ctx); userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}

	// Extract tenant ID from context
	if tenantID := getTenantID(ctx); tenantID != "" {
		attrs = append(attrs, slog.String("tenant_id", tenantID))
	}

	if len(attrs) > 0 {
		args := make([]any, len(attrs))
		for i, a := range attrs {
			args[i] = a
		}
		return &Logger{
			Logger: l.Logger.With(args...),
			config: l.config,
		}
	}

	return l
}

// WithFields adds structured fields to the logger
// Automatically sanitizes PII fields for security
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	// Sanitize fields to prevent PII leakage
	safeFields := security.SecureLogFields(fields)

	attrs := make([]interface{}, 0, len(safeFields)*2)
	for key, value := range safeFields {
		attrs = append(attrs, key, value)
	}

	return &Logger{
		Logger: l.Logger.With(attrs...),
		config: l.config,
	}
}

// WithFieldsUnsafe adds fields without sanitization (use with caution)
// Only use when you're certain the fields contain no PII
func (l *Logger) WithFieldsUnsafe(fields map[string]interface{}) *Logger {
	attrs := make([]interface{}, 0, len(fields)*2)
	for key, value := range fields {
		attrs = append(attrs, key, value)
	}

	return &Logger{
		Logger: l.Logger.With(attrs...),
		config: l.config,
	}
}

// WithError adds an error field to the logger
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With("error", err.Error()),
		config: l.config,
	}
}

// HTTP Logging Methods

// LogRequest logs HTTP request information
// SECURITY: Query string is sanitized to mask tokens, passwords, and PII
func (l *Logger) LogRequest(c *gin.Context) {
	// Sanitize the query string to mask sensitive parameters (tokens, passwords, etc.)
	sanitizedQuery := security.SanitizeQueryString(c.Request.URL.RawQuery)

	l.WithContext(c.Request.Context()).Info("HTTP request",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", sanitizedQuery,
		"remote_addr", c.ClientIP(),
		"user_agent", c.Request.UserAgent(),
	)
}

// LogResponse logs HTTP response information
func (l *Logger) LogResponse(c *gin.Context, duration time.Duration, statusCode int) {
	level := slog.LevelInfo
	if statusCode >= 400 && statusCode < 500 {
		level = slog.LevelWarn
	} else if statusCode >= 500 {
		level = slog.LevelError
	}

	l.WithContext(c.Request.Context()).Log(c.Request.Context(), level, "HTTP response",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"remote_addr", c.ClientIP(),
	)
}

// Database Logging Methods

// LogQuery logs database query information
func (l *Logger) LogQuery(query string, duration time.Duration, err error) {
	fields := map[string]interface{}{
		"query":       query,
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		l.WithFields(fields).WithError(err).Error("Database query failed")
	} else {
		l.WithFields(fields).Debug("Database query executed")
	}
}

// LogTransaction logs database transaction information
func (l *Logger) LogTransaction(operation string, duration time.Duration, err error) {
	fields := map[string]interface{}{
		"operation":   operation,
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		l.WithFields(fields).WithError(err).Error("Database transaction failed")
	} else {
		l.WithFields(fields).Info("Database transaction completed")
	}
}

// Business Logic Logging

// LogBusinessEvent logs business logic events
func (l *Logger) LogBusinessEvent(event string, entityType string, entityID string, fields map[string]interface{}) {
	logFields := map[string]interface{}{
		"event":       event,
		"entity_type": entityType,
		"entity_id":   entityID,
	}

	// Merge additional fields
	for k, v := range fields {
		logFields[k] = v
	}

	l.WithFields(logFields).Info("Business event")
}

// LogAudit logs audit events
func (l *Logger) LogAudit(action string, resource string, resourceID string, userID string, changes map[string]interface{}) {
	l.WithFields(map[string]interface{}{
		"action":      action,
		"resource":    resource,
		"resource_id": resourceID,
		"user_id":     userID,
		"changes":     changes,
	}).Info("Audit event")
}

// Security Logging

// LogSecurityEvent logs security-related events
func (l *Logger) LogSecurityEvent(event string, severity string, details map[string]interface{}) {
	level := slog.LevelWarn
	if severity == "high" || severity == "critical" {
		level = slog.LevelError
	}

	fields := map[string]interface{}{
		"security_event": event,
		"severity":       severity,
	}

	for k, v := range details {
		fields[k] = v
	}

	l.WithFields(fields).Log(context.Background(), level, "Security event")
}

// Performance Logging

// LogPerformance logs performance metrics
func (l *Logger) LogPerformance(operation string, duration time.Duration, metadata map[string]interface{}) {
	fields := map[string]interface{}{
		"operation":   operation,
		"duration_ms": duration.Milliseconds(),
	}

	for k, v := range metadata {
		fields[k] = v
	}

	level := slog.LevelInfo
	if duration > 5*time.Second {
		level = slog.LevelWarn
	}

	l.WithFields(fields).Log(context.Background(), level, "Performance metric")
}

// Helper functions to extract context values

func getRequestID(ctx context.Context) string {
	if c, ok := ctx.(*gin.Context); ok {
		if requestID, exists := c.Get("request_id"); exists {
			if id, ok := requestID.(string); ok {
				return id
			}
		}
	}
	return ""
}

func getUserID(ctx context.Context) string {
	if c, ok := ctx.(*gin.Context); ok {
		if userID, exists := c.Get("user_id"); exists {
			if id, ok := userID.(string); ok {
				return id
			}
		}
	}
	return ""
}

func getTenantID(ctx context.Context) string {
	if c, ok := ctx.(*gin.Context); ok {
		if tenantID, exists := c.Get("tenant_id"); exists {
			if id, ok := tenantID.(string); ok {
				return id
			}
		}
	}
	return ""
}

// GinMiddleware creates a Gin middleware for request/response logging
func (l *Logger) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Log request
		l.LogRequest(c)
		
		// Process request
		c.Next()
		
		// Log response
		duration := time.Since(start)
		l.LogResponse(c, duration, c.Writer.Status())
	}
}

// JSONFormatter formats logs as JSON with custom structure
type JSONFormatter struct{}

func (f JSONFormatter) Format(record *slog.Record) string {
	logEntry := map[string]interface{}{
		"timestamp": record.Time.UTC().Format(time.RFC3339),
		"level":     record.Level.String(),
		"message":   record.Message,
	}

	// Add source information
	if record.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
		logEntry["source"] = map[string]interface{}{
			"file":     frame.File,
			"line":     frame.Line,
			"function": frame.Function,
		}
	}

	// Add attributes
	record.Attrs(func(attr slog.Attr) bool {
		logEntry[attr.Key] = attr.Value.Any()
		return true
	})

	jsonBytes, _ := json.Marshal(logEntry)
	return string(jsonBytes)
}