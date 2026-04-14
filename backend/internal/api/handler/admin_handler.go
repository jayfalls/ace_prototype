// Package handler contains HTTP request handlers.
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
	"ace/internal/api/response"
)

// AdminHandler handles admin-only HTTP requests.
type AdminHandler struct {
	queries *db.Queries
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(queries *db.Queries) (*AdminHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	return &AdminHandler{
		queries: queries,
	}, nil
}

// UserListItem represents a user in a list response.
type UserListItem struct {
	ID        uuid.UUID        `json:"id"`
	Username  string           `json:"username"`
	Role      model.UserRole   `json:"role"`
	Status    model.UserStatus `json:"status"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
}

// UsersListResponse represents a paginated list of users.
type UsersListResponse struct {
	Users []UserListItem `json:"users"`
	Total int64          `json:"total"`
	Page  int32          `json:"page"`
	Limit int32          `json:"limit"`
}

// AdminUserResponse represents a detailed user response for admin.
type AdminUserResponse struct {
	ID              uuid.UUID        `json:"id"`
	Username        string           `json:"username"`
	Role            model.UserRole   `json:"role"`
	Status          model.UserStatus `json:"status"`
	SuspendedAt     *string          `json:"suspended_at,omitempty"`
	SuspendedReason *string          `json:"suspended_reason,omitempty"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

// UpdateRoleRequest represents the request body for updating a user's role.
type UpdateRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=user admin viewer"`
}

// SuspendUserRequest represents the request body for suspending a user.
type SuspendUserRequest struct {
	Reason string `json:"reason"`
}

// MessageResponse represents a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// @Summary List all users (paginated)
// @Tags admin
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20)"
// @Param status query string false "Filter by status"
// @Success 200 {object} UsersListResponse
// @Failure 403 {object} response.APIError
// @Router /admin/users [get]
// ListUsers handles GET /admin/users - Lists all users (paginated)
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role := r.Context().Value(UserRoleKey)
	if role == nil || role != string(model.RoleAdmin) {
		response.Forbidden(w, "Admin access required")
		return
	}

	// Parse pagination parameters
	page, limit := parsePaginationParamsAdmin(r)

	// Parse status filter
	var statusFilter *string
	if status := r.URL.Query().Get("status"); status != "" {
		statusFilter = &status
	}

	// Get users list
	dbUsers, err := h.queries.ListUsers(r.Context(), db.ListUsersParams{
		Column1: statusFilter,
		Limit:   int64(limit),
		Offset:  int64((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(w, "Failed to get users")
		return
	}

	// Get total count
	var total int64
	if statusFilter != nil {
		total, err = h.queries.ListUsersCount(r.Context(), db.ListUsersCountParams{
			Column1: statusFilter,
			Status:  *statusFilter,
		})
	} else {
		total, err = h.queries.CountUsers(r.Context())
	}
	if err != nil {
		response.InternalError(w, "Failed to count users")
		return
	}

	// Convert to response format
	users := make([]UserListItem, len(dbUsers))
	for i, u := range dbUsers {
		id, _ := uuid.Parse(u.ID)
		createdAt, _ := time.Parse(time.RFC3339, u.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, u.UpdatedAt)
		users[i] = UserListItem{
			ID:        id,
			Username:  u.Username,
			Role:      model.UserRole(u.Role),
			Status:    model.UserStatus(u.Status),
			CreatedAt: createdAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: updatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	resp := UsersListResponse{
		Users: users,
		Total: total,
		Page:  page,
		Limit: limit,
	}
	response.Success(w, resp)
}

// @Summary Get user details
// @Tags admin
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} AdminUserResponse
// @Failure 403 {object} response.APIError
// @Failure 404 {object} response.APIError
// @Router /admin/users/{id} [get]
// GetUser handles GET /admin/users/:id - Gets user details
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role := r.Context().Value(UserRoleKey)
	if role == nil || role != string(model.RoleAdmin) {
		response.Forbidden(w, "Admin access required")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid user ID format")
		return
	}

	dbUser, err := h.queries.GetUserByID(r.Context(), userID.String())
	if err != nil {
		response.NotFound(w, "User not found")
		return
	}

	id, _ := uuid.Parse(dbUser.ID)
	createdAt, _ := time.Parse(time.RFC3339, dbUser.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dbUser.UpdatedAt)

	// Build response
	resp := AdminUserResponse{
		ID:        id,
		Username:  dbUser.Username,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if dbUser.SuspendedAt.Valid {
		suspendedAt, _ := time.Parse(time.RFC3339, dbUser.SuspendedAt.String)
		suspendedAtStr := suspendedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.SuspendedAt = &suspendedAtStr
	}

	if dbUser.SuspendedReason.Valid {
		resp.SuspendedReason = &dbUser.SuspendedReason.String
	}

	response.Success(w, resp)
}

// @Summary Update user role
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body UpdateRoleRequest true "New role"
// @Success 200 {object} AdminUserResponse
// @Failure 403 {object} response.APIError
// @Failure 404 {object} response.APIError
// @Router /admin/users/{id}/role [put]
// UpdateUserRole handles PUT /admin/users/:id/role - Updates user role
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role := r.Context().Value(UserRoleKey)
	if role == nil || role != string(model.RoleAdmin) {
		response.Forbidden(w, "Admin access required")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid user ID format")
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	if req.Role == "" {
		response.BadRequest(w, "invalid_request", "Role is required")
		return
	}

	// Validate role
	if req.Role != string(model.RoleUser) && req.Role != string(model.RoleAdmin) && req.Role != string(model.RoleViewer) {
		response.BadRequest(w, "invalid_request", "Invalid role. Must be one of: user, admin, viewer")
		return
	}

	// Update user role
	dbUser, err := h.queries.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{
		ID:   userID.String(),
		Role: req.Role,
	})
	if err != nil {
		response.InternalError(w, "Failed to update user role")
		return
	}

	id, _ := uuid.Parse(dbUser.ID)
	createdAt, _ := time.Parse(time.RFC3339, dbUser.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dbUser.UpdatedAt)

	resp := AdminUserResponse{
		ID:        id,
		Username:  dbUser.Username,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	response.Success(w, resp)
}

// @Summary Suspend a user
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body SuspendUserRequest true "Suspension reason"
// @Success 200 {object} AdminUserResponse
// @Failure 403 {object} response.APIError
// @Failure 404 {object} response.APIError
// @Router /admin/users/{id}/suspend [post]
// SuspendUser handles POST /admin/users/:id/suspend - Suspends user, revokes sessions
func (h *AdminHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role := r.Context().Value(UserRoleKey)
	if role == nil || role != string(model.RoleAdmin) {
		response.Forbidden(w, "Admin access required")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid user ID format")
		return
	}

	var req SuspendUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	// Suspend user in database
	dbUser, err := h.queries.SuspendUser(r.Context(), db.SuspendUserParams{
		ID:              userID.String(),
		SuspendedAt:     sql.NullString{String: time.Now().Format(time.RFC3339), Valid: true},
		SuspendedReason: sql.NullString{String: req.Reason, Valid: req.Reason != ""},
	})
	if err != nil {
		response.InternalError(w, "Failed to suspend user")
		return
	}

	// Revoke all user sessions
	err = h.queries.DeleteAllSessionsByUserID(r.Context(), userID.String())
	if err != nil {
		// Log error but don't fail the request
		// In production, you would log this
		_ = err
	}

	id, _ := uuid.Parse(dbUser.ID)
	createdAt, _ := time.Parse(time.RFC3339, dbUser.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dbUser.UpdatedAt)

	resp := AdminUserResponse{
		ID:              id,
		Username:        dbUser.Username,
		Role:            model.UserRole(dbUser.Role),
		Status:          model.UserStatus(dbUser.Status),
		SuspendedReason: &dbUser.SuspendedReason.String,
		CreatedAt:       createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if dbUser.SuspendedAt.Valid {
		suspendedAt, _ := time.Parse(time.RFC3339, dbUser.SuspendedAt.String)
		suspendedAtStr := suspendedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.SuspendedAt = &suspendedAtStr
	}

	response.Success(w, resp)
}

// @Summary Restore a suspended user
// @Tags admin
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} AdminUserResponse
// @Failure 403 {object} response.APIError
// @Failure 404 {object} response.APIError
// @Router /admin/users/{id}/restore [post]
// RestoreUser handles POST /admin/users/:id/restore - Restores suspended user
func (h *AdminHandler) RestoreUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role := r.Context().Value(UserRoleKey)
	if role == nil || role != string(model.RoleAdmin) {
		response.Forbidden(w, "Admin access required")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(w, "invalid_request", "Invalid user ID format")
		return
	}

	// Restore user in database
	dbUser, err := h.queries.RestoreUser(r.Context(), db.RestoreUserParams{
		ID:        userID.String(),
		UpdatedAt: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		response.InternalError(w, "Failed to restore user")
		return
	}

	id, _ := uuid.Parse(dbUser.ID)
	createdAt, _ := time.Parse(time.RFC3339, dbUser.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dbUser.UpdatedAt)

	resp := AdminUserResponse{
		ID:        id,
		Username:  dbUser.Username,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	response.Success(w, resp)
}

// parsePaginationParamsAdmin extracts page and limit from URL query parameters for admin.
func parsePaginationParamsAdmin(r *http.Request) (page, limit int32) {
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
