package authz

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// ObjectIDFunc extracts an object ID from the request context.
// Used by Require to determine which object to check authorization against.
type ObjectIDFunc func(c *gin.Context) string

// Check represents a single authorization check (relation + object).
type Check struct {
	Relation   string
	ObjectType string
	ObjectID   ObjectIDFunc
}

// ErrorResponse for authorization failures.
type ErrorResponse struct {
	Success bool       `json:"success"`
	Error   ErrorDetail `json:"error"`
}

// ErrorDetail holds error information.
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Middleware provides OpenFGA-backed authorization middleware for Gin.
// It checks relationships directly against OpenFGA — no HTTP call to staff-service,
// no Redis cache. OpenFGA handles caching internally.
type Middleware struct {
	client *Client
}

// NewMiddleware creates a new OpenFGA authorization middleware.
func NewMiddleware(client *Client) *Middleware {
	return &Middleware{client: client}
}

// Require checks if the authenticated user has the given relation on an object.
// The objectType is fixed, and objectIDFunc extracts the object ID from the request.
//
// Usage:
//
//	fgaMw.Require("can_read_staff", "store", StoreIDFromContext)
func (m *Middleware) Require(relation, objectType string, objectIDFunc ObjectIDFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.skipPlatformOwner(c) {
			return
		}

		userID := m.getUserID(c)
		if userID == "" {
			m.forbidden(c, "User context not found", relation)
			return
		}

		objectID := objectIDFunc(c)
		if objectID == "" {
			m.forbidden(c, "Object context not found", relation)
			return
		}

		allowed, err := m.client.Can(c.Request.Context(), userID, relation, objectType, objectID)
		if err != nil {
			slog.Error("FGA check failed",
				"user_id", userID, "relation", relation,
				"object_type", objectType, "object_id", objectID,
				"path", c.Request.URL.Path, "error", err)
			m.forbidden(c, "Failed to verify permissions", relation)
			return
		}

		if !allowed {
			slog.Info("FGA check denied",
				"user_id", userID, "relation", relation,
				"object_type", objectType, "object_id", objectID,
				"path", c.Request.URL.Path)
			m.forbidden(c, "Insufficient permissions", relation)
			return
		}

		c.Next()
	}
}

// RequireAny passes if ANY of the checks succeed.
func (m *Middleware) RequireAny(checks ...Check) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.skipPlatformOwner(c) {
			return
		}

		userID := m.getUserID(c)
		if userID == "" {
			m.forbidden(c, "User context not found", "")
			return
		}

		ctx := c.Request.Context()
		for _, check := range checks {
			objectID := check.ObjectID(c)
			if objectID == "" {
				continue
			}
			allowed, err := m.client.Can(ctx, userID, check.Relation, check.ObjectType, objectID)
			if err != nil {
				slog.Error("FGA check failed in RequireAny",
					"user_id", userID, "relation", check.Relation,
					"object_type", check.ObjectType, "object_id", objectID,
					"path", c.Request.URL.Path, "error", err)
				continue
			}
			if allowed {
				c.Next()
				return
			}
		}

		slog.Info("FGA RequireAny: all checks denied",
			"user_id", userID, "path", c.Request.URL.Path)
		m.forbidden(c, "Insufficient permissions", "")
	}
}

// RequireAll passes only if ALL checks succeed.
func (m *Middleware) RequireAll(checks ...Check) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.skipPlatformOwner(c) {
			return
		}

		userID := m.getUserID(c)
		if userID == "" {
			m.forbidden(c, "User context not found", "")
			return
		}

		ctx := c.Request.Context()
		for _, check := range checks {
			objectID := check.ObjectID(c)
			if objectID == "" {
				m.forbidden(c, "Object context not found", check.Relation)
				return
			}
			allowed, err := m.client.Can(ctx, userID, check.Relation, check.ObjectType, objectID)
			if err != nil {
				slog.Error("FGA check failed in RequireAll",
					"user_id", userID, "relation", check.Relation,
					"object_type", check.ObjectType, "object_id", objectID,
					"path", c.Request.URL.Path, "error", err)
				m.forbidden(c, "Failed to verify permissions", check.Relation)
				return
			}
			if !allowed {
				slog.Info("FGA RequireAll: check denied",
					"user_id", userID, "relation", check.Relation,
					"path", c.Request.URL.Path)
				m.forbidden(c, "Insufficient permissions", check.Relation)
				return
			}
		}

		c.Next()
	}
}

// RequireStorePermission is a convenience method for store-scoped checks.
// It extracts the store ID from the "tenant_id" context value (set by IstioAuth/TenantMiddleware).
//
// Usage:
//
//	fgaMw.RequireStorePermission("can_read_staff")
func (m *Middleware) RequireStorePermission(relation string) gin.HandlerFunc {
	return m.Require(relation, "store", StoreIDFromContext)
}

// RequireVendorPermission is a convenience method for vendor-scoped checks.
// It extracts the vendor ID from the "vendor_id" context value.
//
// Usage:
//
//	fgaMw.RequireVendorPermission("can_manage_staff")
func (m *Middleware) RequireVendorPermission(relation string) gin.HandlerFunc {
	return m.Require(relation, "vendor", VendorIDFromContext)
}

// RequireStoreOrVendor checks store-level permission first, then vendor-level.
// Useful for endpoints accessible by both tenant-level and vendor-level staff.
func (m *Middleware) RequireStoreOrVendor(storeRelation, vendorRelation string) gin.HandlerFunc {
	return m.RequireAny(
		Check{Relation: storeRelation, ObjectType: "store", ObjectID: StoreIDFromContext},
		Check{Relation: vendorRelation, ObjectType: "vendor", ObjectID: VendorIDFromContext},
	)
}

// AllowInternal wraps a permission middleware to let internal service calls bypass checks.
// Internal services must provide X-Internal-Service header with a value matching the
// INTERNAL_SERVICE_SECRET env var. Requests with an invalid secret are rejected.
// Internal services must still provide X-Tenant-ID for tenant isolation.
func AllowInternal(next gin.HandlerFunc) gin.HandlerFunc {
	secret := os.Getenv("INTERNAL_SERVICE_SECRET")
	return func(c *gin.Context) {
		headerVal := c.GetHeader("X-Internal-Service")
		if headerVal != "" {
			if secret == "" {
				slog.Warn("AllowInternal: INTERNAL_SERVICE_SECRET not configured, rejecting internal call",
					"path", c.Request.URL.Path, "client_ip", c.ClientIP())
				next(c)
				return
			}
			if headerVal != secret {
				slog.Warn("AllowInternal: invalid internal service secret",
					"path", c.Request.URL.Path, "client_ip", c.ClientIP())
				c.JSON(http.StatusForbidden, ErrorResponse{
					Success: false,
					Error: ErrorDetail{
						Code:    "FORBIDDEN",
						Message: "Invalid internal service credentials",
					},
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}
		next(c)
	}
}

// HasPermission checks authorization inline from a handler.
// Returns false if the user lacks the relation or on error.
//
// Usage:
//
//	if authz.HasPermission(c, fgaClient, "can_delete_staff", "store", storeID) { ... }
func HasPermission(c *gin.Context, client *Client, relation, objectType, objectID string) bool {
	userID := getUserIDFromContext(c)
	if userID == "" || objectID == "" {
		return false
	}
	allowed, err := client.Can(c.Request.Context(), userID, relation, objectType, objectID)
	return err == nil && allowed
}

// --- Object ID extractors ---

// StoreIDFromContext extracts the store (tenant) ID from gin context.
// In the marketplace, the tenant_id maps to the OpenFGA store object.
func StoreIDFromContext(c *gin.Context) string {
	return c.GetString("tenant_id")
}

// VendorIDFromContext extracts the vendor ID from gin context.
func VendorIDFromContext(c *gin.Context) string {
	return c.GetString("vendor_id")
}

// ParamID returns an ObjectIDFunc that reads from a URL parameter.
//
// Usage:
//
//	fgaMw.Require("can_edit", "order", authz.ParamID("orderId"))
func ParamID(param string) ObjectIDFunc {
	return func(c *gin.Context) string {
		return c.Param(param)
	}
}

// --- Internal helpers ---

// skipPlatformOwner checks if the user is a platform owner and bypasses authz.
// Returns true if the request should continue (c.Next() already called).
func (m *Middleware) skipPlatformOwner(c *gin.Context) bool {
	if c.GetBool("is_platform_owner") {
		c.Next()
		return true
	}
	return false
}

// getUserID extracts the user ID for FGA checks from gin context.
func (m *Middleware) getUserID(c *gin.Context) string {
	return getUserIDFromContext(c)
}

// getUserIDFromContext extracts user ID from gin context.
// Returns the GIP UID (user_id) which is the subject used in FGA tuples.
// Does NOT fall back to staff_id — staff_id is an internal record ID,
// not the IDP subject, and using it would cause FGA check mismatches.
func getUserIDFromContext(c *gin.Context) string {
	return c.GetString("user_id")
}

func (m *Middleware) forbidden(c *gin.Context, message, relation string) {
	resp := ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    "FORBIDDEN",
			Message: message,
		},
	}
	if relation != "" {
		resp.Error.Details = map[string]interface{}{
			"required": relation,
		}
	}
	c.JSON(http.StatusForbidden, resp)
	c.Abort()
}
