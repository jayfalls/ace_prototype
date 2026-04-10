package model

import (
	"time"

	"github.com/google/uuid"
)

// TokenClaims represents the JWT claims for access tokens.
type TokenClaims struct {
	// Standard JWT claims
	Issuer   string    `json:"iss"` // Issuer: "ace-auth"
	Subject  uuid.UUID `json:"sub"` // Subject: user ID
	Audience []string  `json:"aud"` // Audience: ["ace-api"]
	Expiry   time.Time `json:"exp"` // Expiration time
	IssuedAt time.Time `json:"iat"` // Issued at
	JWTID    uuid.UUID `json:"jti"` // JWT ID (unique token identifier)

	// Custom claims
	Role  UserRole `json:"role"`  // User role
	Email string   `json:"email"` // User email
}

// GetAudience returns the audience as a string for comparison.
func (c *TokenClaims) GetAudience() string {
	if len(c.Audience) > 0 {
		return c.Audience[0]
	}
	return ""
}

// HasRole checks if the token has a specific role.
func (c *TokenClaims) HasRole(role UserRole) bool {
	return c.Role == role
}

// IsAdmin checks if the token has admin role.
func (c *TokenClaims) IsAdmin() bool {
	return c.Role == RoleAdmin
}

// RefreshTokenData represents the data stored for refresh tokens.
type RefreshTokenData struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
}

// TokenPair represents a pair of access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}
