// Package handler contains HTTP request handlers.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/api/internal/model"
	db "ace/api/internal/repository/generated"
	"ace/api/internal/response"
	"ace/api/internal/service"
)

// ContextKey type for context values.
type contextKey string

const (
	// UserIDKey is the context key for user ID.
	UserIDKey contextKey = "user_id"
	// SessionIDKey is the context key for session ID.
	SessionIDKey contextKey = "session_id"
	// UserRoleKey is the context key for user role.
	UserRoleKey contextKey = "user_role"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	queries      *db.Queries
	authSvc      *service.AuthService
	tokenSvc     *service.TokenService
	magicLinkSvc *service.MagicLinkService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	queries *db.Queries,
	authSvc *service.AuthService,
	tokenSvc *service.TokenService,
	magicLinkSvc *service.MagicLinkService,
) (*AuthHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}
	if authSvc == nil {
		return nil, errors.New("auth service is required")
	}
	if tokenSvc == nil {
		return nil, errors.New("token service is required")
	}
	if magicLinkSvc == nil {
		return nil, errors.New("magic link service is required")
	}

	return &AuthHandler{
		queries:      queries,
		authSvc:      authSvc,
		tokenSvc:     tokenSvc,
		magicLinkSvc: magicLinkSvc,
	}, nil
}

// RegisterRequest represents the request body for user registration.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// TokenResponse represents the response for token-based operations.
type TokenResponse struct {
	User         *model.User `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	TokenType    string      `json:"token_type"`
}

// RefreshRequest represents the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest represents the request body for logout.
type LogoutRequest struct {
	SessionID string `json:"session_id" validate:"required,uuid"`
}

// PasswordResetRequest represents the request body for requesting password reset.
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmRequest represents the request body for confirming password reset.
type PasswordResetConfirmRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// MagicLinkRequest represents the request body for requesting magic link.
type MagicLinkRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// MagicLinkVerifyRequest represents the request body for verifying magic link.
type MagicLinkVerifyRequest struct {
	Token string `json:"token" validate:"required"`
}

// MagicLinkResponse represents the response for magic link operations.
type MagicLinkResponse struct {
	Message string `json:"message"`
}

// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "User registration data"
// @Success 201 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Failure 409 {object} response.APIError
// @Router /auth/register [post]
// Register handles POST /auth/register - Creates user, returns tokens
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.BadRequest(w, "invalid_request", "Email and password are required")
		return
	}

	user, tokens, err := h.authSvc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, model.ErrUserAlreadyExists) {
			response.Error(w, "user_exists", "User with this email already exists", http.StatusConflict)
			return
		}
		if errors.Is(err, model.ErrWeakPassword) {
			response.BadRequest(w, "weak_password", err.Error())
			return
		}
		response.InternalError(w, "Failed to register user")
		return
	}

	resp := TokenResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}
	response.Created(w, resp)
}

// @Summary Login with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "User credentials"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Failure 401 {object} response.APIError
// @Router /auth/login [post]
// Login handles POST /auth/login - Validates credentials, returns tokens
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.BadRequest(w, "invalid_request", "Email and password are required")
		return
	}

	user, tokens, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, model.ErrInvalidCredentials) {
			response.Unauthorized(w, "Invalid email or password")
			return
		}
		if errors.Is(err, model.ErrAccountSuspended) {
			response.Forbidden(w, "Account has been suspended")
			return
		}
		response.InternalError(w, "Failed to login")
		return
	}

	resp := TokenResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}
	response.Success(w, resp)
}

// @Summary Logout current session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.APIError
// @Router /auth/logout [post]
// Logout handles POST /auth/logout - Invalidates session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid session ID format")
		return
	}

	err = h.authSvc.Logout(r.Context(), sessionID)
	if err != nil {
		response.InternalError(w, "Failed to logout")
		return
	}

	response.Success(w, map[string]string{"message": "Logged out successfully"})
}

// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Failure 401 {object} response.APIError
// @Router /auth/refresh [post]
// Refresh handles POST /auth/refresh - Rotates refresh token, returns new tokens
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.RefreshToken == "" {
		response.BadRequest(w, "invalid_request", "Refresh token is required")
		return
	}

	tokens, err := h.authSvc.RefreshSession(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, model.ErrTokenExpired) {
			response.Unauthorized(w, "Refresh token has expired")
			return
		}
		if errors.Is(err, model.ErrRefreshTokenInvalid) {
			response.Unauthorized(w, "Invalid refresh token")
			return
		}
		if errors.Is(err, model.ErrAccountSuspended) {
			response.Forbidden(w, "Account has been suspended")
			return
		}
		response.InternalError(w, "Failed to refresh session")
		return
	}

	// Get current user from context
	userID := r.Context().Value(UserIDKey)
	if userID == nil {
		response.InternalError(w, "User context not found")
		return
	}

	user, err := h.authSvc.GetCurrentUser(r.Context(), userID.(uuid.UUID))
	if err != nil {
		response.InternalError(w, "Failed to get user")
		return
	}

	resp := TokenResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}
	response.Success(w, resp)
}

// @Summary Request password reset
// @Tags auth
// @Accept json
// @Produce json
// @Param request body PasswordResetRequest true "Email address"
// @Success 200 {object} MagicLinkResponse
// @Router /auth/password/reset/request [post]
// ResetPasswordRequest handles POST /auth/password/reset/request - Sends reset token
func (h *AuthHandler) ResetPasswordRequest(w http.ResponseWriter, r *http.Request) {
	var req PasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		response.BadRequest(w, "invalid_request", "Email is required")
		return
	}

	// Generate magic link token for password reset
	_, err := h.magicLinkSvc.GenerateMagicLink(r.Context(), req.Email, service.TokenTypePasswordReset)
	if err != nil {
		// Don't expose error details - just return success to prevent email enumeration
		// In production, you would log this
	}

	// Always return success to prevent email enumeration
	response.Success(w, MagicLinkResponse{
		Message: "If the email exists, a password reset link has been sent",
	})
}

// @Summary Confirm password reset
// @Tags auth
// @Accept json
// @Produce json
// @Param request body PasswordResetConfirmRequest true "Reset token and new password"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Router /auth/password/reset/confirm [post]
// ResetPasswordConfirm handles POST /auth/password/reset/confirm - Resets password with token
func (h *AuthHandler) ResetPasswordConfirm(w http.ResponseWriter, r *http.Request) {
	var req PasswordResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		response.BadRequest(w, "invalid_request", "Token and new password are required")
		return
	}

	user, err := h.magicLinkSvc.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			response.BadRequest(w, "invalid_token", "Invalid or expired reset token")
			return
		}
		if errors.Is(err, service.ErrTokenExpired) {
			response.BadRequest(w, "token_expired", "Reset token has expired")
			return
		}
		if errors.Is(err, service.ErrTokenAlreadyUsed) {
			response.BadRequest(w, "token_used", "Reset token has already been used")
			return
		}
		if errors.Is(err, model.ErrWeakPassword) {
			response.BadRequest(w, "weak_password", err.Error())
			return
		}
		response.InternalError(w, "Failed to reset password")
		return
	}

	// Generate new tokens after password reset
	tokens, err := h.authSvc.RefreshSession(r.Context(), req.Token)
	if err != nil {
		// If refresh fails, just return user success - password was reset
		response.Success(w, MagicLinkResponse{
			Message: "Password has been reset successfully",
		})
		return
	}

	resp := TokenResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}
	response.Success(w, resp)
}

// @Summary Request magic link
// @Tags auth
// @Accept json
// @Produce json
// @Param request body MagicLinkRequest true "Email address"
// @Success 200 {object} MagicLinkResponse
// @Router /auth/magic-link/request [post]
// MagicLinkRequest handles POST /auth/magic-link/request - Requests magic link
func (h *AuthHandler) MagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		response.BadRequest(w, "invalid_request", "Email is required")
		return
	}

	// Generate magic link token
	_, err := h.magicLinkSvc.GenerateMagicLink(r.Context(), req.Email, service.TokenTypeLogin)
	if err != nil {
		// Don't expose error details
	}

	// Always return success to prevent email enumeration
	response.Success(w, MagicLinkResponse{
		Message: "If the email exists, a magic link has been sent",
	})
}

// @Summary Verify magic link
// @Tags auth
// @Accept json
// @Produce json
// @Param request body MagicLinkVerifyRequest true "Magic link token"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Router /auth/magic-link/verify [post]
// MagicLinkVerify handles POST /auth/magic-link/verify - Verifies magic link, returns tokens
func (h *AuthHandler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Token == "" {
		response.BadRequest(w, "invalid_request", "Token is required")
		return
	}

	// Validate magic link token
	userID, err := h.magicLinkSvc.ValidateMagicLink(r.Context(), req.Token, service.TokenTypeLogin)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			response.BadRequest(w, "invalid_token", "Invalid or expired magic link")
			return
		}
		if errors.Is(err, service.ErrTokenExpired) {
			response.BadRequest(w, "token_expired", "Magic link has expired")
			return
		}
		if errors.Is(err, service.ErrTokenAlreadyUsed) {
			response.BadRequest(w, "token_used", "Magic link has already been used")
			return
		}
		response.InternalError(w, "Failed to verify magic link")
		return
	}

	// Get user by ID
	dbUser, err := h.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		response.InternalError(w, "Failed to get user")
		return
	}

	// Check if user is suspended
	if dbUser.Status == string(model.StatusSuspended) {
		response.Forbidden(w, "Account has been suspended")
		return
	}

	// Generate tokens for user
	user := &model.User{
		ID:        dbUser.ID.Bytes,
		Email:     dbUser.Email,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: dbUser.CreatedAt.Time,
		UpdatedAt: dbUser.UpdatedAt.Time,
	}

	tokens, err := h.authSvc.RefreshSession(r.Context(), req.Token)
	if err != nil {
		// Need to generate new tokens - use the magic link token flow
		tokens, err = h.tokenSvc.GenerateTokenPair(user, uuid.Nil)
		if err != nil {
			response.InternalError(w, "Failed to generate tokens")
			return
		}
	}

	resp := TokenResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}
	response.Success(w, resp)
}
