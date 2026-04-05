// Package testutil provides shared testing utilities for all Tesseract Hub services.
// It includes helpers for tenant isolation, authentication, database setup, and HTTP testing.
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestConfig holds configuration for test setup
type TestConfig struct {
	DatabaseURL   string
	UseInMemoryDB bool
	CleanupAfter  bool
}

// DefaultTestConfig returns default test configuration
func DefaultTestConfig() TestConfig {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://dev:devpass@localhost:5432/tesseract_hub_test?sslmode=disable"
	}
	return TestConfig{
		DatabaseURL:   dbURL,
		UseInMemoryDB: false,
		CleanupAfter:  true,
	}
}

// SetupTestRouter creates a Gin router in test mode
func SetupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

// SetupTestDB creates a test database connection
func SetupTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	config := DefaultTestConfig()

	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto migrate models
	if len(models) > 0 {
		err = db.AutoMigrate(models...)
		require.NoError(t, err, "Failed to migrate test database")
	}

	// Cleanup after test
	if config.CleanupAfter {
		t.Cleanup(func() {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
		})
	}

	return db
}

// TestTenant represents a test tenant
type TestTenant struct {
	ID   string
	Slug string
	Name string
}

// NewTestTenant creates a new test tenant with unique ID
func NewTestTenant() TestTenant {
	id := uuid.New().String()
	return TestTenant{
		ID:   id,
		Slug: fmt.Sprintf("test-tenant-%s", id[:8]),
		Name: fmt.Sprintf("Test Tenant %s", id[:8]),
	}
}

// TestUser represents a test user
type TestUser struct {
	ID             string
	IDPUserID string
	Email          string
	Role           string
	TenantID       string
	Permissions    []string
}

// NewTestUser creates a new test user
func NewTestUser(tenantID string, role string) TestUser {
	id := uuid.New().String()
	return TestUser{
		ID:             id,
		IDPUserID: uuid.New().String(),
		Email:          fmt.Sprintf("test-%s@example.com", id[:8]),
		Role:           role,
		TenantID:       tenantID,
	}
}

// CreateTestUser creates a test user with default role for the given tenant.
func CreateTestUser(tenantID string) TestUser {
	return NewTestUser(tenantID, "viewer")
}

// GenerateTestJWT generates a simple HMAC-signed JWT for testing.
// Uses HS256 with the provided secret. expiresInSec sets token lifetime.
func GenerateTestJWT(user TestUser, secret string, expiresInSec int64) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":       user.IDPUserID,
		"email":     user.Email,
		"tenant_id": user.TenantID,
		"role":      user.Role,
		"exp":       now.Add(time.Duration(expiresInSec) * time.Second).Unix(),
		"iat":       now.Unix(),
	}
	if len(user.Permissions) > 0 {
		claims["permissions"] = user.Permissions
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(fmt.Sprintf("GenerateTestJWT: failed to sign token: %v", err))
	}
	return signed
}

// ContextKey is a type for context keys
type ContextKey string

// Context keys
const (
	TenantIDKey    ContextKey = "tenant_id"
	UserIDKey      ContextKey = "user_id"
	UserRoleKey    ContextKey = "user_role"
	RequestIDKey   ContextKey = "request_id"
	IDPUserID ContextKey = "idp_user_id"
)

// SetGinContext sets standard context values in a Gin context
func SetGinContext(c *gin.Context, tenant TestTenant, user TestUser) {
	c.Set(string(TenantIDKey), tenant.ID)
	c.Set(string(UserIDKey), user.ID)
	c.Set(string(UserRoleKey), user.Role)
	c.Set(string(RequestIDKey), uuid.New().String())
	c.Set(string(IDPUserID), user.IDPUserID)
	c.Set("vendor_id", tenant.ID)
}

// SetTenantHeader sets the X-Tenant-ID header on a request
func SetTenantHeader(req *http.Request, tenantID string) {
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Vendor-ID", tenantID)
}

// SetAuthHeader sets the Authorization header with a test token
func SetAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
}

// HTTPTestHelper provides utilities for HTTP testing
type HTTPTestHelper struct {
	Router   *gin.Engine
	Recorder *httptest.ResponseRecorder
	t        *testing.T
}

// NewHTTPTestHelper creates a new HTTP test helper
func NewHTTPTestHelper(t *testing.T, router *gin.Engine) *HTTPTestHelper {
	return &HTTPTestHelper{
		Router: router,
		t:      t,
	}
}

// Request performs an HTTP request and returns the response
func (h *HTTPTestHelper) Request(method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(h.t, err)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, path, reqBody)
	require.NoError(h.t, err)

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	h.Recorder = httptest.NewRecorder()
	h.Router.ServeHTTP(h.Recorder, req)

	return h.Recorder
}

// GET performs a GET request
func (h *HTTPTestHelper) GET(path string, headers map[string]string) *httptest.ResponseRecorder {
	return h.Request(http.MethodGet, path, nil, headers)
}

// POST performs a POST request
func (h *HTTPTestHelper) POST(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return h.Request(http.MethodPost, path, body, headers)
}

// PUT performs a PUT request
func (h *HTTPTestHelper) PUT(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return h.Request(http.MethodPut, path, body, headers)
}

// DELETE performs a DELETE request
func (h *HTTPTestHelper) DELETE(path string, headers map[string]string) *httptest.ResponseRecorder {
	return h.Request(http.MethodDelete, path, nil, headers)
}

// WithTenant adds tenant header to headers map
func WithTenant(tenantID string) map[string]string {
	return map[string]string{
		"X-Tenant-ID": tenantID,
		"X-Vendor-ID": tenantID,
	}
}

// WithTenantAndAuth adds tenant and auth headers
func WithTenantAndAuth(tenantID, token string) map[string]string {
	return map[string]string{
		"X-Tenant-ID":   tenantID,
		"X-Vendor-ID":   tenantID,
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
}

// ParseJSONResponse parses the response body into the given struct
func ParseJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	err := json.Unmarshal(recorder.Body.Bytes(), v)
	require.NoError(t, err, "Failed to parse JSON response: %s", recorder.Body.String())
}

// AssertStatus asserts the HTTP status code
func AssertStatus(t *testing.T, recorder *httptest.ResponseRecorder, expected int) {
	t.Helper()
	require.Equal(t, expected, recorder.Code, "Unexpected status code. Body: %s", recorder.Body.String())
}

// TimeoutContext creates a context with timeout
func TimeoutContext(t *testing.T, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// CleanupTable cleans up a table after tests
func CleanupTable(t *testing.T, db *gorm.DB, tableName string, tenantID string) {
	t.Helper()
	result := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", tableName), tenantID)
	require.NoError(t, result.Error)
}

// IdempotencyKey generates a unique idempotency key for testing
func IdempotencyKey() string {
	return fmt.Sprintf("test-idempotency-%s", uuid.New().String())
}
