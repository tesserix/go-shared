package middleware

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds CORS configuration options
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns a default CORS configuration
// SECURITY: When using wildcard origin (*), AllowCredentials MUST be false
// per CORS specification. Use ProductionCORSConfig() or EnvironmentAwareCORS()
// for production with specific origins and credentials support.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders: []string{
			"Content-Type",
			"Authorization",
			"X-Tenant-ID",
			"X-User-ID",
			"X-Request-ID",
			"x-user-id",
			"x-tenant-id",
			"Accept",
			"Accept-Language",
			"Content-Language",
			"Origin",
		},
		ExposedHeaders: []string{
			"Content-Length",
			"X-Request-ID",
			"X-Total-Count",
		},
		// SECURITY: Must be false when AllowedOrigins contains "*"
		// Wildcard + credentials is invalid per CORS spec
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// ProductionCORSConfig returns a secure CORS configuration for production
// SECURITY: Uses specific origins (not wildcard) so credentials are allowed
func ProductionCORSConfig() CORSConfig {
	allowedOrigins := []string{
		// Tesserix platform domains
		"https://tesserix.app",
		"https://admin.tesserix.app",
		"https://onboarding.tesserix.app",
		// Mark8ly marketplace domains
		"https://mark8ly.com",
		"https://www.mark8ly.com",
		"https://admin.mark8ly.com",
		"https://onboarding.mark8ly.com",
		// Wildcard for tenant storefronts (e.g., mystore.mark8ly.com)
		"https://*.mark8ly.com",
	}

	// Allow environment-specific origins
	if envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); envOrigins != "" {
		allowedOrigins = strings.Split(envOrigins, ",")
		for i, origin := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(origin)
		}
	}

	config := DefaultCORSConfig()
	config.AllowedOrigins = allowedOrigins
	// SECURITY: Credentials allowed because we're using specific origins, not wildcard
	config.AllowCredentials = true
	return config
}

// CORS creates a CORS middleware with default configuration
func CORS() gin.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig creates a CORS middleware with custom configuration
// SECURITY: Enforces CORS spec - credentials header is NOT sent when using wildcard origin
func CORSWithConfig(config CORSConfig) gin.HandlerFunc {
	// Check if using wildcard origin
	isWildcard := len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Set Access-Control-Allow-Origin
		if isWildcard {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if isOriginAllowed(origin, config.AllowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// Set other CORS headers
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))

		if len(config.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
		}

		// SECURITY: Only set credentials header if NOT using wildcard origin
		// Per CORS spec, wildcard + credentials is invalid
		if config.AllowCredentials && !isWildcard {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}

		// Handle preflight OPTIONS request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the origin is in the allowed list
// Supports wildcard subdomains (e.g., "https://*.mark8ly.com")
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Support wildcard subdomain matching: https://*.example.com
		if strings.HasPrefix(allowed, "https://*.") {
			suffix := allowed[len("https://*"):]  // e.g., ".mark8ly.com"
			if strings.HasPrefix(origin, "https://") && strings.HasSuffix(origin, suffix) {
				// Ensure there's a subdomain (not just the suffix itself)
				subdomain := origin[len("https://"):len(origin)-len(suffix)]
				if subdomain != "" && !strings.Contains(subdomain, "/") {
					return true
				}
			}
		}
	}
	return false
}

// DevelopmentCORS returns a permissive CORS middleware for development
func DevelopmentCORS() gin.HandlerFunc {
	return CORS()
}

// ProductionCORS returns a secure CORS middleware for production
func ProductionCORS() gin.HandlerFunc {
	return CORSWithConfig(ProductionCORSConfig())
}

// EnvironmentAwareCORS returns appropriate CORS middleware based on ENVIRONMENT env var
// SECURITY: Recommended for production services - automatically uses secure settings
// - Development (ENVIRONMENT=development): Permissive wildcard CORS (no credentials)
// - Production (ENVIRONMENT=production): Specific origins with credentials support
func EnvironmentAwareCORS() gin.HandlerFunc {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("GO_ENV")
	}

	switch strings.ToLower(env) {
	case "production", "prod":
		return ProductionCORS()
	default:
		// Development mode - permissive but secure (no credentials with wildcard)
		return CORS()
	}
}
