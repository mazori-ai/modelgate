package crypto

import (
	"testing"
)

func TestEncryptionService(t *testing.T) {
	// Generate a test key
	key, err := GenerateKey(32) // AES-256
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	svc, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	t.Run("encrypt and decrypt string", func(t *testing.T) {
		plaintext := "sk-test-api-key-12345"
		
		ciphertext, err := svc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		if ciphertext == plaintext {
			t.Error("Ciphertext should not equal plaintext")
		}

		decrypted, err := svc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted text doesn't match: got %q, want %q", decrypted, plaintext)
		}
	})

	t.Run("encrypt empty string", func(t *testing.T) {
		ciphertext, err := svc.Encrypt("")
		if err != nil {
			t.Fatalf("Encrypt empty string failed: %v", err)
		}

		if ciphertext != "" {
			t.Error("Encrypting empty string should return empty string")
		}

		decrypted, err := svc.Decrypt("")
		if err != nil {
			t.Fatalf("Decrypt empty string failed: %v", err)
		}

		if decrypted != "" {
			t.Error("Decrypting empty string should return empty string")
		}
	})

	t.Run("different encryptions produce different ciphertexts", func(t *testing.T) {
		plaintext := "test-data"
		
		ciphertext1, _ := svc.Encrypt(plaintext)
		ciphertext2, _ := svc.Encrypt(plaintext)

		if ciphertext1 == ciphertext2 {
			t.Error("Same plaintext should produce different ciphertexts (due to random nonce)")
		}

		// Both should decrypt to same plaintext
		decrypted1, _ := svc.Decrypt(ciphertext1)
		decrypted2, _ := svc.Decrypt(ciphertext2)

		if decrypted1 != decrypted2 {
			t.Error("Both ciphertexts should decrypt to same plaintext")
		}
	})

	t.Run("decrypt with wrong key fails", func(t *testing.T) {
		plaintext := "secret-data"
		ciphertext, _ := svc.Encrypt(plaintext)

		// Create a different encryption service with different key
		wrongKey, _ := GenerateKey(32)
		wrongSvc, _ := NewEncryptionService(wrongKey)

		_, err := wrongSvc.Decrypt(ciphertext)
		if err != ErrDecryptionFailed {
			t.Errorf("Expected ErrDecryptionFailed, got: %v", err)
		}
	})

	t.Run("decrypt invalid ciphertext", func(t *testing.T) {
		_, err := svc.Decrypt("invalid-base64!")
		if err == nil {
			t.Error("Expected error for invalid base64")
		}

		// Too short ciphertext
		_, err = svc.Decrypt("YWJj") // "abc" in base64
		if err != ErrInvalidCiphertext {
			t.Errorf("Expected ErrInvalidCiphertext, got: %v", err)
		}
	})
}

func TestNewEncryptionService(t *testing.T) {
	t.Run("valid key sizes", func(t *testing.T) {
		validSizes := []int{16, 24, 32}
		for _, size := range validSizes {
			key := make([]byte, size)
			_, err := NewEncryptionService(key)
			if err != nil {
				t.Errorf("Failed to create service with %d-byte key: %v", size, err)
			}
		}
	})

	t.Run("invalid key sizes", func(t *testing.T) {
		invalidSizes := []int{0, 8, 15, 17, 23, 25, 31, 33, 64}
		for _, size := range invalidSizes {
			key := make([]byte, size)
			_, err := NewEncryptionService(key)
			if err != ErrInvalidKey {
				t.Errorf("Expected ErrInvalidKey for %d-byte key, got: %v", size, err)
			}
		}
	})
}

func TestNewEncryptionServiceFromString(t *testing.T) {
	t.Run("valid base64 key", func(t *testing.T) {
		// Generate a key and encode it
		keyStr, err := GenerateKeyString(32)
		if err != nil {
			t.Fatalf("Failed to generate key string: %v", err)
		}

		svc, err := NewEncryptionServiceFromString(keyStr)
		if err != nil {
			t.Fatalf("Failed to create service from string: %v", err)
		}

		// Test encryption/decryption works
		plaintext := "test"
		ciphertext, _ := svc.Encrypt(plaintext)
		decrypted, _ := svc.Decrypt(ciphertext)
		if decrypted != plaintext {
			t.Error("Encryption/decryption failed")
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := NewEncryptionServiceFromString("not-valid-base64!!!")
		if err == nil {
			t.Error("Expected error for invalid base64")
		}
	})
}

func TestEncryptBytes(t *testing.T) {
	key, _ := GenerateKey(32)
	svc, _ := NewEncryptionService(key)

	t.Run("encrypt and decrypt bytes", func(t *testing.T) {
		plaintext := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}

		ciphertext, err := svc.EncryptBytes(plaintext)
		if err != nil {
			t.Fatalf("EncryptBytes failed: %v", err)
		}

		decrypted, err := svc.DecryptBytes(ciphertext)
		if err != nil {
			t.Fatalf("DecryptBytes failed: %v", err)
		}

		if string(decrypted) != string(plaintext) {
			t.Error("Decrypted bytes don't match")
		}
	})

	t.Run("encrypt nil/empty bytes", func(t *testing.T) {
		ciphertext, err := svc.EncryptBytes(nil)
		if err != nil || ciphertext != nil {
			t.Error("Encrypting nil should return nil, nil")
		}

		ciphertext, err = svc.EncryptBytes([]byte{})
		if err != nil || ciphertext != nil {
			t.Error("Encrypting empty slice should return nil, nil")
		}
	})
}

func TestKeyID(t *testing.T) {
	key1, _ := GenerateKey(32)
	key2, _ := GenerateKey(32)

	svc1, _ := NewEncryptionService(key1)
	svc2, _ := NewEncryptionService(key2)

	if svc1.KeyID() == "" {
		t.Error("KeyID should not be empty")
	}

	if svc1.KeyID() == svc2.KeyID() {
		t.Error("Different keys should produce different KeyIDs")
	}

	// Same key should produce same KeyID
	svc1b, _ := NewEncryptionService(key1)
	if svc1.KeyID() != svc1b.KeyID() {
		t.Error("Same key should produce same KeyID")
	}
}

func TestGenerateKey(t *testing.T) {
	t.Run("valid sizes", func(t *testing.T) {
		for _, size := range []int{16, 24, 32} {
			key, err := GenerateKey(size)
			if err != nil {
				t.Errorf("GenerateKey(%d) failed: %v", size, err)
			}
			if len(key) != size {
				t.Errorf("GenerateKey(%d) returned %d bytes", size, len(key))
			}
		}
	})

	t.Run("invalid sizes", func(t *testing.T) {
		for _, size := range []int{0, 8, 64} {
			_, err := GenerateKey(size)
			if err != ErrInvalidKey {
				t.Errorf("GenerateKey(%d) should return ErrInvalidKey", size)
			}
		}
	})

	t.Run("keys are random", func(t *testing.T) {
		key1, _ := GenerateKey(32)
		key2, _ := GenerateKey(32)
		if string(key1) == string(key2) {
			t.Error("Generated keys should be random")
		}
	})
}

func BenchmarkEncrypt(b *testing.B) {
	key, _ := GenerateKey(32)
	svc, _ := NewEncryptionService(key)
	plaintext := "sk-1234567890abcdefghijklmnopqrstuvwxyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Encrypt(plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key, _ := GenerateKey(32)
	svc, _ := NewEncryptionService(key)
	plaintext := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	ciphertext, _ := svc.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Decrypt(ciphertext)
	}
}

