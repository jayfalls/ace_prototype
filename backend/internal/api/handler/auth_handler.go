// Package handler contains HTTP request handlers.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
	"ace/internal/api/response"
	"ace/internal/api/service"
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
	queries  *db.Queries
	authSvc  *service.AuthService
	tokenSvc *service.TokenService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	queries *db.Queries,
	authSvc *service.AuthService,
	tokenSvc *service.TokenService,
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

	return &AuthHandler{
		queries:  queries,
		authSvc:  authSvc,
		tokenSvc: tokenSvc,
	}, nil
}

// RegisterWithPINRequest represents the request body for PIN-based registration.
type RegisterWithPINRequest struct {
	Username string `json:"username" validate:"required"`
	PIN      string `json:"pin" validate:"required"`
}

// LoginWithPINRequest represents the request body for PIN-based login.
type LoginWithPINRequest struct {
	Username string `json:"username" validate:"required"`
	PIN      string `json:"pin" validate:"required"`
}

// LoginUsersResponse represents the response for listing users on login screen.
type LoginUsersResponse struct {
	Users []model.UserListItem `json:"users"`
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

// @Summary Register with username and PIN (OS-style)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterWithPINRequest true "User registration data"
// @Success 201 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Failure 409 {object} response.APIError
// @Router /auth/register [post]
// RegisterWithPIN handles POST /auth/register - Creates user with username/PIN, returns tokens
func (h *AuthHandler) RegisterWithPIN(w http.ResponseWriter, r *http.Request) {
	var req RegisterWithPINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Username == "" || req.PIN == "" {
		response.BadRequest(w, "invalid_request", "Username and PIN are required")
		return
	}

	user, tokens, err := h.authSvc.RegisterWithPIN(r.Context(), req.Username, req.PIN)
	if err != nil {
		if errors.Is(err, model.ErrUserAlreadyExists) {
			response.Error(w, "user_exists", "User with this username already exists", http.StatusConflict)
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

// @Summary Login with username and PIN (OS-style)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginWithPINRequest true "User credentials"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} response.APIError
// @Failure 401 {object} response.APIError
// @Router /auth/login [post]
// LoginWithPIN handles POST /auth/login - Validates username/PIN, returns tokens
func (h *AuthHandler) LoginWithPIN(w http.ResponseWriter, r *http.Request) {
	var req LoginWithPINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Username == "" || req.PIN == "" {
		response.BadRequest(w, "invalid_request", "Username and PIN are required")
		return
	}

	user, tokens, err := h.authSvc.LoginWithPIN(r.Context(), req.Username, req.PIN)
	if err != nil {
		if errors.Is(err, model.ErrInvalidCredentials) {
			response.Unauthorized(w, "Invalid username or PIN")
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

// @Summary List users for login screen
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} LoginUsersResponse
// @Router /users [get]
// ListUsers handles GET /users - Returns list of users for login screen
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authSvc.ListUsersForLogin(r.Context())
	if err != nil {
		response.InternalError(w, "Failed to list users")
		return
	}

	response.Success(w, LoginUsersResponse{Users: users})
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
