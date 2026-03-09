package services_test

import (
	"testing"
	"time"

	"github.com/ace/framework/backend/internal/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAuthService_GenerateAndValidateToken(t *testing.T) {
	authService := services.NewAuthService("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	username := "testuser"
	email := "test@example.com"
	role := "admin"

	// Generate tokens
	tokens, err := authService.GenerateTokenPair(userID, username, email, role)
	if err != nil {
		t.Fatalf("Failed to generate tokens: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("Access token should not be empty")
	}

	if tokens.RefreshToken == "" {
		t.Error("Refresh token should not be empty")
	}

	if tokens.ExpiresIn == 0 {
		t.Error("ExpiresIn should not be zero")
	}

	// Validate access token
	claims, err := authService.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("Failed to validate access token: %v", err)
	}

	if claims.Username != username {
		t.Errorf("Expected username %s, got %s", username, claims.Username)
	}

	if claims.Email != email {
		t.Errorf("Expected email %s, got %s", email, claims.Email)
	}

	if claims.Role != role {
		t.Errorf("Expected role %s, got %s", role, claims.Role)
	}
}

func TestAuthService_ValidateInvalidToken(t *testing.T) {
	authService := services.NewAuthService("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	// Test with empty token
	_, err := authService.ValidateToken("")
	if err == nil {
		t.Error("Should fail with empty token")
	}

	// Test with invalid token
	_, err = authService.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Should fail with invalid token")
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	authService := services.NewAuthService("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Generate initial tokens
	tokens, err := authService.GenerateTokenPair(userID, "user", "user@test.com", "user")
	if err != nil {
		t.Fatalf("Failed to generate tokens: %v", err)
	}

	// Refresh tokens
	newTokens, err := authService.RefreshToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	// Both should be valid JWT tokens (different but both valid)
	_, err = authService.ValidateToken(newTokens.AccessToken)
	if err != nil {
		t.Error("New access token should be valid")
	}
}

func TestAuthService_HashAndCheckPassword(t *testing.T) {
	authService := services.NewAuthService("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	password := "test-password-123"

	// Hash password
	hash, err := authService.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == password {
		t.Error("Hash should be different from password")
	}

	// Check correct password
	if !authService.CheckPassword(password, hash) {
		t.Error("Should verify correct password")
	}

	// Check wrong password
	if authService.CheckPassword("wrong-password", hash) {
		t.Error("Should reject wrong password")
	}
}

func TestClaims_Register(t *testing.T) {
	testUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	// Test JWT claims registration
	claims := &services.Claims{
		UserID:   testUserID,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	if claims.UserID != testUserID {
		t.Errorf("Expected UserID %s, got %s", testUserID, claims.UserID)
	}
}
