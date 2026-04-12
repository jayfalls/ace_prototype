// Package handler contains HTTP request handlers.
package handler

import (
	"encoding/json"
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
	Email     string           `json:"email"`
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
	Email           string           `json:"email"`
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
	var statusFilter string
	if status := r.URL.Query().Get("status"); status != "" {
		statusFilter = status
	}

	// Get users list
	dbUsers, err := h.queries.ListUsers(r.Context(), db.ListUsersParams{
		Column1: statusFilter,
		Limit:   limit,
		Offset:  (page - 1) * limit,
	})
	if err != nil {
		response.InternalError(w, "Failed to get users")
		return
	}

	// Get total count
	var total int64
	if statusFilter != "" {
		total, err = h.queries.ListUsersCount(r.Context(), statusFilter)
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
		users[i] = UserListItem{
			ID:        u.ID.Bytes,
			Email:     u.Email,
			Role:      model.UserRole(u.Role),
			Status:    model.UserStatus(u.Status),
			CreatedAt: u.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: u.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
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

	dbUser, err := h.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		response.NotFound(w, "User not found")
		return
	}

	// Build response
	resp := AdminUserResponse{
		ID:        dbUser.ID.Bytes,
		Email:     dbUser.Email,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: dbUser.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: dbUser.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}

	if dbUser.SuspendedAt.Valid {
		suspendedAt := dbUser.SuspendedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		resp.SuspendedAt = &suspendedAt
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
		ID:   pgtype.UUID{Bytes: userID, Valid: true},
		Role: req.Role,
	})
	if err != nil {
		response.InternalError(w, "Failed to update user role")
		return
	}

	resp := AdminUserResponse{
		ID:        dbUser.ID.Bytes,
		Email:     dbUser.Email,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: dbUser.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: dbUser.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
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
		ID:              pgtype.UUID{Bytes: userID, Valid: true},
		SuspendedReason: pgtype.Text{String: req.Reason, Valid: req.Reason != ""},
	})
	if err != nil {
		response.InternalError(w, "Failed to suspend user")
		return
	}

	// Revoke all user sessions
	err = h.queries.DeleteAllSessionsByUserID(r.Context(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		// Log error but don't fail the request
		// In production, you would log this
		_ = err
	}

	resp := AdminUserResponse{
		ID:              dbUser.ID.Bytes,
		Email:           dbUser.Email,
		Role:            model.UserRole(dbUser.Role),
		Status:          model.UserStatus(dbUser.Status),
		SuspendedReason: &dbUser.SuspendedReason.String,
		CreatedAt:       dbUser.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       dbUser.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}

	if dbUser.SuspendedAt.Valid {
		suspendedAt := dbUser.SuspendedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		resp.SuspendedAt = &suspendedAt
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
	dbUser, err := h.queries.RestoreUser(r.Context(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		response.InternalError(w, "Failed to restore user")
		return
	}

	resp := AdminUserResponse{
		ID:        dbUser.ID.Bytes,
		Email:     dbUser.Email,
		Role:      model.UserRole(dbUser.Role),
		Status:    model.UserStatus(dbUser.Status),
		CreatedAt: dbUser.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: dbUser.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
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
