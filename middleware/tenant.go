package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TenantContext holds tenant information extracted from request
type TenantContext struct {
	VendorID     string
	StorefrontID string
	TenantSlug   string
}

// TenantMiddleware extracts tenant info from request headers
// Supports both legacy X-Tenant-ID and new X-Vendor-ID headers
func TenantMiddleware() gin.HandlerFunc {
	return TenantMiddlewareWithOptions(TenantOptions{})
}

// TenantOptions configures the tenant middleware behavior
type TenantOptions struct {
	// RequireVendorID makes vendor ID mandatory (default: false for backwards compat)
	RequireVendorID bool

	// DefaultVendorID to use in development when no vendor ID is provided
	DefaultVendorID string

	// ExcludedPaths are paths that don't require tenant context
	ExcludedPaths []string
}

// TenantMiddlewareWithOptions creates a tenant middleware with custom options
func TenantMiddlewareWithOptions(opts TenantOptions) gin.HandlerFunc {
	// Default excluded paths
	excludedPaths := []string{
		"/health",
		"/ready",
		"/swagger",
		"/metrics",
	}
	excludedPaths = append(excludedPaths, opts.ExcludedPaths...)

	return func(c *gin.Context) {
		// Check if path is excluded
		for _, path := range excludedPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Try X-Vendor-ID first (new architecture)
		vendorID := c.GetHeader("X-Vendor-ID")

		// Fall back to X-Tenant-ID (backwards compatibility)
		if vendorID == "" {
			vendorID = c.GetHeader("X-Tenant-ID")
		}

		// Fall back to tenant_id already set in gin context by Auth()/GIPAuth().
		// Auth() strips X-Tenant-ID from request headers after extracting it
		// from x-jwt-claim-tenant-id, so the header read above may be empty
		// even though Auth() already set tenant_id in the gin context.
		if vendorID == "" {
			vendorID = c.GetString("tenant_id")
		}

		// Use default if provided and no vendor ID found
		if vendorID == "" && opts.DefaultVendorID != "" {
			vendorID = opts.DefaultVendorID
		}

		// Check if vendor ID is required
		if opts.RequireVendorID && vendorID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VENDOR_REQUIRED",
					"message": "X-Vendor-ID or X-Tenant-ID header is required",
				},
			})
			c.Abort()
			return
		}

		// Get optional storefront ID
		storefrontID := c.GetHeader("X-Storefront-ID")

		// Get optional tenant slug
		tenantSlug := c.GetHeader("X-Tenant-Slug")
		if tenantSlug == "" {
			tenantSlug = c.GetString("tenant_slug")
		}

		// Create tenant context
		tenantCtx := TenantContext{
			VendorID:     vendorID,
			StorefrontID: storefrontID,
			TenantSlug:   tenantSlug,
		}

		// Set values in context
		c.Set("tenant_context", tenantCtx)
		c.Set("vendor_id", vendorID)
		c.Set("tenant_id", vendorID) // backwards compatibility
		if storefrontID != "" {
			c.Set("storefront_id", storefrontID)
		}
		if tenantSlug != "" {
			c.Set("tenant_slug", tenantSlug)
		}

		c.Next()
	}
}

// RequireVendorMiddleware middleware that strictly requires vendor ID
func RequireVendorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		vendorID := c.GetHeader("X-Vendor-ID")
		if vendorID == "" {
			vendorID = c.GetHeader("X-Tenant-ID")
		}
		if vendorID == "" {
			vendorID = c.GetString("tenant_id")
		}

		if vendorID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VENDOR_REQUIRED",
					"message": "X-Vendor-ID or X-Tenant-ID header is required",
				},
			})
			c.Abort()
			return
		}

		c.Set("vendor_id", vendorID)
		c.Set("tenant_id", vendorID)
		c.Next()
	}
}

// GetTenantContext extracts the full tenant context from gin context
func GetTenantContext(c *gin.Context) *TenantContext {
	ctx, exists := c.Get("tenant_context")
	if !exists {
		return nil
	}
	tenantCtx, ok := ctx.(TenantContext)
	if !ok {
		return nil
	}
	return &tenantCtx
}

// GetStorefrontID extracts storefront ID from gin context
func GetStorefrontID(c *gin.Context) string {
	storefrontID, exists := c.Get("storefront_id")
	if !exists {
		return ""
	}
	return storefrontID.(string)
}

// MustGetVendorID extracts vendor ID or panics if not present
// Use only when you're certain the middleware has set the value
func MustGetVendorID(c *gin.Context) string {
	vendorID := GetVendorID(c)
	if vendorID == "" {
		panic("vendor_id not found in context - ensure TenantMiddleware is applied")
	}
	return vendorID
}

// GetTenantSlug extracts the tenant slug from gin context
func GetTenantSlug(c *gin.Context) string {
	slug, _ := c.Get("tenant_slug")
	if slug == nil {
		return ""
	}
	return slug.(string)
}
