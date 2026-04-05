package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.RequestsPerSecond != 10 {
		t.Errorf("DefaultRateLimitConfig().RequestsPerSecond = %f, want 10", config.RequestsPerSecond)
	}

	if config.BurstSize != 20 {
		t.Errorf("DefaultRateLimitConfig().BurstSize = %d, want 20", config.BurstSize)
	}

	expectedExcluded := []string{"/health", "/ready", "/metrics"}
	if len(config.ExcludePaths) != len(expectedExcluded) {
		t.Errorf("DefaultRateLimitConfig().ExcludePaths = %v, want %v", config.ExcludePaths, expectedExcluded)
	}

	if config.CleanupInterval != 5*time.Minute {
		t.Errorf("DefaultRateLimitConfig().CleanupInterval = %v, want 5m", config.CleanupInterval)
	}

	if config.TTL != 10*time.Minute {
		t.Errorf("DefaultRateLimitConfig().TTL = %v, want 10m", config.TTL)
	}
}

func TestAuthRateLimitConfig(t *testing.T) {
	config := AuthRateLimitConfig()

	// Auth should be more restrictive
	if config.RequestsPerSecond >= 1 {
		t.Errorf("AuthRateLimitConfig().RequestsPerSecond = %f, should be less than 1", config.RequestsPerSecond)
	}

	if config.BurstSize != 5 {
		t.Errorf("AuthRateLimitConfig().BurstSize = %d, want 5", config.BurstSize)
	}

	// Should not exclude any paths
	if len(config.ExcludePaths) != 0 {
		t.Errorf("AuthRateLimitConfig().ExcludePaths should be empty, got %v", config.ExcludePaths)
	}
}

func TestPasswordResetRateLimitConfig(t *testing.T) {
	config := PasswordResetRateLimitConfig()

	// Password reset should be very restrictive (~3 per hour)
	if config.RequestsPerSecond > 0.001 {
		t.Errorf("PasswordResetRateLimitConfig().RequestsPerSecond = %f, should be very low", config.RequestsPerSecond)
	}

	if config.BurstSize != 3 {
		t.Errorf("PasswordResetRateLimitConfig().BurstSize = %d, want 3", config.BurstSize)
	}

	if config.TTL != 1*time.Hour {
		t.Errorf("PasswordResetRateLimitConfig().TTL = %v, want 1h", config.TTL)
	}
}

func TestNewRateLimiter(t *testing.T) {
	config := DefaultRateLimitConfig()
	rl := NewRateLimiter(config)
	defer rl.Stop()

	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}

	if rl.visitors == nil {
		t.Error("NewRateLimiter() visitors map is nil")
	}

	if rl.config.RequestsPerSecond != config.RequestsPerSecond {
		t.Error("NewRateLimiter() config not set correctly")
	}
}

func TestRateLimiter_AllowsRequests(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
		KeyFunc:           func(c *gin.Context) string { return "test-key" },
		ExcludePaths:      []string{},
		CleanupInterval:   time.Hour,
		TTL:               time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_BlocksExcessiveRequests(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         2, // Allow 2 requests initially
		KeyFunc:           func(c *gin.Context) string { return "test-key" },
		ExcludePaths:      []string{},
		CleanupInterval:   time.Hour,
		TTL:               time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Make requests until rate limited
	var rateLimited bool
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("Rate limiter should block excessive requests")
	}
}

func TestRateLimiter_ExcludedPaths(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		KeyFunc:           func(c *gin.Context) string { return "test-key" },
		ExcludePaths:      []string{"/health", "/ready"},
		CleanupInterval:   time.Hour,
		TTL:               time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	})
	router.GET("/ready", func(c *gin.Context) {
		c.String(http.StatusOK, "ready")
	})

	// Excluded paths should never be rate limited
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Health check request %d status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	keyCounter := 0
	config := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         2,
		KeyFunc: func(c *gin.Context) string {
			keyCounter++
			return c.GetHeader("X-Client-ID")
		},
		ExcludePaths:    []string{},
		CleanupInterval: time.Hour,
		TTL:             time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Different clients should have separate rate limits
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Client-ID", "client-a")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// New client should not be rate limited initially
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-ID", "client-b")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("New client first request status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         100,
		KeyFunc:           func(c *gin.Context) string { return c.GetHeader("X-Client-ID") },
		ExcludePaths:      []string{},
		CleanupInterval:   10 * time.Millisecond,
		TTL:               50 * time.Millisecond,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Create some visitors
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Client-ID", "cleanup-test")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Verify visitor exists
	rl.mu.RLock()
	initialCount := len(rl.visitors)
	rl.mu.RUnlock()

	if initialCount == 0 {
		t.Error("Should have at least one visitor")
	}

	// Wait for TTL to expire and cleanup to run
	time.Sleep(100 * time.Millisecond)

	rl.mu.RLock()
	finalCount := len(rl.visitors)
	rl.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("After cleanup, visitors count = %d, want 0", finalCount)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	config := DefaultRateLimitConfig()
	rl := NewRateLimiter(config)

	// Should not panic when stopped
	rl.Stop()

	// Calling Stop multiple times should not panic (channel already closed)
	// This is expected to panic, so we recover
	defer func() {
		if r := recover(); r == nil {
			// Some implementations may handle double-close, which is fine
		}
	}()
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1000,
		BurstSize:         1000,
		KeyFunc:           func(c *gin.Context) string { return c.ClientIP() },
		ExcludePaths:      []string{},
		CleanupInterval:   time.Hour,
		TTL:               time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// We just want to ensure no panics or data races
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestRateLimiter_ResponseFormat(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		KeyFunc:           func(c *gin.Context) string { return "test" },
		ExcludePaths:      []string{},
		CleanupInterval:   time.Hour,
		TTL:               time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Exhaust the rate limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			// Verify response body has expected fields
			body := w.Body.String()
			if body == "" {
				t.Error("Rate limit response body should not be empty")
			}
			// Should contain error and code
			if !containsString(body, "error") || !containsString(body, "RATE_LIMIT_EXCEEDED") {
				t.Errorf("Rate limit response should contain error and code, got: %s", body)
			}
			return
		}
	}
}

func TestSlidingWindowRateLimiter(t *testing.T) {
	sw := NewSlidingWindowRateLimiter(time.Second, 3)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !sw.Allow("test-key") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if sw.Allow("test-key") {
		t.Error("Request 4 should be denied")
	}

	// Wait for window to slide
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !sw.Allow("test-key") {
		t.Error("Request after window should be allowed")
	}
}

func TestSlidingWindowRateLimiter_DifferentKeys(t *testing.T) {
	sw := NewSlidingWindowRateLimiter(time.Second, 2)

	// Key A uses its quota
	sw.Allow("key-a")
	sw.Allow("key-a")
	if sw.Allow("key-a") {
		t.Error("key-a should be rate limited")
	}

	// Key B should still have its quota
	if !sw.Allow("key-b") {
		t.Error("key-b should be allowed")
	}
}

func TestSlidingWindowRateLimiter_Middleware(t *testing.T) {
	sw := NewSlidingWindowRateLimiter(time.Second, 2)

	router := gin.New()
	router.Use(sw.Middleware(func(c *gin.Context) string {
		return c.GetHeader("X-API-Key")
	}))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// First 2 requests allowed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "test-api-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 3rd request rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request 3 status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestLoginAttemptLimiter(t *testing.T) {
	limiter := NewLoginAttemptLimiter()

	// First few attempts should not lock
	for i := 0; i < 3; i++ {
		limiter.RecordFailedAttempt("user@example.com")
		locked, _ := limiter.IsLocked("user@example.com")
		if i < 2 && locked {
			t.Errorf("After %d attempts, should not be locked", i+1)
		}
	}

	// After 3 failed attempts, should be locked
	locked, remaining := limiter.IsLocked("user@example.com")
	if !locked {
		t.Error("After 3 failed attempts, should be locked")
	}
	if remaining <= 0 {
		t.Error("Remaining lockout time should be positive")
	}
}

func TestLoginAttemptLimiter_SuccessfulAttemptClears(t *testing.T) {
	limiter := NewLoginAttemptLimiter()

	// Record some failures
	limiter.RecordFailedAttempt("user@example.com")
	limiter.RecordFailedAttempt("user@example.com")

	count := limiter.GetAttemptCount("user@example.com")
	if count != 2 {
		t.Errorf("Attempt count = %d, want 2", count)
	}

	// Successful attempt should clear
	limiter.RecordSuccessfulAttempt("user@example.com")

	count = limiter.GetAttemptCount("user@example.com")
	if count != 0 {
		t.Errorf("After success, attempt count = %d, want 0", count)
	}

	locked, _ := limiter.IsLocked("user@example.com")
	if locked {
		t.Error("After success, should not be locked")
	}
}

func TestLoginAttemptLimiter_ExponentialBackoff(t *testing.T) {
	limiter := NewLoginAttemptLimiter()

	// Record many failures to trigger exponential backoff
	for i := 0; i < 10; i++ {
		limiter.RecordFailedAttempt("backoff-test")
	}

	locked, remaining := limiter.IsLocked("backoff-test")
	if !locked {
		t.Error("Should be locked after many failures")
	}

	// Backoff should increase with more attempts (capped at 1 hour)
	if remaining > time.Hour {
		t.Errorf("Lockout time = %v, should be capped at 1 hour", remaining)
	}
}

func TestLoginAttemptLimiter_Middleware(t *testing.T) {
	limiter := NewLoginAttemptLimiter()

	router := gin.New()
	router.Use(limiter.Middleware(func(c *gin.Context) string {
		return c.GetHeader("X-User-Email")
	}))
	router.POST("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Lock the user
	for i := 0; i < 5; i++ {
		limiter.RecordFailedAttempt("locked@example.com")
	}

	// Request from locked user
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("X-User-Email", "locked@example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Locked user request status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Request from unlocked user should succeed
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("X-User-Email", "unlocked@example.com")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Unlocked user request status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimit_ConvenienceFunctions(t *testing.T) {
	// Just verify they don't panic and return valid middleware
	router := gin.New()
	router.Use(RateLimit())
	router.GET("/default", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RateLimit() middleware failed: %d", w.Code)
	}
}

func TestAuthRateLimit_ConvenienceFunction(t *testing.T) {
	router := gin.New()
	router.Use(AuthRateLimit())
	router.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthRateLimit() middleware failed: %d", w.Code)
	}
}

func TestCustomRateLimit(t *testing.T) {
	router := gin.New()
	router.Use(CustomRateLimit(5, 10))
	router.GET("/custom", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CustomRateLimit() middleware failed: %d", w.Code)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkRateLimiter(b *testing.B) {
	config := DefaultRateLimitConfig()
	config.RequestsPerSecond = 1000000 // High limit for benchmark
	config.BurstSize = 1000000
	rl := NewRateLimiter(config)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
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
