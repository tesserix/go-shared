// Package testutil provides test templates for tenant isolation validation.
// Copy and adapt these tests for each service.
package testutil

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TenantIsolationTestSuite provides reusable tenant isolation test patterns
type TenantIsolationTestSuite struct {
	Router      *gin.Engine
	t           *testing.T
	TenantA     TestTenant
	TenantB     TestTenant
	UserA       TestUser
	UserB       TestUser
	SetupData   func(tenantID string) interface{} // Returns created resource ID
	CleanupData func(tenantID string)
}

// NewTenantIsolationTestSuite creates a new test suite
func NewTenantIsolationTestSuite(t *testing.T, router *gin.Engine) *TenantIsolationTestSuite {
	return &TenantIsolationTestSuite{
		Router:  router,
		t:       t,
		TenantA: NewTestTenant(),
		TenantB: NewTestTenant(),
		UserA:   NewTestUser("", "admin"),
		UserB:   NewTestUser("", "admin"),
	}
}

// TestDataVisibleOnlyToOwningTenant verifies tenant A's data is not visible to tenant B
// This is the template - copy and customize for your service
func TestDataVisibleOnlyToOwningTenant(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example for orders-service:

		func TestOrders_TenantIsolation(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenantA := testutil.NewTestTenant()
			tenantB := testutil.NewTestTenant()

			// Create order for tenant A
			orderA := createTestOrder(t, db, tenantA.ID)

			// Tenant A can see their order
			helper := testutil.NewHTTPTestHelper(t, router)
			resp := helper.GET("/api/v1/orders", testutil.WithTenant(tenantA.ID))
			testutil.AssertStatus(t, resp, http.StatusOK)

			var ordersA []Order
			testutil.ParseJSONResponse(t, resp, &ordersA)
			assert.Len(t, ordersA, 1)
			assert.Equal(t, orderA.ID, ordersA[0].ID)

			// Tenant B cannot see tenant A's order
			resp = helper.GET("/api/v1/orders", testutil.WithTenant(tenantB.ID))
			testutil.AssertStatus(t, resp, http.StatusOK)

			var ordersB []Order
			testutil.ParseJSONResponse(t, resp, &ordersB)
			assert.Len(t, ordersB, 0, "Tenant B should not see Tenant A's orders")
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestMissingTenantIDReturns400 verifies requests without tenant ID are rejected
func TestMissingTenantIDReturns400(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestOrders_MissingTenantID(t *testing.T) {
			router := setupTestRouter()

			helper := testutil.NewHTTPTestHelper(t, router)
			// Request without X-Tenant-ID header
			resp := helper.GET("/api/v1/orders", nil)

			// Should return 400 or 401, not 500
			assert.True(t, resp.Code == http.StatusBadRequest || resp.Code == http.StatusUnauthorized,
				"Expected 400 or 401, got %d", resp.Code)

			// Should not be internal server error
			assert.NotEqual(t, http.StatusInternalServerError, resp.Code,
				"Missing tenant should not cause 500 error")
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestCrossTenantAccessDenied verifies direct access to another tenant's resource is denied
func TestCrossTenantAccessDenied(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestOrders_CrossTenantAccess(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenantA := testutil.NewTestTenant()
			tenantB := testutil.NewTestTenant()

			// Create order for tenant A
			orderA := createTestOrder(t, db, tenantA.ID)

			helper := testutil.NewHTTPTestHelper(t, router)

			// Tenant B tries to access tenant A's order directly
			resp := helper.GET(fmt.Sprintf("/api/v1/orders/%s", orderA.ID), testutil.WithTenant(tenantB.ID))

			// Should return 404 (not found) or 403 (forbidden), not the order
			assert.True(t, resp.Code == http.StatusNotFound || resp.Code == http.StatusForbidden,
				"Cross-tenant access should be denied, got %d", resp.Code)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestTenantIDCannotBeInjectedInBody verifies tenant_id in request body is ignored
func TestTenantIDCannotBeInjectedInBody(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestOrders_TenantInjectionPrevented(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenantA := testutil.NewTestTenant()
			tenantB := testutil.NewTestTenant()

			helper := testutil.NewHTTPTestHelper(t, router)

			// Try to create order with mismatched tenant_id in body
			orderData := map[string]interface{}{
				"tenant_id": tenantB.ID, // Trying to inject different tenant
				"items": []map[string]interface{}{
					{"product_id": "prod-123", "quantity": 1},
				},
			}

			// Request with tenant A header but tenant B in body
			resp := helper.POST("/api/v1/orders", orderData, testutil.WithTenant(tenantA.ID))

			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				// If created, verify it used header tenant, not body tenant
				var order Order
				testutil.ParseJSONResponse(t, resp, &order)
				assert.Equal(t, tenantA.ID, order.TenantID,
					"Order should use header tenant_id, not body tenant_id")
			}
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestTenantScopedQueries verifies all DB queries include tenant_id filter
func TestTenantScopedQueries(t *testing.T) {
	/*
		This test requires inspecting SQL queries.
		Use GORM hooks or query logging to verify.

		Example with query logging:

		func TestOrders_QueriesIncludeTenantID(t *testing.T) {
			// Enable query logging
			db := setupTestDB(t).Debug()

			var capturedQueries []string
			db.Callback().Query().Before("*").Register("capture_query", func(tx *gorm.DB) {
				capturedQueries = append(capturedQueries, tx.Statement.SQL.String())
			})

			// Perform operations
			tenantID := "test-tenant-123"
			repo.GetOrders(ctx, tenantID)

			// Verify all SELECT queries include tenant_id
			for _, query := range capturedQueries {
				if strings.Contains(query, "SELECT") && strings.Contains(query, "orders") {
					assert.Contains(t, query, "tenant_id",
						"Query missing tenant_id filter: %s", query)
				}
			}
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// RunTenantIsolationSuite runs all tenant isolation tests for a service
func (s *TenantIsolationTestSuite) RunTenantIsolationSuite(endpoint string) {
	s.t.Run("TenantA_CanSee_OwnData", func(t *testing.T) {
		// Setup data for tenant A
		if s.SetupData != nil {
			s.SetupData(s.TenantA.ID)
		}

		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenant(s.TenantA.ID))

		AssertStatus(t, resp, http.StatusOK)
	})

	s.t.Run("TenantB_CannotSee_TenantA_Data", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenant(s.TenantB.ID))

		AssertStatus(t, resp, http.StatusOK)
		// Response should be empty or not contain tenant A's data
		assert.NotContains(t, resp.Body.String(), s.TenantA.ID)
	})

	s.t.Run("MissingTenant_Returns_4xx", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, nil)

		// Should return 400 or 401, not 500
		assert.True(t, resp.Code >= 400 && resp.Code < 500,
			"Expected 4xx, got %d", resp.Code)
		assert.NotEqual(t, http.StatusInternalServerError, resp.Code)
	})

	// Cleanup
	if s.CleanupData != nil {
		s.CleanupData(s.TenantA.ID)
		s.CleanupData(s.TenantB.ID)
	}
}
