package security

import (
	"context"
	"encoding/base64"
	"sync"
	"testing"
)

func TestNewPIIEncryptor(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr error
	}{
		{
			name:    "valid 32-byte key",
			keyLen:  32,
			wantErr: nil,
		},
		{
			name:    "invalid 16-byte key",
			keyLen:  16,
			wantErr: ErrInvalidKeyLength,
		},
		{
			name:    "invalid 24-byte key",
			keyLen:  24,
			wantErr: ErrInvalidKeyLength,
		},
		{
			name:    "invalid empty key",
			keyLen:  0,
			wantErr: ErrInvalidKeyLength,
		},
		{
			name:    "invalid 64-byte key",
			keyLen:  64,
			wantErr: ErrInvalidKeyLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			for i := range key {
				key[i] = byte(i % 256)
			}

			enc, err := NewPIIEncryptor(key)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("NewPIIEncryptor() error = %v, wantErr %v", err, tt.wantErr)
				}
				if enc != nil {
					t.Error("NewPIIEncryptor() should return nil encryptor on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewPIIEncryptor() unexpected error = %v", err)
				}
				if enc == nil {
					t.Error("NewPIIEncryptor() should return non-nil encryptor")
				}
				if !enc.ready {
					t.Error("NewPIIEncryptor() encryptor should be ready")
				}
			}
		})
	}
}

func TestNewPIIEncryptorFromBase64(t *testing.T) {
	// Generate a valid 32-byte key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	validBase64 := base64.StdEncoding.EncodeToString(key)

	tests := []struct {
		name      string
		keyBase64 string
		wantErr   bool
	}{
		{
			name:      "valid base64 key",
			keyBase64: validBase64,
			wantErr:   false,
		},
		{
			name:      "invalid base64",
			keyBase64: "not-valid-base64!!!",
			wantErr:   true,
		},
		{
			name:      "valid base64 but wrong length",
			keyBase64: base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewPIIEncryptorFromBase64(tt.keyBase64)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPIIEncryptorFromBase64() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && enc == nil {
				t.Error("NewPIIEncryptorFromBase64() should return non-nil encryptor")
			}
		})
	}
}

func TestPIIEncryptor_EncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
		},
		{
			name:      "email address",
			plaintext: "user@example.com",
		},
		{
			name:      "phone number",
			plaintext: "+1-555-123-4567",
		},
		{
			name:      "SSN",
			plaintext: "123-45-6789",
		},
		{
			name:      "unicode text",
			plaintext: "こんにちは世界 🌍",
		},
		{
			name:      "long text",
			plaintext: "This is a much longer piece of text that includes various characters and punctuation! It should still encrypt and decrypt correctly, even with special chars: @#$%^&*()",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "single character",
			plaintext: "X",
		},
		{
			name:      "whitespace",
			plaintext: "   \t\n   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Errorf("Encrypt() error = %v", err)
				return
			}

			// Empty string should return empty
			if tt.plaintext == "" {
				if ciphertext != "" {
					t.Error("Encrypt() empty plaintext should return empty ciphertext")
				}
				return
			}

			// Ciphertext should be different from plaintext
			if ciphertext == tt.plaintext {
				t.Error("Encrypt() ciphertext should not equal plaintext")
			}

			// Should be valid base64
			_, err = base64.StdEncoding.DecodeString(ciphertext)
			if err != nil {
				t.Errorf("Encrypt() output should be valid base64: %v", err)
			}

			// Decrypt should return original plaintext
			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Errorf("Decrypt() error = %v", err)
				return
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestPIIEncryptor_EncryptProducesUniqueCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := "test data for uniqueness"
	ciphertexts := make(map[string]bool)

	// Encrypt the same plaintext multiple times
	for i := 0; i < 100; i++ {
		ct, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}
		if ciphertexts[ct] {
			t.Error("Encrypt() should produce unique ciphertext each time (due to random nonce)")
		}
		ciphertexts[ct] = true
	}
}

func TestPIIEncryptor_DecryptInvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!!!",
			wantErr:    nil, // Will fail on base64 decode
		},
		{
			name:       "too short ciphertext",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:    ErrInvalidCiphertext,
		},
		{
			name:       "corrupted ciphertext",
			ciphertext: base64.StdEncoding.EncodeToString(make([]byte, 50)),
			wantErr:    ErrDecryptionFailed,
		},
		{
			name:       "empty ciphertext",
			ciphertext: "",
			wantErr:    nil, // Empty returns empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enc.Decrypt(tt.ciphertext)
			if tt.ciphertext == "" {
				if err != nil {
					t.Errorf("Decrypt() empty should not error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Error("Decrypt() should error on invalid ciphertext")
			}
		})
	}
}

func TestPIIEncryptor_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1) // Different key
	}

	enc1, _ := NewPIIEncryptor(key1)
	enc2, _ := NewPIIEncryptor(key2)

	plaintext := "secret data"
	ciphertext, err := enc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypting with wrong key should fail
	_, err = enc2.Decrypt(ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt() with wrong key should return ErrDecryptionFailed, got %v", err)
	}
}

func TestPIIEncryptor_ConcurrentAccess(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent encryptions
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			plaintext := "concurrent test data"
			ct, err := enc.Encrypt(plaintext)
			if err != nil {
				errors <- err
				return
			}
			pt, err := enc.Decrypt(ct)
			if err != nil {
				errors <- err
				return
			}
			if pt != plaintext {
				errors <- ErrDecryptionFailed
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestPIIEncryptor_EncryptWithContext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	ctx := context.Background()
	plaintext := "test with context"

	ciphertext, err := enc.EncryptWithContext(ctx, plaintext)
	if err != nil {
		t.Errorf("EncryptWithContext() error = %v", err)
	}

	decrypted, err := enc.DecryptWithContext(ctx, ciphertext)
	if err != nil {
		t.Errorf("DecryptWithContext() error = %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("DecryptWithContext() = %q, want %q", decrypted, plaintext)
	}
}

func TestPIIEncryptor_NotReady(t *testing.T) {
	enc := &PIIEncryptor{ready: false}

	_, err := enc.Encrypt("test")
	if err != ErrEncryptorNotReady {
		t.Errorf("Encrypt() on unready encryptor should return ErrEncryptorNotReady, got %v", err)
	}

	_, err = enc.Decrypt("test")
	if err != ErrEncryptorNotReady {
		t.Errorf("Decrypt() on unready encryptor should return ErrEncryptorNotReady, got %v", err)
	}
}

func TestPIIField(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	field := NewPIIField(enc)

	plaintext := "field test data"
	encrypted, err := field.Set(plaintext)
	if err != nil {
		t.Errorf("PIIField.Set() error = %v", err)
	}

	decrypted, err := field.Get(encrypted)
	if err != nil {
		t.Errorf("PIIField.Get() error = %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("PIIField.Get() = %q, want %q", decrypted, plaintext)
	}
}

func TestPIIField_NoEncryptor(t *testing.T) {
	field := &PIIField{encryptor: nil}

	_, err := field.Set("test")
	if err != ErrEncryptorNotReady {
		t.Errorf("PIIField.Set() with no encryptor should return ErrEncryptorNotReady, got %v", err)
	}

	_, err = field.Get("test")
	if err != ErrEncryptorNotReady {
		t.Errorf("PIIField.Get() with no encryptor should return ErrEncryptorNotReady, got %v", err)
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Errorf("GenerateEncryptionKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Errorf("GenerateEncryptionKey() key length = %d, want 32", len(key))
	}

	// Should be able to create an encryptor with generated key
	enc, err := NewPIIEncryptor(key)
	if err != nil {
		t.Errorf("NewPIIEncryptor(GenerateEncryptionKey()) error = %v", err)
	}
	if enc == nil {
		t.Error("NewPIIEncryptor(GenerateEncryptionKey()) should return non-nil encryptor")
	}

	// Generated keys should be unique
	key2, _ := GenerateEncryptionKey()
	if string(key) == string(key2) {
		t.Error("GenerateEncryptionKey() should generate unique keys")
	}
}

func TestGenerateEncryptionKeyBase64(t *testing.T) {
	keyBase64, err := GenerateEncryptionKeyBase64()
	if err != nil {
		t.Errorf("GenerateEncryptionKeyBase64() error = %v", err)
	}

	// Should be valid base64
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		t.Errorf("GenerateEncryptionKeyBase64() should return valid base64: %v", err)
	}

	if len(key) != 32 {
		t.Errorf("GenerateEncryptionKeyBase64() decoded key length = %d, want 32", len(key))
	}

	// Should be able to create encryptor from it
	enc, err := NewPIIEncryptorFromBase64(keyBase64)
	if err != nil {
		t.Errorf("NewPIIEncryptorFromBase64() error = %v", err)
	}
	if enc == nil {
		t.Error("NewPIIEncryptorFromBase64() should return non-nil encryptor")
	}
}

func TestDeterministicEncryptor(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewDeterministicEncryptor(key)
	if err != nil {
		t.Fatalf("NewDeterministicEncryptor() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "hello world"},
		{"email", "user@example.com"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct1, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Errorf("Encrypt() error = %v", err)
				return
			}

			ct2, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Errorf("Encrypt() error = %v", err)
				return
			}

			// Deterministic encryption should produce same ciphertext
			if ct1 != ct2 {
				t.Error("DeterministicEncryptor.Encrypt() should produce same ciphertext for same plaintext")
			}

			// Should decrypt correctly
			if tt.plaintext != "" {
				decrypted, err := enc.Decrypt(ct1)
				if err != nil {
					t.Errorf("Decrypt() error = %v", err)
					return
				}
				if decrypted != tt.plaintext {
					t.Errorf("Decrypt() = %q, want %q", decrypted, tt.plaintext)
				}
			}
		})
	}
}

func TestDeterministicEncryptor_DifferentPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, _ := NewDeterministicEncryptor(key)

	ct1, _ := enc.Encrypt("plaintext1")
	ct2, _ := enc.Encrypt("plaintext2")

	if ct1 == ct2 {
		t.Error("DeterministicEncryptor should produce different ciphertext for different plaintext")
	}
}

func TestDeterministicEncryptor_InvalidKey(t *testing.T) {
	_, err := NewDeterministicEncryptor([]byte("short"))
	if err != ErrInvalidKeyLength {
		t.Errorf("NewDeterministicEncryptor() with invalid key should return ErrInvalidKeyLength, got %v", err)
	}
}

func TestDeterministicEncryptor_DecryptInvalid(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, _ := NewDeterministicEncryptor(key)

	tests := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{
			name:       "too short",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("tiny")),
			wantErr:    ErrInvalidCiphertext,
		},
		{
			name:       "corrupted",
			ciphertext: base64.StdEncoding.EncodeToString(make([]byte, 50)),
			wantErr:    ErrDecryptionFailed,
		},
		{
			name:       "empty",
			ciphertext: "",
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := enc.Decrypt(tt.ciphertext)
			if tt.ciphertext == "" {
				if err != nil || result != "" {
					t.Errorf("Decrypt() empty should return empty without error")
				}
				return
			}
			if err == nil {
				t.Error("Decrypt() invalid should error")
			}
		})
	}
}

func BenchmarkPIIEncryptor_Encrypt(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewPIIEncryptor(key)
	plaintext := "test@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Encrypt(plaintext)
	}
}

func BenchmarkPIIEncryptor_Decrypt(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewPIIEncryptor(key)
	ciphertext, _ := enc.Encrypt("test@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Decrypt(ciphertext)
	}
}

func BenchmarkDeterministicEncryptor_Encrypt(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewDeterministicEncryptor(key)
	plaintext := "test@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Encrypt(plaintext)
	}
}
