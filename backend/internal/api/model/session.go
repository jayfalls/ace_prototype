package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a user session in the system.
type Session struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	RefreshTokenHash *string   `json:"-"`
	UserAgent        string    `json:"user_agent,omitempty"`
	IPAddress        string    `json:"ip_address,omitempty"`
	LastUsedAt       time.Time `json:"last_used_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid returns true if the session is valid (not expired).
func (s *Session) IsValid() bool {
	return !s.IsExpired()
}
