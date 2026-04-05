package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// SignatureHeader is the header name for approval service signatures
const SignatureHeader = "X-Approval-Signature"

// DefaultSecretEnvVar is the environment variable name for the shared secret
const DefaultSecretEnvVar = "APPROVAL_CALLBACK_SECRET"

// GenerateSignature creates an HMAC-SHA256 signature for the given payload
func GenerateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies an HMAC-SHA256 signature
func VerifySignature(payload []byte, signature, secret string) bool {
	expectedSignature := GenerateSignature(payload, secret)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// VerifySignatureFromEnv verifies signature using secret from environment variable
func VerifySignatureFromEnv(payload []byte, signature string) bool {
	secret := os.Getenv(DefaultSecretEnvVar)
	if secret == "" {
		// If no secret is configured, skip verification (development mode)
		// In production, ensure APPROVAL_CALLBACK_SECRET is set
		return true
	}
	return VerifySignature(payload, signature, secret)
}

// GetSecretFromEnv retrieves the callback secret from environment
func GetSecretFromEnv() string {
	return os.Getenv(DefaultSecretEnvVar)
}
