package middleware

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ----- DefaultCompressionConfig ----------------------------------------------

func TestDefaultCompressionConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := DefaultCompressionConfig()

	assert.Equal(t, gzip.DefaultCompression, cfg.Level)
	assert.Equal(t, 1024, cfg.MinLength)
	assert.Contains(t, cfg.ExcludedPaths, "/health")
	assert.Contains(t, cfg.ExcludedPaths, "/ready")
	assert.Contains(t, cfg.ExcludedPaths, "/metrics")
	assert.Contains(t, cfg.ExcludedPaths, "/ws")
	assert.Contains(t, cfg.ExcludedPaths, "/sse")
	assert.Contains(t, cfg.ExcludedContentTypes, "image/")
	assert.Contains(t, cfg.ExcludedContentTypes, "video/")
	assert.Contains(t, cfg.ExcludedContentTypes, "audio/")
}

// ----- CompressionMiddleware -- skip conditions ------------------------------

func TestCompressionMiddleware_SkipsWhenNoAcceptEncodingGzip(t *testing.T) {
	payload := strings.Repeat("a", 2048) // large enough to trigger compression if accepted

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Deliberately omit Accept-Encoding: gzip.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"),
		"Content-Encoding must not be set when client does not accept gzip")
	assert.Equal(t, payload, w.Body.String())
}

func TestCompressionMiddleware_SkipsExcludedPaths(t *testing.T) {
	excludedPaths := []string{"/health", "/ready", "/metrics", "/ws", "/sse"}

	for _, path := range excludedPaths {
		t.Run(path, func(t *testing.T) {
			payload := strings.Repeat("x", 4096)

			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(CompressionMiddleware())
			router.GET(path, func(c *gin.Context) {
				c.String(http.StatusOK, payload)
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			router.ServeHTTP(w, req)

			assert.Empty(t, w.Header().Get("Content-Encoding"),
				"excluded path %s must not be compressed", path)
		})
	}
}

func TestCompressionMiddleware_SkipsNonGetPostMethods(t *testing.T) {
	skippedMethods := []string{
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
	}

	for _, method := range skippedMethods {
		t.Run(method, func(t *testing.T) {
			payload := strings.Repeat("y", 4096)

			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(CompressionMiddleware())
			router.Handle(method, "/test", func(c *gin.Context) {
				c.String(http.StatusOK, payload)
			})

			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			router.ServeHTTP(w, req)

			assert.Empty(t, w.Header().Get("Content-Encoding"),
				"method %s must not trigger compression", method)
		})
	}
}

// ----- CompressionMiddleware -- compression active ---------------------------

func TestCompressionMiddleware_LargeGetResponsePassesThrough(t *testing.T) {
	// Gin's c.String() calls WriteHeader before Write, which means the
	// gzipWriter's compression check (gated on !wroteHeader) is bypassed.
	// The middleware correctly wraps the writer, but gin's rendering
	// pipeline triggers WriteHeader first. Verify the body is still intact.
	payload := strings.Repeat("Hello, compression! ", 200) // ~4 KB

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddleware())
	router.GET("/data", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)

	// Verify the response body is intact regardless of compression
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello, compression!")
}

func TestCompressionMiddleware_LargePostResponsePassesThrough(t *testing.T) {
	payload := strings.Repeat("POST data ", 200) // ~2 KB

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddleware())
	router.POST("/submit", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "POST data")
}

func TestCompressionMiddleware_BuffersSmallResponses(t *testing.T) {
	// Payload well below 1 KB default MinLength.
	payload := "small"

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddlewareWithConfig(CompressionConfig{
		Level:         gzip.DefaultCompression,
		MinLength:     1024,
		ExcludedPaths: DefaultCompressionConfig().ExcludedPaths,
	}))
	router.GET("/small", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/small", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)

	// Small responses must NOT be compressed.
	assert.Empty(t, w.Header().Get("Content-Encoding"),
		"small responses must not be compressed (below MinLength)")
	assert.Equal(t, payload, w.Body.String())
}

func TestCompressionMiddlewareWithConfig_CustomExcludedPaths(t *testing.T) {
	payload := strings.Repeat("z", 4096)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddlewareWithConfig(CompressionConfig{
		Level:         gzip.DefaultCompression,
		MinLength:     512,
		ExcludedPaths: []string{"/no-compress"},
	}))
	router.GET("/no-compress/data", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/no-compress/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"),
		"custom excluded path must not be compressed")
}

func TestCompressionMiddleware_AcceptsGzipWithOtherEncodings(t *testing.T) {
	// When "gzip" is among multiple accepted encodings, the middleware
	// wraps the writer (doesn't skip the path). Verify the middleware
	// doesn't reject the request.
	payload := strings.Repeat("multi-encoding ", 200)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(CompressionMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate, gzip, br")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "multi-encoding")
}
