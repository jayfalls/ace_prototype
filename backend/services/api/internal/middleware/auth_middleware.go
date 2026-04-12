// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ace/api/internal/model"
	"ace/api/internal/service"
)

// Context keys for storing user information in request context.
const (
	UserIDKey      contextKey = "user_id"
	UserRoleKey    contextKey = "user_role"
	TokenClaimsKey contextKey = "token_claims"
	UserEmailKey   contextKey = "user_email"
)

// contextKey is a type for context keys.
type contextKey string

// ErrUnauthorized is returned when authentication fails.
var ErrUnauthorized = errors.New("unauthorized")

// TokenBlacklistChecker defines an interface for checking token blacklist.
// Implement this to add Valkey-based token revocation checks.
type TokenBlacklistChecker interface {
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}

// AuthMiddleware handles JWT token validation.
type AuthMiddleware struct {
	tokenService   *service.TokenService
	tokenBlacklist TokenBlacklistChecker
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(tokenService *service.TokenService) *AuthMiddleware {
	return &AuthMiddleware{
		tokenService: tokenService,
	}
}

// SetTokenBlacklist sets the token blacklist checker (optional).
func (m *AuthMiddleware) SetTokenBlacklist(checker TokenBlacklistChecker) {
	m.tokenBlacklist = checker
}

// RequireAuth returns a middleware that validates JWT tokens from the Authorization header.
// It expects the header to be in the format: "Bearer <token>"
// On success, it attaches user information to the request context:
//   - user ID (uuid.UUID)
//   - user role (model.UserRole)
//   - token claims (*model.TokenClaims)
//   - user email (string)
//
// Returns 401 if:
//   - Authorization header is missing or malformed
//   - Token is invalid or expired
//   - Token has been revoked
func (m *AuthMiddleware) RequireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Expect "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			if tokenString == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := m.tokenService.ValidateAccessToken(tokenString)
			if err != nil {
				if errors.Is(err, service.ErrTokenExpired) {
					http.Error(w, "token has expired", http.StatusUnauthorized)
					return
				}
				if errors.Is(err, service.ErrTokenInvalid) {
					http.Error(w, "invalid token", http.StatusUnauthorized)
					return
				}
				http.Error(w, "token validation failed", http.StatusUnauthorized)
				return
			}

			// Check token blacklist if configured (optional)
			if m.tokenBlacklist != nil {
				isRevoked, err := m.tokenBlacklist.IsTokenRevoked(r.Context(), claims.Jti.String())
				if err != nil {
					http.Error(w, "failed to check token status", http.StatusInternalServerError)
					return
				}
				if isRevoked {
					http.Error(w, "token has been revoked", http.StatusUnauthorized)
					return
				}
			}

			// Attach user information to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.Sub)
			ctx = context.WithValue(ctx, UserRoleKey, model.UserRole(claims.Role))
			ctx = context.WithValue(ctx, TokenClaimsKey, claims)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext retrieves the user ID from the request context.
// Returns uuid.Nil if not found.
func GetUserIDFromContext(ctx context.Context) interface{} {
	return ctx.Value(UserIDKey)
}

// GetUserRoleFromContext retrieves the user role from the request context.
// Returns empty string if not found.
func GetUserRoleFromContext(ctx context.Context) model.UserRole {
	if role, ok := ctx.Value(UserRoleKey).(model.UserRole); ok {
		return role
	}
	return ""
}

// GetTokenClaimsFromContext retrieves the token claims from the request context.
// Returns nil if not found.
func GetTokenClaimsFromContext(ctx context.Context) *model.TokenClaims {
	if claims, ok := ctx.Value(TokenClaimsKey).(*model.TokenClaims); ok {
		return claims
	}
	return nil
}

// GetUserEmailFromContext retrieves the user email from the request context.
// Returns empty string if not found.
func GetUserEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}
