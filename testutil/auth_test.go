// Package testutil provides test templates for authentication validation.
// Copy and adapt these tests for each service.
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// TestJWTConfig holds configuration for test JWT generation
type TestJWTConfig struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	KeyID      string
	Issuer     string
	Audience   string
}

// NewTestJWTConfig creates a new test JWT configuration
func NewTestJWTConfig() (*TestJWTConfig, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return &TestJWTConfig{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		KeyID:      "test-key-id",
		Issuer:     "https://test-idp.tesserix.app/realms/tesserix-customer",
		Audience:   "tesseract-hub",
	}, nil
}

// TestTokenClaims represents claims for test tokens
type TestTokenClaims struct {
	UserID           string
	IDPUserID   string
	Email            string
	TenantID         string
	Role             string
	Permissions      []string
	ExpiresAt        time.Time
	IssuedAt         time.Time
	PreferredUsername string
}

// DefaultTestClaims returns default test claims
func DefaultTestClaims(tenantID string) TestTokenClaims {
	now := time.Now()
	return TestTokenClaims{
		UserID:            "test-user-id",
		IDPUserID:    "idp-user-123",
		Email:             "test@example.com",
		TenantID:          tenantID,
		Role:              "admin",
		Permissions:       []string{"products:read", "products:write", "orders:read"},
		ExpiresAt:         now.Add(1 * time.Hour),
		IssuedAt:          now,
		PreferredUsername: "testuser",
	}
}

// GenerateTestToken generates a test JWT token
func (c *TestJWTConfig) GenerateTestToken(claims TestTokenClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                claims.IDPUserID,
		"iss":                c.Issuer,
		"aud":                c.Audience,
		"exp":                claims.ExpiresAt.Unix(),
		"iat":                claims.IssuedAt.Unix(),
		"idp_user_id":   claims.IDPUserID,
		"preferred_username": claims.PreferredUsername,
		"email":              claims.Email,
		"tenant_id":          claims.TenantID,
		"realm_access": map[string]interface{}{
			"roles": []string{claims.Role},
		},
		"resource_access": map[string]interface{}{
			"tesseract-hub": map[string]interface{}{
				"roles": claims.Permissions,
			},
		},
	})

	token.Header["kid"] = c.KeyID

	return token.SignedString(c.PrivateKey)
}

// GenerateExpiredToken generates an expired test token
func (c *TestJWTConfig) GenerateExpiredToken(claims TestTokenClaims) (string, error) {
	claims.ExpiresAt = time.Now().Add(-1 * time.Hour)
	claims.IssuedAt = time.Now().Add(-2 * time.Hour)
	return c.GenerateTestToken(claims)
}

// GenerateInvalidSignatureToken generates a token with invalid signature
func (c *TestJWTConfig) GenerateInvalidSignatureToken(claims TestTokenClaims) (string, error) {
	// Generate with a different key
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": claims.IDPUserID,
		"exp": claims.ExpiresAt.Unix(),
	})
	return token.SignedString(otherKey)
}

// GetJWKS returns a JWKS JSON for the test public key
func (c *TestJWTConfig) GetJWKS() string {
	n := base64.RawURLEncoding.EncodeToString(c.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(c.PublicKey.E)).Bytes())

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": c.KeyID,
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}

	jsonBytes, _ := json.Marshal(jwks)
	return string(jsonBytes)
}

// StartMockJWKSServer starts a mock JWKS server for testing
func (c *TestJWTConfig) StartMockJWKSServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(c.GetJWKS()))
	}))
}

// AuthTestSuite provides reusable authentication test patterns
type AuthTestSuite struct {
	Router    *gin.Engine
	JWTConfig *TestJWTConfig
	t         *testing.T
}

// NewAuthTestSuite creates a new authentication test suite
func NewAuthTestSuite(t *testing.T, router *gin.Engine) (*AuthTestSuite, error) {
	jwtConfig, err := NewTestJWTConfig()
	if err != nil {
		return nil, err
	}

	return &AuthTestSuite{
		Router:    router,
		JWTConfig: jwtConfig,
		t:         t,
	}, nil
}

// TestValidTokenAccepted is a template for testing valid token acceptance
func TestValidTokenAccepted(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestAuth_ValidToken(t *testing.T) {
			router := setupTestRouter()

			authSuite, err := testutil.NewAuthTestSuite(t, router)
			require.NoError(t, err)

			tenant := testutil.NewTestTenant()
			claims := testutil.DefaultTestClaims(tenant.ID)
			token, err := authSuite.JWTConfig.GenerateTestToken(claims)
			require.NoError(t, err)

			helper := testutil.NewHTTPTestHelper(t, router)
			resp := helper.GET("/api/v1/orders", testutil.WithTenantAndAuth(tenant.ID, token))

			testutil.AssertStatus(t, resp, http.StatusOK)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestExpiredTokenRejected is a template for testing expired token rejection
func TestExpiredTokenRejected(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestAuth_ExpiredToken(t *testing.T) {
			router := setupTestRouter()

			authSuite, err := testutil.NewAuthTestSuite(t, router)
			require.NoError(t, err)

			tenant := testutil.NewTestTenant()
			claims := testutil.DefaultTestClaims(tenant.ID)
			token, err := authSuite.JWTConfig.GenerateExpiredToken(claims)
			require.NoError(t, err)

			helper := testutil.NewHTTPTestHelper(t, router)
			resp := helper.GET("/api/v1/orders", testutil.WithTenantAndAuth(tenant.ID, token))

			testutil.AssertStatus(t, resp, http.StatusUnauthorized)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestInvalidSignatureRejected is a template for testing invalid signature rejection
func TestInvalidSignatureRejected(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestAuth_InvalidSignature(t *testing.T) {
			router := setupTestRouter()

			authSuite, err := testutil.NewAuthTestSuite(t, router)
			require.NoError(t, err)

			tenant := testutil.NewTestTenant()
			claims := testutil.DefaultTestClaims(tenant.ID)
			token, err := authSuite.JWTConfig.GenerateInvalidSignatureToken(claims)
			require.NoError(t, err)

			helper := testutil.NewHTTPTestHelper(t, router)
			resp := helper.GET("/api/v1/orders", testutil.WithTenantAndAuth(tenant.ID, token))

			testutil.AssertStatus(t, resp, http.StatusUnauthorized)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// TestMissingAuthorizationRejected is a template for testing missing auth rejection
func TestMissingAuthorizationRejected(t *testing.T) {
	/*
		TEMPLATE: Copy and adapt for your service

		Example:

		func TestAuth_MissingToken(t *testing.T) {
			router := setupTestRouter()

			tenant := testutil.NewTestTenant()

			helper := testutil.NewHTTPTestHelper(t, router)
			// Request with tenant header but no Authorization header
			resp := helper.GET("/api/v1/orders", testutil.WithTenant(tenant.ID))

			testutil.AssertStatus(t, resp, http.StatusUnauthorized)
		}
	*/
	t.Skip("This is a template - implement in your service")
}

// RunAuthTestSuite runs all authentication tests for an endpoint
func (s *AuthTestSuite) RunAuthTestSuite(endpoint string) {
	tenant := NewTestTenant()
	claims := DefaultTestClaims(tenant.ID)

	s.t.Run("ValidToken_Accepted", func(t *testing.T) {
		token, err := s.JWTConfig.GenerateTestToken(claims)
		assert.NoError(t, err)

		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenantAndAuth(tenant.ID, token))

		// Should not be 401 or 403 for valid token
		assert.NotEqual(t, http.StatusUnauthorized, resp.Code)
		assert.NotEqual(t, http.StatusForbidden, resp.Code)
	})

	s.t.Run("ExpiredToken_Rejected", func(t *testing.T) {
		token, err := s.JWTConfig.GenerateExpiredToken(claims)
		assert.NoError(t, err)

		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenantAndAuth(tenant.ID, token))

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	s.t.Run("InvalidSignature_Rejected", func(t *testing.T) {
		token, err := s.JWTConfig.GenerateInvalidSignatureToken(claims)
		assert.NoError(t, err)

		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenantAndAuth(tenant.ID, token))

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	s.t.Run("MissingAuthorization_Rejected", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		resp := helper.GET(endpoint, WithTenant(tenant.ID))

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	s.t.Run("MalformedToken_Rejected", func(t *testing.T) {
		helper := NewHTTPTestHelper(t, s.Router)
		headers := WithTenant(tenant.ID)
		headers["Authorization"] = "Bearer not-a-valid-jwt"
		resp := helper.GET(endpoint, headers)

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})
}

// RBACTestCase represents a test case for RBAC validation
type RBACTestCase struct {
	Name           string
	Role           string
	Permission     string
	ExpectedStatus int
}

// RunRBACTests runs RBAC permission tests
func RunRBACTests(t *testing.T, router *gin.Engine, endpoint string, testCases []RBACTestCase) {
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.Role, tc.Name), func(t *testing.T) {
			jwtConfig, err := NewTestJWTConfig()
			assert.NoError(t, err)

			tenant := NewTestTenant()
			claims := TestTokenClaims{
				UserID:         "test-user",
				IDPUserID: "gip-sub-123",
				Email:          "test@example.com",
				TenantID:       tenant.ID,
				Role:           tc.Role,
				Permissions:    []string{tc.Permission},
				ExpiresAt:      time.Now().Add(1 * time.Hour),
				IssuedAt:       time.Now(),
			}

			token, err := jwtConfig.GenerateTestToken(claims)
			assert.NoError(t, err)

			helper := NewHTTPTestHelper(t, router)
			resp := helper.GET(endpoint, WithTenantAndAuth(tenant.ID, token))

			assert.Equal(t, tc.ExpectedStatus, resp.Code,
				"Expected %d for role %s with permission %s, got %d",
				tc.ExpectedStatus, tc.Role, tc.Permission, resp.Code)
		})
	}
}
