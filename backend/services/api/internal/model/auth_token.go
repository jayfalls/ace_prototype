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

// AuthToken represents a one-time authentication token.
type AuthToken struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id"`
	TokenType AuthTokenType `json:"token_type"`
	TokenHash *string       `json:"-"`
	ExpiresAt time.Time     `json:"expires_at"`
	UsedAt    *time.Time    `json:"used_at,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// IsExpired returns true if the token has expired.
func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed returns true if the token has been used.
func (t *AuthToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsValid returns true if the token is valid (not expired and not used).
func (t *AuthToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed()
}
