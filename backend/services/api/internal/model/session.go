package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a user session with refresh token.
type Session struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	RefreshTokenHash string     `json:"-"` // Never exposed in JSON
	UserAgent        string     `json:"user_agent,omitempty"`
	IPAddress        string     `json:"ip_address,omitempty"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// IsExpired checks if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid checks if the session is still valid (not expired, not used too long ago).
func (s *Session) IsValid() bool {
	if s.IsExpired() {
		return false
	}
	// Consider session invalid if not used in 30 days
	if s.LastUsedAt != nil {
		if time.Since(*s.LastUsedAt) > 30*24*time.Hour {
			return false
		}
	}
	return true
}
