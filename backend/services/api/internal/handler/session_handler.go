// Package handler contains HTTP request handlers.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/api/internal/model"
	db "ace/api/internal/repository/generated"
	"ace/api/internal/response"
)

// SessionHandler handles session-related HTTP requests.
type SessionHandler struct {
	queries *db.Queries
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(queries *db.Queries) (*SessionHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	return &SessionHandler{
		queries: queries,
	}, nil
}

// UserResponse represents a user response for API endpoints.
type UserResponse struct {
	ID        uuid.UUID        `json:"id"`
	Email     string           `json:"email"`
	Role      model.UserRole   `json:"role"`
	Status    model.UserStatus `json:"status"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
}

// SessionResponse represents a session for API responses.
type SessionResponse struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	UserAgent  string    `json:"user_agent,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	LastUsedAt string    `json:"last_used_at"`
	ExpiresAt  string    `json:"expires_at"`
	CreatedAt  string    `json:"created_at"`
}

// SessionsListResponse represents a paginated list of sessions.
type SessionsListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int64             `json:"total"`
	Page     int32             `json:"page"`
	Limit    int32             `json:"limit"`
}

// @Summary Get current user profile
// @Tags session
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} response.APIError
// @Router /auth/me [get]
// Me handles GET /auth/me - Returns current user profile
func (h *SessionHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey)
	if userID == nil {
		response.Unauthorized(w, "User not authenticated")
		return
	}

	dbUser, err := h.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: userID.(uuid.UUID), Valid: true})
	if err != nil {
		response.NotFound(w, "User not found")
		return
	}

	resp := UserResponse{
		ID:        dbUser.ID.Bytes,
		Email:     dbUser.Email,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: dbUser.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: dbUser.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
	response.Success(w, resp)
}

// @Summary List user's active sessions
// @Tags session
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20)"
// @Success 200 {object} SessionsListResponse
// @Failure 401 {object} response.APIError
// @Router /auth/me/sessions [get]
// ListSessions handles GET /auth/me/sessions - Lists user's active sessions
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey)
	if userID == nil {
		response.Unauthorized(w, "User not authenticated")
		return
	}

	// Parse pagination parameters
	page, limit := parsePaginationParams(r)

	// Get sessions for user
	dbSessions, err := h.queries.GetSessionByUserID(r.Context(), pgtype.UUID{Bytes: userID.(uuid.UUID), Valid: true})
	if err != nil {
		response.InternalError(w, "Failed to get sessions")
		return
	}

	// Convert to response format
	sessions := make([]SessionResponse, len(dbSessions))
	for i, s := range dbSessions {
		sessions[i] = SessionResponse{
			ID:         s.ID.Bytes,
			UserID:     s.UserID.Bytes,
			UserAgent:  s.UserAgent.String,
			IPAddress:  s.IpAddress,
			LastUsedAt: s.LastUsedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:  s.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt:  s.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	resp := SessionsListResponse{
		Sessions: sessions,
		Total:    int64(len(sessions)),
		Page:     page,
		Limit:    limit,
	}
	response.Success(w, resp)
}

// @Summary Revoke a specific session
// @Tags session
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} response.APIError
// @Failure 404 {object} response.APIError
// @Router /auth/me/sessions/{id} [delete]
// RevokeSession handles DELETE /auth/me/sessions/:id - Revokes specific session
func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey)
	if userID == nil {
		response.Unauthorized(w, "User not authenticated")
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid session ID format")
		return
	}

	// Verify session belongs to user
	_, err = h.queries.GetSessionByIDAndUserID(r.Context(), db.GetSessionByIDAndUserIDParams{
		ID:     pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID.(uuid.UUID), Valid: true},
	})
	if err != nil {
		response.NotFound(w, "Session not found")
		return
	}

	// Delete session
	err = h.queries.DeleteSession(r.Context(), pgtype.UUID{Bytes: sessionID, Valid: true})
	if err != nil {
		response.InternalError(w, "Failed to revoke session")
		return
	}

	response.Success(w, map[string]string{"message": "Session revoked successfully"})
}

// parsePaginationParams extracts page and limit from URL query parameters.
func parsePaginationParams(r *http.Request) (page, limit int32) {
	page = 1
	limit = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = int32(parsed)
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	return page, limit
}
