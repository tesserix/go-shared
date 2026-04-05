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

// decodeTenantBody is a small helper to unmarshal the response body into a map.
func decodeTenantBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body must be valid JSON")
	return body
}

// ----- TenantMiddleware (default, no options) ---------------------------------

func TestTenantMiddleware_ExtractsXVendorID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Vendor-ID", "vendor-abc")
	router.ServeHTTP(w, req)

	assert.Equal(t, "vendor-abc", capturedVendorID)
}

func TestTenantMiddleware_FallsBackToXTenantID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-xyz")
	router.ServeHTTP(w, req)

	assert.Equal(t, "tenant-xyz", capturedVendorID)
}

func TestTenantMiddleware_VendorIDTakesPrecedenceOverTenantID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Vendor-ID", "vendor-wins")
	req.Header.Set("X-Tenant-ID", "tenant-loses")
	router.ServeHTTP(w, req)

	assert.Equal(t, "vendor-wins", capturedVendorID)
}

func TestTenantMiddleware_SetsStorefrontID(t *testing.T) {
	var capturedStorefrontID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedStorefrontID = GetStorefrontID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Vendor-ID", "vendor-1")
	req.Header.Set("X-Storefront-ID", "storefront-99")
	router.ServeHTTP(w, req)

	assert.Equal(t, "storefront-99", capturedStorefrontID)
}

// ----- TenantMiddlewareWithOptions -------------------------------------------

func TestTenantMiddlewareWithOptions_RequireVendorIDRejectsMissing(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddlewareWithOptions(TenantOptions{RequireVendorID: true}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := decodeTenantBody(t, w)
	assert.Equal(t, false, body["success"])
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "VENDOR_REQUIRED", errObj["code"])
}

func TestTenantMiddlewareWithOptions_UsesDefaultVendorID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddlewareWithOptions(TenantOptions{
		DefaultVendorID: "default-vendor",
	}))
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "default-vendor", capturedVendorID)
}

func TestTenantMiddlewareWithOptions_SkipsExcludedPaths(t *testing.T) {
	excludedPaths := []string{"/health", "/ready", "/metrics", "/swagger"}

	for _, path := range excludedPaths {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			router.Use(TenantMiddlewareWithOptions(TenantOptions{
				RequireVendorID: true,
			}))
			router.GET(path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			// No X-Vendor-ID header — middleware must skip this path entirely.
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code,
				"path %s must bypass vendor ID requirement", path)
		})
	}
}

func TestTenantMiddlewareWithOptions_SkipsCustomExcludedPaths(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddlewareWithOptions(TenantOptions{
		RequireVendorID: true,
		ExcludedPaths:   []string{"/internal"},
	}))
	router.GET("/internal/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ----- GetTenantContext ------------------------------------------------------

func TestGetTenantContext_ReturnsContext(t *testing.T) {
	var capturedCtx *TenantContext

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(TenantMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedCtx = GetTenantContext(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Vendor-ID", "v-111")
	req.Header.Set("X-Storefront-ID", "s-222")
	router.ServeHTTP(w, req)

	require.NotNil(t, capturedCtx)
	assert.Equal(t, "v-111", capturedCtx.VendorID)
	assert.Equal(t, "s-222", capturedCtx.StorefrontID)
}

func TestGetTenantContext_ReturnsNilWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	result := GetTenantContext(c)
	assert.Nil(t, result)
}

// ----- GetVendorID -----------------------------------------------------------

func TestGetVendorID_ReturnsVendorID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("vendor_id", "my-vendor")

	assert.Equal(t, "my-vendor", GetVendorID(c))
}

func TestGetVendorID_ReturnsEmptyWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.Equal(t, "", GetVendorID(c))
}

// ----- GetStorefrontID -------------------------------------------------------

func TestGetStorefrontID_ReturnsStorefrontID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("storefront_id", "my-storefront")

	assert.Equal(t, "my-storefront", GetStorefrontID(c))
}

func TestGetStorefrontID_ReturnsEmptyWhenNotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	assert.Equal(t, "", GetStorefrontID(c))
}

// ----- RequireVendorMiddleware -----------------------------------------------

func TestRequireVendorMiddleware_RejectsMissingVendorID(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(RequireVendorMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := decodeTenantBody(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "VENDOR_REQUIRED", errObj["code"])
}

func TestRequireVendorMiddleware_AcceptsXVendorID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(RequireVendorMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Vendor-ID", "vendor-from-header")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "vendor-from-header", capturedVendorID)
}

func TestRequireVendorMiddleware_AcceptsXTenantID(t *testing.T) {
	var capturedVendorID string

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)
	router.Use(RequireVendorMiddleware())
	router.GET("/test", func(c *gin.Context) {
		capturedVendorID = GetVendorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-as-vendor")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "tenant-as-vendor", capturedVendorID)
}

// ----- MustGetVendorID -------------------------------------------------------

func TestMustGetVendorID_PanicsWhenNoVendor(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	assert.Panics(t, func() {
		MustGetVendorID(c)
	}, "MustGetVendorID must panic when vendor_id is not set in context")
}

func TestMustGetVendorID_ReturnsVendorWhenSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("vendor_id", "present-vendor")

	assert.NotPanics(t, func() {
		id := MustGetVendorID(c)
		assert.Equal(t, "present-vendor", id)
	})
}
