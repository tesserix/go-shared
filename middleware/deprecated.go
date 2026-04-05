package middleware

import "github.com/gin-gonic/gin"

// Deprecated aliases for backward compatibility.
// All services should migrate to the new names.

// parseIstioHeaders is deprecated. Use parseJWTClaimHeaders.
func parseIstioHeaders(c *gin.Context) *AuthContext { return parseJWTClaimHeaders(c) }

// IstioAuthConfig is deprecated. Use AuthConfig.
type IstioAuthConfig = AuthConfig

// IstioAuth is deprecated. Use Auth.
func IstioAuth(config IstioAuthConfig) gin.HandlerFunc { return Auth(config) }

// IstioAuthHeaderKey is deprecated. Use AuthHeaderKey.
const IstioAuthHeaderKey = AuthHeaderKey

// GetIstioUserID is deprecated. Use GetUserID.
func GetIstioUserID(c *gin.Context) string { return GetUserID(c) }

// GetIstioTenantID is deprecated. Use GetTenantID.
func GetIstioTenantID(c *gin.Context) string { return GetTenantID(c) }

// GetIstioStaffID is deprecated. Use GetStaffID.
func GetIstioStaffID(c *gin.Context) string { return GetStaffID(c) }

// GetIstioVendorID is deprecated. Use GetVendorID.
func GetIstioVendorID(c *gin.Context) string { return GetVendorID(c) }

// HasIstioRole is deprecated. Use HasRole.
func HasIstioRole(c *gin.Context, role string) bool { return HasRole(c, role) }

// RequireIstioRole is deprecated. Use RequireRole.
func RequireIstioRole(role string) gin.HandlerFunc { return RequireRole(role) }
