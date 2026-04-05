package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeIstioBody unmarshals the recorder body into a map.
func decodeIstioBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body must be valid JSON")
	return body
}

// newIstioRequest builds an http.Request pre-populated with the given Istio JWT claim headers.
func newIstioRequest(headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// ----- parseIstioHeaders -----------------------------------------------------

func TestParseIstioHeaders_ExtractsAllFields(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":         "user-uuid-001",
		"x-jwt-claim-email":       "user@example.com",
		"x-jwt-claim-tenant-id":   "tenant-abc",
		"x-jwt-claim-tenant-slug": "my-store",
		"x-jwt-claim-staff-id":    "staff-111",
		"x-jwt-claim-customer-id": "cust-222",
		"x-jwt-claim-vendor-id":   "vendor-333",
		"x-jwt-claim-roles":       `["admin","manager"]`,
	})

	authCtx := parseIstioHeaders(c)

	assert.Equal(t, "user-uuid-001", authCtx.UserID)
	assert.Equal(t, "user@example.com", authCtx.Email)
	assert.Equal(t, "tenant-abc", authCtx.TenantID)
	assert.Equal(t, "my-store", authCtx.TenantSlug)
	assert.Equal(t, "staff-111", authCtx.StaffID)
	assert.Equal(t, "cust-222", authCtx.CustomerID)
	assert.Equal(t, "vendor-333", authCtx.VendorID)
	assert.Equal(t, []string{"admin", "manager"}, authCtx.Roles)
}

func TestParseIstioHeaders_ParsesJSONArrayRoles(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":   "uid",
		"x-jwt-claim-roles": `["owner","super_admin","staff"]`,
	})

	authCtx := parseIstioHeaders(c)

	require.Len(t, authCtx.Roles, 3)
	assert.Equal(t, []string{"owner", "super_admin", "staff"}, authCtx.Roles)
}

func TestParseIstioHeaders_ParsesCommaSeparatedRoles(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":   "uid",
		"x-jwt-claim-roles": "admin,manager",
	})

	authCtx := parseIstioHeaders(c)

	require.Len(t, authCtx.Roles, 2)
	assert.Contains(t, authCtx.Roles, "admin")
	assert.Contains(t, authCtx.Roles, "manager")
}

func TestParseIstioHeaders_HandlesPlatformOwnerTrue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":            "uid",
		"x-jwt-claim-platform-owner": "true",
	})

	authCtx := parseIstioHeaders(c)
	assert.True(t, authCtx.IsPlatformOwner)
}

func TestParseIstioHeaders_HandlesPlatformOwnerFalse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":            "uid",
		"x-jwt-claim-platform-owner": "false",
	})

	authCtx := parseIstioHeaders(c)
	assert.False(t, authCtx.IsPlatformOwner)
}

func TestParseIstioHeaders_FallsBackToRealmRoles(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newIstioRequest(map[string]string{
		"x-jwt-claim-sub":        "uid",
		"x-jwt-claim-realm-roles": `["customer"]`,
	})

	authCtx := parseIstioHeaders(c)
	assert.Equal(t, []string{"customer"}, authCtx.Roles)
}

// ----- IstioAuth middleware --------------------------------------------------

func TestIstioAuth_RequireAuthRejectsUnauthenticated(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{RequireAuth: true}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestIstioAuth_SetsAllContextValues(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	var capturedAuth *AuthContext

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{RequireAuth: true}))
	router.GET("/test", func(c *gin.Context) {
		capturedAuth = GetAuthContext(c)
		c.Status(http.StatusOK)
	})

	req := newIstioRequest(map[string]string{
		"x-jwt-claim-sub":         "user-uuid-007",
		"x-jwt-claim-email":       "agent@example.com",
		"x-jwt-claim-tenant-id":   "ten-001",
		"x-jwt-claim-tenant-slug": "acme",
		"x-jwt-claim-staff-id":    "staff-007",
		"x-jwt-claim-vendor-id":   "vendor-007",
		"x-jwt-claim-roles":       `["admin"]`,
	})
	router.ServeHTTP(w, req)

	require.NotNil(t, capturedAuth, "AuthContext must be set in gin context")
	assert.Equal(t, "user-uuid-007", capturedAuth.UserID)
	assert.Equal(t, "agent@example.com", capturedAuth.Email)
	assert.Equal(t, "ten-001", capturedAuth.TenantID)
	assert.Equal(t, "acme", capturedAuth.TenantSlug)
	assert.Equal(t, "staff-007", capturedAuth.StaffID)
	assert.Equal(t, "vendor-007", capturedAuth.VendorID)
}

func TestIstioAuth_StripsLegacyHeadersWhenIstioPresent(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	var xUserIDAfter string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{RequireAuth: true}))
	router.GET("/test", func(c *gin.Context) {
		xUserIDAfter = c.GetHeader("X-User-ID")
		c.Status(http.StatusOK)
	})

	req := newIstioRequest(map[string]string{
		"x-jwt-claim-sub":       "real-user",
		"x-jwt-claim-tenant-id": "ten-abc",
		"X-User-ID":             "spoofed-user-id", // legacy header should be stripped
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, xUserIDAfter, "X-User-ID must be stripped when Istio auth headers are present")
}

func TestIstioAuth_AllowsInternalServiceCalls(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	var isInternal interface{}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{RequireAuth: true}))
	router.GET("/test", func(c *gin.Context) {
		isInternal, _ = c.Get("is_internal_service")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(InternalServiceHeader, "true")
	req.Header.Set("X-Tenant-ID", "ten-xyz")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, isInternal)
}

func TestIstioAuth_SkipsConfiguredPaths(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{
		RequireAuth: true,
		SkipPaths:   []string{"/api/v1/internal/"},
	}))
	router.GET("/api/v1/internal/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/internal/ping", nil)
	// No auth headers — should still pass because path is skipped.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIstioAuth_SkippedPathForwardsTenantID(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	var capturedTenantID interface{}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{
		RequireAuth: true,
		SkipPaths:   []string{"/internal"},
	}))
	router.GET("/internal/op", func(c *gin.Context) {
		capturedTenantID, _ = c.Get("tenant_id")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/op", nil)
	req.Header.Set("X-Tenant-ID", "ten-forwarded")
	router.ServeHTTP(w, req)

	assert.Equal(t, "ten-forwarded", capturedTenantID)
}

func TestIstioAuth_AllowLegacyHeadersParsesXUserID(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	var capturedAuth *AuthContext

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{
		RequireAuth:        false,
		AllowLegacyHeaders: true,
	}))
	router.GET("/test", func(c *gin.Context) {
		capturedAuth = GetAuthContext(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-ID", "legacy-user")
	req.Header.Set("X-Tenant-ID", "legacy-tenant")
	req.Header.Set("X-User-Email", "legacy@example.com")
	req.Header.Set("X-Vendor-ID", "legacy-vendor")
	router.ServeHTTP(w, req)

	require.NotNil(t, capturedAuth)
	assert.Equal(t, "legacy-user", capturedAuth.UserID)
	assert.Equal(t, "legacy-tenant", capturedAuth.TenantID)
	assert.Equal(t, "legacy@example.com", capturedAuth.Email)
	assert.Equal(t, "legacy-vendor", capturedAuth.VendorID)
}

func TestIstioAuth_AllowLegacyHeaders_RequireAuthRejectsWhenNoLegacyUserID(t *testing.T) {
	t.Setenv("ISTIO_AUTH_ENABLED", "true")

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(IstioAuth(IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-User-ID and no Istio headers — should be rejected.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ----- GetAuthContext --------------------------------------------------------

func TestGetAuthContext_ReturnsNilWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	assert.Nil(t, GetAuthContext(c))
}

func TestGetAuthContext_ReturnsAuthContextWhenSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	expected := &AuthContext{UserID: "user-001", TenantID: "ten-001"}
	c.Set(AuthContextKey, expected)

	result := GetAuthContext(c)
	require.NotNil(t, result)
	assert.Equal(t, "user-001", result.UserID)
}

// ----- GetIstioUserID --------------------------------------------------------

func TestGetIstioUserID_FromAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{UserID: "uid-from-auth"})

	assert.Equal(t, "uid-from-auth", GetIstioUserID(c))
}

func TestGetIstioUserID_FallsBackToContextValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "uid-fallback")

	assert.Equal(t, "uid-fallback", GetIstioUserID(c))
}

func TestGetIstioUserID_ReturnsEmptyWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.Equal(t, "", GetIstioUserID(c))
}

// ----- GetIstioTenantID ------------------------------------------------------

func TestGetIstioTenantID_FromAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{TenantID: "ten-from-auth"})

	assert.Equal(t, "ten-from-auth", GetIstioTenantID(c))
}

func TestGetIstioTenantID_FallsBackToContextValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("tenant_id", "ten-fallback")

	assert.Equal(t, "ten-fallback", GetIstioTenantID(c))
}

// ----- GetIstioStaffID -------------------------------------------------------

func TestGetIstioStaffID_FromAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{StaffID: "staff-from-auth"})

	assert.Equal(t, "staff-from-auth", GetIstioStaffID(c))
}

func TestGetIstioStaffID_FallsBackToContextValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("staff_id", "staff-fallback")

	assert.Equal(t, "staff-fallback", GetIstioStaffID(c))
}

// ----- GetIstioVendorID ------------------------------------------------------

func TestGetIstioVendorID_FromAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{VendorID: "vendor-from-auth"})

	assert.Equal(t, "vendor-from-auth", GetIstioVendorID(c))
}

func TestGetIstioVendorID_ReturnsEmptyWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.Equal(t, "", GetIstioVendorID(c))
}

// ----- HasIstioRole ----------------------------------------------------------

func TestHasIstioRole_ReturnsTrueWhenRolePresent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{Roles: []string{"admin", "manager"}})

	assert.True(t, HasIstioRole(c, "admin"))
	assert.True(t, HasIstioRole(c, "manager"))
}

func TestHasIstioRole_ReturnsFalseWhenRoleAbsent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{Roles: []string{"staff"}})

	assert.False(t, HasIstioRole(c, "admin"))
}

func TestHasIstioRole_ReturnsFalseWhenNoAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.False(t, HasIstioRole(c, "admin"))
}

// ----- IsPlatformOwner -------------------------------------------------------

func TestIsPlatformOwner_ReturnsTrueWhenSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{IsPlatformOwner: true})

	assert.True(t, IsPlatformOwner(c))
}

func TestIsPlatformOwner_ReturnsFalseWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(AuthContextKey, &AuthContext{IsPlatformOwner: false})

	assert.False(t, IsPlatformOwner(c))
}

func TestIsPlatformOwner_ReturnsFalseWhenNoAuthContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.False(t, IsPlatformOwner(c))
}

// ----- RequireIstioRole ------------------------------------------------------

func TestRequireIstioRole_BlocksWhenRoleMissing(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{Roles: []string{"staff"}})
		c.Next()
	})
	router.Use(RequireIstioRole("admin"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	body := decodeIstioBody(t, w)
	assert.Equal(t, "forbidden", body["error"])
}

func TestRequireIstioRole_PassesWhenRolePresent(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{Roles: []string{"admin", "manager"}})
		c.Next()
	})
	router.Use(RequireIstioRole("admin"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- RequireTenant ---------------------------------------------------------

func TestRequireTenant_BlocksWhenTenantMissing(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(RequireTenant())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := decodeIstioBody(t, w)
	assert.Equal(t, "tenant_required", body["error"])
}

func TestRequireTenant_PassesWhenTenantPresent(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{TenantID: "ten-001"})
		c.Next()
	})
	router.Use(RequireTenant())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- VendorScopeFilter -----------------------------------------------------

func TestVendorScopeFilter_PlatformOwnerBypasses(t *testing.T) {
	var filter string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{
			IsPlatformOwner: true,
			VendorID:        "vendor-x",
			Roles:           []string{"staff"},
		})
		c.Next()
	})
	router.Use(VendorScopeFilter())
	router.GET("/test", func(c *gin.Context) {
		filter = GetVendorScopeFilter(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Empty(t, filter, "platform owner must not have vendor scope filter applied")
}

func TestVendorScopeFilter_TenantLevelRolesBypasses(t *testing.T) {
	tenantLevelRoles := []string{"owner", "store_owner", "super_admin", "admin", "manager"}

	for _, role := range tenantLevelRoles {
		t.Run(role, func(t *testing.T) {
			var filter string

			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(func(c *gin.Context) {
				c.Set(AuthContextKey, &AuthContext{
					IsPlatformOwner: false,
					VendorID:        "some-vendor",
					Roles:           []string{role},
				})
				c.Next()
			})
			router.Use(VendorScopeFilter())
			router.GET("/test", func(c *gin.Context) {
				filter = GetVendorScopeFilter(c)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			router.ServeHTTP(w, req)

			assert.Empty(t, filter,
				"tenant-level role %q must not have vendor scope filter applied", role)
		})
	}
}

func TestVendorScopeFilter_VendorScopedSetsFilter(t *testing.T) {
	var filter string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{
			IsPlatformOwner: false,
			VendorID:        "vendor-scoped-123",
			Roles:           []string{"staff"}, // not a tenant-level role
		})
		c.Next()
	})
	router.Use(VendorScopeFilter())
	router.GET("/test", func(c *gin.Context) {
		filter = GetVendorScopeFilter(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, "vendor-scoped-123", filter)
}

// ----- GetVendorScopeFilter --------------------------------------------------

func TestGetVendorScopeFilter_ReturnsEmptyWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.Equal(t, "", GetVendorScopeFilter(c))
}

func TestGetVendorScopeFilter_ReturnsValueWhenSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("vendor_scope_filter", "my-vendor")

	assert.Equal(t, "my-vendor", GetVendorScopeFilter(c))
}

// ----- GetActorInfo ----------------------------------------------------------

func TestGetActorInfo_ExtractsActorInfo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("User-Agent", "TestAgent/1.0")

	c.Set("user_id", "actor-uuid")
	c.Set("user_email", "actor@example.com")
	c.Set("username", "Actor Name")

	actor := GetActorInfo(c)

	assert.Equal(t, "actor-uuid", actor.ActorID)
	assert.Equal(t, "actor@example.com", actor.ActorEmail)
	assert.Equal(t, "Actor Name", actor.ActorName)
	assert.Equal(t, "TestAgent/1.0", actor.UserAgent)
}

func TestGetActorInfo_FallsBackActorNameToEmail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	c.Set("user_id", "uid-fallback")
	c.Set("user_email", "fallback@example.com")
	// No "username" set — should fall back to email.

	actor := GetActorInfo(c)

	assert.Equal(t, "fallback@example.com", actor.ActorName)
}

func TestGetActorInfo_EmptyWhenNoContextValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	actor := GetActorInfo(c)

	assert.Equal(t, "", actor.ActorID)
	assert.Equal(t, "", actor.ActorEmail)
}

// ----- RequireVendorMatch ----------------------------------------------------

func TestRequireVendorMatch_PlatformOwnerBypasses(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{IsPlatformOwner: true})
		c.Next()
	})
	router.Use(RequireVendorMatch("vendorId"))
	router.GET("/vendors/:vendorId/products", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendors/any-vendor/products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireVendorMatch_TenantLevelUserBypasses(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		// No VendorID means tenant-level access.
		c.Set(AuthContextKey, &AuthContext{TenantID: "ten-001"})
		c.Next()
	})
	router.Use(RequireVendorMatch("vendorId"))
	router.GET("/vendors/:vendorId/products", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendors/some-vendor/products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireVendorMatch_BlocksMismatchedVendor(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{VendorID: "vendor-A"})
		c.Next()
	})
	router.Use(RequireVendorMatch("vendorId"))
	router.GET("/vendors/:vendorId/products", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendors/vendor-B/products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	body := decodeIstioBody(t, w)
	assert.Equal(t, "vendor_forbidden", body["error"])
}

func TestRequireVendorMatch_AllowsMatchingVendor(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(func(c *gin.Context) {
		c.Set(AuthContextKey, &AuthContext{VendorID: "vendor-X"})
		c.Next()
	})
	router.Use(RequireVendorMatch("vendorId"))
	router.GET("/vendors/:vendorId/products", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendors/vendor-X/products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- getPrimaryRole --------------------------------------------------------

func TestGetPrimaryRole_ReturnsHighestPriorityRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		wantRole string
	}{
		{
			name:     "owner beats admin",
			roles:    []string{"admin", "owner"},
			wantRole: "owner",
		},
		{
			name:     "store_owner beats manager",
			roles:    []string{"manager", "store_owner"},
			wantRole: "store_owner",
		},
		{
			name:     "super_admin beats staff",
			roles:    []string{"staff", "super_admin"},
			wantRole: "super_admin",
		},
		{
			name:     "single role returned directly",
			roles:    []string{"customer"},
			wantRole: "customer",
		},
		{
			name:     "unknown role used as fallback",
			roles:    []string{"some_custom_role"},
			wantRole: "some_custom_role",
		},
		{
			name:     "empty roles returns empty string",
			roles:    []string{},
			wantRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPrimaryRole(tt.roles)
			assert.Equal(t, tt.wantRole, got)
		})
	}
}

// ----- isTenantLevelRole -----------------------------------------------------

func TestIsTenantLevelRole_IdentifiesTenantLevelRoles(t *testing.T) {
	tenantRoles := []struct {
		role     string
		expected bool
	}{
		{"owner", true},
		{"store_owner", true},
		{"super_admin", true},
		{"admin", true},
		{"manager", true},
		{"staff", false},
		{"customer", false},
		{"viewer", false},
		{"vendor_admin", false},
		{"ADMIN", true}, // case-insensitive
	}

	for _, tt := range tenantRoles {
		t.Run(tt.role, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			c.Set(AuthContextKey, &AuthContext{Roles: []string{tt.role}})

			got := isTenantLevelRole(c)
			assert.Equal(t, tt.expected, got, "isTenantLevelRole with role %q", tt.role)
		})
	}
}

func TestIsTenantLevelRole_ChecksGinContextRoles(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	// Set via gin context "roles" key (legacy path), not AuthContext.
	c.Set("roles", []string{"admin"})

	assert.True(t, isTenantLevelRole(c))
}
