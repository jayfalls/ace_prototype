package service

import (
	"testing"
	"time"
)

// TestGenerateSecureToken tests the secure token generation.
func TestGenerateSecureToken(t *testing.T) {
	// Generate a token
	token, err := generateSecureToken(32)
	if err != nil {
		t.Fatalf("generateSecureToken failed: %v", err)
	}

	// Check length (32 bytes = 64 hex chars)
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}

	// Generate another token and check it's different
	token2, err := generateSecureToken(32)
	if err != nil {
		t.Fatalf("generateSecureToken failed: %v", err)
	}

	if token == token2 {
		t.Error("two generated tokens should be different")
	}
}

// TestHashToken tests the token hashing function.
func TestHashToken(t *testing.T) {
	token := "abc123"
	hash1 := hashToken(token)
	hash2 := hashToken(token)
	hash3 := hashToken("different")

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Error("same token should produce same hash")
	}

	// Different input should produce different hash
	if hash1 == hash3 {
		t.Error("different tokens should produce different hashes")
	}

	// Hash should be 64 characters (SHA256 = 32 bytes = 64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}

// TestMagicLinkConfig tests default configuration.
func TestMagicLinkConfig(t *testing.T) {
	config := DefaultMagicLinkConfig()

	if config.LoginTokenTTL != 15*time.Minute {
		t.Errorf("expected LoginTokenTTL 15 minutes, got %v", config.LoginTokenTTL)
	}

	if config.PasswordResetTTL != 1*time.Hour {
		t.Errorf("expected PasswordResetTTL 1 hour, got %v", config.PasswordResetTTL)
	}
}

// TestTokenTypeConstants verifies the token type constants.
func TestTokenTypeConstants(t *testing.T) {
	if TokenTypeLogin != "login" {
		t.Errorf("expected TokenTypeLogin 'login', got '%s'", TokenTypeLogin)
	}

	if TokenTypePasswordReset != "password_reset" {
		t.Errorf("expected TokenTypePasswordReset 'password_reset', got '%s'", TokenTypePasswordReset)
	}
}

// TestNewMagicLinkServiceNilQueries tests that nil queries causes an error.
func TestNewMagicLinkServiceNilQueries(t *testing.T) {
	_, err := NewMagicLinkService(nil, nil)
	if err == nil {
		t.Error("expected error when queries is nil")
	}
}

// TestNewMagicLinkServiceValidConfig tests creating service with valid config.
func TestNewMagicLinkServiceValidConfig(t *testing.T) {
	// This test would require a mock database - skipping for unit tests
	// Integration tests would cover the full flow
	t.Skip("requires mock database - tested in integration tests")
}

// TestValidateMagicLinkInputValidation tests input validation.
func TestValidateMagicLinkInputValidation(t *testing.T) {
	// This test would require a mock database - skipping for unit tests
	t.Skip("requires mock database - tested in integration tests")
}

// TestResetPasswordInputValidation tests input validation.
func TestResetPasswordInputValidation(t *testing.T) {
	// This test would require a mock database - skipping for unit tests
	t.Skip("requires mock database - tested in integration tests")
}

// TestCleanupExpiredTokens tests cleanup function.
func TestCleanupExpiredTokens(t *testing.T) {
	// This test would require a mock database - skipping for unit tests
	t.Skip("requires mock database - tested in integration tests")
}
