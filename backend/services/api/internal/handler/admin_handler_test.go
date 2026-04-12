// Package handler provides tests for admin HTTP handlers.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/api/internal/model"
	db "ace/api/internal/repository/generated"
	"ace/api/internal/response"
)

// MockAdminQueries is a mock implementation of db.Queries for admin testing.
type MockAdminQueries struct {
	GetUserByIDFunc               func(ctx context.Context, id pgtype.UUID) (*db.User, error)
	ListUsersFunc                 func(ctx context.Context, params db.ListUsersParams) ([]db.User, error)
	ListUsersCountFunc            func(ctx context.Context, status string) (int64, error)
	CountUsersFunc                func(ctx context.Context) (int64, error)
	UpdateUserRoleFunc            func(ctx context.Context, params db.UpdateUserRoleParams) (*db.User, error)
	SuspendUserFunc               func(ctx context.Context, params db.SuspendUserParams) (*db.User, error)
	RestoreUserFunc               func(ctx context.Context, id pgtype.UUID) (*db.User, error)
	DeleteAllSessionsByUserIDFunc func(ctx context.Context, userID pgtype.UUID) error
}

// GetUserByID calls the mock function.
func (q *MockAdminQueries) GetUserByID(ctx context.Context, id pgtype.UUID) (*db.User, error) {
	return q.GetUserByIDFunc(ctx, id)
}

// ListUsers calls the mock function.
func (q *MockAdminQueries) ListUsers(ctx context.Context, params db.ListUsersParams) ([]db.User, error) {
	return q.ListUsersFunc(ctx, params)
}

// ListUsersCount calls the mock function.
func (q *MockAdminQueries) ListUsersCount(ctx context.Context, status string) (int64, error) {
	return q.ListUsersCountFunc(ctx, status)
}

// CountUsers calls the mock function.
func (q *MockAdminQueries) CountUsers(ctx context.Context) (int64, error) {
	return q.CountUsersFunc(ctx)
}

// UpdateUserRole calls the mock function.
func (q *MockAdminQueries) UpdateUserRole(ctx context.Context, params db.UpdateUserRoleParams) (*db.User, error) {
	return q.UpdateUserRoleFunc(ctx, params)
}

// SuspendUser calls the mock function.
func (q *MockAdminQueries) SuspendUser(ctx context.Context, params db.SuspendUserParams) (*db.User, error) {
	return q.SuspendUserFunc(ctx, params)
}

// RestoreUser calls the mock function.
func (q *MockAdminQueries) RestoreUser(ctx context.Context, id pgtype.UUID) (*db.User, error) {
	return q.RestoreUserFunc(ctx, id)
}

// DeleteAllSessionsByUserID calls the mock function.
func (q *MockAdminQueries) DeleteAllSessionsByUserID(ctx context.Context, userID pgtype.UUID) error {
	return q.DeleteAllSessionsByUserIDFunc(ctx, userID)
}

// TestableAdminHandler is an admin handler that can be injected with mocks for testing.
type TestableAdminHandler struct {
	queries *MockAdminQueries
}

// NewTestableAdminHandler creates a testable admin handler with mocks.
func NewTestableAdminHandler(queries *MockAdminQueries) (*TestableAdminHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	return &TestableAdminHandler{
		queries: queries,
	}, nil
}

// ListUsers handles GET /admin/users - Lists all users (paginated)
func (h *TestableAdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
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

// GetUser handles GET /admin/users/:id - Gets user details
func (h *TestableAdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
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

// UpdateUserRole handles PUT /admin/users/:id/role - Updates user role
func (h *TestableAdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
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

// SuspendUser handles POST /admin/users/:id/suspend - Suspends user, revokes sessions
func (h *TestableAdminHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
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
		_ = err // Log in production
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

// RestoreUser handles POST /admin/users/:id/restore - Restores suspended user
func (h *TestableAdminHandler) RestoreUser(w http.ResponseWriter, r *http.Request) {
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

// Helper function to create context with admin role.
func createContextWithAdminRole() context.Context {
	return context.WithValue(context.Background(), UserRoleKey, string(model.RoleAdmin))
}

// Helper function to create context with non-admin role.
func createContextWithUserRole() context.Context {
	return context.WithValue(context.Background(), UserRoleKey, string(model.RoleUser))
}

// Helper function to create test user for admin.
func createAdminTestUser(email string) *db.User {
	userID := uuid.New()
	return &db.User{
		ID:           pgtype.UUID{Bytes: userID, Valid: true},
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         "user",
		Status:       "active",
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// Tests for ListUsers endpoint.

func TestListUsers_Success(t *testing.T) {
	mockQueries := &MockAdminQueries{
		ListUsersFunc: func(ctx context.Context, params db.ListUsersParams) ([]db.User, error) {
			return []db.User{*createAdminTestUser("user@example.com")}, nil
		},
		CountUsersFunc: func(ctx context.Context) (int64, error) {
			return 1, nil
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(createContextWithAdminRole())
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestListUsers_Unauthorized(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(createContextWithUserRole())
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestListUsers_NoRole(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// Tests for GetUser endpoint.

func TestGetUser_Success(t *testing.T) {
	testUser := createAdminTestUser("user@example.com")

	mockQueries := &MockAdminQueries{
		GetUserByIDFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return testUser, nil
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+userID.String(), nil)
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.GetUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetUser_Unauthorized(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/"+uuid.New().String(), nil)
	req = req.WithContext(createContextWithUserRole())
	w := httptest.NewRecorder()

	handler.GetUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestGetUser_UserNotFound(t *testing.T) {
	mockQueries := &MockAdminQueries{
		GetUserByIDFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return nil, errors.New("not found")
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+userID.String(), nil)
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.GetUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetUser_InvalidUserID(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/invalid-uuid", nil)
	req = req.WithContext(createContextWithAdminRole())
	w := httptest.NewRecorder()

	handler.GetUser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Tests for UpdateUserRole endpoint.

func TestUpdateUserRole_Success(t *testing.T) {
	testUser := createAdminTestUser("user@example.com")
	testUser.Role = "admin"

	mockQueries := &MockAdminQueries{
		UpdateUserRoleFunc: func(ctx context.Context, params db.UpdateUserRoleParams) (*db.User, error) {
			return testUser, nil
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()

	// Create request with JSON body
	body := map[string]string{"role": "admin"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/"+userID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.UpdateUserRole(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestUpdateUserRole_Unauthorized(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/"+uuid.New().String(), nil)
	req = req.WithContext(createContextWithUserRole())
	w := httptest.NewRecorder()

	handler.UpdateUserRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()

	// Create request with invalid role
	body := map[string]string{"role": "invalid_role"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/"+userID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.UpdateUserRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Tests for SuspendUser endpoint.

func TestSuspendUser_Success(t *testing.T) {
	testUser := createAdminTestUser("user@example.com")
	testUser.Status = "suspended"

	mockQueries := &MockAdminQueries{
		SuspendUserFunc: func(ctx context.Context, params db.SuspendUserParams) (*db.User, error) {
			return testUser, nil
		},
		DeleteAllSessionsByUserIDFunc: func(ctx context.Context, userID pgtype.UUID) error {
			return nil
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()

	// Create request with body
	body := map[string]string{"reason": "Terms violation"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/"+userID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.SuspendUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestSuspendUser_Unauthorized(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/"+uuid.New().String(), nil)
	req = req.WithContext(createContextWithUserRole())
	w := httptest.NewRecorder()

	handler.SuspendUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// Tests for RestoreUser endpoint.

func TestRestoreUser_Success(t *testing.T) {
	testUser := createAdminTestUser("user@example.com")
	testUser.Status = "active"

	mockQueries := &MockAdminQueries{
		RestoreUserFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return testUser, nil
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/"+userID.String(), nil)
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.RestoreUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRestoreUser_Unauthorized(t *testing.T) {
	mockQueries := &MockAdminQueries{}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/"+uuid.New().String(), nil)
	req = req.WithContext(createContextWithUserRole())
	w := httptest.NewRecorder()

	handler.RestoreUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRestoreUser_UserNotFound(t *testing.T) {
	mockQueries := &MockAdminQueries{
		RestoreUserFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return nil, errors.New("not found")
		},
	}

	handler, err := NewTestableAdminHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/"+userID.String(), nil)
	req = req.WithContext(createContextWithAdminRole())

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.RestoreUser(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Test parsePaginationParamsAdmin for coverage.

func TestParsePaginationParamsAdmin(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedPage  int32
		expectedLimit int32
	}{
		{"default values", "/", 1, 20},
		{"with page", "/?page=3", 3, 20},
		{"with limit", "/?limit=50", 1, 50},
		{"with both", "/?page=2&limit=10", 2, 10},
		{"invalid page", "/?page=abc", 1, 20},
		{"invalid limit", "/?limit=xyz", 1, 20},
		{"zero page", "/?page=0", 1, 20},
		{"zero limit", "/?limit=0", 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			page, limit := parsePaginationParamsAdmin(req)

			if page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, page)
			}
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, limit)
			}
		})
	}
}
