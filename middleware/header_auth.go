package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HeaderAuthConfig configures the header-based auth middleware
type HeaderAuthConfig struct {
	// RequireTenant fails the request if X-Tenant-ID is missing
	RequireTenant bool

	// RequireUser fails the request if X-User-ID is missing
	RequireUser bool

	// SkipPaths is a list of path prefixes that bypass authentication
	SkipPaths []string
}

// DefaultHeaderAuthConfig returns the default configuration
// - Requires tenant ID for multi-tenant isolation
// - Requires user ID for RBAC permission checks
func DefaultHeaderAuthConfig() HeaderAuthConfig {
	return HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   true,
		SkipPaths: []string{
			"/health",
			"/ready",
			"/metrics",
			"/swagger",
		},
	}
}

// HeaderAuth is a simple middleware that extracts auth info from headers
// This is used in the BFF (Backend for Frontend) pattern where:
// - The admin/storefront app validates JWT and extracts user info
// - Headers are sent to backend services over the internal network
// - Backend services trust these headers for RBAC checks
//
// Required headers:
// - X-Tenant-ID: Tenant UUID for multi-tenant data isolation
// - X-User-ID: User/Staff UUID for RBAC permission lookup
// - X-User-Email: User email for staff lookup fallback
//
// Optional headers:
// - X-Vendor-ID: Vendor UUID for marketplace vendor isolation
// - Authorization: JWT token (forwarded but not validated here)
func HeaderAuth(config HeaderAuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this path should skip authentication
		for _, skipPath := range config.SkipPaths {
			if len(c.Request.URL.Path) >= len(skipPath) && c.Request.URL.Path[:len(skipPath)] == skipPath {
				c.Next()
				return
			}
		}

		// Extract tenant ID
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = c.GetHeader("x-tenant-id")
		}

		if config.RequireTenant && tenantID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "X-Tenant-ID header is required",
				},
			})
			return
		}

		// Extract user ID
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = c.GetHeader("x-user-id")
		}

		if config.RequireUser && userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "USER_REQUIRED",
					"message": "X-User-ID header is required",
				},
			})
			return
		}

		// Extract optional headers
		userEmail := c.GetHeader("X-User-Email")
		if userEmail == "" {
			userEmail = c.GetHeader("x-user-email")
		}

		vendorID := c.GetHeader("X-Vendor-ID")
		if vendorID == "" {
			vendorID = c.GetHeader("x-vendor-id")
		}

		// Set all values in gin context for downstream middleware (RBAC, handlers)
		// Use both snake_case and camelCase for compatibility
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
			c.Set("tenantId", tenantID)
		}

		if userID != "" {
			c.Set("user_id", userID)
			c.Set("staff_id", userID) // RBAC middleware checks staff_id
			c.Set("userId", userID)
		}

		if userEmail != "" {
			c.Set("user_email", userEmail)
			c.Set("userEmail", userEmail)
		}

		if vendorID != "" {
			c.Set("vendor_id", vendorID)
			c.Set("vendorId", vendorID)
		}

		c.Next()
	}
}

// HeaderAuthSimple is a convenience function with default config
func HeaderAuthSimple() gin.HandlerFunc {
	return HeaderAuth(DefaultHeaderAuthConfig())
}

// GetHeaderTenantID extracts tenant ID from gin context
func GetHeaderTenantID(c *gin.Context) string {
	return c.GetString("tenant_id")
}

// GetHeaderUserID extracts user ID from gin context
func GetHeaderUserID(c *gin.Context) string {
	return c.GetString("user_id")
}

// GetHeaderUserEmail extracts user email from gin context
func GetHeaderUserEmail(c *gin.Context) string {
	return c.GetString("user_email")
}

// GetHeaderVendorID extracts vendor ID from gin context
func GetHeaderVendorID(c *gin.Context) string {
	return c.GetString("vendor_id")
}
