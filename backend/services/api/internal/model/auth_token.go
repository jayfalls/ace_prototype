package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthTokenType represents the type of authentication token.
type AuthTokenType string

const (
	TokenTypeLogin         AuthTokenType = "login"
	TokenTypeVerification  AuthTokenType = "verification"
	TokenTypePasswordReset AuthTokenType = "password_reset"
)

// AuthToken represents a magic link token or verification token.
type AuthToken struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id"`
	TokenType AuthTokenType `json:"token_type"`
	TokenHash string        `json:"-"` // Never exposed in JSON
	ExpiresAt time.Time     `json:"expires_at"`
	UsedAt    *time.Time    `json:"used_at,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// IsExpired checks if the token has expired.
func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed checks if the token has been used.
func (t *AuthToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsValid checks if the token is valid (not expired, not used).
func (t *AuthToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed()
}
