// Package crypto provides encryption and decryption services for sensitive data
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	// ErrInvalidKey is returned when the encryption key is invalid
	ErrInvalidKey = errors.New("invalid encryption key: must be 16, 24, or 32 bytes")

	// ErrInvalidCiphertext is returned when the ciphertext is malformed
	ErrInvalidCiphertext = errors.New("invalid ciphertext: too short")

	// ErrDecryptionFailed is returned when decryption fails
	ErrDecryptionFailed = errors.New("decryption failed: authentication failed")
)

// EncryptionService handles encryption and decryption of sensitive data
// Uses AES-GCM for authenticated encryption
type EncryptionService struct {
	key    []byte
	gcm    cipher.AEAD
	mu     sync.RWMutex
	keyID  string // Identifier for key rotation tracking
}

// NewEncryptionService creates a new encryption service with the given key
// Key must be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes
func NewEncryptionService(key []byte) (*EncryptionService, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a key ID from the key hash (first 8 bytes)
	keyHash := sha256.Sum256(key)
	keyID := base64.RawURLEncoding.EncodeToString(keyHash[:8])

	return &EncryptionService{
		key:   key,
		gcm:   gcm,
		keyID: keyID,
	}, nil
}

// NewEncryptionServiceFromString creates an encryption service from a base64-encoded key
func NewEncryptionServiceFromString(encodedKey string) (*EncryptionService, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}
	return NewEncryptionService(key)
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext
// The ciphertext includes the nonce prepended to the encrypted data
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generate a random nonce
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext
func (s *EncryptionService) Decrypt(encodedCiphertext string) (string, error) {
	if encodedCiphertext == "" {
		return "", nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Check minimum length (nonce + at least 1 byte + auth tag)
	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize+s.gcm.Overhead()+1 {
		return "", ErrInvalidCiphertext
	}

	// Extract nonce and encrypted data
	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := s.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// EncryptBytes encrypts binary data
func (s *EncryptionService) EncryptBytes(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return s.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBytes decrypts binary data
func (s *EncryptionService) DecryptBytes(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize+s.gcm.Overhead()+1 {
		return nil, ErrInvalidCiphertext
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	plaintext, err := s.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// KeyID returns the identifier for this encryption key
// Useful for key rotation tracking
func (s *EncryptionService) KeyID() string {
	return s.keyID
}

// GenerateKey generates a random encryption key of the specified size
// Size should be 16, 24, or 32 bytes
func GenerateKey(size int) ([]byte, error) {
	if size != 16 && size != 24 && size != 32 {
		return nil, ErrInvalidKey
	}

	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	return key, nil
}

// GenerateKeyString generates a random encryption key and returns it as base64
func GenerateKeyString(size int) (string, error) {
	key, err := GenerateKey(size)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

