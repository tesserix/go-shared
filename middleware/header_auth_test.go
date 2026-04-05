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

// decodeHeaderAuthBody unmarshals the recorder body into a map.
func decodeHeaderAuthBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body must be valid JSON")
	return body
}

// ----- DefaultHeaderAuthConfig -----------------------------------------------

func TestDefaultHeaderAuthConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := DefaultHeaderAuthConfig()

	assert.True(t, cfg.RequireTenant)
	assert.True(t, cfg.RequireUser)
	assert.Contains(t, cfg.SkipPaths, "/health")
	assert.Contains(t, cfg.SkipPaths, "/ready")
	assert.Contains(t, cfg.SkipPaths, "/metrics")
	assert.Contains(t, cfg.SkipPaths, "/swagger")
}

// ----- HeaderAuth -- skip paths ----------------------------------------------

func TestHeaderAuth_SkipsConfiguredPaths(t *testing.T) {
	skipPaths := []string{"/health", "/ready", "/metrics", "/swagger"}

	for _, path := range skipPaths {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(HeaderAuth(DefaultHeaderAuthConfig()))
			router.GET(path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			// No auth headers — path should be skipped.
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code,
				"path %s must bypass header auth", path)
		})
	}
}

func TestHeaderAuth_SkipsCustomPaths(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   true,
		SkipPaths:     []string{"/internal"},
	}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/internal/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- HeaderAuth -- tenant requirement --------------------------------------

func TestHeaderAuth_RequiresTenantWhenConfigured(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   false,
	}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-Tenant-ID header.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := decodeHeaderAuthBody(t, w)
	assert.Equal(t, false, body["success"])
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "TENANT_REQUIRED", errObj["code"])
}

func TestHeaderAuth_AcceptsTenantFromXTenantIDHeader(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   false,
	}

	var capturedTenantID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		capturedTenantID = GetHeaderTenantID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-001")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "tenant-001", capturedTenantID)
}

func TestHeaderAuth_AcceptsTenantFromLowercaseHeader(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   false,
	}

	var capturedTenantID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		capturedTenantID = GetHeaderTenantID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("x-tenant-id", "lower-tenant")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "lower-tenant", capturedTenantID)
}

// ----- HeaderAuth -- user requirement ----------------------------------------

func TestHeaderAuth_RequiresUserWhenConfigured(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: false,
		RequireUser:   true,
	}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-User-ID header.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := decodeHeaderAuthBody(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "USER_REQUIRED", errObj["code"])
}

func TestHeaderAuth_AcceptsUserFromXUserIDHeader(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: false,
		RequireUser:   true,
	}

	var capturedUserID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		capturedUserID = GetHeaderUserID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-ID", "user-555")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-555", capturedUserID)
}

// ----- HeaderAuth -- context values ------------------------------------------

func TestHeaderAuth_SetsAllContextValues(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   true,
	}

	var (
		capturedTenantID string
		capturedUserID   string
		capturedEmail    string
		capturedVendorID string
	)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		capturedTenantID = GetHeaderTenantID(c)
		capturedUserID = GetHeaderUserID(c)
		capturedEmail = GetHeaderUserEmail(c)
		capturedVendorID = GetHeaderVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "ten-abc")
	req.Header.Set("X-User-ID", "user-abc")
	req.Header.Set("X-User-Email", "user@example.com")
	req.Header.Set("X-Vendor-ID", "vendor-abc")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ten-abc", capturedTenantID)
	assert.Equal(t, "user-abc", capturedUserID)
	assert.Equal(t, "user@example.com", capturedEmail)
	assert.Equal(t, "vendor-abc", capturedVendorID)
}

func TestHeaderAuth_SetsStaffIDToUserID(t *testing.T) {
	// HeaderAuth sets staff_id = userID for RBAC middleware compatibility.
	cfg := HeaderAuthConfig{
		RequireTenant: false,
		RequireUser:   true,
	}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		staffID, _ := c.Get("staff_id")
		userID, _ := c.Get("user_id")
		assert.Equal(t, userID, staffID, "staff_id must equal user_id")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-ID", "staff-via-user")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHeaderAuth_SetsBothCamelCaseAndSnakeCaseValues(t *testing.T) {
	cfg := HeaderAuthConfig{
		RequireTenant: true,
		RequireUser:   true,
	}

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuth(cfg))
	router.GET("/test", func(c *gin.Context) {
		tenantSnake, _ := c.Get("tenant_id")
		tenantCamel, _ := c.Get("tenantId")
		userSnake, _ := c.Get("user_id")
		userCamel, _ := c.Get("userId")

		assert.Equal(t, "ten-cc", tenantSnake)
		assert.Equal(t, "ten-cc", tenantCamel)
		assert.Equal(t, "user-cc", userSnake)
		assert.Equal(t, "user-cc", userCamel)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "ten-cc")
	req.Header.Set("X-User-ID", "user-cc")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- HeaderAuthSimple ------------------------------------------------------

func TestHeaderAuthSimple_UsesDefaultConfig(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(HeaderAuthSimple())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No headers: both tenant and user are required by default.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ----- GetHeaderTenantID / GetHeaderUserID / GetHeaderUserEmail / GetHeaderVendorID --

func TestGetHeaderTenantID_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Equal(t, "", GetHeaderTenantID(c))
}

func TestGetHeaderTenantID_ReturnsValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("tenant_id", "abc")
	assert.Equal(t, "abc", GetHeaderTenantID(c))
}

func TestGetHeaderUserID_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Equal(t, "", GetHeaderUserID(c))
}

func TestGetHeaderUserID_ReturnsValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "uid-999")
	assert.Equal(t, "uid-999", GetHeaderUserID(c))
}

func TestGetHeaderUserEmail_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Equal(t, "", GetHeaderUserEmail(c))
}

func TestGetHeaderUserEmail_ReturnsValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_email", "test@example.com")
	assert.Equal(t, "test@example.com", GetHeaderUserEmail(c))
}

func TestGetHeaderVendorID_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Equal(t, "", GetHeaderVendorID(c))
}

func TestGetHeaderVendorID_ReturnsValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("vendor_id", "vendor-xyz")
	assert.Equal(t, "vendor-xyz", GetHeaderVendorID(c))
}

// ----- Table-driven: RequireUser / RequireTenant combinations ----------------

func TestHeaderAuth_RequirementCombinations(t *testing.T) {
	tests := []struct {
		name          string
		cfg           HeaderAuthConfig
		tenantHeader  string
		userHeader    string
		wantStatus    int
	}{
		{
			name:         "both required, both present",
			cfg:          HeaderAuthConfig{RequireTenant: true, RequireUser: true},
			tenantHeader: "ten-1",
			userHeader:   "usr-1",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "both required, tenant missing",
			cfg:          HeaderAuthConfig{RequireTenant: true, RequireUser: true},
			tenantHeader: "",
			userHeader:   "usr-1",
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "both required, user missing",
			cfg:          HeaderAuthConfig{RequireTenant: true, RequireUser: true},
			tenantHeader: "ten-1",
			userHeader:   "",
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "neither required, both absent",
			cfg:          HeaderAuthConfig{RequireTenant: false, RequireUser: false},
			tenantHeader: "",
			userHeader:   "",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "only tenant required, tenant present",
			cfg:          HeaderAuthConfig{RequireTenant: true, RequireUser: false},
			tenantHeader: "ten-only",
			userHeader:   "",
			wantStatus:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(HeaderAuth(tt.cfg))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.tenantHeader != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantHeader)
			}
			if tt.userHeader != "" {
				req.Header.Set("X-User-ID", tt.userHeader)
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
