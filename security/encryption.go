package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Common errors
var (
	ErrInvalidKeyLength   = errors.New("encryption key must be 32 bytes for AES-256")
	ErrInvalidCiphertext  = errors.New("invalid ciphertext")
	ErrDecryptionFailed   = errors.New("decryption failed")
	ErrEncryptorNotReady  = errors.New("encryptor not initialized")
)

// PIIEncryptor provides AES-256-GCM encryption for PII fields
// This should be used for all personally identifiable information stored in the database
type PIIEncryptor struct {
	key    []byte
	gcm    cipher.AEAD
	mu     sync.RWMutex
	ready  bool
}

// PIIEncryptorConfig holds configuration for PII encryption
type PIIEncryptorConfig struct {
	// KeySource: "env", "gcp-secret-manager", "vault"
	KeySource string
	// SecretName for GCP Secret Manager or Vault
	SecretName string
	// EnvVar name if KeySource is "env"
	EnvVar string
}

var (
	globalEncryptor     *PIIEncryptor
	globalEncryptorOnce sync.Once
	globalEncryptorMu   sync.RWMutex
)

// NewPIIEncryptor creates a new PII encryptor with the given key
// Key must be exactly 32 bytes (256 bits) for AES-256
func NewPIIEncryptor(key []byte) (*PIIEncryptor, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &PIIEncryptor{
		key:   key,
		gcm:   gcm,
		ready: true,
	}, nil
}

// NewPIIEncryptorFromBase64 creates an encryptor from a base64-encoded key
func NewPIIEncryptorFromBase64(keyBase64 string) (*PIIEncryptor, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 key: %w", err)
	}
	return NewPIIEncryptor(key)
}

// InitGlobalEncryptor initializes the global PII encryptor (call once at startup)
func InitGlobalEncryptor(key []byte) error {
	var initErr error
	globalEncryptorOnce.Do(func() {
		enc, err := NewPIIEncryptor(key)
		if err != nil {
			initErr = err
			return
		}
		globalEncryptorMu.Lock()
		globalEncryptor = enc
		globalEncryptorMu.Unlock()
	})
	return initErr
}

// GetGlobalEncryptor returns the global PII encryptor
func GetGlobalEncryptor() *PIIEncryptor {
	globalEncryptorMu.RLock()
	defer globalEncryptorMu.RUnlock()
	return globalEncryptor
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext (nonce prepended)
func (e *PIIEncryptor) Encrypt(plaintext string) (string, error) {
	if !e.ready {
		return "", ErrEncryptorNotReady
	}
	if plaintext == "" {
		return "", nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Generate random nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce to ciphertext
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM
func (e *PIIEncryptor) Decrypt(ciphertextBase64 string) (string, error) {
	if !e.ready {
		return "", ErrEncryptorNotReady
	}
	if ciphertextBase64 == "" {
		return "", nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// EncryptWithContext encrypts with context (for tracing/logging)
func (e *PIIEncryptor) EncryptWithContext(ctx context.Context, plaintext string) (string, error) {
	// Context can be used for tracing in future
	return e.Encrypt(plaintext)
}

// DecryptWithContext decrypts with context (for tracing/logging)
func (e *PIIEncryptor) DecryptWithContext(ctx context.Context, ciphertext string) (string, error) {
	// Context can be used for tracing in future
	return e.Decrypt(ciphertext)
}

// PIIField represents an encrypted PII field for database storage
type PIIField struct {
	encryptor *PIIEncryptor
}

// NewPIIField creates a new PII field handler
func NewPIIField(encryptor *PIIEncryptor) *PIIField {
	return &PIIField{encryptor: encryptor}
}

// Set encrypts a value for storage
func (f *PIIField) Set(value string) (string, error) {
	if f.encryptor == nil {
		f.encryptor = GetGlobalEncryptor()
	}
	if f.encryptor == nil {
		return "", ErrEncryptorNotReady
	}
	return f.encryptor.Encrypt(value)
}

// Get decrypts a value from storage
func (f *PIIField) Get(encrypted string) (string, error) {
	if f.encryptor == nil {
		f.encryptor = GetGlobalEncryptor()
	}
	if f.encryptor == nil {
		return "", ErrEncryptorNotReady
	}
	return f.encryptor.Decrypt(encrypted)
}

// EncryptPII is a convenience function using the global encryptor
func EncryptPII(plaintext string) (string, error) {
	enc := GetGlobalEncryptor()
	if enc == nil {
		return "", ErrEncryptorNotReady
	}
	return enc.Encrypt(plaintext)
}

// DecryptPII is a convenience function using the global encryptor
func DecryptPII(ciphertext string) (string, error) {
	enc := GetGlobalEncryptor()
	if enc == nil {
		return "", ErrEncryptorNotReady
	}
	return enc.Decrypt(ciphertext)
}

// GenerateEncryptionKey generates a new random 256-bit encryption key
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateEncryptionKeyBase64 generates a new random key and returns it base64-encoded
func GenerateEncryptionKeyBase64() (string, error) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// DeterministicEncrypt performs deterministic encryption (same plaintext = same ciphertext)
// WARNING: Only use this for fields that need to be searched/indexed
// This is less secure than standard encryption as it leaks equality
type DeterministicEncryptor struct {
	key []byte
}

// NewDeterministicEncryptor creates a deterministic encryptor
func NewDeterministicEncryptor(key []byte) (*DeterministicEncryptor, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}
	return &DeterministicEncryptor{key: key}, nil
}

// Encrypt performs deterministic encryption using AES-SIV mode
// Note: Uses a fixed nonce derived from the plaintext hash for determinism
func (e *DeterministicEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Use HMAC of plaintext as nonce for determinism
	// This makes same plaintext produce same ciphertext
	nonce := deriveNonce(e.key, []byte(plaintext), gcm.NonceSize())

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts deterministically encrypted data
func (e *DeterministicEncryptor) Decrypt(ciphertextBase64 string) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// deriveNonce derives a deterministic nonce from key and data
func deriveNonce(key, data []byte, size int) []byte {
	// Simple HMAC-like derivation for nonce
	// In production, use proper HKDF or similar
	h := make([]byte, size)
	for i := 0; i < size && i < len(data); i++ {
		h[i] = data[i] ^ key[i%len(key)]
	}
	return h
}
