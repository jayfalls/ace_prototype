package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
)

// Magic link token type constants
const (
	TokenTypeLogin         = "login"
	TokenTypePasswordReset = "password_reset"
)

// Magic link service errors
var (
	ErrTokenAlreadyUsed = errors.New("token has already been used")
	ErrWrongTokenType   = errors.New("wrong token type")
)

// MagicLinkConfig holds configuration for magic link tokens.
type MagicLinkConfig struct {
	LoginTokenTTL    time.Duration // TTL for login magic links (default 15 minutes)
	PasswordResetTTL time.Duration // TTL for password reset magic links (default 1 hour)
}

// DefaultMagicLinkConfig returns sensible defaults for magic link configuration.
func DefaultMagicLinkConfig() *MagicLinkConfig {
	return &MagicLinkConfig{
		LoginTokenTTL:    15 * time.Minute,
		PasswordResetTTL: 1 * time.Hour,
	}
}

// MagicLinkService handles magic link token generation and validation.
type MagicLinkService struct {
	queries *db.Queries
	config  *MagicLinkConfig
}

// NewMagicLinkService creates a new magic link service.
func NewMagicLinkService(
	queries *db.Queries,
	config *MagicLinkConfig,
) (*MagicLinkService, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	// Apply defaults if config is nil
	if config == nil {
		config = DefaultMagicLinkConfig()
	}

	return &MagicLinkService{
		queries: queries,
		config:  config,
	}, nil
}

// GenerateMagicLink generates a magic link token for the specified email and token type.
// It creates a cryptographically secure random token, hashes it for storage,
// stores it in the database, and returns the plain token to be sent via email.
//
// Parameters:
//   - ctx: Context for database operations
//   - email: User's email address
//   - tokenType: Either "login" or "password_reset"
//
// Returns:
//   - The plain token (to be sent via email)
//   - Error if operation fails
func (s *MagicLinkService) GenerateMagicLink(ctx context.Context, email, tokenType string) (string, error) {
	if ctx == nil {
		return "", errors.New("context is required")
	}
	if email == "" {
		return "", errors.New("email is required")
	}
	if tokenType != TokenTypeLogin && tokenType != TokenTypePasswordReset {
		return "", errors.New("invalid token type: must be 'login' or 'password_reset'")
	}

	// Get user by email
	dbUser, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		// Return empty string but no error - prevents email enumeration
		// In production, you might log this
		return "", nil
	}

	// Determine TTL based on token type
	var ttl time.Duration
	if tokenType == TokenTypeLogin {
		ttl = s.config.LoginTokenTTL
	} else {
		ttl = s.config.PasswordResetTTL
	}

	// Generate cryptographically secure random token (32 bytes = 64 hex chars)
	token, err := generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	// Hash token for storage
	tokenHash := hashToken(token)

	// Calculate expiry time
	expiresAt := time.Now().Add(ttl)

	// Store token in database
	_, err = s.queries.CreateAuthToken(ctx, db.CreateAuthTokenParams{
		UserID:    dbUser.ID,
		TokenType: tokenType,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}

	// Return the plain token (not hashed) - caller will send via email
	return token, nil
}

// ValidateMagicLink validates a magic link token and returns the user ID.
// It verifies the token hash, checks that it hasn't been used or expired,
// marks it as used, and returns the associated user ID.
//
// Parameters:
//   - ctx: Context for database operations
//   - token: The plain token from the magic link
//   - tokenType: Either "login" or "password_reset"
//
// Returns:
//   - User ID if token is valid
//   - Error if token is invalid, expired, or already used
func (s *MagicLinkService) ValidateMagicLink(ctx context.Context, token, tokenType string) (uuid.UUID, error) {
	if ctx == nil {
		return uuid.Nil, errors.New("context is required")
	}
	if token == "" {
		return uuid.Nil, ErrInvalidToken
	}
	if tokenType != TokenTypeLogin && tokenType != TokenTypePasswordReset {
		return uuid.Nil, errors.New("invalid token type: must be 'login' or 'password_reset'")
	}

	// Hash the token to look up in database
	tokenHash := hashToken(token)

	// Look up token in database
	dbToken, err := s.queries.GetAuthTokenByHash(ctx, tokenHash)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	// Verify token type matches
	if dbToken.TokenType != tokenType {
		return uuid.Nil, ErrWrongTokenType
	}

	// Verify token hasn't expired (query already checks this, but double-check)
	if dbToken.ExpiresAt.Time.Before(time.Now()) {
		return uuid.Nil, ErrTokenExpired
	}

	// Verify token hasn't been used (query already checks this)
	if dbToken.UsedAt.Valid {
		return uuid.Nil, ErrTokenAlreadyUsed
	}

	// Mark token as used
	_, err = s.queries.MarkAuthTokenUsed(ctx, dbToken.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("mark token used: %w", err)
	}

	// Return user ID
	return dbToken.UserID.Bytes, nil
}

// ResetPassword validates a password reset token and updates the user's password.
// It verifies the magic link token, validates the new password strength,
// hashes the new password, updates the user in the database, and deletes the used token.
//
// Parameters:
//   - ctx: Context for database operations
//   - token: The password reset token from the magic link
//   - newPassword: The new password to set
//
// Returns:
//   - User model if password was reset successfully
//   - Error if token is invalid, password is weak, or operation fails
func (s *MagicLinkService) ResetPassword(ctx context.Context, token, newPassword string) (*model.User, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if token == "" {
		return nil, ErrInvalidToken
	}
	if newPassword == "" {
		return nil, errors.New("new password is required")
	}

	// Validate magic link token for password reset
	userID, err := s.ValidateMagicLink(ctx, token, TokenTypePasswordReset)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}

	// Validate password strength using the exported function from password_service.go
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrWeakPassword, err)
	}

	// Hash the new password using the exported function from password_service.go
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Update user's password in database using UpdateUser with password_hash
	_, err = s.queries.UpdateUser(ctx, db.UpdateUserParams{
		ID:           pgtype.UUID{Bytes: userID, Valid: true},
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, fmt.Errorf("update password: %w", err)
	}

	// Get updated user
	dbUser, err := s.queries.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Convert to model user
	user := &model.User{
		ID:           dbUser.ID.Bytes,
		Email:        dbUser.Email,
		PasswordHash: nil, // Never return password hash
		Role:         model.UserRole(dbUser.Role),
		Status:       model.UserStatus(dbUser.Status),
		CreatedAt:    dbUser.CreatedAt.Time,
		UpdatedAt:    dbUser.UpdatedAt.Time,
	}

	return user, nil
}

// CleanupExpiredTokens deletes all expired auth tokens from the database.
// This should be called periodically (e.g., via a cron job or background worker).
//
// Parameters:
//   - ctx: Context for database operations
//
// Returns:
//   - Error if cleanup fails
func (s *MagicLinkService) CleanupExpiredTokens(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}

	err := s.queries.DeleteExpiredAuthTokens(ctx)
	if err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}

	return nil
}

// generateSecureToken generates a cryptographically secure random token.
// The token is returned as a hex-encoded string.
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// hashToken creates a SHA256 hash of the token for storage.
// The hash is returned as a hex-encoded string.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
