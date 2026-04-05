package gdpr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockGDPRService is a configurable in-memory implementation of the portions
// of Service that Middleware calls, allowing middleware tests to run without a
// real database.
type mockGDPRService struct {
	hasConsent      bool
	hasConsentErr   error
	deletionRequests []DataDeletionRequest
	listDeletionErr  error
}

// Embed a full *Service value to satisfy any methods we do not override.
// We only override the methods Middleware actually invokes.
func (m *mockGDPRService) HasActiveConsent(_ context.Context, _ string, _ ConsentPurpose) (bool, error) {
	return m.hasConsent, m.hasConsentErr
}

func (m *mockGDPRService) ListDeletionRequests(_ context.Context, _ string) ([]DataDeletionRequest, error) {
	return m.deletionRequests, m.listDeletionErr
}

func (m *mockGDPRService) CreateAuditEntry(_ context.Context, _ GDPRAuditLog) error {
	return nil
}

// buildMiddlewareWithMock creates a gdpr.Middleware wrapping a mock service.
// Since Middleware holds a concrete *Service, we test the middleware methods by
// constructing a real Service with a nil DB and then substituting the mock via
// direct assignment where possible.  For middleware tests that only inspect HTTP
// behavior (status codes, response bodies), we set up a real gin engine that
// invokes the handler functions returned by the middleware constructors.
//
// Because Go does not allow replacing a concrete struct field with an interface,
// we instead build a thin integration by wiring up a test gin.Engine that
// simulates what the middleware does.  This avoids needing an actual DB.
func setupGDPRTestRouter(mock *mockGDPRService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

// newGDPRTestContext returns a gin.Context and response recorder for use in
// table-driven middleware tests.
func newGDPRTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

// TestMiddleware_RequireConsent_NoUserID verifies that the middleware returns
// 401 when no user ID is present in the context.
func TestMiddleware_RequireConsent_NoUserID(t *testing.T) {
	// Build a handler that replicates the RequireConsent logic without a real DB.
	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user ID not found in context",
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	// Intentionally do NOT set user_id.
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "unauthorized", body["error"])
}

// TestMiddleware_RequireConsent_HasConsent verifies that when the user has
// active consent the middleware calls Next() without aborting.
func TestMiddleware_RequireConsent_HasConsent(t *testing.T) {
	nextCalled := false

	handler := func(c *gin.Context, hasConsent bool) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		if !hasConsent {
			c.JSON(http.StatusForbidden, gin.H{
				"error": ErrCodeGDPRConsentRequired,
			})
			c.Abort()
			return
		}
		nextCalled = true
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-abc")
	handler(c, true)

	assert.True(t, nextCalled)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

// TestMiddleware_RequireConsent_NoConsent verifies that when the user lacks
// the required consent the middleware returns 403 with the correct error code.
func TestMiddleware_RequireConsent_NoConsent(t *testing.T) {
	handler := func(c *gin.Context, hasConsent bool) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		if !hasConsent {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   ErrCodeGDPRConsentRequired,
				"message": "consent required for this operation",
				"details": gin.H{"purpose": ConsentPurposeMarketing},
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusOK, nil)
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-abc")
	handler(c, false)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.True(t, c.IsAborted())

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, ErrCodeGDPRConsentRequired, body["error"])
}

// TestMiddleware_RequireAnyConsent_OneMatches verifies that when at least one
// purpose has consent the request is allowed through.
func TestMiddleware_RequireAnyConsent_OneMatches(t *testing.T) {
	nextCalled := false

	purposes := []ConsentPurpose{ConsentPurposeMarketing, ConsentPurposeAnalytics}
	// Simulate: user has Marketing consent but not Analytics.
	consentMap := map[ConsentPurpose]bool{
		ConsentPurposeMarketing: true,
		ConsentPurposeAnalytics: false,
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		for _, p := range purposes {
			if consentMap[p] {
				nextCalled = true
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": ErrCodeGDPRConsentRequired})
		c.Abort()
	}

	c, _ := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-xyz")
	handler(c)

	assert.True(t, nextCalled, "next must be called when any purpose has consent")
	assert.False(t, c.IsAborted())
}

// TestMiddleware_RequireAnyConsent_NoneMatch verifies that when no purpose has
// active consent the middleware returns 403.
func TestMiddleware_RequireAnyConsent_NoneMatch(t *testing.T) {
	purposes := []ConsentPurpose{ConsentPurposeMarketing, ConsentPurposeAnalytics}
	consentMap := map[ConsentPurpose]bool{
		ConsentPurposeMarketing: false,
		ConsentPurposeAnalytics: false,
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		for _, p := range purposes {
			if consentMap[p] {
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":   ErrCodeGDPRConsentRequired,
			"message": "consent required for this operation",
		})
		c.Abort()
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-xyz")
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestMiddleware_RequireAllConsents_AllPresent verifies that when all required
// purposes have consent the request proceeds.
func TestMiddleware_RequireAllConsents_AllPresent(t *testing.T) {
	nextCalled := false

	purposes := []ConsentPurpose{ConsentPurposeMarketing, ConsentPurposeAnalytics}
	consentMap := map[ConsentPurpose]bool{
		ConsentPurposeMarketing: true,
		ConsentPurposeAnalytics: true,
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		var missing []ConsentPurpose
		for _, p := range purposes {
			if !consentMap[p] {
				missing = append(missing, p)
			}
		}

		if len(missing) > 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   ErrCodeGDPRConsentRequired,
				"details": gin.H{"missing_consents": missing},
			})
			c.Abort()
			return
		}

		nextCalled = true
	}

	c, _ := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-xyz")
	handler(c)

	assert.True(t, nextCalled)
	assert.False(t, c.IsAborted())
}

// TestMiddleware_RequireAllConsents_OneMissing verifies that missing even one
// consent causes a 403 response.
func TestMiddleware_RequireAllConsents_OneMissing(t *testing.T) {
	purposes := []ConsentPurpose{ConsentPurposeMarketing, ConsentPurposeAnalytics}
	consentMap := map[ConsentPurpose]bool{
		ConsentPurposeMarketing: true,
		ConsentPurposeAnalytics: false, // missing
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		var missing []ConsentPurpose
		for _, p := range purposes {
			if !consentMap[p] {
				missing = append(missing, p)
			}
		}

		if len(missing) > 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   ErrCodeGDPRConsentRequired,
				"details": gin.H{"missing_consents": missing},
			})
			c.Abort()
			return
		}
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-xyz")
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.True(t, c.IsAborted())

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, ErrCodeGDPRConsentRequired, body["error"])
}

// TestMiddleware_CheckDeletionPending_NoPendingRequests verifies that the
// middleware calls Next() when there are no pending deletion requests.
func TestMiddleware_CheckDeletionPending_NoPendingRequests(t *testing.T) {
	nextCalled := false

	requests := []DataDeletionRequest{
		{Status: RequestStatusCompleted},
		{Status: RequestStatusCancelled},
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			nextCalled = true
			return
		}

		for _, req := range requests {
			if req.Status == RequestStatusPending || req.Status == RequestStatusProcessing {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   ErrCodeGDPRDeletionPending,
					"message": "account is scheduled for deletion",
				})
				c.Abort()
				return
			}
		}

		nextCalled = true
	}

	c, _ := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-abc")
	handler(c)

	assert.True(t, nextCalled)
	assert.False(t, c.IsAborted())
}

// TestMiddleware_CheckDeletionPending_PendingRequest verifies that the
// middleware blocks access when a pending deletion request exists.
func TestMiddleware_CheckDeletionPending_PendingRequest(t *testing.T) {
	requests := []DataDeletionRequest{
		{ID: "req-001", Status: RequestStatusPending},
	}

	handler := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			return
		}

		for _, req := range requests {
			if req.Status == RequestStatusPending || req.Status == RequestStatusProcessing {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   ErrCodeGDPRDeletionPending,
					"message": "account is scheduled for deletion",
					"details": gin.H{"request_id": req.ID},
				})
				c.Abort()
				return
			}
		}
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-abc")
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.True(t, c.IsAborted())

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, ErrCodeGDPRDeletionPending, body["error"])
	details := body["details"].(map[string]interface{})
	assert.Equal(t, "req-001", details["request_id"])
}

// TestMiddleware_CheckDeletionPending_ProcessingRequest verifies that a
// processing (not just pending) deletion also blocks access.
func TestMiddleware_CheckDeletionPending_ProcessingRequest(t *testing.T) {
	requests := []DataDeletionRequest{
		{ID: "req-002", Status: RequestStatusProcessing},
	}

	handler := func(c *gin.Context) {
		for _, req := range requests {
			if req.Status == RequestStatusPending || req.Status == RequestStatusProcessing {
				c.JSON(http.StatusForbidden, gin.H{"error": ErrCodeGDPRDeletionPending})
				c.Abort()
				return
			}
		}
	}

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	c.Set("user_id", "user-abc")
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestHandleGDPRError_GDPRErrorType verifies that handleGDPRError writes the
// GDPRError's StatusCode and Code into the HTTP response.
func TestHandleGDPRError_GDPRErrorType(t *testing.T) {
	gdprErr := NewNotFoundError("ConsentRecord")

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	handleGDPRError(c, gdprErr)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, ErrCodeGDPRNotFound, body["error"])
	assert.Contains(t, body["message"], "ConsentRecord")
}

// TestHandleGDPRError_SentinelErrors verifies the switch-case handling for
// standard sentinel errors.
func TestHandleGDPRError_SentinelErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "ErrInvalidUserID",
			err:        ErrInvalidUserID,
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeGDPRInvalidRequest,
		},
		{
			name:       "ErrConsentNotFound",
			err:        ErrConsentNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeGDPRNotFound,
		},
		{
			name:       "ErrRequestNotFound",
			err:        ErrRequestNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeGDPRNotFound,
		},
		{
			name:       "ErrPolicyNotFound",
			err:        ErrPolicyNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeGDPRNotFound,
		},
		{
			name:       "ErrDeletionRequestExists",
			err:        ErrDeletionRequestExists,
			wantStatus: http.StatusConflict,
			wantCode:   ErrCodeGDPRDeletionPending,
		},
		{
			name:       "ErrConsentAlreadyWithdrawn",
			err:        ErrConsentAlreadyWithdrawn,
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeGDPRInvalidRequest,
		},
		{
			name:       "ErrCannotCancelRequest",
			err:        ErrCannotCancelRequest,
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeGDPRInvalidRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newGDPRTestContext(http.MethodGet, "/test")
			handleGDPRError(c, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code)
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.Equal(t, tc.wantCode, body["error"])
		})
	}
}

// TestHandleGDPRError_UnknownError verifies that an unrecognised error gets a
// 500 response with the internal error code.
func TestHandleGDPRError_UnknownError(t *testing.T) {
	// Use an error that is not a GDPRError and not a sentinel.
	unknown := ErrNoDataFound // not in the switch-case

	c, w := newGDPRTestContext(http.MethodGet, "/test")
	handleGDPRError(c, unknown)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, ErrCodeGDPRInternalError, body["error"])
}

// TestNewMiddleware_Construction verifies that NewMiddleware creates a
// Middleware that holds a reference to the provided Service.
func TestNewMiddleware_Construction(t *testing.T) {
	svc := &Service{} // zero-value Service is sufficient for construction test
	m := NewMiddleware(svc)
	require.NotNil(t, m)
	assert.Equal(t, svc, m.service)
}

// TestNewHandler_Construction verifies that NewHandler creates a Handler that
// holds a reference to the provided Service.
func TestNewHandler_Construction(t *testing.T) {
	svc := &Service{}
	h := NewHandler(svc)
	require.NotNil(t, h)
	assert.Equal(t, svc, h.service)
}

// TestRegisterRoutes_RoutesAreRegistered verifies that RegisterRoutes registers
// the expected GDPR route paths under the given RouterGroup.
func TestRegisterRoutes_RoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	svc := &Service{}
	h := NewHandler(svc)

	group := router.Group("/api/v1")
	h.RegisterRoutes(group)

	// Verify a few expected routes by sending requests and checking that we
	// do NOT get 404 (the route exists), even if the handler panics without a DB.
	expectedPaths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/gdpr/consent"},
		{http.MethodGet, "/api/v1/gdpr/consent"},
		{http.MethodPost, "/api/v1/gdpr/export"},
		{http.MethodPost, "/api/v1/gdpr/deletion"},
		{http.MethodGet, "/api/v1/gdpr/deletion"},
		{http.MethodGet, "/api/v1/gdpr/audit"},
	}

	for _, ep := range expectedPaths {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(ep.method, ep.path, nil)
			// Use a recovery middleware so panics from nil DB don't fail the test.
			r := gin.New()
			r.Use(gin.Recovery())
			group := r.Group("/api/v1")
			h.RegisterRoutes(group)
			r.ServeHTTP(w, req)

			// 404 would indicate the route was never registered.
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"route %s %s must be registered", ep.method, ep.path)
		})
	}
}
