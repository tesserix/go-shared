package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// NewApprovalClient
// -----------------------------------------------------------------------------

func TestNewApprovalClient_UsesDefaultBaseURLWhenEmpty(t *testing.T) {
	client := NewApprovalClient("")

	assert.Equal(t, "", client.baseURL)
}

func TestNewApprovalClient_UsesProvidedBaseURL(t *testing.T) {
	client := NewApprovalClient("http://custom-approval-service:9000")

	assert.Equal(t, "http://custom-approval-service:9000", client.baseURL)
}

func TestNewApprovalClient_HasHTTPClientWithTimeout(t *testing.T) {
	client := NewApprovalClient("http://example.com")

	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
}

// -----------------------------------------------------------------------------
// CheckApproval
// -----------------------------------------------------------------------------

func TestCheckApproval_SuccessfulCheck(t *testing.T) {
	requesterID := uuid.New()
	workflowID := uuid.New()

	serverResponse := CheckResponse{
		RequiresApproval:     true,
		AutoApproved:         false,
		WorkflowID:           &workflowID,
		RequiredApproverRole: "manager",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CheckRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "/api/v1/approvals/check", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "PRODUCT_CREATE", req.ActionType)
		assert.Equal(t, requesterID, req.RequesterID)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(serverResponse)
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	req := CheckRequest{
		ActionType:  "PRODUCT_CREATE",
		RequesterID: requesterID,
		ActionData:  map[string]interface{}{"productName": "Widget"},
	}

	resp, err := client.CheckApproval(context.Background(), "tenant-123", "auth-token-abc", req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.RequiresApproval)
	assert.False(t, resp.AutoApproved)
	assert.Equal(t, workflowID, *resp.WorkflowID)
	assert.Equal(t, "manager", resp.RequiredApproverRole)
}

func TestCheckApproval_SetsContentTypeHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, err := client.CheckApproval(context.Background(), "tenant-abc", "", CheckRequest{})
	require.NoError(t, err)
}

func TestCheckApproval_SetsTenantIDHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "my-tenant-id", r.Header.Get("x-jwt-claim-tenant-id"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, err := client.CheckApproval(context.Background(), "my-tenant-id", "", CheckRequest{})
	require.NoError(t, err)
}

func TestCheckApproval_SetsAuthorizationHeaderWhenTokenProvided(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-jwt-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, err := client.CheckApproval(context.Background(), "tenant-id", "my-jwt-token", CheckRequest{})
	require.NoError(t, err)
}

func TestCheckApproval_OmitsAuthorizationHeaderWhenTokenEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"),
			"Authorization header should not be set when token is empty")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, err := client.CheckApproval(context.Background(), "tenant-id", "", CheckRequest{})
	require.NoError(t, err)
}

func TestCheckApproval_Non200StatusReturnsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewApprovalClient(server.URL)
			resp, err := client.CheckApproval(context.Background(), "tenant", "token", CheckRequest{})

			assert.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), "unexpected status code")
		})
	}
}

func TestCheckApproval_RespectsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server — the context should cancel before this returns
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewApprovalClient(server.URL)
	resp, err := client.CheckApproval(ctx, "tenant", "token", CheckRequest{})

	assert.Error(t, err, "Should return an error when context is cancelled")
	assert.Nil(t, resp)
}

// -----------------------------------------------------------------------------
// CreateRequest
// -----------------------------------------------------------------------------

func TestCreateRequest_SuccessfulCreation(t *testing.T) {
	requesterID := uuid.New()
	workflowID := uuid.New()
	resourceID := uuid.New()

	serverResponse := ApprovalRequest{
		ID:          uuid.New(),
		TenantID:    "tenant-xyz",
		WorkflowID:  workflowID,
		RequesterID: requesterID,
		Status:      "pending",
		ActionType:  "ORDER_REFUND",
		ActionData:  map[string]interface{}{"amount": 100.0},
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		CreatedAt:   time.Now(),
	}

	var receivedInput CreateRequestInput

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedInput)
		require.NoError(t, err)

		assert.Equal(t, "/api/v1/approvals", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(serverResponse)
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	input := CreateRequestInput{
		WorkflowName: "order-refund-workflow",
		ActionType:   "ORDER_REFUND",
		ActionData:   map[string]interface{}{"orderId": "order-123"},
		ResourceType: "order",
		ResourceID:   &resourceID,
		Reason:       "Customer requested refund",
		Priority:     "high",
	}

	result, err := client.CreateRequest(context.Background(), "tenant-xyz", "auth-token", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, serverResponse.ID, result.ID)
	assert.Equal(t, "pending", result.Status)
	assert.Equal(t, "ORDER_REFUND", result.ActionType)
	// Verify the server received the correct input
	assert.Equal(t, "order-refund-workflow", receivedInput.WorkflowName)
	assert.Equal(t, "ORDER_REFUND", receivedInput.ActionType)
	assert.Equal(t, "order", receivedInput.ResourceType)
}

func TestCreateRequest_Accepts200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Some approval service implementations return 200 instead of 201
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ApprovalRequest{
			ID:     uuid.New(),
			Status: "pending",
		})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	result, err := client.CreateRequest(context.Background(), "tenant", "token", CreateRequestInput{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCreateRequest_SetsRequiredHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "tenant-abc", r.Header.Get("x-jwt-claim-tenant-id"))
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ApprovalRequest{ID: uuid.New()})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, err := client.CreateRequest(context.Background(), "tenant-abc", "my-token", CreateRequestInput{})
	require.NoError(t, err)
}

func TestCreateRequest_Non201Non200StatusReturnsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"404 Not Found", http.StatusNotFound},
		{"409 Conflict", http.StatusConflict},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewApprovalClient(server.URL)
			result, err := client.CreateRequest(context.Background(), "tenant", "token", CreateRequestInput{})

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "unexpected status code")
		})
	}
}

func TestCreateRequest_SendsRequestBody(t *testing.T) {
	resourceID := uuid.New()
	var received CreateRequestInput

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ApprovalRequest{ID: uuid.New()})
	}))
	defer server.Close()

	input := CreateRequestInput{
		WorkflowName: "test-workflow",
		ActionType:   "PRICE_CHANGE",
		ActionData:   map[string]interface{}{"newPrice": 99.99},
		ResourceType: "product",
		ResourceID:   &resourceID,
		Reason:       "Promotional pricing",
		Priority:     "normal",
	}

	client := NewApprovalClient(server.URL)
	_, err := client.CreateRequest(context.Background(), "tenant", "token", input)
	require.NoError(t, err)

	assert.Equal(t, "test-workflow", received.WorkflowName)
	assert.Equal(t, "PRICE_CHANGE", received.ActionType)
	assert.Equal(t, "product", received.ResourceType)
	assert.Equal(t, &resourceID, received.ResourceID)
	assert.Equal(t, "Promotional pricing", received.Reason)
	assert.Equal(t, "normal", received.Priority)
}

// -----------------------------------------------------------------------------
// CheckAndCreateIfNeeded
// -----------------------------------------------------------------------------

func TestCheckAndCreateIfNeeded_NoApprovalNeeded_AutoApproved(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Only the check endpoint should be hit
		assert.Equal(t, "/api/v1/approvals/check", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{
			RequiresApproval: true,
			AutoApproved:     true, // auto-approved means no further action needed
		})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	checkResp, approvalReq, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"tenant", "token",
		CheckRequest{ActionType: "VIEW"},
		CreateRequestInput{WorkflowName: "view-workflow"},
	)

	require.NoError(t, err)
	require.NotNil(t, checkResp)
	assert.Nil(t, approvalReq, "No approval request should be created when auto-approved")
	assert.True(t, checkResp.AutoApproved)
	assert.Equal(t, 1, callCount, "Only the check endpoint should be called")
}

func TestCheckAndCreateIfNeeded_NoApprovalRequired(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CheckResponse{
			RequiresApproval: false,
			AutoApproved:     false,
		})
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	checkResp, approvalReq, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"tenant", "token",
		CheckRequest{ActionType: "LIST"},
		CreateRequestInput{WorkflowName: "list-workflow"},
	)

	require.NoError(t, err)
	require.NotNil(t, checkResp)
	assert.Nil(t, approvalReq, "No approval request should be created when approval is not required")
	assert.False(t, checkResp.RequiresApproval)
	assert.Equal(t, 1, callCount, "Only the check endpoint should be called")
}

func TestCheckAndCreateIfNeeded_ApprovalNeededCreatesRequest(t *testing.T) {
	workflowID := uuid.New()
	newRequestID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/approvals/check":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CheckResponse{
				RequiresApproval:     true,
				AutoApproved:         false,
				WorkflowID:           &workflowID,
				RequiredApproverRole: "admin",
			})

		case "/api/v1/approvals":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ApprovalRequest{
				ID:         newRequestID,
				WorkflowID: workflowID,
				Status:     "pending",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	checkResp, approvalReq, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"tenant", "token",
		CheckRequest{ActionType: "DELETE_PRODUCT"},
		CreateRequestInput{WorkflowName: "product-delete-workflow"},
	)

	require.NoError(t, err)
	require.NotNil(t, checkResp)
	require.NotNil(t, approvalReq)

	assert.True(t, checkResp.RequiresApproval)
	assert.False(t, checkResp.AutoApproved)
	assert.Equal(t, newRequestID, approvalReq.ID)
	assert.Equal(t, "pending", approvalReq.Status)

	// The check response should have been updated with the created approval request ID
	require.NotNil(t, checkResp.ApprovalRequestID)
	assert.Equal(t, newRequestID, *checkResp.ApprovalRequestID)
}

func TestCheckAndCreateIfNeeded_CheckFailsReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the approval service being unavailable during check
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	checkResp, approvalReq, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"tenant", "token",
		CheckRequest{ActionType: "ANY_ACTION"},
		CreateRequestInput{},
	)

	assert.Error(t, err)
	assert.Nil(t, checkResp)
	assert.Nil(t, approvalReq)
	assert.Contains(t, err.Error(), "failed to check approval")
}

func TestCheckAndCreateIfNeeded_CheckSucceedsButCreateFails(t *testing.T) {
	workflowID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/approvals/check":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CheckResponse{
				RequiresApproval: true,
				AutoApproved:     false,
				WorkflowID:       &workflowID,
			})

		case "/api/v1/approvals":
			// Simulate creation failure
			w.WriteHeader(http.StatusInternalServerError)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	checkResp, approvalReq, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"tenant", "token",
		CheckRequest{ActionType: "BULK_DELETE"},
		CreateRequestInput{WorkflowName: "bulk-delete-workflow"},
	)

	assert.Error(t, err)
	// The check response is returned even when creation fails
	assert.NotNil(t, checkResp, "checkResp should be returned even when creation fails")
	assert.Nil(t, approvalReq)
	assert.Contains(t, err.Error(), "failed to create approval request")
}

func TestCheckAndCreateIfNeeded_PassesTenantAndAuthToSubCalls(t *testing.T) {
	workflowID := uuid.New()
	checkTenantID := ""
	checkAuthToken := ""
	createTenantID := ""
	createAuthToken := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/approvals/check":
			checkTenantID = r.Header.Get("x-jwt-claim-tenant-id")
			checkAuthToken = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CheckResponse{
				RequiresApproval: true,
				AutoApproved:     false,
				WorkflowID:       &workflowID,
			})

		case "/api/v1/approvals":
			createTenantID = r.Header.Get("x-jwt-claim-tenant-id")
			createAuthToken = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ApprovalRequest{ID: uuid.New()})
		}
	}))
	defer server.Close()

	client := NewApprovalClient(server.URL)
	_, _, err := client.CheckAndCreateIfNeeded(
		context.Background(),
		"expected-tenant", "expected-token",
		CheckRequest{ActionType: "TEST"},
		CreateRequestInput{WorkflowName: "test-wf"},
	)
	require.NoError(t, err)

	assert.Equal(t, "expected-tenant", checkTenantID)
	assert.Equal(t, "Bearer expected-token", checkAuthToken)
	assert.Equal(t, "expected-tenant", createTenantID)
	assert.Equal(t, "Bearer expected-token", createAuthToken)
}
