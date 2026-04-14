package model

import (
	"github.com/google/uuid"
)

// TokenClaims represents the claims in a JWT token.
type TokenClaims struct {
	// Registered claims
	Iss string    `json:"iss"` // Issuer
	Sub uuid.UUID `json:"sub"` // Subject (user ID)
	Aud string    `json:"aud"` // Audience
	Exp int64     `json:"exp"` // Expiration time
	Iat int64     `json:"iat"` // Issued at
	Jti uuid.UUID `json:"jti"` // JWT ID

	// Custom claims
	Role string `json:"role"`
}

// RefreshTokenData represents the data stored in a refresh token.
type RefreshTokenData struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
	JTI       uuid.UUID `json:"jti"`
}

// TokenPair represents a pair of access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // Access token expiry in seconds
	TokenType    string `json:"token_type"` // "Bearer"
}
