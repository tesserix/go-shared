package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ApprovalClient provides methods to interact with the approval service
type ApprovalClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewApprovalClient creates a new ApprovalClient
func NewApprovalClient(baseURL string) *ApprovalClient {
	if baseURL == "" {
		baseURL = ""
	}
	return &ApprovalClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckRequest represents a request to check if approval is needed
type CheckRequest struct {
	ActionType  string                 `json:"actionType"`
	ActionData  map[string]interface{} `json:"actionData"`
	RequesterID uuid.UUID              `json:"requesterId"`
}

// CheckResponse represents the response from an approval check
type CheckResponse struct {
	RequiresApproval     bool       `json:"requiresApproval"`
	AutoApproved         bool       `json:"autoApproved"`
	WorkflowID           *uuid.UUID `json:"workflowId,omitempty"`
	RequiredApproverRole string     `json:"requiredApproverRole,omitempty"`
	ApprovalRequestID    *uuid.UUID `json:"approvalRequestId,omitempty"`
}

// CreateRequestInput represents input for creating an approval request
type CreateRequestInput struct {
	WorkflowName string                 `json:"workflowName"`
	ActionType   string                 `json:"actionType"`
	ActionData   map[string]interface{} `json:"actionData"`
	ResourceType string                 `json:"resourceType,omitempty"`
	ResourceID   *uuid.UUID             `json:"resourceId,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	Priority     string                 `json:"priority,omitempty"`
}

// ApprovalRequest represents a created approval request
type ApprovalRequest struct {
	ID                  uuid.UUID              `json:"id"`
	TenantID            string                 `json:"tenantId"`
	WorkflowID          uuid.UUID              `json:"workflowId"`
	RequesterID         uuid.UUID              `json:"requesterId"`
	Status              string                 `json:"status"`
	ActionType          string                 `json:"actionType"`
	ActionData          map[string]interface{} `json:"actionData"`
	CurrentApproverRole string                 `json:"currentApproverRole,omitempty"`
	ExpiresAt           time.Time              `json:"expiresAt"`
	CreatedAt           time.Time              `json:"createdAt"`
}

// CheckApproval checks if an action requires approval
func (c *ApprovalClient) CheckApproval(ctx context.Context, tenantID string, authToken string, req CheckRequest) (*CheckResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/approvals/check", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)
	if authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var checkResp CheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &checkResp, nil
}

// CreateRequest creates a new approval request
func (c *ApprovalClient) CreateRequest(ctx context.Context, tenantID string, authToken string, input CreateRequestInput) (*ApprovalRequest, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/approvals", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)
	if authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var approvalReq ApprovalRequest
	if err := json.NewDecoder(resp.Body).Decode(&approvalReq); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &approvalReq, nil
}

// CheckAndCreateIfNeeded checks if approval is needed and creates a request if so
// Returns the approval request if one was created, nil if auto-approved or no approval needed
func (c *ApprovalClient) CheckAndCreateIfNeeded(ctx context.Context, tenantID, authToken string, req CheckRequest, input CreateRequestInput) (*CheckResponse, *ApprovalRequest, error) {
	// First check if approval is needed
	checkResp, err := c.CheckApproval(ctx, tenantID, authToken, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check approval: %w", err)
	}

	// If auto-approved or no approval needed, return early
	if !checkResp.RequiresApproval || checkResp.AutoApproved {
		return checkResp, nil, nil
	}

	// Create approval request
	approvalReq, err := c.CreateRequest(ctx, tenantID, authToken, input)
	if err != nil {
		return checkResp, nil, fmt.Errorf("failed to create approval request: %w", err)
	}

	checkResp.ApprovalRequestID = &approvalReq.ID
	return checkResp, approvalReq, nil
}
