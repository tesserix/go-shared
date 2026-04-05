package middleware

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// GIPAuthConfig configures Google Identity Platform Bearer token auth.
type GIPAuthConfig struct {
	// GCPProjectID is the Firebase/GIP project ID used to verify the issuer.
	// If empty, reads from GCP_PROJECT_ID env var.
	GCPProjectID string

	// RequireAuth returns 401 when no valid Bearer token is present.
	RequireAuth bool

	// SkipPaths are path prefixes that bypass auth entirely.
	SkipPaths []string

	// TrustToken when true skips signature verification and only decodes claims.
	// WARNING: Only set this to true in development/testing. In production, tokens
	// MUST be verified to prevent forged JWT attacks.
	// Default: false (tokens are cryptographically verified against Google's public keys).
	TrustToken bool
}

// gipClaims mirrors the Firebase ID token JWT payload.
type gipClaims struct {
	Subject       string                 `json:"sub"`
	Email         string                 `json:"email"`
	EmailVerified bool                   `json:"email_verified"`
	Name          string                 `json:"name"`
	Picture       string                 `json:"picture"`
	Issuer        string                 `json:"iss"`
	Audience      string                 `json:"aud"`
	AuthTime      int64                  `json:"auth_time"`
	UserID        string                 `json:"user_id"` // same as sub for Firebase
	Firebase      map[string]interface{} `json:"firebase"`
}

// googleCertsURL is the endpoint for Google's public x509 certificates used to verify Firebase ID tokens.
const googleCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// cachedCerts holds the cached Google public keys for JWT verification.
var cachedCerts = struct {
	sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}{}

// fetchGooglePublicKeys fetches and caches Google's public certificates for Firebase token verification.
func fetchGooglePublicKeys() (map[string]*rsa.PublicKey, error) {
	cachedCerts.RLock()
	if time.Now().Before(cachedCerts.expiresAt) && cachedCerts.keys != nil {
		keys := cachedCerts.keys
		cachedCerts.RUnlock()
		return keys, nil
	}
	cachedCerts.RUnlock()

	cachedCerts.Lock()
	defer cachedCerts.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(cachedCerts.expiresAt) && cachedCerts.keys != nil {
		return cachedCerts.keys, nil
	}

	resp, err := http.Get(googleCertsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google public keys: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google public keys response: %w", err)
	}

	var rawCerts map[string]string
	if err := json.Unmarshal(body, &rawCerts); err != nil {
		return nil, fmt.Errorf("failed to parse Google public keys: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(rawCerts))
	for kid, certPEM := range rawCerts {
		block, _ := pem.Decode([]byte(certPEM))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			continue
		}
		keys[kid] = rsaKey
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid RSA public keys found")
	}

	// Cache with TTL from Cache-Control max-age, or default 1 hour
	cacheTTL := 1 * time.Hour
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		if idx := strings.Index(cc, "max-age="); idx >= 0 {
			maxAgeStr := cc[idx+8:]
			if end := strings.IndexAny(maxAgeStr, ", "); end > 0 {
				maxAgeStr = maxAgeStr[:end]
			}
			if maxAge, err := strconv.Atoi(maxAgeStr); err == nil && maxAge > 0 {
				cacheTTL = time.Duration(maxAge) * time.Second
				// Clamp between 5 minutes and 24 hours
				if cacheTTL < 5*time.Minute {
					cacheTTL = 5 * time.Minute
				}
				if cacheTTL > 24*time.Hour {
					cacheTTL = 24 * time.Hour
				}
			}
		}
	}
	cachedCerts.keys = keys
	cachedCerts.expiresAt = time.Now().Add(cacheTTL)

	return keys, nil
}

// verifyFirebaseJWT verifies and parses a Firebase ID token with full signature verification.
func verifyFirebaseJWT(tokenStr string, projectID string) (*gipClaims, error) {
	keys, err := fetchGooglePublicKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get public keys: %w", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid: %s", kid)
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://securetoken.google.com/"+projectID),
		jwt.WithAudience(projectID),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Map to gipClaims
	claims := &gipClaims{}
	if sub, _ := mapClaims["sub"].(string); sub != "" {
		claims.Subject = sub
	} else {
		return nil, fmt.Errorf("missing sub claim")
	}
	claims.Email, _ = mapClaims["email"].(string)
	claims.EmailVerified, _ = mapClaims["email_verified"].(bool)
	claims.Name, _ = mapClaims["name"].(string)
	claims.Picture, _ = mapClaims["picture"].(string)
	claims.Issuer, _ = mapClaims["iss"].(string)
	claims.Audience, _ = mapClaims["aud"].(string)
	claims.UserID, _ = mapClaims["user_id"].(string)
	if authTime, ok := mapClaims["auth_time"].(float64); ok {
		claims.AuthTime = int64(authTime)
	}
	if fb, ok := mapClaims["firebase"].(map[string]interface{}); ok {
		claims.Firebase = fb
	}

	return claims, nil
}

// GIPAuth middleware validates a Bearer token from Google Identity Platform.
// By default, tokens are cryptographically verified against Google's public keys.
// The caller (tesserix-home) sends the GIP ID token as Authorization: Bearer.
// Works on both Cloud Run and GKE deployments.
func GIPAuth(config GIPAuthConfig) gin.HandlerFunc {
	projectID := config.GCPProjectID
	if projectID == "" {
		projectID = os.Getenv("GCP_PROJECT_ID")
	}

	return func(c *gin.Context) {
		// Skip configured paths
		for _, skipPath := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, skipPath) {
				extractTenantHeaders(c)
				c.Next()
				return
			}
		}

		// Accept x-jwt-claim-* headers (set by tesserix-home/marketplace-admin).
		// Service-to-service calls send trusted claim headers extracted from
		// the BFF session. Works on both Cloud Run and GKE (via Istio).
		if c.GetHeader(AuthHeaderKey) != "" {
			authCtx := parseJWTClaimHeaders(c)
			// Also pick up X-Tenant-ID/X-Tenant-Slug for compat with callers
			// that set legacy headers alongside claim headers.
			if authCtx.TenantID == "" {
				if tid := c.GetHeader("X-Tenant-ID"); tid != "" {
					authCtx.TenantID = tid
				}
			}
			if authCtx.TenantSlug == "" {
				if ts := c.GetHeader("X-Tenant-Slug"); ts != "" {
					authCtx.TenantSlug = ts
				}
			}
			stripUntrustedHeaders(c, nil)
			setAuthContextValues(c, authCtx)
			c.Next()
			return
		}

		// Extract Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			if config.RequireAuth {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "unauthorized",
					"message": "Authentication required",
				})
				return
			}
			c.Next()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		var claims *gipClaims
		var err error

		if config.TrustToken {
			// Decode-only mode: no signature verification. Use only in dev/test.
			if projectID == "" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "configuration_error",
					"message": "GCP_PROJECT_ID is required even in TrustToken mode for issuer validation",
				})
				return
			}
			claims, err = decodeFirebaseJWT(tokenStr)
			if err == nil {
				expectedIssuer := "https://securetoken.google.com/" + projectID
				if claims.Issuer != "" && claims.Issuer != expectedIssuer {
					err = fmt.Errorf("invalid token issuer")
				}
			}
		} else {
			// Production mode: full signature + issuer + audience + expiry verification
			if projectID == "" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "configuration_error",
					"message": "GCP_PROJECT_ID is required for token verification",
				})
				return
			}
			claims, err = verifyFirebaseJWT(tokenStr, projectID)
		}

		if err != nil {
			if config.RequireAuth {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "unauthorized",
					"message": "Invalid token",
				})
				return
			}
			c.Next()
			return
		}

		// Build AuthContext from Firebase claims
		authCtx := buildAuthContextFromGIP(c, claims)

		// Set AuthContext and individual context values
		setAuthContextValues(c, authCtx)

		c.Next()
	}
}

// decodeFirebaseJWT decodes a Firebase/GIP JWT without signature verification.
// Still checks expiry to prevent accepting stale tokens.
func decodeFirebaseJWT(tokenStr string) (*gipClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (part 1)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse into a map first to check expiry
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Check expiry even in trust mode
	if exp, ok := raw["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	var claims gipClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	return &claims, nil
}

// buildAuthContextFromGIP creates an AuthContext from Firebase JWT claims.
func buildAuthContextFromGIP(c *gin.Context, claims *gipClaims) *AuthContext {
	authCtx := &AuthContext{
		UserID: claims.Subject,
		Email:  claims.Email,
		Name:   claims.Name,
	}

	// Extract Firebase tenant from the firebase claim
	if claims.Firebase != nil {
		if tenant, ok := claims.Firebase["tenant"].(string); ok {
			// Firebase tenant ID is the GIP tenant, not the app tenant.
			// The actual app-level tenant_id comes from X-Tenant-ID header.
			// Platform tenant users (e.g. "Platform-e1vyf") are platform owners
			// who can bypass per-store authorization checks.
			if strings.HasPrefix(strings.ToLower(tenant), "platform") {
				authCtx.IsPlatformOwner = true
			}
		}
	}

	// App-level tenant context comes from X-Tenant-ID header (set by tesserix-home)
	if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
		authCtx.TenantID = tenantID
	}
	if tenantSlug := c.GetHeader("X-Tenant-Slug"); tenantSlug != "" {
		authCtx.TenantSlug = tenantSlug
	}
	if vendorID := c.GetHeader("X-Vendor-ID"); vendorID != "" {
		authCtx.VendorID = vendorID
	}

	return authCtx
}

// setAuthContextValues sets all the standard context keys from an AuthContext.
// Shared by both Auth and GIPAuth to ensure consistent context values.
func setAuthContextValues(c *gin.Context, authCtx *AuthContext) {
	c.Set(AuthContextKey, authCtx)

	c.Set("user_id", authCtx.UserID)
	c.Set("userId", authCtx.UserID)
	c.Set("staff_id", authCtx.StaffID)
	c.Set("staffId", authCtx.StaffID)
	c.Set("tenant_id", authCtx.TenantID)
	c.Set("tenantId", authCtx.TenantID)
	c.Set("tenant_slug", authCtx.TenantSlug)
	c.Set("user_email", authCtx.Email)
	c.Set("userEmail", authCtx.Email)

	username := authCtx.Name
	if username == "" {
		username = authCtx.Username
	}
	if username == "" {
		username = authCtx.Email
	}
	c.Set("username", username)
	c.Set("userName", username)

	if authCtx.VendorID != "" {
		c.Set("vendor_id", authCtx.VendorID)
		c.Set("vendorId", authCtx.VendorID)
	}

	c.Set("roles", authCtx.Roles)
	userRole := getPrimaryRole(authCtx.Roles)
	c.Set("userRole", userRole)
	c.Set("user_role", userRole)
	c.Set("is_platform_owner", authCtx.IsPlatformOwner)
}

// extractTenantHeaders reads tenant context from headers for skipped paths.
func extractTenantHeaders(c *gin.Context) {
	if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
		c.Set("tenant_id", tenantID)
	}
	if tenantSlug := c.GetHeader("X-Tenant-Slug"); tenantSlug != "" {
		c.Set("tenant_slug", tenantSlug)
	}
}
