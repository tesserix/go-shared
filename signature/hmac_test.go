package signature

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants verifies the exported constant values are not accidentally changed.
func TestConstants(t *testing.T) {
	assert.Equal(t, "X-Approval-Signature", SignatureHeader)
	assert.Equal(t, "APPROVAL_CALLBACK_SECRET", DefaultSecretEnvVar)
}

// TestGenerateSignature_OutputFormat verifies that GenerateSignature returns a
// lowercase hex-encoded string (64 characters for SHA-256) and that consecutive
// calls with identical inputs produce the same value — confirming both the output
// format and determinism in a single test without hard-coding a digest value that
// would require an out-of-band computation to verify.
func TestGenerateSignature_OutputFormat(t *testing.T) {
	sig := GenerateSignature([]byte("hello world"), "test-secret")

	// HMAC-SHA256 produces a 32-byte digest; hex-encoded that is always 64 chars.
	assert.Len(t, sig, 64, "GenerateSignature must return a 64-character hex string")

	// Must contain only valid lowercase hex characters.
	for i, ch := range sig {
		isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
		assert.True(t, isHex, "character at position %d (%c) is not a lowercase hex digit", i, ch)
	}

	// Determinism: calling again must produce identical output.
	sig2 := GenerateSignature([]byte("hello world"), "test-secret")
	assert.Equal(t, sig, sig2, "GenerateSignature must be deterministic")
}

// TestGenerateSignature_Deterministic confirms that repeated calls with the same
// payload and secret always produce the same output.
func TestGenerateSignature_Deterministic(t *testing.T) {
	payload := []byte(`{"event":"approval","id":"abc-123"}`)
	secret := "my-shared-secret"

	first := GenerateSignature(payload, secret)
	second := GenerateSignature(payload, secret)
	third := GenerateSignature(payload, secret)

	assert.Equal(t, first, second, "GenerateSignature must be deterministic")
	assert.Equal(t, second, third, "GenerateSignature must be deterministic across repeated calls")
}

// TestGenerateSignature_DifferentSecrets verifies that different secrets produce
// different signatures for the same payload.
func TestGenerateSignature_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"order":"42"}`)

	sigA := GenerateSignature(payload, "secret-alpha")
	sigB := GenerateSignature(payload, "secret-beta")

	assert.NotEqual(t, sigA, sigB,
		"GenerateSignature must produce different results for different secrets")
}

// TestGenerateSignature_DifferentPayloads verifies that different payloads produce
// different signatures for the same secret.
func TestGenerateSignature_DifferentPayloads(t *testing.T) {
	secret := "same-secret"

	sigA := GenerateSignature([]byte("payload-one"), secret)
	sigB := GenerateSignature([]byte("payload-two"), secret)

	assert.NotEqual(t, sigA, sigB,
		"GenerateSignature must produce different results for different payloads")
}

// TestGenerateSignature_EmptyPayload ensures an empty payload is handled without
// panicking and produces a consistent hex-encoded string.
func TestGenerateSignature_EmptyPayload(t *testing.T) {
	sig := GenerateSignature([]byte{}, "secret")
	assert.NotEmpty(t, sig, "GenerateSignature on empty payload must still return a non-empty hex digest")
	// Repeated calls on empty payload must still be deterministic.
	assert.Equal(t, sig, GenerateSignature([]byte{}, "secret"))
}

// TestGenerateSignature_EmptySecret ensures an empty secret is handled gracefully.
func TestGenerateSignature_EmptySecret(t *testing.T) {
	sigEmpty := GenerateSignature([]byte("payload"), "")
	sigNonEmpty := GenerateSignature([]byte("payload"), "nonempty")

	assert.NotEmpty(t, sigEmpty, "GenerateSignature with empty secret must still return a digest")
	assert.NotEqual(t, sigEmpty, sigNonEmpty,
		"Empty secret and non-empty secret must produce different signatures")
}

// TestVerifySignature_ValidSignature checks the happy path: a freshly generated
// signature must be accepted by VerifySignature.
func TestVerifySignature_ValidSignature(t *testing.T) {
	payload := []byte(`{"status":"approved"}`)
	secret := "callback-secret"

	sig := GenerateSignature(payload, secret)
	ok := VerifySignature(payload, sig, secret)

	assert.True(t, ok, "VerifySignature must return true for a valid signature")
}

// TestVerifySignature_TableDriven covers a range of valid and invalid scenarios.
func TestVerifySignature_TableDriven(t *testing.T) {
	validPayload := []byte(`{"id":"evt-001"}`)
	validSecret := "s3cr3t"
	validSig := GenerateSignature(validPayload, validSecret)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		wantOK    bool
	}{
		{
			name:      "correct payload, signature, and secret",
			payload:   validPayload,
			signature: validSig,
			secret:    validSecret,
			wantOK:    true,
		},
		{
			name:      "tampered payload",
			payload:   []byte(`{"id":"evt-002"}`),
			signature: validSig,
			secret:    validSecret,
			wantOK:    false,
		},
		{
			name:      "wrong secret",
			payload:   validPayload,
			signature: validSig,
			secret:    "wrong-secret",
			wantOK:    false,
		},
		{
			name:      "empty signature string",
			payload:   validPayload,
			signature: "",
			secret:    validSecret,
			wantOK:    false,
		},
		{
			name:      "signature from different payload",
			payload:   validPayload,
			signature: GenerateSignature([]byte("other"), validSecret),
			secret:    validSecret,
			wantOK:    false,
		},
		{
			name:      "signature from different secret",
			payload:   validPayload,
			signature: GenerateSignature(validPayload, "different-secret"),
			secret:    validSecret,
			wantOK:    false,
		},
		{
			name:      "hex garbage as signature",
			payload:   validPayload,
			signature: "deadbeefdeadbeefdeadbeefdeadbeef",
			secret:    validSecret,
			wantOK:    false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := VerifySignature(tc.payload, tc.signature, tc.secret)
			assert.Equal(t, tc.wantOK, got,
				"VerifySignature(%q, %q, %q)", tc.payload, tc.signature, tc.secret)
		})
	}
}

// TestVerifySignature_ConstantTime asserts that VerifySignature uses constant-time
// comparison — this is enforced by the use of hmac.Equal internally.  We cannot
// time the function in a unit test, but we can confirm that a signature that only
// differs in the last character is still rejected.
func TestVerifySignature_ConstantTime(t *testing.T) {
	payload := []byte("data")
	secret := "key"
	correct := GenerateSignature(payload, secret)

	// Flip the last hex character of the correct signature.
	runes := []rune(correct)
	if runes[len(runes)-1] == 'f' {
		runes[len(runes)-1] = '0'
	} else {
		runes[len(runes)-1] = 'f'
	}
	tampered := string(runes)

	assert.False(t, VerifySignature(payload, tampered, secret),
		"VerifySignature must reject a signature that differs by a single character")
}

// TestVerifySignatureFromEnv_SecretSet checks that VerifySignatureFromEnv correctly
// delegates to VerifySignature when the env var is populated.
func TestVerifySignatureFromEnv_SecretSet(t *testing.T) {
	const secret = "env-secret-value"
	t.Setenv(DefaultSecretEnvVar, secret)

	payload := []byte(`{"ref":"env-test"}`)
	validSig := GenerateSignature(payload, secret)

	assert.True(t, VerifySignatureFromEnv(payload, validSig),
		"VerifySignatureFromEnv must accept a correctly signed payload when env var is set")
	assert.False(t, VerifySignatureFromEnv(payload, "wrong-signature"),
		"VerifySignatureFromEnv must reject an invalid signature when env var is set")
}

// TestVerifySignatureFromEnv_SecretUnset confirms the documented development-mode
// behaviour: when the env var is absent, verification is skipped and true is returned.
func TestVerifySignatureFromEnv_SecretUnset(t *testing.T) {
	os.Unsetenv(DefaultSecretEnvVar)

	// Any signature (including empty) must be accepted in development mode.
	assert.True(t, VerifySignatureFromEnv([]byte("payload"), "any-sig"),
		"VerifySignatureFromEnv must return true when env var is unset (development mode)")
	assert.True(t, VerifySignatureFromEnv([]byte("payload"), ""),
		"VerifySignatureFromEnv must return true for empty sig when env var is unset")
}

// TestVerifySignatureFromEnv_SecretSetInvalidPayload ensures that once a secret is
// configured, a modified payload is rejected.
func TestVerifySignatureFromEnv_SecretSetInvalidPayload(t *testing.T) {
	const secret = "strict-mode-secret"
	t.Setenv(DefaultSecretEnvVar, secret)

	originalPayload := []byte(`{"amount":100}`)
	tamperedPayload := []byte(`{"amount":999}`)

	sig := GenerateSignature(originalPayload, secret)

	assert.False(t, VerifySignatureFromEnv(tamperedPayload, sig),
		"VerifySignatureFromEnv must reject a tampered payload")
}

// TestGetSecretFromEnv_EnvSet checks the value is returned verbatim.
func TestGetSecretFromEnv_EnvSet(t *testing.T) {
	const expected = "super-secret-123"
	t.Setenv(DefaultSecretEnvVar, expected)

	assert.Equal(t, expected, GetSecretFromEnv(),
		"GetSecretFromEnv must return the exact env var value")
}

// TestGetSecretFromEnv_EnvUnset confirms an empty string is returned when the
// variable is not set.
func TestGetSecretFromEnv_EnvUnset(t *testing.T) {
	os.Unsetenv(DefaultSecretEnvVar)

	assert.Equal(t, "", GetSecretFromEnv(),
		"GetSecretFromEnv must return empty string when env var is unset")
}

// TestGetSecretFromEnv_EmptyValue checks that an explicitly empty env var is returned
// as an empty string (not treated as absent).
func TestGetSecretFromEnv_EmptyValue(t *testing.T) {
	t.Setenv(DefaultSecretEnvVar, "")

	assert.Equal(t, "", GetSecretFromEnv(),
		"GetSecretFromEnv must return empty string when env var is set to empty")
}

// TestRoundTrip_MultiplePayloads confirms that generate-then-verify works for a
// variety of payload types without any false positives or false negatives.
func TestRoundTrip_MultiplePayloads(t *testing.T) {
	secret := "round-trip-secret"

	payloads := [][]byte{
		[]byte(`{}`),
		[]byte(`{"key":"value"}`),
		[]byte("plain text"),
		[]byte("unicode: \u00e9\u00e0\u00fc"),
		make([]byte, 1024), // 1 KiB of zero bytes
	}

	for _, p := range payloads {
		sig := GenerateSignature(p, secret)
		assert.True(t, VerifySignature(p, sig, secret),
			"round-trip verification must succeed for payload %q", p)
	}
}
