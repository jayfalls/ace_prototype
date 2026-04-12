package service

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
)

func TestNewTokenService(t *testing.T) {
	t.Run("creates service with generated keys", func(t *testing.T) {
		config := &TokenConfig{
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			AccessTokenTTL: 15 * time.Minute,
		}

		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if service == nil {
			t.Fatal("service should not be nil")
		}

		if service.config.PrivateKey == nil {
			t.Fatal("private key should be generated")
		}

		if service.config.PublicKey == nil {
			t.Fatal("public key should be generated")
		}
	})

	t.Run("uses provided keys", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate key: %v", err)
		}

		config := &TokenConfig{
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			PrivateKey:     privateKey,
			PublicKey:      &privateKey.PublicKey,
			AccessTokenTTL: 15 * time.Minute,
		}

		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if service.config.PrivateKey != privateKey {
			t.Error("should use provided private key")
		}
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewTokenService(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("applies defaults", func(t *testing.T) {
		config := &TokenConfig{}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if service.config.Issuer != "ace-auth" {
			t.Errorf("expected default issuer 'ace-auth', got %q", service.config.Issuer)
		}

		if service.config.Audience != "ace-api" {
			t.Errorf("expected default audience 'ace-api', got %q", service.config.Audience)
		}

		if service.config.AccessTokenTTL != 15*time.Minute {
			t.Errorf("expected default access TTL 15m, got %v", service.config.AccessTokenTTL)
		}

		if service.config.RefreshTokenTTL != 7*24*time.Hour {
			t.Errorf("expected default refresh TTL 7d, got %v", service.config.RefreshTokenTTL)
		}
	})
}

func TestTokenService_GenerateAccessToken(t *testing.T) {
	setupService := func() *TokenService {
		config := &TokenConfig{
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			AccessTokenTTL: 15 * time.Minute,
		}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		return service
	}

	t.Run("generates valid JWT token", func(t *testing.T) {
		service := setupService()

		claims := &model.TokenClaims{
			Sub:   uuid.New(),
			Role:  "user",
			Email: "test@example.com",
		}

		token, err := service.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if token == "" {
			t.Fatal("token should not be empty")
		}

		// Token should have 3 parts separated by dots
		parts := splitToken(token)
		if len(parts) != 3 {
			t.Errorf("expected 3 parts, got %d", len(parts))
		}
	})

	t.Run("includes JTI if not provided", func(t *testing.T) {
		service := setupService()

		claims := &model.TokenClaims{
			Sub:   uuid.New(),
			Role:  "user",
			Email: "test@example.com",
		}

		token, err := service.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Validate the token to check JTI was generated
		validatedClaims, err := service.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if validatedClaims.Jti == uuid.Nil {
			t.Error("JTI should be generated")
		}
	})

	t.Run("returns error for nil claims", func(t *testing.T) {
		service := setupService()

		_, err := service.GenerateAccessToken(nil)
		if err == nil {
			t.Fatal("expected error for nil claims")
		}
	})
}

func TestTokenService_GenerateRefreshToken(t *testing.T) {
	setupService := func() *TokenService {
		config := &TokenConfig{
			Issuer:          "test-issuer",
			Audience:        "test-audience",
			RefreshTokenTTL: 7 * 24 * time.Hour,
		}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		return service
	}

	t.Run("generates valid refresh token", func(t *testing.T) {
		service := setupService()

		userID := uuid.New()
		sessionID := uuid.New()

		token, err := service.GenerateRefreshToken(userID, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if token == "" {
			t.Fatal("token should not be empty")
		}

		parts := splitToken(token)
		if len(parts) != 3 {
			t.Errorf("expected 3 parts, got %d", len(parts))
		}
	})

	t.Run("returns error for nil user ID", func(t *testing.T) {
		service := setupService()

		_, err := service.GenerateRefreshToken(uuid.Nil, uuid.New())
		if err == nil {
			t.Fatal("expected error for nil user ID")
		}
	})

	t.Run("returns error for nil session ID", func(t *testing.T) {
		service := setupService()

		_, err := service.GenerateRefreshToken(uuid.New(), uuid.Nil)
		if err == nil {
			t.Fatal("expected error for nil session ID")
		}
	})
}

func TestTokenService_ValidateAccessToken(t *testing.T) {
	setupService := func() *TokenService {
		config := &TokenConfig{
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			AccessTokenTTL: 15 * time.Minute,
		}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		return service
	}

	t.Run("validates correct token", func(t *testing.T) {
		service := setupService()

		claims := &model.TokenClaims{
			Sub:   uuid.New(),
			Role:  "user",
			Email: "test@example.com",
		}

		token, err := service.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		validatedClaims, err := service.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if validatedClaims.Sub != claims.Sub {
			t.Errorf("expected subject %v, got %v", claims.Sub, validatedClaims.Sub)
		}

		if validatedClaims.Role != claims.Role {
			t.Errorf("expected role %q, got %q", claims.Role, validatedClaims.Role)
		}

		if validatedClaims.Email != claims.Email {
			t.Errorf("expected email %q, got %q", claims.Email, validatedClaims.Email)
		}
	})

	t.Run("rejects token with wrong issuer", func(t *testing.T) {
		service := setupService()

		// Generate token with different service
		otherConfig := &TokenConfig{
			Issuer:         "other-issuer",
			Audience:       "test-audience",
			AccessTokenTTL: 15 * time.Minute,
		}
		otherService, _ := NewTokenService(otherConfig)

		claims := &model.TokenClaims{
			Sub:   uuid.New(),
			Role:  "user",
			Email: "test@example.com",
		}

		token, _ := otherService.GenerateAccessToken(claims)

		_, err := service.ValidateAccessToken(token)
		if err != ErrTokenInvalid {
			t.Errorf("expected ErrTokenInvalid, got %v", err)
		}
	})

	t.Run("rejects token with wrong audience", func(t *testing.T) {
		service := setupService()

		otherConfig := &TokenConfig{
			Issuer:         "test-issuer",
			Audience:       "other-audience",
			AccessTokenTTL: 15 * time.Minute,
		}
		otherService, _ := NewTokenService(otherConfig)

		claims := &model.TokenClaims{
			Sub:   uuid.New(),
			Role:  "user",
			Email: "test@example.com",
		}

		token, _ := otherService.GenerateAccessToken(claims)

		_, err := service.ValidateAccessToken(token)
		if err != ErrTokenInvalid {
			t.Errorf("expected ErrTokenInvalid, got %v", err)
		}
	})

	t.Run("rejects malformed token", func(t *testing.T) {
		service := setupService()

		_, err := service.ValidateAccessToken("not-a-valid-jwt")
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("rejects empty token", func(t *testing.T) {
		service := setupService()

		_, err := service.ValidateAccessToken("")
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})
}

func TestTokenService_ValidateRefreshToken(t *testing.T) {
	setupService := func() *TokenService {
		config := &TokenConfig{
			Issuer:          "test-issuer",
			Audience:        "test-audience",
			RefreshTokenTTL: 7 * 24 * time.Hour,
		}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		return service
	}

	t.Run("validates refresh token correctly", func(t *testing.T) {
		service := setupService()

		userID := uuid.New()
		sessionID := uuid.New()

		token, err := service.GenerateRefreshToken(userID, sessionID)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		refreshData, err := service.ValidateRefreshToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if refreshData.UserID != userID {
			t.Errorf("expected user ID %v, got %v", userID, refreshData.UserID)
		}

		if refreshData.SessionID != sessionID {
			t.Errorf("expected session ID %v, got %v", sessionID, refreshData.SessionID)
		}

		if refreshData.JTI == uuid.Nil {
			t.Error("JTI should be set")
		}
	})

	t.Run("rejects invalid refresh token", func(t *testing.T) {
		service := setupService()

		_, err := service.ValidateRefreshToken("invalid-token")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})
}

func TestTokenService_GenerateTokenPair(t *testing.T) {
	setupService := func() *TokenService {
		config := &TokenConfig{
			Issuer:          "test-issuer",
			Audience:        "test-audience",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		}
		service, err := NewTokenService(config)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		return service
	}

	t.Run("generates token pair", func(t *testing.T) {
		service := setupService()

		user := &model.User{
			ID:    uuid.New(),
			Email: "test@example.com",
			Role:  model.RoleUser,
		}
		sessionID := uuid.New()

		pair, err := service.GenerateTokenPair(user, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pair.AccessToken == "" {
			t.Error("access token should not be empty")
		}

		if pair.RefreshToken == "" {
			t.Error("refresh token should not be empty")
		}

		if pair.TokenType != "Bearer" {
			t.Errorf("expected token type 'Bearer', got %q", pair.TokenType)
		}

		// Default 15 minutes = 900 seconds
		if pair.ExpiresIn != 900 {
			t.Errorf("expected expires_in 900, got %d", pair.ExpiresIn)
		}
	})

	t.Run("returns error for nil user", func(t *testing.T) {
		service := setupService()

		_, err := service.GenerateTokenPair(nil, uuid.New())
		if err == nil {
			t.Fatal("expected error for nil user")
		}
	})

	t.Run("returns error for nil session ID", func(t *testing.T) {
		service := setupService()

		user := &model.User{
			ID:    uuid.New(),
			Email: "test@example.com",
			Role:  model.RoleUser,
		}

		_, err := service.GenerateTokenPair(user, uuid.Nil)
		if err == nil {
			t.Fatal("expected error for nil session ID")
		}
	})
}

func TestTokenService_RoundTrip(t *testing.T) {
	// Test that tokens generated by one service can be validated by another
	// This simulates using the same keys across restarts
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	config1 := &TokenConfig{
		Issuer:         "test-issuer",
		Audience:       "test-audience",
		PrivateKey:     privateKey,
		PublicKey:      &privateKey.PublicKey,
		AccessTokenTTL: 15 * time.Minute,
	}
	service1, _ := NewTokenService(config1)

	config2 := &TokenConfig{
		Issuer:         "test-issuer",
		Audience:       "test-audience",
		PrivateKey:     privateKey,
		PublicKey:      &privateKey.PublicKey,
		AccessTokenTTL: 15 * time.Minute,
	}
	service2, _ := NewTokenService(config2)

	claims := &model.TokenClaims{
		Sub:   uuid.New(),
		Role:  "admin",
		Email: "admin@example.com",
	}

	// Generate with service1
	token, err := service1.GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Validate with service2 (same keys)
	validatedClaims, err := service2.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("failed to validate token with service2: %v", err)
	}

	if validatedClaims.Sub != claims.Sub {
		t.Errorf("subject mismatch")
	}

	if validatedClaims.Role != claims.Role {
		t.Errorf("role mismatch")
	}
}

func TestHashRefreshToken(t *testing.T) {
	t.Run("generates consistent hash", func(t *testing.T) {
		token := "test-refresh-token"

		hash1 := HashRefreshToken(token)
		hash2 := HashRefreshToken(token)

		if hash1 != hash2 {
			t.Error("hash should be consistent for same token")
		}
	})

	t.Run("different tokens produce different hashes", func(t *testing.T) {
		hash1 := HashRefreshToken("token1")
		hash2 := HashRefreshToken("token2")

		if hash1 == hash2 {
			t.Error("different tokens should produce different hashes")
		}
	})
}

// splitToken is a test helper that splits a JWT into its parts
func splitToken(token string) []string {
	var parts []string
	var current []byte

	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, string(current))
			current = nil
		} else {
			current = append(current, token[i])
		}
	}

	if len(current) > 0 {
		parts = append(parts, string(current))
	}

	return parts
}
