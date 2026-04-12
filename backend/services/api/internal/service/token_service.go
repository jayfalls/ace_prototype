// Package service provides authentication and authorization services.
package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"ace/api/internal/model"
)

// Token service errors
var (
	ErrTokenInvalid  = errors.New("token is invalid")
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenRevoked  = errors.New("token has been revoked")
	ErrKeyNotLoaded  = errors.New("RSA key not loaded")
	ErrSigningFailed = errors.New("failed to sign token")
	ErrInvalidToken  = errors.New("invalid token format")
)

// TokenConfig holds configuration for the token service.
type TokenConfig struct {
	Issuer          string        // Issuer identifier (e.g., "ace-auth")
	Audience        string        // Expected audience (e.g., "ace-api")
	AccessTokenTTL  time.Duration // Access token lifetime (default 15 minutes)
	RefreshTokenTTL time.Duration // Refresh token lifetime (default 7 days)
	PrivateKey      *rsa.PrivateKey
	PublicKey       *rsa.PublicKey
}

// TokenService handles JWT token generation and validation using RS256.
type TokenService struct {
	config *TokenConfig
}

// NewTokenService creates a new token service with the given configuration.
// If no keys are provided, it will generate a new key pair on startup.
func NewTokenService(config *TokenConfig) (*TokenService, error) {
	if config == nil {
		return nil, errors.New("token config is required")
	}

	// Set defaults
	if config.Issuer == "" {
		config.Issuer = "ace-auth"
	}
	if config.Audience == "" {
		config.Audience = "ace-api"
	}
	if config.AccessTokenTTL == 0 {
		config.AccessTokenTTL = 15 * time.Minute
	}
	if config.RefreshTokenTTL == 0 {
		config.RefreshTokenTTL = 7 * 24 * time.Hour
	}

	// Generate keys if not provided
	if config.PrivateKey == nil || config.PublicKey == nil {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate RSA key pair: %w", err)
		}
		config.PrivateKey = privateKey
		config.PublicKey = &privateKey.PublicKey
	}

	return &TokenService{config: config}, nil
}

// GenerateAccessToken creates a new RS256 signed JWT access token.
func (s *TokenService) GenerateAccessToken(claims *model.TokenClaims) (string, error) {
	if s.config.PrivateKey == nil {
		return "", ErrKeyNotLoaded
	}

	if claims == nil {
		return "", errors.New("claims cannot be nil")
	}

	// Set registered claims
	now := time.Now()
	claims.Iss = s.config.Issuer
	claims.Aud = s.config.Audience
	claims.Iat = now.Unix()
	claims.Exp = now.Add(s.config.AccessTokenTTL).Unix()

	// Generate JTI if not set
	if claims.Jti == uuid.Nil {
		claims.Jti = uuid.New()
	}

	// Create JWT header
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature - use SHA256 hash
	signingInput := headerBase64 + "." + claimsBase64
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.config.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSigningFailed, err)
	}
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}

// GenerateRefreshToken creates a new JWT refresh token.
// Refresh tokens are also JWTs signed with RS256, containing only essential claims.
func (s *TokenService) GenerateRefreshToken(userID uuid.UUID, sessionID uuid.UUID) (string, error) {
	if s.config.PrivateKey == nil {
		return "", ErrKeyNotLoaded
	}

	if userID == uuid.Nil {
		return "", errors.New("user ID is required")
	}
	if sessionID == uuid.Nil {
		return "", errors.New("session ID is required")
	}

	// Create refresh token claims (minimal for size and security)
	refreshClaims := &model.TokenClaims{
		Iss: s.config.Issuer,
		Sub: userID,
		Aud: s.config.Audience,
		Jti: uuid.New(), // Unique token ID
	}

	now := time.Now()
	refreshClaims.Iat = now.Unix()
	refreshClaims.Exp = now.Add(s.config.RefreshTokenTTL).Unix()

	// Add custom claims for refresh token validation
	refreshClaims.Role = sessionID.String() // Store session ID in role field

	// Create JWT header
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, err := json.Marshal(refreshClaims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature - use SHA256 hash
	signingInput := headerBase64 + "." + claimsBase64
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.config.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSigningFailed, err)
	}
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}

// ValidateAccessToken validates an access token and returns the claims.
// It verifies signature, expiry, issuer, and audience.
func (s *TokenService) ValidateAccessToken(tokenString string) (*model.TokenClaims, error) {
	if s.config.PublicKey == nil {
		return nil, ErrKeyNotLoaded
	}

	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	// Parse JWT parts
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerBase64, payloadBase64, signatureBase64 := parts[0], parts[1], parts[2]

	// Verify signature
	signingInput := headerBase64 + "." + payloadBase64
	signature, err := base64.RawURLEncoding.DecodeString(signatureBase64)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	hash := sha256.Sum256([]byte(signingInput))
	err = rsa.VerifyPKCS1v15(s.config.PublicKey, crypto.SHA256, hash[:], signature)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims model.TokenClaims
	err = json.Unmarshal(claimsJSON, &claims)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	// Validate registered claims
	if claims.Iss != s.config.Issuer {
		return nil, ErrTokenInvalid
	}

	if claims.Aud != s.config.Audience {
		return nil, ErrTokenInvalid
	}

	// Check expiration
	now := time.Now().Unix()
	if claims.Exp < now {
		return nil, ErrTokenExpired
	}

	// Check issued at (allow some clock skew)
	if claims.Iat > now+60 {
		return nil, ErrTokenInvalid
	}

	return &claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the data.
// It verifies signature, expiry, issuer, and audience.
func (s *TokenService) ValidateRefreshToken(tokenString string) (*model.RefreshTokenData, error) {
	// First validate as access token to verify signature and claims
	claims, err := s.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Extract refresh token data from claims
	// For refresh tokens, the role field contains the session ID
	if claims.Sub == uuid.Nil {
		return nil, ErrTokenInvalid
	}

	// The JTI is the unique token identifier
	if claims.Jti == uuid.Nil {
		return nil, ErrTokenInvalid
	}

	// Parse session ID from role field (stored as session ID during generation)
	var sessionID uuid.UUID
	if claims.Role != "" {
		sessionID, err = uuid.Parse(claims.Role)
		if err != nil {
			return nil, ErrTokenInvalid
		}
	}

	return &model.RefreshTokenData{
		UserID:    claims.Sub,
		SessionID: sessionID,
		JTI:       claims.Jti,
	}, nil
}

// GenerateTokenPair creates both an access token and refresh token for a user.
func (s *TokenService) GenerateTokenPair(user *model.User, sessionID uuid.UUID) (*model.TokenPair, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}

	if sessionID == uuid.Nil {
		return nil, errors.New("session ID is required")
	}

	// Create access token claims
	accessClaims := &model.TokenClaims{
		Sub:   user.ID,
		Role:  string(user.Role),
		Email: user.Email,
	}

	// Generate access token
	accessToken, err := s.GenerateAccessToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.GenerateRefreshToken(user.ID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Return token pair
	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.config.AccessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// HashRefreshToken creates a SHA256 hash of a refresh token for storage.
// This should be used when storing refresh tokens in the database.
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// GetAccessTokenTTL returns the configured access token TTL.
func (s *TokenService) GetAccessTokenTTL() time.Duration {
	return s.config.AccessTokenTTL
}

// GetRefreshTokenTTL returns the configured refresh token TTL.
func (s *TokenService) GetRefreshTokenTTL() time.Duration {
	return s.config.RefreshTokenTTL
}

// GetPublicKey returns the public key for JWKS endpoints.
func (s *TokenService) GetPublicKey() *rsa.PublicKey {
	return s.config.PublicKey
}
