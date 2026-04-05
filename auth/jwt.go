package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims
type Claims struct {
	UserID     string   `json:"user_id"`
	Email      string   `json:"email"`
	TenantID   string   `json:"tenant_id"`
	Roles      []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret           string
	ExpirationTime   time.Duration
	RefreshTime      time.Duration
	Issuer           string
	SkipPaths        []string
	RequireHTTPS     bool
	AllowRefresh     bool
}

// DefaultJWTConfig returns a default JWT configuration
func DefaultJWTConfig(secret string) JWTConfig {
	return JWTConfig{
		Secret:         secret,
		ExpirationTime: 15 * time.Minute,
		RefreshTime:    24 * time.Hour,
		Issuer:         "tesseract-hub",
		SkipPaths: []string{
			"/health",
			"/ready",
			"/swagger",
			"/login",
			"/register",
			"/forgot-password",
			"/reset-password",
		},
		RequireHTTPS: false,
		AllowRefresh: true,
	}
}

// ProductionJWTConfig returns a secure JWT configuration for production
func ProductionJWTConfig(secret string) JWTConfig {
	config := DefaultJWTConfig(secret)
	config.RequireHTTPS = true
	config.ExpirationTime = 5 * time.Minute
	return config
}

// GenerateToken generates a new JWT token
func GenerateToken(claims Claims, config JWTConfig) (string, error) {
	// Set expiration time
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(config.ExpirationTime))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	claims.Issuer = config.Issuer

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	// Sign token
	tokenString, err := token.SignedString([]byte(config.Secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns claims
func ValidateToken(tokenString string, config JWTConfig) (*Claims, error) {
	claims := &Claims{}
	
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrInvalidType
	}

	return claims, nil
}

// RefreshToken generates a new token from existing claims
func RefreshToken(oldClaims *Claims, config JWTConfig) (string, error) {
	// Create new claims with updated expiration
	newClaims := Claims{
		UserID:   oldClaims.UserID,
		Email:    oldClaims.Email,
		TenantID: oldClaims.TenantID,
		Roles:    oldClaims.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.ExpirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    config.Issuer,
		},
	}

	return GenerateToken(newClaims, config)
}