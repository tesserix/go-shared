package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// RequestsPerSecond is the rate limit (e.g., 1 means 1 request per second)
	RequestsPerSecond float64
	// BurstSize is the maximum burst size
	BurstSize int
	// KeyFunc extracts the rate limit key from the request (defaults to IP)
	KeyFunc func(*gin.Context) string
	// ExcludePaths are paths that bypass rate limiting
	ExcludePaths []string
	// CleanupInterval is how often to clean up old limiters
	CleanupInterval time.Duration
	// TTL is how long to keep a limiter after last use
	TTL time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for general API rate limiting
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         20,
		KeyFunc:           func(c *gin.Context) string { return c.ClientIP() },
		ExcludePaths:      []string{"/health", "/ready", "/metrics"},
		CleanupInterval:   5 * time.Minute,
		TTL:               10 * time.Minute,
	}
}

// AuthRateLimitConfig returns strict config for authentication endpoints
func AuthRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 0.2,  // 1 request per 5 seconds
		BurstSize:         5,    // Allow burst of 5
		KeyFunc:           func(c *gin.Context) string { return c.ClientIP() },
		ExcludePaths:      []string{},
		CleanupInterval:   5 * time.Minute,
		TTL:               15 * time.Minute,
	}
}

// PasswordResetRateLimitConfig returns very strict config for password reset
func PasswordResetRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 0.0008, // ~3 per hour
		BurstSize:         3,
		KeyFunc:           func(c *gin.Context) string { return c.ClientIP() },
		ExcludePaths:      []string{},
		CleanupInterval:   10 * time.Minute,
		TTL:               1 * time.Hour,
	}
}

// visitor holds rate limiter state for a single key
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter implements IP-based rate limiting
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	config   RateLimitConfig
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		config:   config,
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// getVisitor retrieves or creates a rate limiter for the given key
func (rl *RateLimiter) getVisitor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
		rl.visitors[key] = &visitor{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupLoop periodically removes old visitors
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes stale visitors
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.config.TTL)
	for key, v := range rl.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(rl.visitors, key)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// isExcluded checks if the path should bypass rate limiting
func (rl *RateLimiter) isExcluded(path string) bool {
	for _, excluded := range rl.config.ExcludePaths {
		if path == excluded {
			return true
		}
	}
	return false
}

// Middleware returns a Gin middleware for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip excluded paths
		if rl.isExcluded(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get rate limit key
		key := rl.config.KeyFunc(c)
		limiter := rl.getVisitor(key)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"code":        "RATE_LIMIT_EXCEEDED",
				"retry_after": int(1 / rl.config.RequestsPerSecond),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimit creates a rate limiting middleware with default config
func RateLimit() gin.HandlerFunc {
	limiter := NewRateLimiter(DefaultRateLimitConfig())
	return limiter.Middleware()
}

// AuthRateLimit creates strict rate limiting for auth endpoints
func AuthRateLimit() gin.HandlerFunc {
	limiter := NewRateLimiter(AuthRateLimitConfig())
	return limiter.Middleware()
}

// PasswordResetRateLimit creates very strict rate limiting for password reset
func PasswordResetRateLimit() gin.HandlerFunc {
	limiter := NewRateLimiter(PasswordResetRateLimitConfig())
	return limiter.Middleware()
}

// CustomRateLimit creates rate limiting with custom config
func CustomRateLimit(requestsPerSecond float64, burstSize int) gin.HandlerFunc {
	config := DefaultRateLimitConfig()
	config.RequestsPerSecond = requestsPerSecond
	config.BurstSize = burstSize
	limiter := NewRateLimiter(config)
	return limiter.Middleware()
}

// SlidingWindowRateLimiter implements sliding window rate limiting
// More accurate than token bucket for strict rate enforcement
type SlidingWindowRateLimiter struct {
	requests   map[string][]time.Time
	mu         sync.RWMutex
	windowSize time.Duration
	maxRequests int
}

// NewSlidingWindowRateLimiter creates a sliding window rate limiter
func NewSlidingWindowRateLimiter(windowSize time.Duration, maxRequests int) *SlidingWindowRateLimiter {
	return &SlidingWindowRateLimiter{
		requests:    make(map[string][]time.Time),
		windowSize:  windowSize,
		maxRequests: maxRequests,
	}
}

// Allow checks if a request is allowed
func (sw *SlidingWindowRateLimiter) Allow(key string) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.windowSize)

	// Get existing requests
	reqs, exists := sw.requests[key]
	if !exists {
		sw.requests[key] = []time.Time{now}
		return true
	}

	// Filter to only requests in current window
	var validReqs []time.Time
	for _, t := range reqs {
		if t.After(windowStart) {
			validReqs = append(validReqs, t)
		}
	}

	// Check if under limit
	if len(validReqs) >= sw.maxRequests {
		sw.requests[key] = validReqs
		return false
	}

	// Add current request
	validReqs = append(validReqs, now)
	sw.requests[key] = validReqs
	return true
}

// Middleware returns a Gin middleware for sliding window rate limiting
func (sw *SlidingWindowRateLimiter) Middleware(keyFunc func(*gin.Context) string) gin.HandlerFunc {
	if keyFunc == nil {
		keyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}

	return func(c *gin.Context) {
		key := keyFunc(c)
		if !sw.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"code":        "RATE_LIMIT_EXCEEDED",
				"retry_after": int(sw.windowSize.Seconds()),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// LoginAttemptLimiter tracks failed login attempts with exponential backoff
type LoginAttemptLimiter struct {
	attempts map[string]*loginAttempt
	mu       sync.RWMutex
}

type loginAttempt struct {
	count      int
	lastFailed time.Time
	lockedUntil time.Time
}

// NewLoginAttemptLimiter creates a new login attempt limiter
func NewLoginAttemptLimiter() *LoginAttemptLimiter {
	return &LoginAttemptLimiter{
		attempts: make(map[string]*loginAttempt),
	}
}

// RecordFailedAttempt records a failed login attempt
func (l *LoginAttemptLimiter) RecordFailedAttempt(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.attempts[key]
	if !exists {
		l.attempts[key] = &loginAttempt{
			count:      1,
			lastFailed: time.Now(),
		}
		return
	}

	attempt.count++
	attempt.lastFailed = time.Now()

	// Exponential backoff: 2^(attempts-3) seconds, capped at 1 hour
	if attempt.count >= 3 {
		backoff := time.Duration(1<<(attempt.count-3)) * time.Second
		if backoff > time.Hour {
			backoff = time.Hour
		}
		attempt.lockedUntil = time.Now().Add(backoff)
	}
}

// RecordSuccessfulAttempt clears failed attempts on successful login
func (l *LoginAttemptLimiter) RecordSuccessfulAttempt(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// IsLocked checks if the key is currently locked out
func (l *LoginAttemptLimiter) IsLocked(key string) (bool, time.Duration) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[key]
	if !exists {
		return false, 0
	}

	if attempt.lockedUntil.After(time.Now()) {
		return true, time.Until(attempt.lockedUntil)
	}

	return false, 0
}

// GetAttemptCount returns the number of failed attempts
func (l *LoginAttemptLimiter) GetAttemptCount(key string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if attempt, exists := l.attempts[key]; exists {
		return attempt.count
	}
	return 0
}

// Middleware returns a Gin middleware for login attempt limiting
func (l *LoginAttemptLimiter) Middleware(keyFunc func(*gin.Context) string) gin.HandlerFunc {
	if keyFunc == nil {
		keyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}

	return func(c *gin.Context) {
		key := keyFunc(c)
		locked, remaining := l.IsLocked(key)
		if locked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Account temporarily locked due to too many failed attempts",
				"code":        "ACCOUNT_LOCKED",
				"retry_after": int(remaining.Seconds()),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
