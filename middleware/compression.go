package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// CompressionConfig holds configuration for compression middleware
type CompressionConfig struct {
	// Level is the compression level (gzip.BestSpeed to gzip.BestCompression)
	// Default: gzip.DefaultCompression
	Level int

	// MinLength is the minimum response size to trigger compression
	// Default: 1024 bytes (1KB)
	MinLength int

	// ExcludedPaths are paths that should not be compressed
	// Default: ["/health", "/ready", "/metrics"]
	ExcludedPaths []string

	// ExcludedContentTypes are content types that should not be compressed
	// Default: ["image/", "video/", "audio/"]
	ExcludedContentTypes []string
}

// DefaultCompressionConfig returns the default compression configuration
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Level:     gzip.DefaultCompression,
		MinLength: 1024,
		ExcludedPaths: []string{
			"/health",
			"/ready",
			"/metrics",
			"/ws",
			"/sse",
		},
		ExcludedContentTypes: []string{
			"image/",
			"video/",
			"audio/",
			"application/octet-stream",
		},
	}
}

// gzipWriter wraps gin.ResponseWriter to provide gzip compression
type gzipWriter struct {
	gin.ResponseWriter
	writer      *gzip.Writer
	minLength   int
	buffer      []byte
	wroteHeader bool
	compressed  bool
}

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

func (g *gzipWriter) WriteHeader(code int) {
	g.wroteHeader = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	if !g.wroteHeader {
		// Check content length and decide whether to compress
		contentType := g.ResponseWriter.Header().Get("Content-Type")

		// If content type is not set, try to detect it
		if contentType == "" && len(data) > 0 {
			contentType = http.DetectContentType(data)
			g.ResponseWriter.Header().Set("Content-Type", contentType)
		}

		// Check if we should compress based on content type
		shouldCompress := true
		for _, excluded := range []string{"image/", "video/", "audio/"} {
			if strings.HasPrefix(contentType, excluded) {
				shouldCompress = false
				break
			}
		}

		if shouldCompress && len(g.buffer)+len(data) >= g.minLength {
			g.compressed = true
			g.ResponseWriter.Header().Set("Content-Encoding", "gzip")
			g.ResponseWriter.Header().Set("Vary", "Accept-Encoding")
			g.ResponseWriter.Header().Del("Content-Length")

			// Flush buffered data first
			if len(g.buffer) > 0 {
				g.writer.Write(g.buffer)
				g.buffer = nil
			}
			g.WriteHeader(http.StatusOK)
			return g.writer.Write(data)
		}

		// Buffer small responses
		g.buffer = append(g.buffer, data...)
		return len(data), nil
	}

	if g.compressed {
		return g.writer.Write(data)
	}
	return g.ResponseWriter.Write(data)
}

func (g *gzipWriter) Flush() {
	if g.compressed && g.writer != nil {
		g.writer.Flush()
	}
	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (g *gzipWriter) Close() error {
	// Flush any remaining buffered data
	if !g.wroteHeader && len(g.buffer) > 0 {
		g.ResponseWriter.Write(g.buffer)
	}
	if g.compressed && g.writer != nil {
		return g.writer.Close()
	}
	return nil
}

// CompressionMiddleware returns a gzip compression middleware with default config
// Performance: Reduces response size by 60-80% for JSON/HTML responses
func CompressionMiddleware() gin.HandlerFunc {
	return CompressionMiddlewareWithConfig(DefaultCompressionConfig())
}

// CompressionMiddlewareWithConfig returns a gzip compression middleware with custom config
func CompressionMiddlewareWithConfig(config CompressionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if client doesn't accept gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Skip excluded paths
		path := c.Request.URL.Path
		for _, excludedPath := range config.ExcludedPaths {
			if strings.HasPrefix(path, excludedPath) {
				c.Next()
				return
			}
		}

		// Skip for non-GET/POST requests (usually small responses)
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		// Get gzip writer from pool
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		// Reset writer with response writer
		gz.Reset(c.Writer)

		gw := &gzipWriter{
			ResponseWriter: c.Writer,
			writer:         gz,
			minLength:      config.MinLength,
			buffer:         make([]byte, 0, config.MinLength),
		}

		c.Writer = gw
		defer func() {
			gw.Close()
		}()

		c.Next()
	}
}

// BrotliCompressionMiddleware placeholder for future Brotli support
// Brotli provides ~20% better compression than gzip but requires more CPU
// Consider implementing when Brotli becomes more widely supported in Go
func BrotliCompressionMiddleware() gin.HandlerFunc {
	// For now, fall back to gzip
	return CompressionMiddleware()
}
