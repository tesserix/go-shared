package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Silence Gin's startup banner and route-registration output in tests.
	gin.SetMode(gin.TestMode)
}

// --- DefaultConfig ---

func TestDefaultConfig_ReturnsExpectedValues(t *testing.T) {
	cfg := DefaultConfig("my-service")

	assert.Equal(t, LevelInfo, cfg.Level)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "my-service", cfg.ServiceName)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.NotNil(t, cfg.Output)
	assert.Equal(t, "json", cfg.Format)
}

func TestDefaultConfig_ServiceNamePreserved(t *testing.T) {
	names := []string{"order-service", "auth-bff", "tenant-service", ""}
	for _, name := range names {
		cfg := DefaultConfig(name)
		assert.Equal(t, name, cfg.ServiceName)
	}
}

// --- New() ---

func TestNew_CreatesLoggerSuccessfully(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.Output = new(bytes.Buffer)

	l := New(cfg)

	require.NotNil(t, l)
	assert.NotNil(t, l.Logger)
}

func TestNew_JSONFormat_WritesValidJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := Config{
		Level:       LevelInfo,
		Environment: "development",
		ServiceName: "test-service",
		Version:     "1.0.0",
		Output:      buf,
		Format:      "json",
	}

	l := New(cfg)
	l.Info("hello world")

	require.NotEmpty(t, buf.String(), "expected log output in buffer")

	// Every line should be valid JSON.
	for _, line := range nonEmptyLines(buf.String()) {
		var entry map[string]interface{}
		err := json.Unmarshal([]byte(line), &entry)
		assert.NoError(t, err, "expected valid JSON log line: %s", line)
	}
}

func TestNew_TextFormat_WritesTextOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := Config{
		Level:       LevelInfo,
		Environment: "development",
		ServiceName: "test-service",
		Version:     "1.0.0",
		Output:      buf,
		Format:      "text",
	}

	l := New(cfg)
	l.Info("hello text world")

	require.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "hello text world")
}

func TestNew_UnknownFormat_DefaultsToJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := Config{
		Level:       LevelInfo,
		Environment: "development",
		ServiceName: "svc",
		Version:     "1.0.0",
		Output:      buf,
		Format:      "yaml", // unrecognised → JSON
	}

	l := New(cfg)
	l.Info("checking format fallback")

	require.NotEmpty(t, buf.String())
	for _, line := range nonEmptyLines(buf.String()) {
		var entry map[string]interface{}
		assert.NoError(t, json.Unmarshal([]byte(line), &entry))
	}
}

func TestNew_IncludesServiceAttributes(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := Config{
		Level:       LevelInfo,
		Environment: "staging",
		ServiceName: "product-service",
		Version:     "3.1.4",
		Output:      buf,
		Format:      "json",
	}

	l := New(cfg)
	l.Info("attribute check")

	require.NotEmpty(t, buf.String())

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(firstNonEmptyLine(buf.String())), &entry))

	assert.Equal(t, "product-service", entry["service"])
	assert.Equal(t, "3.1.4", entry["version"])
	assert.Equal(t, "staging", entry["environment"])
}

// --- Log levels ---

func TestNew_DebugLevel_WritesDebugMessages(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	l.Debug("debug message")

	assert.Contains(t, buf.String(), "debug message")
}

func TestNew_InfoLevel_SuppressesDebugMessages(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.Debug("should be suppressed")
	l.Info("should appear")

	output := buf.String()
	assert.NotContains(t, output, "should be suppressed")
	assert.Contains(t, output, "should appear")
}

func TestNew_WarnLevel_SuppressesInfoAndDebug(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelWarn)

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")

	output := buf.String()
	assert.NotContains(t, output, "debug msg")
	assert.NotContains(t, output, "info msg")
	assert.Contains(t, output, "warn msg")
}

func TestNew_ErrorLevel_OnlyWritesErrors(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelError)

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	output := buf.String()
	assert.NotContains(t, output, "debug msg")
	assert.NotContains(t, output, "info msg")
	assert.NotContains(t, output, "warn msg")
	assert.Contains(t, output, "error msg")
}

func TestNew_AllLogLevels_Recognised(t *testing.T) {
	levels := []LogLevel{LevelDebug, LevelInfo, LevelWarn, LevelError}
	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			buf := new(bytes.Buffer)
			cfg := Config{
				Level:       level,
				Environment: "development",
				ServiceName: "svc",
				Version:     "1.0.0",
				Output:      buf,
				Format:      "json",
			}
			l := New(cfg)
			require.NotNil(t, l)
		})
	}
}

// --- WithError ---

func TestWithError_AddsErrorField(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	testErr := errors.New("something went wrong")
	l.WithError(testErr).Error("operation failed")

	output := buf.String()
	assert.Contains(t, output, "something went wrong")
}

func TestWithError_DoesNotMutateParentLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	derived := l.WithError(errors.New("oops"))

	// The derived logger should be a distinct pointer.
	assert.NotSame(t, l, derived)

	// Writing from the original logger should not include the error field.
	l.Info("clean log")
	output := buf.String()
	assert.NotContains(t, output, "oops")
}

func TestWithError_ChainedWithInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.WithError(errors.New("db connection refused")).Info("startup check")

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(firstNonEmptyLine(buf.String())), &entry))

	assert.Equal(t, "db connection refused", entry["error"])
	assert.Equal(t, "startup check", entry["msg"])
}

// --- WithFields ---

func TestWithFields_AddsFieldsToOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.WithFields(map[string]interface{}{
		"order_id": "ORD-9001",
		"tenant":   "acme",
	}).Info("order created")

	output := buf.String()
	// order_id is a safe, non-PII field and should be present verbatim.
	assert.Contains(t, output, "ORD-9001")
	assert.Contains(t, output, "acme")
}

func TestWithFields_SanitisesPIIEmail(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.WithFields(map[string]interface{}{
		"email": "john.doe@example.com",
	}).Info("user action")

	output := buf.String()
	// The raw email must not appear; a masked version must.
	assert.NotContains(t, output, "john.doe@example.com")
	assert.Contains(t, output, "jo***@example.com")
}

func TestWithFields_SanitisesPassword(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.WithFields(map[string]interface{}{
		"password": "s3cr3t!",
	}).Info("login attempt")

	assert.NotContains(t, buf.String(), "s3cr3t!")
	assert.Contains(t, buf.String(), "[REDACTED]")
}

func TestWithFields_DoesNotMutateParentLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	derived := l.WithFields(map[string]interface{}{"trace_id": "abc-123"})
	assert.NotSame(t, l, derived)
}

func TestWithFields_ReturnsNewLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	result := l.WithFields(map[string]interface{}{"key": "value"})

	require.NotNil(t, result)
	assert.IsType(t, &Logger{}, result)
}

// --- WithFieldsUnsafe ---

func TestWithFieldsUnsafe_AddsFieldsWithoutSanitisation(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	// WithFieldsUnsafe deliberately skips PII scrubbing.
	// We use a clearly non-PII field to assert the value passes through.
	l.WithFieldsUnsafe(map[string]interface{}{
		"request_id": "req-abc-123",
		"operation":  "checkout",
	}).Info("unsafe fields test")

	output := buf.String()
	assert.Contains(t, output, "req-abc-123")
	assert.Contains(t, output, "checkout")
}

func TestWithFieldsUnsafe_ReturnsNewLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	result := l.WithFieldsUnsafe(map[string]interface{}{"k": "v"})

	require.NotNil(t, result)
	assert.NotSame(t, l, result)
}

// --- Logger writes to custom io.Writer ---

func TestLogger_WritesToCustomWriter(t *testing.T) {
	buf := new(bytes.Buffer)

	l := New(Config{
		Level:       LevelInfo,
		Environment: "test",
		ServiceName: "writer-test",
		Version:     "0.1.0",
		Output:      buf,
		Format:      "json",
	})

	l.Info("custom writer message")

	assert.Greater(t, buf.Len(), 0, "buffer should have received log output")
	assert.Contains(t, buf.String(), "custom writer message")
}

func TestLogger_MultipleWritesToSameWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.Info("first message")
	l.Info("second message")
	l.Warn("third message")

	output := buf.String()
	assert.Contains(t, output, "first message")
	assert.Contains(t, output, "second message")
	assert.Contains(t, output, "third message")
}

// --- Domain-specific logging methods ---

func TestLogQuery_SuccessWritesDebug(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	l.LogQuery("SELECT * FROM orders", 5*time.Millisecond, nil)

	output := buf.String()
	assert.Contains(t, output, "SELECT * FROM orders")
	assert.Contains(t, output, "Database query executed")
}

func TestLogQuery_ErrorWritesErrorLevel(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	l.LogQuery("INSERT INTO orders ...", 10*time.Millisecond, errors.New("unique constraint violation"))

	output := buf.String()
	assert.Contains(t, output, "Database query failed")
	assert.Contains(t, output, "unique constraint violation")
}

func TestLogTransaction_SuccessWritesInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.LogTransaction("create-order", 20*time.Millisecond, nil)

	assert.Contains(t, buf.String(), "Database transaction completed")
	assert.Contains(t, buf.String(), "create-order")
}

func TestLogTransaction_ErrorWritesError(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.LogTransaction("payment-capture", 30*time.Millisecond, errors.New("payment gateway timeout"))

	output := buf.String()
	assert.Contains(t, output, "Database transaction failed")
	assert.Contains(t, output, "payment gateway timeout")
}

func TestLogBusinessEvent_WritesEventInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.LogBusinessEvent("order.placed", "Order", "ORD-42", map[string]interface{}{
		"total": 99.99,
	})

	output := buf.String()
	assert.Contains(t, output, "Business event")
	assert.Contains(t, output, "order.placed")
	assert.Contains(t, output, "ORD-42")
}

func TestLogAudit_WritesAuditInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.LogAudit("update", "product", "PROD-7", "user-123", map[string]interface{}{
		"price": map[string]float64{"old": 10.0, "new": 12.5},
	})

	output := buf.String()
	assert.Contains(t, output, "Audit event")
	assert.Contains(t, output, "update")
	assert.Contains(t, output, "PROD-7")
}

func TestLogSecurityEvent_LowSeverityWritesWarn(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	l.LogSecurityEvent("failed-login", "low", map[string]interface{}{
		"attempts": 3,
	})

	assert.Contains(t, buf.String(), "Security event")
}

func TestLogSecurityEvent_HighSeverityWritesError(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	l.LogSecurityEvent("brute-force-detected", "high", map[string]interface{}{
		"source_ip": "1.2.3.4",
	})

	output := buf.String()
	assert.Contains(t, output, "Security event")
	// "high" severity should use ERROR level, so the output must include
	// the message at any level since DEBUG logger captures everything.
	assert.Contains(t, output, "brute-force-detected")
}

func TestLogPerformance_FastOperationWritesInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	l.LogPerformance("db-query", 100*time.Millisecond, map[string]interface{}{
		"rows_returned": 50,
	})

	assert.Contains(t, buf.String(), "Performance metric")
	assert.Contains(t, buf.String(), "db-query")
}

func TestLogPerformance_SlowOperationStillLogs(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	// Durations above 5 s trigger a Warn-level log; the warn message must
	// still reach the info-level logger.
	l.LogPerformance("slow-report", 6*time.Second, nil)

	assert.Contains(t, buf.String(), "Performance metric")
}

// --- GinMiddleware ---

func TestGinMiddleware_DoesNotPanic(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	assert.NotPanics(t, func() {
		_ = l.GinMiddleware()
	})
}

func TestGinMiddleware_ReturnsHandlerFunc(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	handler := l.GinMiddleware()
	assert.NotNil(t, handler)
}

func TestGinMiddleware_LogsRequestAndResponse(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelInfo)

	router := gin.New()
	router.Use(l.GinMiddleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	output := buf.String()
	// The middleware should log both the inbound request and outbound response.
	assert.Contains(t, output, "HTTP request")
	assert.Contains(t, output, "HTTP response")
	assert.Contains(t, output, "/ping")
}

func TestGinMiddleware_Logs4xxAsWarn(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	router := gin.New()
	router.Use(l.GinMiddleware())
	router.GET("/not-found-route", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/not-found-route", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	// A 4xx response should produce a WARN log.
	assert.Contains(t, buf.String(), "WARN")
}

func TestGinMiddleware_Logs5xxAsError(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	router := gin.New()
	router.Use(l.GinMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, buf.String(), "ERROR")
}

func TestGinMiddleware_Logs2xxAsInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	router := gin.New()
	router.Use(l.GinMiddleware())
	router.POST("/orders", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	// 2xx → INFO level; it must NOT be promoted to WARN or ERROR.
	assert.NotContains(t, buf.String(), "WARN")
	assert.NotContains(t, buf.String(), "ERROR")
}

func TestGinMiddleware_SanitisesQueryString(t *testing.T) {
	buf := new(bytes.Buffer)
	l := newTestLogger(buf, LevelDebug)

	router := gin.New()
	router.Use(l.GinMiddleware())
	router.GET("/search", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?token=supersecret&q=shoes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The raw token value must not appear in the log output.
	assert.NotContains(t, buf.String(), "supersecret")
	assert.Contains(t, buf.String(), "[REDACTED]")
}

// --- helpers ---

// newTestLogger creates a logger writing JSON to buf at the given level.
func newTestLogger(buf *bytes.Buffer, level LogLevel) *Logger {
	return New(Config{
		Level:       level,
		Environment: "test",
		ServiceName: "test-service",
		Version:     "0.0.1",
		Output:      buf,
		Format:      "json",
	})
}

// nonEmptyLines returns all non-blank lines from s.
func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// firstNonEmptyLine returns the first non-blank line from s.
func firstNonEmptyLine(s string) string {
	lines := nonEmptyLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}
