package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tesserix/go-shared/auth"
	"github.com/tesserix/go-shared/errors"
)

// AuthMiddleware validates JWT tokens
func AuthMiddleware(config auth.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for configured paths
		for _, skipPath := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, skipPath) {
				c.Next()
				return
			}
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			ErrorResponse(c, errors.NewMissingTokenError())
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			ErrorResponse(c, errors.NewInvalidTokenFormatError())
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := auth.ValidateToken(tokenString, config)
		if err != nil {
			if err == jwt.ErrTokenExpired {
				ErrorResponse(c, errors.NewInvalidTokenError())
			} else {
				ErrorResponse(c, errors.NewInvalidTokenError())
			}
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)

		// Override tenant_id from token if available
		if claims.TenantID != "" {
			c.Set("tenant_id", claims.TenantID)
		}

		c.Next()
	}
}

// RequireJWTRole middleware checks if user has required role (legacy JWT auth).
// Deprecated: Use RequireRole from auth_context.go instead.
func RequireJWTRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			ErrorResponse(c, errors.NewNoRolesError())
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			ErrorResponse(c, errors.NewInvalidRolesError())
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range userRoles {
			if role == requiredRole || role == "super_admin" {
				hasRole = true
				break
			}
		}

		if !hasRole {
			ErrorResponse(c, errors.NewInsufficientPermissionsError(requiredRole))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware checks if user has any of the required roles
func RequireAnyRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			ErrorResponse(c, errors.NewNoRolesError())
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			ErrorResponse(c, errors.NewInvalidRolesError())
			c.Abort()
			return
		}

		// Check if user has any required role
		hasRole := false
		for _, userRole := range userRoles {
			if userRole == "super_admin" {
				hasRole = true
				break
			}
			for _, requiredRole := range requiredRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			ErrorResponse(c, errors.NewInsufficientPermissionsAnyError(requiredRoles))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAllRoles middleware checks if user has all of the required roles
func RequireAllRoles(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			ErrorResponse(c, errors.NewNoRolesError())
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			ErrorResponse(c, errors.NewInvalidRolesError())
			c.Abort()
			return
		}

		// Super admin bypasses all role checks
		for _, userRole := range userRoles {
			if userRole == "super_admin" {
				c.Next()
				return
			}
		}

		// Check if user has all required roles
		for _, requiredRole := range requiredRoles {
			hasRole := false
			for _, userRole := range userRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if !hasRole {
				ErrorResponse(c, errors.NewInsufficientPermissionsError(fmt.Sprintf("Required all roles: %v", requiredRoles)))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// OptionalAuth middleware allows but doesn't require authentication
func OptionalAuth(config auth.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := auth.ValidateToken(tokenString, config)
		if err != nil {
			// Just continue without setting user context
			c.Next()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)

		if claims.TenantID != "" {
			c.Set("tenant_id", claims.TenantID)
		}

		c.Next()
	}
}

// DevelopmentAuthMiddleware provides a simple auth for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health checks
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		// Set dummy user for development
		c.Set("user_id", "dev-user-123")
		c.Set("user_email", "dev@tesseract.com")
		c.Set("user_roles", []string{"admin", "developer"})

		// Check for tenant ID from header first, then use default UUID
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "00000000-0000-0000-0000-000000000001" // Primary tenant UUID
		}
		c.Set("tenant_id", tenantID)

		c.Next()
	}
}
