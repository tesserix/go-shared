package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RedisRateLimitConfig holds Redis-based rate limiting configuration
type RedisRateLimitConfig struct {
	// RequestsPerSecond is the maximum requests allowed per second
	RequestsPerSecond int

	// BurstSize is the maximum burst size (token bucket capacity)
	BurstSize int

	// WindowDuration is the sliding window duration
	WindowDuration time.Duration

	// KeyPrefix is the prefix for Redis keys
	KeyPrefix string

	// ExcludedPaths are paths that bypass rate limiting
	ExcludedPaths []string

	// ByTenant enables per-tenant rate limiting
	ByTenant bool

	// ByIP enables per-IP rate limiting
	ByIP bool

	// ByUser enables per-user rate limiting
	ByUser bool
}

// DefaultRedisRateLimitConfig returns default Redis rate limiting configuration
func DefaultRedisRateLimitConfig() RedisRateLimitConfig {
	return RedisRateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         200,
		WindowDuration:    time.Second,
		KeyPrefix:         "ratelimit:",
		ExcludedPaths: []string{
			"/health",
			"/ready",
			"/metrics",
		},
		ByTenant: true,
		ByIP:     true,
		ByUser:   false,
	}
}

// RedisRateLimitProfiles for different service types
var RedisRateLimitProfiles = map[string]RedisRateLimitConfig{
	// High-volume APIs (products, categories)
	"high_volume": {
		RequestsPerSecond: 500,
		BurstSize:         1000,
		WindowDuration:    time.Second,
		KeyPrefix:         "ratelimit:hv:",
		ByTenant:          true,
		ByIP:              true,
	},
	// Standard APIs (orders, customers)
	"standard": {
		RequestsPerSecond: 100,
		BurstSize:         200,
		WindowDuration:    time.Second,
		KeyPrefix:         "ratelimit:std:",
		ByTenant:          true,
		ByIP:              true,
	},
	// Sensitive APIs (auth, payments)
	"sensitive": {
		RequestsPerSecond: 20,
		BurstSize:         50,
		WindowDuration:    time.Second,
		KeyPrefix:         "ratelimit:sens:",
		ByTenant:          true,
		ByIP:              true,
		ByUser:            true,
	},
	// Webhooks (high burst for event processing)
	"webhook": {
		RequestsPerSecond: 1000,
		BurstSize:         2000,
		WindowDuration:    time.Second,
		KeyPrefix:         "ratelimit:wh:",
		ByTenant:          true,
		ByIP:              false,
	},
}

// RedisRateLimiter provides distributed rate limiting using Redis
type RedisRateLimiter struct {
	client *redis.Client
	config RedisRateLimitConfig
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client, config RedisRateLimitConfig) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		config: config,
	}
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
	RetryAfter time.Duration
}

// slidingWindowLua is a Lua script for atomic sliding window rate limiting
// This ensures correct behavior even under high concurrency
const slidingWindowLua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

-- Remove old entries outside the window
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

-- Count current requests in window
local current = redis.call('ZCARD', key)

if current < limit then
    -- Add new request with current timestamp as score
    redis.call('ZADD', key, now, now .. '-' .. math.random())
    redis.call('EXPIRE', key, math.ceil(window / 1000))
    return {1, limit - current - 1, 0}
else
    -- Get the oldest entry to calculate retry time
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retryAfter = 0
    if #oldest > 0 then
        retryAfter = oldest[2] + window - now
    end
    return {0, 0, retryAfter}
end
`

// Check performs a rate limit check and returns the result
func (r *RedisRateLimiter) Check(ctx context.Context, identifier string) (*RateLimitResult, error) {
	key := r.config.KeyPrefix + identifier
	now := time.Now().UnixMilli()
	windowMs := r.config.WindowDuration.Milliseconds()

	result, err := r.client.Eval(ctx, slidingWindowLua, []string{key}, now, windowMs, r.config.RequestsPerSecond).Result()
	if err != nil {
		// On Redis error, allow the request (fail-open for availability)
		return &RateLimitResult{
			Allowed:   true,
			Limit:     r.config.RequestsPerSecond,
			Remaining: r.config.RequestsPerSecond,
			ResetAt:   time.Now().Add(r.config.WindowDuration),
		}, nil
	}

	// Parse Lua result
	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := int(res[1].(int64))
	retryAfterMs := res[2].(int64)

	return &RateLimitResult{
		Allowed:    allowed,
		Limit:      r.config.RequestsPerSecond,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(r.config.WindowDuration),
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
	}, nil
}

// buildIdentifier creates a rate limit key identifier
func (r *RedisRateLimiter) buildIdentifier(c *gin.Context) string {
	parts := make([]string, 0, 3)

	if r.config.ByTenant {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = c.GetHeader("X-Vendor-ID")
		}
		if tenantID != "" {
			parts = append(parts, "t:"+tenantID)
		}
	}

	if r.config.ByUser {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			parts = append(parts, "u:"+userID)
		}
	}

	if r.config.ByIP {
		ip := getClientIP(c)
		if ip != "" {
			parts = append(parts, "ip:"+ip)
		}
	}

	if len(parts) == 0 {
		return "global"
	}

	identifier := ""
	for i, part := range parts {
		if i > 0 {
			identifier += ":"
		}
		identifier += part
	}
	return identifier
}

// getClientIP extracts the client IP from various headers
func getClientIP(c *gin.Context) string {
	xff := c.GetHeader("X-Forwarded-For")
	xri := c.GetHeader("X-Real-IP")
	xea := c.GetHeader("X-Envoy-External-Address")
	cfip := c.GetHeader("CF-Connecting-IP")
	xrci := c.GetHeader("X-Real-Client-IP")
	ginIP := c.ClientIP()

	// Check X-Real-Client-IP first (set by our Lua EnvoyFilter at gateway)
	if xrci != "" {
		// Remove port if present (e.g., "1.2.3.4:12345" -> "1.2.3.4")
		if idx := strings.LastIndex(xrci, ":"); idx > 0 {
			// Check if it's IPv6 (contains multiple colons) or IPv4 with port
			if strings.Count(xrci, ":") == 1 {
				xrci = xrci[:idx]
			}
		}
		return strings.TrimSpace(xrci)
	}

	// Check various headers in order of preference
	// X-Forwarded-For may contain multiple IPs: "client, proxy1, proxy2" - take the first
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Check other common proxy headers
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	if xea != "" {
		return strings.TrimSpace(xea)
	}
	if cfip != "" {
		return strings.TrimSpace(cfip)
	}

	return ginIP
}

// isExcludedPath checks if the path should bypass rate limiting
func (r *RedisRateLimiter) isExcludedPath(path string) bool {
	for _, excluded := range r.config.ExcludedPaths {
		if path == excluded || (len(excluded) > 0 && path[:min(len(path), len(excluded))] == excluded) {
			return true
		}
	}
	return false
}

// Middleware returns a Gin middleware for rate limiting
func (r *RedisRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip excluded paths
		if r.isExcludedPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Build identifier and check rate limit
		identifier := r.buildIdentifier(c)
		result, err := r.Check(c.Request.Context(), identifier)
		if err != nil {
			// Log error but allow request (fail-open)
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please try again later.",
					"details": gin.H{
						"retry_after_seconds": int(result.RetryAfter.Seconds()) + 1,
						"limit":               result.Limit,
					},
				},
			})
			return
		}

		c.Next()
	}
}

// RedisRateLimitMiddleware creates a rate limiting middleware with default configuration
func RedisRateLimitMiddleware(client *redis.Client) gin.HandlerFunc {
	limiter := NewRedisRateLimiter(client, DefaultRedisRateLimitConfig())
	return limiter.Middleware()
}

// RedisRateLimitMiddlewareWithConfig creates a rate limiting middleware with custom configuration
func RedisRateLimitMiddlewareWithConfig(client *redis.Client, config RedisRateLimitConfig) gin.HandlerFunc {
	limiter := NewRedisRateLimiter(client, config)
	return limiter.Middleware()
}

// RedisRateLimitMiddlewareWithProfile creates a rate limiting middleware with a predefined profile
func RedisRateLimitMiddlewareWithProfile(client *redis.Client, profile string) gin.HandlerFunc {
	config, ok := RedisRateLimitProfiles[profile]
	if !ok {
		config = DefaultRedisRateLimitConfig()
	}
	limiter := NewRedisRateLimiter(client, config)
	return limiter.Middleware()
}

// TenantQuota represents per-tenant resource quotas
type TenantQuota struct {
	MaxRequestsPerMinute  int   `json:"maxRequestsPerMinute"`
	MaxStorageBytes       int64 `json:"maxStorageBytes"`
	MaxProducts           int   `json:"maxProducts"`
	MaxOrders             int   `json:"maxOrders"`
	MaxConcurrentRequests int   `json:"maxConcurrentRequests"`
}

// DefaultTenantQuotas for different tenant tiers
var DefaultTenantQuotas = map[string]TenantQuota{
	"free": {
		MaxRequestsPerMinute:  100,
		MaxStorageBytes:       100 * 1024 * 1024, // 100 MB
		MaxProducts:           100,
		MaxOrders:             1000,
		MaxConcurrentRequests: 10,
	},
	"starter": {
		MaxRequestsPerMinute:  1000,
		MaxStorageBytes:       1024 * 1024 * 1024, // 1 GB
		MaxProducts:           1000,
		MaxOrders:             10000,
		MaxConcurrentRequests: 50,
	},
	"business": {
		MaxRequestsPerMinute:  10000,
		MaxStorageBytes:       10 * 1024 * 1024 * 1024, // 10 GB
		MaxProducts:           10000,
		MaxOrders:             100000,
		MaxConcurrentRequests: 200,
	},
	"enterprise": {
		MaxRequestsPerMinute:  100000,
		MaxStorageBytes:       100 * 1024 * 1024 * 1024, // 100 GB
		MaxProducts:           -1,                        // Unlimited
		MaxOrders:             -1,                        // Unlimited
		MaxConcurrentRequests: 1000,
	},
}

// QuotaMiddleware provides per-tenant quota enforcement
type QuotaMiddleware struct {
	client *redis.Client
	quotas map[string]TenantQuota
}

// NewQuotaMiddleware creates a new quota middleware
func NewQuotaMiddleware(client *redis.Client, quotas map[string]TenantQuota) *QuotaMiddleware {
	if quotas == nil {
		quotas = DefaultTenantQuotas
	}
	return &QuotaMiddleware{
		client: client,
		quotas: quotas,
	}
}

// CheckQuota checks if a tenant has exceeded their quota
func (q *QuotaMiddleware) CheckQuota(ctx context.Context, tenantID, tier, resource string, amount int64) (bool, int64, error) {
	quota, ok := q.quotas[tier]
	if !ok {
		quota = q.quotas["free"]
	}

	key := fmt.Sprintf("quota:%s:%s", tenantID, resource)

	// Get current usage
	current, err := q.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return true, 0, nil // Fail-open
	}

	var limit int64
	switch resource {
	case "products":
		limit = int64(quota.MaxProducts)
	case "orders":
		limit = int64(quota.MaxOrders)
	case "storage":
		limit = quota.MaxStorageBytes
	default:
		return true, 0, nil
	}

	// -1 means unlimited
	if limit == -1 {
		return true, -1, nil
	}

	remaining := limit - current - amount
	if remaining < 0 {
		return false, 0, nil
	}

	return true, remaining, nil
}

// Middleware returns a Gin middleware for quota enforcement
func (q *QuotaMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add quota headers for visibility
		tenantID := c.GetHeader("X-Tenant-ID")
		tier := c.GetHeader("X-Tenant-Tier")

		if tenantID != "" && tier != "" {
			quota, ok := q.quotas[tier]
			if ok {
				c.Header("X-Quota-Products-Limit", strconv.Itoa(quota.MaxProducts))
				c.Header("X-Quota-Orders-Limit", strconv.Itoa(quota.MaxOrders))
				c.Header("X-Quota-Requests-Limit", strconv.Itoa(quota.MaxRequestsPerMinute))
			}
		}

		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
