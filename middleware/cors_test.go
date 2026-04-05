package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Errorf("DefaultCORSConfig().AllowedOrigins = %v, want [*]", config.AllowedOrigins)
	}

	// SECURITY: Wildcard origin should have credentials disabled
	if config.AllowCredentials {
		t.Error("DefaultCORSConfig().AllowCredentials should be false when using wildcard origin")
	}

	expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	if len(config.AllowedMethods) != len(expectedMethods) {
		t.Errorf("DefaultCORSConfig().AllowedMethods = %v, want %v", config.AllowedMethods, expectedMethods)
	}

	if config.MaxAge != 86400 {
		t.Errorf("DefaultCORSConfig().MaxAge = %d, want 86400", config.MaxAge)
	}
}

func TestProductionCORSConfig(t *testing.T) {
	// Clear environment variable for consistent testing
	oldEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")
	defer func() {
		if oldEnv != "" {
			os.Setenv("CORS_ALLOWED_ORIGINS", oldEnv)
		}
	}()

	config := ProductionCORSConfig()

	// Should have specific origins, not wildcard
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			t.Error("ProductionCORSConfig should not use wildcard origin")
		}
	}

	// SECURITY: Credentials should be allowed with specific origins
	if !config.AllowCredentials {
		t.Error("ProductionCORSConfig().AllowCredentials should be true for specific origins")
	}
}

func TestProductionCORSConfigWithEnv(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com, https://api.example.com")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	config := ProductionCORSConfig()

	if len(config.AllowedOrigins) != 2 {
		t.Errorf("ProductionCORSConfig with env should have 2 origins, got %d", len(config.AllowedOrigins))
	}

	if config.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("First origin = %q, want https://example.com", config.AllowedOrigins[0])
	}

	if config.AllowedOrigins[1] != "https://api.example.com" {
		t.Errorf("Second origin = %q, want https://api.example.com", config.AllowedOrigins[1])
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should allow any origin with wildcard
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}

	// SECURITY: Should NOT have credentials header with wildcard
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, should be empty with wildcard origin", got)
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins:   []string{"https://allowed.com", "https://also-allowed.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	router := gin.New()
	router.Use(CORSWithConfig(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Test allowed origin
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://allowed.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://allowed.com", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET"},
	}

	router := gin.New()
	router.Use(CORSWithConfig(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://not-allowed.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should not set Access-Control-Allow-Origin for disallowed origin
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, should be empty for disallowed origin", got)
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Preflight should return 204 No Content
	if w.Code != http.StatusNoContent {
		t.Errorf("Preflight response code = %d, want %d", w.Code, http.StatusNoContent)
	}

	// Should have CORS headers
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods should be set for preflight")
	}
}

func TestCORS_Headers(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Custom-Header"},
		AllowCredentials: false,
		MaxAge:           7200,
	}

	router := gin.New()
	router.Use(CORSWithConfig(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check allowed methods
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") || !strings.Contains(methods, "POST") {
		t.Errorf("Access-Control-Allow-Methods = %q, should contain GET and POST", methods)
	}

	// Check allowed headers
	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") || !strings.Contains(headers, "Authorization") {
		t.Errorf("Access-Control-Allow-Headers = %q, should contain Content-Type and Authorization", headers)
	}

	// Check exposed headers
	exposed := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "X-Custom-Header") {
		t.Errorf("Access-Control-Expose-Headers = %q, should contain X-Custom-Header", exposed)
	}

	// Check max age
	if got := w.Header().Get("Access-Control-Max-Age"); got != "7200" {
		t.Errorf("Access-Control-Max-Age = %q, want 7200", got)
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		expected       bool
	}{
		{
			name:           "exact match",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expected:       true,
		},
		{
			name:           "wildcard",
			origin:         "https://any.com",
			allowedOrigins: []string{"*"},
			expected:       true,
		},
		{
			name:           "multiple origins - match",
			origin:         "https://b.com",
			allowedOrigins: []string{"https://a.com", "https://b.com", "https://c.com"},
			expected:       true,
		},
		{
			name:           "no match",
			origin:         "https://notallowed.com",
			allowedOrigins: []string{"https://allowed.com"},
			expected:       false,
		},
		{
			name:           "empty origin",
			origin:         "",
			allowedOrigins: []string{"https://example.com"},
			expected:       false,
		},
		{
			name:           "empty allowed list",
			origin:         "https://example.com",
			allowedOrigins: []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOriginAllowed(tt.origin, tt.allowedOrigins)
			if got != tt.expected {
				t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowedOrigins, got, tt.expected)
			}
		})
	}
}

func TestDevelopmentCORS(t *testing.T) {
	router := gin.New()
	router.Use(DevelopmentCORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Development CORS should allow any origin
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("DevelopmentCORS Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestProductionCORS(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://prod.example.com")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	router := gin.New()
	router.Use(ProductionCORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Test allowed production origin
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://prod.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://prod.example.com" {
		t.Errorf("ProductionCORS Access-Control-Allow-Origin = %q, want https://prod.example.com", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ProductionCORS Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestEnvironmentAwareCORS_Development(t *testing.T) {
	os.Setenv("ENVIRONMENT", "development")
	defer os.Unsetenv("ENVIRONMENT")

	router := gin.New()
	router.Use(EnvironmentAwareCORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Development should use wildcard
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("EnvironmentAwareCORS (dev) Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestEnvironmentAwareCORS_Production(t *testing.T) {
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("CORS_ALLOWED_ORIGINS")
	}()

	router := gin.New()
	router.Use(EnvironmentAwareCORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Production should use specific origin
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("EnvironmentAwareCORS (prod) Access-Control-Allow-Origin = %q, want https://app.example.com", got)
	}

	// And allow credentials
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("EnvironmentAwareCORS (prod) Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestEnvironmentAwareCORS_GoEnvFallback(t *testing.T) {
	os.Unsetenv("ENVIRONMENT")
	os.Setenv("GO_ENV", "prod")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://fallback.example.com")
	defer func() {
		os.Unsetenv("GO_ENV")
		os.Unsetenv("CORS_ALLOWED_ORIGINS")
	}()

	router := gin.New()
	router.Use(EnvironmentAwareCORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://fallback.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should use GO_ENV as fallback
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://fallback.example.com" {
		t.Errorf("EnvironmentAwareCORS (GO_ENV prod) Access-Control-Allow-Origin = %q, want https://fallback.example.com", got)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Request without Origin header (same-origin request)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should still work and set wildcard
	if w.Code != http.StatusOK {
		t.Errorf("Response code = %d, want %d", w.Code, http.StatusOK)
	}
}

func BenchmarkCORS(b *testing.B) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkCORSPreflight(b *testing.B) {
	router := gin.New()
	router.Use(CORS())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
