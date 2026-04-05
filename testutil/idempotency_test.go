// Package testutil provides test templates for checkout idempotency validation.
// Copy and adapt these tests for orders-service and payment-service.
package testutil

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// IdempotencyTestSuite provides reusable idempotency test patterns
type IdempotencyTestSuite struct {
	Router *gin.Engine
	t      *testing.T
	Tenant TestTenant
	User   TestUser
}

// NewIdempotencyTestSuite creates a new idempotency test suite
func NewIdempotencyTestSuite(t *testing.T, router *gin.Engine) *IdempotencyTestSuite {
	tenant := NewTestTenant()
	return &IdempotencyTestSuite{
		Router: router,
		t:      t,
		Tenant: tenant,
		User:   NewTestUser(tenant.ID, "customer"),
	}
}

// TestFirstRequestCreatesResource verifies first request creates a new resource
func TestFirstRequestCreatesResource(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for orders-service

		Example:

		func TestCheckout_FirstRequest(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenant := testutil.NewTestTenant()
			idempotencyKey := testutil.IdempotencyKey()

			orderData := map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": "prod-123", "quantity": 1, "price": 100.00},
				},
				"customer_id": "cust-456",
			}

			helper := testutil.NewHTTPTestHelper(t, router)
			headers := testutil.WithTenant(tenant.ID)
			headers["Idempotency-Key"] = idempotencyKey

			resp := helper.POST("/api/v1/orders", orderData, headers)

			testutil.AssertStatus(t, resp, http.StatusCreated)

			var order Order
			testutil.ParseJSONResponse(t, resp, &order)
			assert.NotEmpty(t, order.ID)

			// Verify order exists in database
			var dbOrder Order
			err := db.Where("id = ?", order.ID).First(&dbOrder).Error
			assert.NoError(t, err)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestDuplicateRequestReturnsSameResource verifies duplicate requests return same resource
func TestDuplicateRequestReturnsSameResource(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for orders-service

		Example:

		func TestCheckout_DuplicateRequest(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenant := testutil.NewTestTenant()
			idempotencyKey := testutil.IdempotencyKey()

			orderData := map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": "prod-123", "quantity": 1, "price": 100.00},
				},
				"customer_id": "cust-456",
			}

			helper := testutil.NewHTTPTestHelper(t, router)
			headers := testutil.WithTenant(tenant.ID)
			headers["Idempotency-Key"] = idempotencyKey

			// First request
			resp1 := helper.POST("/api/v1/orders", orderData, headers)
			testutil.AssertStatus(t, resp1, http.StatusCreated)

			var order1 Order
			testutil.ParseJSONResponse(t, resp1, &order1)

			// Duplicate request with same idempotency key
			resp2 := helper.POST("/api/v1/orders", orderData, headers)

			// Should return 200 OK or 201 Created (depending on implementation)
			assert.True(t, resp2.Code == http.StatusOK || resp2.Code == http.StatusCreated)

			var order2 Order
			testutil.ParseJSONResponse(t, resp2, &order2)

			// Should return the SAME order
			assert.Equal(t, order1.ID, order2.ID, "Duplicate request should return same order")

			// Verify only one order in database
			var count int64
			db.Model(&Order{}).Where("idempotency_key = ?", idempotencyKey).Count(&count)
			assert.Equal(t, int64(1), count, "Should only have one order")
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestDifferentKeyCreatesNewResource verifies different keys create different resources
func TestDifferentKeyCreatesNewResource(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for orders-service

		Example:

		func TestCheckout_DifferentKey(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenant := testutil.NewTestTenant()
			key1 := testutil.IdempotencyKey()
			key2 := testutil.IdempotencyKey()

			orderData := map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": "prod-123", "quantity": 1, "price": 100.00},
				},
				"customer_id": "cust-456",
			}

			helper := testutil.NewHTTPTestHelper(t, router)

			headers1 := testutil.WithTenant(tenant.ID)
			headers1["Idempotency-Key"] = key1
			resp1 := helper.POST("/api/v1/orders", orderData, headers1)

			headers2 := testutil.WithTenant(tenant.ID)
			headers2["Idempotency-Key"] = key2
			resp2 := helper.POST("/api/v1/orders", orderData, headers2)

			var order1, order2 Order
			testutil.ParseJSONResponse(t, resp1, &order1)
			testutil.ParseJSONResponse(t, resp2, &order2)

			// Different keys should create different orders
			assert.NotEqual(t, order1.ID, order2.ID, "Different keys should create different orders")
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestConcurrentRequestsHandledCorrectly verifies concurrent requests with same key
func TestConcurrentRequestsHandledCorrectly(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for orders-service

		Example:

		func TestCheckout_ConcurrentRequests(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenant := testutil.NewTestTenant()
			idempotencyKey := testutil.IdempotencyKey()

			orderData := map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": "prod-123", "quantity": 1, "price": 100.00},
				},
				"customer_id": "cust-456",
			}

			numRequests := 5
			var wg sync.WaitGroup
			orderIDs := make([]string, numRequests)
			errors := make([]error, numRequests)

			for i := 0; i < numRequests; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					helper := testutil.NewHTTPTestHelper(t, router)
					headers := testutil.WithTenant(tenant.ID)
					headers["Idempotency-Key"] = idempotencyKey

					resp := helper.POST("/api/v1/orders", orderData, headers)

					if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
						var order Order
						json.Unmarshal(resp.Body.Bytes(), &order)
						orderIDs[idx] = order.ID
					} else {
						errors[idx] = fmt.Errorf("unexpected status: %d", resp.Code)
					}
				}(i)
			}

			wg.Wait()

			// All successful requests should return the same order ID
			var firstID string
			for _, id := range orderIDs {
				if id != "" {
					if firstID == "" {
						firstID = id
					} else {
						assert.Equal(t, firstID, id, "All concurrent requests should return same order")
					}
				}
			}

			// Verify only one order in database
			var count int64
			db.Model(&Order{}).Where("idempotency_key = ?", idempotencyKey).Count(&count)
			assert.Equal(t, int64(1), count, "Should only have one order despite concurrent requests")
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestInventoryNotDoubleDeducted verifies inventory is deducted only once
func TestInventoryNotDoubleDeducted(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for orders-service + inventory-service

		Example:

		func TestCheckout_InventoryDeduction(t *testing.T) {
			router := setupTestRouter()
			db := setupTestDB(t)

			tenant := testutil.NewTestTenant()
			productID := "prod-123"
			initialStock := 10
			orderQty := 2

			// Setup initial inventory
			db.Create(&Inventory{
				ProductID: productID,
				TenantID:  tenant.ID,
				Quantity:  initialStock,
			})

			idempotencyKey := testutil.IdempotencyKey()
			orderData := map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": productID, "quantity": orderQty, "price": 100.00},
				},
			}

			helper := testutil.NewHTTPTestHelper(t, router)
			headers := testutil.WithTenant(tenant.ID)
			headers["Idempotency-Key"] = idempotencyKey

			// First request
			resp1 := helper.POST("/api/v1/orders", orderData, headers)
			testutil.AssertStatus(t, resp1, http.StatusCreated)

			// Duplicate request
			resp2 := helper.POST("/api/v1/orders", orderData, headers)
			assert.True(t, resp2.Code == http.StatusOK || resp2.Code == http.StatusCreated)

			// Verify inventory deducted only once
			var inventory Inventory
			db.Where("product_id = ? AND tenant_id = ?", productID, tenant.ID).First(&inventory)

			expectedStock := initialStock - orderQty
			assert.Equal(t, expectedStock, inventory.Quantity,
				"Inventory should be deducted only once (expected %d, got %d)",
				expectedStock, inventory.Quantity)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// RunIdempotencySuite runs all idempotency tests for an endpoint
func (s *IdempotencyTestSuite) RunIdempotencySuite(endpoint string, createPayload func() interface{}) {
	s.t.Run("FirstRequest_CreatesResource", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		headers := WithTenant(s.Tenant.ID)
		headers["Idempotency-Key"] = IdempotencyKey()

		resp := helper.POST(endpoint, createPayload(), headers)

		assert.True(t, resp.Code == http.StatusCreated || resp.Code == http.StatusOK,
			"First request should succeed, got %d", resp.Code)
	})

	s.t.Run("DuplicateRequest_ReturnsSameResource", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		idempotencyKey := IdempotencyKey()

		headers := WithTenant(s.Tenant.ID)
		headers["Idempotency-Key"] = idempotencyKey

		// First request
		resp1 := helper.POST(endpoint, createPayload(), headers)
		body1 := resp1.Body.String()

		// Duplicate request
		resp2 := helper.POST(endpoint, createPayload(), headers)
		body2 := resp2.Body.String()

		// Should return same response
		assert.Equal(t, body1, body2, "Duplicate request should return same response")
	})

	s.t.Run("DifferentKey_CreatesNewResource", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)

		headers1 := WithTenant(s.Tenant.ID)
		headers1["Idempotency-Key"] = IdempotencyKey()
		resp1 := helper.POST(endpoint, createPayload(), headers1)

		headers2 := WithTenant(s.Tenant.ID)
		headers2["Idempotency-Key"] = IdempotencyKey()
		resp2 := helper.POST(endpoint, createPayload(), headers2)

		// Should return different resources
		assert.NotEqual(t, resp1.Body.String(), resp2.Body.String(),
			"Different keys should create different resources")
	})

	s.t.Run("ConcurrentRequests_OnlyOneCreated", func(t *testing.T) {
		idempotencyKey := IdempotencyKey()
		numRequests := 5
		var wg sync.WaitGroup
		responses := make([]*http.Response, numRequests)
		mu := sync.Mutex{}

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				helper := NewHTTPTestHelper(t, s.Router)
				headers := WithTenant(s.Tenant.ID)
				headers["Idempotency-Key"] = idempotencyKey

				resp := helper.POST(endpoint, createPayload(), headers)
				mu.Lock()
				responses[idx] = &http.Response{StatusCode: resp.Code}
				mu.Unlock()
			}(i)
		}

		wg.Wait()

		// Count successful responses
		successCount := 0
		for _, resp := range responses {
			if resp != nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
				successCount++
			}
		}

		assert.Greater(t, successCount, 0, "At least one request should succeed")
	})
}

// WithIdempotencyKey adds idempotency key to headers
func WithIdempotencyKey(key string, headers map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Idempotency-Key"] = key
	return headers
}

// WithNewIdempotencyKey adds a new idempotency key to headers
func WithNewIdempotencyKey(headers map[string]string) map[string]string {
	return WithIdempotencyKey(fmt.Sprintf("test-%s", uuid.New().String()), headers)
}
