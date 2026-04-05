package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// uuidRegexp matches the standard UUID v4 format.
var uuidRegexp = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestRequestIDMiddleware_GeneratesUUID(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)
	_ = c

	router.Use(RequestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	got := w.Header().Get("X-Request-ID")
	require.NotEmpty(t, got, "X-Request-ID response header must be set when no incoming header is present")
	assert.True(t, uuidRegexp.MatchString(got), "generated request ID %q must be a valid UUID v4", got)
}

func TestRequestIDMiddleware_UsesExistingHeader(t *testing.T) {
	const existingID = "my-existing-request-id-123"

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(RequestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	router.ServeHTTP(w, req)

	assert.Equal(t, existingID, w.Header().Get("X-Request-ID"),
		"middleware must echo back the incoming X-Request-ID header unchanged")
}

func TestRequestIDMiddleware_SetsContextValues(t *testing.T) {
	var capturedRequestID, capturedTraceID interface{}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(RequestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		capturedRequestID, _ = ctx.Get("request_id")
		capturedTraceID, _ = ctx.Get("trace_id")
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	require.NotNil(t, capturedRequestID, "request_id must be set in gin context")
	require.NotNil(t, capturedTraceID, "trace_id must be set in gin context")
	assert.Equal(t, capturedRequestID, capturedTraceID,
		"request_id and trace_id must be identical")
}

func TestRequestIDMiddleware_SetsResponseHeader(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(RequestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"),
		"X-Request-ID must be present in the response headers")
}

func TestRequestIDMiddleware_GeneratedIDIsValidUUID(t *testing.T) {
	tests := []struct {
		name       string
		incomingID string
		wantUUID   bool
	}{
		{
			name:       "no incoming header produces UUID",
			incomingID: "",
			wantUUID:   true,
		},
		{
			name:       "non-UUID incoming header is passed through unchanged",
			incomingID: "custom-trace-abc",
			wantUUID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)

			router.Use(RequestIDMiddleware())
			router.GET("/test", func(ctx *gin.Context) {
				ctx.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.incomingID != "" {
				req.Header.Set("X-Request-ID", tt.incomingID)
			}
			router.ServeHTTP(w, req)

			got := w.Header().Get("X-Request-ID")
			require.NotEmpty(t, got)
			if tt.wantUUID {
				assert.True(t, uuidRegexp.MatchString(got),
					"generated ID %q must match UUID v4 format", got)
			} else {
				assert.Equal(t, tt.incomingID, got)
			}
		})
	}
}

func TestRequestIDMiddleware_ContextIDMatchesResponseHeader(t *testing.T) {
	var contextID interface{}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(RequestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		contextID, _ = ctx.Get("request_id")
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, contextID, w.Header().Get("X-Request-ID"),
		"the request_id stored in context must match the X-Request-ID response header")
}
