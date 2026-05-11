// Package crypto provides envelope encryption for provider API keys.
//
// Each provider API key is encrypted with a random Data Encryption Key (DEK).
// The DEK is then encrypted with a master Key Encryption Key (KEK) provided
// at startup from the PROVIDER_ENCRYPTION_KEY environment variable.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// EncryptedField holds all components of an envelope-encrypted value.
type EncryptedField struct {
	Ciphertext        []byte // API key encrypted with DEK
	Nonce             []byte // 12-byte nonce for API key AES-GCM
	EncryptedDEK      []byte // DEK encrypted with master key
	DEKNonce          []byte // 12-byte nonce for DEK AES-GCM
	EncryptionVersion int    // version of the encryption scheme
}

const (
	// CurrentEncryptionVersion is the active encryption scheme version.
	CurrentEncryptionVersion = 1

	// keySize is the AES-256 key size in bytes.
	keySize = 32
	// nonceSize is the AES-GCM nonce size in bytes (always 12).
	nonceSize = 12
)

// EncryptField encrypts plaintext using envelope encryption with the provided master key.
//
// Algorithm:
//  1. Generate a random 32-byte DEK.
//  2. Generate a random 12-byte nonce for the API key.
//  3. Encrypt plaintext with AES-256-GCM using DEK + API key nonce.
//  4. Generate a random 12-byte nonce for the DEK.
//  5. Encrypt DEK with AES-256-GCM using masterKey + DEK nonce.
//  6. Return all four values plus version 1.
func EncryptField(plaintext string, masterKey []byte) (EncryptedField, error) {
	if err := validateMasterKey(masterKey); err != nil {
		return EncryptedField{}, err
	}

	// Step 1: Generate random 32-byte DEK
	dek := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return EncryptedField{}, fmt.Errorf("generate DEK: %w", err)
	}

	// Step 2 & 4: Generate random nonces
	apiKeyNonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, apiKeyNonce); err != nil {
		return EncryptedField{}, fmt.Errorf("generate API key nonce: %w", err)
	}

	dekNonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, dekNonce); err != nil {
		return EncryptedField{}, fmt.Errorf("generate DEK nonce: %w", err)
	}

	// Step 5: Encrypt DEK with masterKey
	encryptedDEK, err := encrypt(dek, masterKey, dekNonce)
	if err != nil {
		return EncryptedField{}, fmt.Errorf("encrypt DEK: %w", err)
	}

	// Step 3: Encrypt plaintext with DEK
	ciphertext, err := encrypt([]byte(plaintext), dek, apiKeyNonce)
	if err != nil {
		return EncryptedField{}, fmt.Errorf("encrypt API key: %w", err)
	}

	return EncryptedField{
		Ciphertext:        ciphertext,
		Nonce:             apiKeyNonce,
		EncryptedDEK:      encryptedDEK,
		DEKNonce:          dekNonce,
		EncryptionVersion: CurrentEncryptionVersion,
	}, nil
}

// DecryptField decrypts an EncryptedField using the provided master key.
//
// Algorithm:
//  1. Decrypt the DEK using the masterKey + DEK nonce.
//  2. Decrypt the plaintext using the DEK + API key nonce.
//  3. Return the plaintext as a string.
func DecryptField(field EncryptedField, masterKey []byte) (string, error) {
	if err := validateMasterKey(masterKey); err != nil {
		return "", err
	}

	// Step 1: Decrypt DEK using masterKey
	dek, err := decrypt(field.EncryptedDEK, masterKey, field.DEKNonce)
	if err != nil {
		return "", fmt.Errorf("decrypt DEK: %w", err)
	}

	if len(dek) != keySize {
		return "", errors.New("decrypted DEK has invalid length")
	}

	// Step 2: Decrypt plaintext using DEK
	plaintext, err := decrypt(field.Ciphertext, dek, field.Nonce)
	if err != nil {
		return "", fmt.Errorf("decrypt API key: %w", err)
	}

	return string(plaintext), nil
}

// GenerateMasterKey generates a new random 32-byte master key suitable for use
// as a KEK. The hex-encoded result should be stored in the PROVIDER_ENCRYPTION_KEY
// environment variable.
func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	return key, nil
}

// validateMasterKey ensures the master key is exactly 32 bytes.
func validateMasterKey(key []byte) error {
	if len(key) != keySize {
		return fmt.Errorf("master key must be exactly %d bytes, got %d", keySize, len(key))
	}
	return nil
}

// encrypt encrypts plaintext using AES-256-GCM with the provided key and nonce.
// The nonce is prepended to the ciphertext (standard GCM convention).
func encrypt(plaintext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Seal appends the ciphertext to the nil slice and includes the nonce as
	// authentication data implicitly. The nonce is NOT prepended automatically.
	return aesgcm.Seal(nil, nonce, plaintext, nil), nil
}

// decrypt decrypts ciphertext using AES-256-GCM with the provided key and nonce.
func decrypt(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
