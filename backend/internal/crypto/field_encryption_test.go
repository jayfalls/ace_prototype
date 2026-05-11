package crypto

import (
	"encoding/hex"
	"strings"
	"testing"
)

// validMasterKey is a valid 32-byte key for testing.
// Hex: 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
var validMasterKey = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple string", "sk-test-api-key-12345"},
		{"empty string", ""},
		{"single character", "a"},
		{"unicode string", "🔑 test-ключ-日本語"},
		{"long key", strings.Repeat("sk-very-long-api-key-pattern-", 50)},
		{"special characters", "key with spaces and\nnewlines\tand \"quotes\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := EncryptField(tt.plaintext, validMasterKey)
			if err != nil {
				t.Fatalf("EncryptField failed: %v", err)
			}

			decrypted, err := DecryptField(field, validMasterKey)
			if err != nil {
				t.Fatalf("DecryptField failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("round-trip failed: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestDecryptFieldTamperDetection(t *testing.T) {
	plaintext := "sk-sensitive-api-key"

	field, err := EncryptField(plaintext, validMasterKey)
	if err != nil {
		t.Fatalf("EncryptField failed: %v", err)
	}

	// Tamper with ciphertext by flipping a byte
	tamperedField := field
	tamperedField.Ciphertext = make([]byte, len(field.Ciphertext))
	copy(tamperedField.Ciphertext, field.Ciphertext)
	if len(tamperedField.Ciphertext) > 0 {
		tamperedField.Ciphertext[0] ^= 0xff
	}

	_, err = DecryptField(tamperedField, validMasterKey)
	if err == nil {
		t.Error("decrypt should have failed on tampered ciphertext")
	}

	// Tamper with EncryptedDEK
	tamperedDEK := field
	tamperedDEK.EncryptedDEK = make([]byte, len(field.EncryptedDEK))
	copy(tamperedDEK.EncryptedDEK, field.EncryptedDEK)
	if len(tamperedDEK.EncryptedDEK) > 0 {
		tamperedDEK.EncryptedDEK[0] ^= 0xff
	}

	_, err = DecryptField(tamperedDEK, validMasterKey)
	if err == nil {
		t.Error("decrypt should have failed on tampered encrypted DEK")
	}

	// Tamper with DEK nonce
	tamperedDEKNonce := field
	tamperedDEKNonce.DEKNonce = make([]byte, len(field.DEKNonce))
	copy(tamperedDEKNonce.DEKNonce, field.DEKNonce)
	if len(tamperedDEKNonce.DEKNonce) > 0 {
		tamperedDEKNonce.DEKNonce[0] ^= 0xff
	}

	_, err = DecryptField(tamperedDEKNonce, validMasterKey)
	if err == nil {
		t.Error("decrypt should have failed on tampered DEK nonce")
	}
}

func TestEncryptFieldInvalidMasterKey(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"nil key", nil},
		{"empty key", []byte{}},
		{"too short (16 bytes)", make([]byte, 16)},
		{"too short (31 bytes)", make([]byte, 31)},
		{"too long (33 bytes)", make([]byte, 33)},
		{"too long (64 bytes)", make([]byte, 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncryptField("test", tt.key)
			if err == nil {
				t.Error("EncryptField should have failed with invalid key")
			}
		})
	}
}

func TestDecryptFieldInvalidMasterKey(t *testing.T) {
	plaintext := "sk-test"
	field, err := EncryptField(plaintext, validMasterKey)
	if err != nil {
		t.Fatalf("EncryptField failed: %v", err)
	}

	_, err = DecryptField(field, nil)
	if err == nil {
		t.Error("DecryptField should have failed with nil key")
	}

	_, err = DecryptField(field, make([]byte, 16))
	if err == nil {
		t.Error("DecryptField should have failed with 16-byte key")
	}
}

func TestDecryptFieldWrongMasterKey(t *testing.T) {
	plaintext := "sk-test"

	field, err := EncryptField(plaintext, validMasterKey)
	if err != nil {
		t.Fatalf("EncryptField failed: %v", err)
	}

	// Use a different valid key
	differentKey := make([]byte, 32)
	copy(differentKey, validMasterKey)
	differentKey[0] ^= 0xff

	_, err = DecryptField(field, differentKey)
	if err == nil {
		t.Error("DecryptField should have failed with wrong master key")
	}
}

func TestEncryptionVersionIsSet(t *testing.T) {
	field, err := EncryptField("test", validMasterKey)
	if err != nil {
		t.Fatalf("EncryptField failed: %v", err)
	}

	if field.EncryptionVersion != CurrentEncryptionVersion {
		t.Errorf("EncryptionVersion = %d, want %d", field.EncryptionVersion, CurrentEncryptionVersion)
	}
}

func TestEncryptFieldProducesDifferentOutputs(t *testing.T) {
	plaintext := "same-api-key"
	key1, err := EncryptField(plaintext, validMasterKey)
	if err != nil {
		t.Fatalf("first EncryptField failed: %v", err)
	}

	key2, err := EncryptField(plaintext, validMasterKey)
	if err != nil {
		t.Fatalf("second EncryptField failed: %v", err)
	}

	// Each encryption should produce different nonces and ciphertext
	if hex.EncodeToString(key1.Ciphertext) == hex.EncodeToString(key2.Ciphertext) {
		t.Error("two encryptions of the same plaintext produced identical ciphertext")
	}
	if hex.EncodeToString(key1.Nonce) == hex.EncodeToString(key2.Nonce) {
		t.Error("two encryptions produced identical API key nonces")
	}
	if hex.EncodeToString(key1.EncryptedDEK) == hex.EncodeToString(key2.EncryptedDEK) {
		t.Error("two encryptions produced identical encrypted DEKs")
	}
}

func TestFieldSizes(t *testing.T) {
	field, err := EncryptField("test", validMasterKey)
	if err != nil {
		t.Fatalf("EncryptField failed: %v", err)
	}

	if len(field.Nonce) != 12 {
		t.Errorf("API key nonce length = %d, want 12", len(field.Nonce))
	}
	if len(field.DEKNonce) != 12 {
		t.Errorf("DEK nonce length = %d, want 12", len(field.DEKNonce))
	}
	if len(field.EncryptedDEK) == 0 {
		t.Error("encrypted DEK must not be empty")
	}
	if len(field.Ciphertext) == 0 {
		t.Error("ciphertext must not be empty")
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("GenerateMasterKey returned %d bytes, want 32", len(key))
	}

	// Test round-trip with generated key
	plaintext := "round-trip-with-generated-key"
	field, err := EncryptField(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptField with generated key failed: %v", err)
	}

	decrypted, err := DecryptField(field, key)
	if err != nil {
		t.Fatalf("DecryptField with generated key failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("generated key round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}
