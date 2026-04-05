package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig holds configuration for security headers
type SecurityHeadersConfig struct {
	// ContentSecurityPolicy defines the CSP header
	ContentSecurityPolicy string
	// FrameOptions controls X-Frame-Options (DENY, SAMEORIGIN)
	FrameOptions string
	// HSTSMaxAge is the max-age for Strict-Transport-Security (0 to disable)
	HSTSMaxAge int
	// HSTSIncludeSubdomains includes subdomains in HSTS
	HSTSIncludeSubdomains bool
	// HSTSPreload adds preload directive to HSTS
	HSTSPreload bool
	// ReferrerPolicy controls Referrer-Policy header
	ReferrerPolicy string
	// PermissionsPolicy controls Permissions-Policy header
	PermissionsPolicy string
	// CrossOriginOpenerPolicy controls Cross-Origin-Opener-Policy
	CrossOriginOpenerPolicy string
	// CrossOriginResourcePolicy controls Cross-Origin-Resource-Policy
	CrossOriginResourcePolicy string
	// CrossOriginEmbedderPolicy controls Cross-Origin-Embedder-Policy
	CrossOriginEmbedderPolicy string
}

// DefaultSecurityHeadersConfig returns secure defaults
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentSecurityPolicy:     "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'",
		FrameOptions:             "DENY",
		HSTSMaxAge:               31536000, // 1 year
		HSTSIncludeSubdomains:    true,
		HSTSPreload:              true,
		ReferrerPolicy:           "strict-origin-when-cross-origin",
		PermissionsPolicy:        "geolocation=(self), microphone=(), camera=()",
		CrossOriginOpenerPolicy:  "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
	}
}

// APISecurityHeadersConfig returns config suitable for API services
func APISecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentSecurityPolicy:     "default-src 'none'",
		FrameOptions:             "DENY",
		HSTSMaxAge:               31536000,
		HSTSIncludeSubdomains:    true,
		HSTSPreload:              false,
		ReferrerPolicy:           "no-referrer",
		PermissionsPolicy:        "geolocation=(self), microphone=(), camera=()",
		CrossOriginOpenerPolicy:  "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "",
	}
}

// SecurityHeaders returns a middleware that sets security headers
func SecurityHeaders() gin.HandlerFunc {
	return SecurityHeadersWithConfig(APISecurityHeadersConfig())
}

// SecurityHeadersWithConfig returns a middleware with custom config
func SecurityHeadersWithConfig(config SecurityHeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// XSS Protection (legacy but still useful for older browsers)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Prevent clickjacking
		if config.FrameOptions != "" {
			c.Header("X-Frame-Options", config.FrameOptions)
		}

		// Content Security Policy
		if config.ContentSecurityPolicy != "" {
			c.Header("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		// HSTS
		if config.HSTSMaxAge > 0 {
			hstsValue := "max-age=" + itoa(config.HSTSMaxAge)
			if config.HSTSIncludeSubdomains {
				hstsValue += "; includeSubDomains"
			}
			if config.HSTSPreload {
				hstsValue += "; preload"
			}
			c.Header("Strict-Transport-Security", hstsValue)
		}

		// Referrer Policy
		if config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		// Permissions Policy (formerly Feature-Policy)
		if config.PermissionsPolicy != "" {
			c.Header("Permissions-Policy", config.PermissionsPolicy)
		}

		// Cross-Origin headers
		if config.CrossOriginOpenerPolicy != "" {
			c.Header("Cross-Origin-Opener-Policy", config.CrossOriginOpenerPolicy)
		}
		if config.CrossOriginResourcePolicy != "" {
			c.Header("Cross-Origin-Resource-Policy", config.CrossOriginResourcePolicy)
		}
		if config.CrossOriginEmbedderPolicy != "" {
			c.Header("Cross-Origin-Embedder-Policy", config.CrossOriginEmbedderPolicy)
		}

		// Prevent caching of sensitive responses
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		c.Next()
	}
}

// itoa converts int to string (avoiding strconv import for simple cases)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var result []byte
	negative := false
	if i < 0 {
		negative = true
		i = -i
	}

	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}

// SensitiveEndpointHeaders adds extra security headers for sensitive endpoints
func SensitiveEndpointHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Disable caching completely
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		// Prevent the response from being embedded
		c.Header("X-Frame-Options", "DENY")

		// Strict CSP
		c.Header("Content-Security-Policy", "default-src 'none'")

		c.Next()
	}
}

// NoCacheHeaders prevents caching of responses
func NoCacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("Surrogate-Control", "no-store")
		c.Next()
	}
}

// RemoveServerHeader removes the Server header to prevent information disclosure
func RemoveServerHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Server", "")
		c.Next()
	}
}
