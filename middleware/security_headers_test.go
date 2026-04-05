package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	config := DefaultSecurityHeadersConfig()

	if config.FrameOptions != "DENY" {
		t.Errorf("DefaultSecurityHeadersConfig().FrameOptions = %q, want %q", config.FrameOptions, "DENY")
	}

	if config.HSTSMaxAge != 31536000 {
		t.Errorf("DefaultSecurityHeadersConfig().HSTSMaxAge = %d, want %d", config.HSTSMaxAge, 31536000)
	}

	if !config.HSTSIncludeSubdomains {
		t.Error("DefaultSecurityHeadersConfig().HSTSIncludeSubdomains should be true")
	}

	if !config.HSTSPreload {
		t.Error("DefaultSecurityHeadersConfig().HSTSPreload should be true")
	}

	if config.ReferrerPolicy != "strict-origin-when-cross-origin" {
		t.Errorf("DefaultSecurityHeadersConfig().ReferrerPolicy = %q, want %q", config.ReferrerPolicy, "strict-origin-when-cross-origin")
	}

	if config.CrossOriginOpenerPolicy != "same-origin" {
		t.Errorf("DefaultSecurityHeadersConfig().CrossOriginOpenerPolicy = %q, want %q", config.CrossOriginOpenerPolicy, "same-origin")
	}

	if config.CrossOriginResourcePolicy != "same-origin" {
		t.Errorf("DefaultSecurityHeadersConfig().CrossOriginResourcePolicy = %q, want %q", config.CrossOriginResourcePolicy, "same-origin")
	}
}

func TestAPISecurityHeadersConfig(t *testing.T) {
	config := APISecurityHeadersConfig()

	if config.ContentSecurityPolicy != "default-src 'none'" {
		t.Errorf("APISecurityHeadersConfig().ContentSecurityPolicy = %q, want %q", config.ContentSecurityPolicy, "default-src 'none'")
	}

	if config.ReferrerPolicy != "no-referrer" {
		t.Errorf("APISecurityHeadersConfig().ReferrerPolicy = %q, want %q", config.ReferrerPolicy, "no-referrer")
	}

	// API should not preload HSTS by default
	if config.HSTSPreload {
		t.Error("APISecurityHeadersConfig().HSTSPreload should be false")
	}
}

func TestSecurityHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check required security headers
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "1; mode=block",
		"X-Frame-Options":        "DENY",
		"Content-Security-Policy": "default-src 'none'",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store, no-cache, must-revalidate, proxy-revalidate",
		"Pragma":                 "no-cache",
		"Expires":                "0",
	}

	for header, expected := range headers {
		got := w.Header().Get(header)
		if got != expected {
			t.Errorf("Header %q = %q, want %q", header, got, expected)
		}
	}
}

func TestSecurityHeadersWithConfig(t *testing.T) {
	config := SecurityHeadersConfig{
		ContentSecurityPolicy:     "default-src 'self'",
		FrameOptions:             "SAMEORIGIN",
		HSTSMaxAge:               3600,
		HSTSIncludeSubdomains:    false,
		HSTSPreload:              false,
		ReferrerPolicy:           "origin",
		PermissionsPolicy:        "camera=()",
		CrossOriginOpenerPolicy:  "unsafe-none",
		CrossOriginResourcePolicy: "cross-origin",
		CrossOriginEmbedderPolicy: "unsafe-none",
	}

	router := gin.New()
	router.Use(SecurityHeadersWithConfig(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check custom headers
	if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("CSP = %q, want %q", got, "default-src 'self'")
	}

	if got := w.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want %q", got, "SAMEORIGIN")
	}

	if got := w.Header().Get("Referrer-Policy"); got != "origin" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "origin")
	}

	if got := w.Header().Get("Permissions-Policy"); got != "camera=()" {
		t.Errorf("Permissions-Policy = %q, want %q", got, "camera=()")
	}

	if got := w.Header().Get("Cross-Origin-Opener-Policy"); got != "unsafe-none" {
		t.Errorf("Cross-Origin-Opener-Policy = %q, want %q", got, "unsafe-none")
	}

	if got := w.Header().Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
		t.Errorf("Cross-Origin-Resource-Policy = %q, want %q", got, "cross-origin")
	}

	if got := w.Header().Get("Cross-Origin-Embedder-Policy"); got != "unsafe-none" {
		t.Errorf("Cross-Origin-Embedder-Policy = %q, want %q", got, "unsafe-none")
	}

	// Check HSTS with custom max-age
	hsts := w.Header().Get("Strict-Transport-Security")
	if !strings.HasPrefix(hsts, "max-age=3600") {
		t.Errorf("HSTS = %q, should start with max-age=3600", hsts)
	}
	if strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("HSTS = %q, should NOT contain includeSubDomains", hsts)
	}
	if strings.Contains(hsts, "preload") {
		t.Errorf("HSTS = %q, should NOT contain preload", hsts)
	}
}

func TestSecurityHeadersHSTS(t *testing.T) {
	tests := []struct {
		name                  string
		maxAge                int
		includeSubdomains     bool
		preload               bool
		expectedHSTSContains  []string
		expectedHSTSNotContains []string
	}{
		{
			name:                 "full HSTS",
			maxAge:               31536000,
			includeSubdomains:    true,
			preload:              true,
			expectedHSTSContains: []string{"max-age=31536000", "includeSubDomains", "preload"},
		},
		{
			name:                    "HSTS without subdomains",
			maxAge:                  86400,
			includeSubdomains:       false,
			preload:                 false,
			expectedHSTSContains:    []string{"max-age=86400"},
			expectedHSTSNotContains: []string{"includeSubDomains", "preload"},
		},
		{
			name:                    "HSTS disabled",
			maxAge:                  0,
			includeSubdomains:       true,
			preload:                 true,
			expectedHSTSContains:    []string{},
			expectedHSTSNotContains: []string{"max-age"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SecurityHeadersConfig{
				HSTSMaxAge:            tt.maxAge,
				HSTSIncludeSubdomains: tt.includeSubdomains,
				HSTSPreload:           tt.preload,
			}

			router := gin.New()
			router.Use(SecurityHeadersWithConfig(config))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			hsts := w.Header().Get("Strict-Transport-Security")

			for _, expected := range tt.expectedHSTSContains {
				if !strings.Contains(hsts, expected) {
					t.Errorf("HSTS = %q, should contain %q", hsts, expected)
				}
			}

			for _, notExpected := range tt.expectedHSTSNotContains {
				if hsts != "" && strings.Contains(hsts, notExpected) {
					t.Errorf("HSTS = %q, should NOT contain %q", hsts, notExpected)
				}
			}
		})
	}
}

func TestSensitiveEndpointHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SensitiveEndpointHeaders())
	router.GET("/sensitive", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/sensitive", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check sensitive endpoint headers
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") || !strings.Contains(got, "private") {
		t.Errorf("Cache-Control = %q, should contain no-store and private", got)
	}

	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}

	if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'none'" {
		t.Errorf("CSP = %q, want default-src 'none'", got)
	}
}

func TestNoCacheHeaders(t *testing.T) {
	router := gin.New()
	router.Use(NoCacheHeaders())
	router.GET("/nocache", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/nocache", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	requiredDirectives := []string{"no-store", "no-cache", "must-revalidate", "max-age=0"}
	for _, directive := range requiredDirectives {
		if !strings.Contains(cacheControl, directive) {
			t.Errorf("Cache-Control = %q, should contain %q", cacheControl, directive)
		}
	}

	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want no-cache", got)
	}

	if got := w.Header().Get("Expires"); got != "0" {
		t.Errorf("Expires = %q, want 0", got)
	}

	if got := w.Header().Get("Surrogate-Control"); got != "no-store" {
		t.Errorf("Surrogate-Control = %q, want no-store", got)
	}
}

func TestRemoveServerHeader(t *testing.T) {
	router := gin.New()
	router.Use(RemoveServerHeader())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Server"); got != "" {
		t.Errorf("Server header = %q, want empty", got)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{12345, "12345"},
		{31536000, "31536000"},
		{-1, "-1"},
		{-100, "-100"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.expected {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSecurityHeadersChaining(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.Use(SensitiveEndpointHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should have combined headers from both middlewares
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}

	// SensitiveEndpointHeaders should override Cache-Control
	cacheControl := w.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "private") {
		t.Errorf("Cache-Control = %q, should contain 'private' from SensitiveEndpointHeaders", cacheControl)
	}
}

func BenchmarkSecurityHeaders(b *testing.B) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
