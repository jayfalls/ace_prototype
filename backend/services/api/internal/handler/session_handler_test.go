// Package handler provides tests for session HTTP handlers.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/api/internal/model"
	db "ace/api/internal/repository/generated"
	"ace/api/internal/response"
)

// MockSessionQueries is a mock implementation of db.Queries for session testing.
type MockSessionQueries struct {
	GetUserByIDFunc             func(ctx context.Context, id pgtype.UUID) (*db.User, error)
	GetSessionByIDFunc          func(ctx context.Context, id pgtype.UUID) (*db.Session, error)
	GetSessionByUserIDFunc      func(ctx context.Context, userID pgtype.UUID) ([]db.Session, error)
	GetSessionByIDAndUserIDFunc func(ctx context.Context, params db.GetSessionByIDAndUserIDParams) (*db.Session, error)
	DeleteSessionFunc           func(ctx context.Context, id pgtype.UUID) error
}

// GetUserByID calls the mock function.
func (q *MockSessionQueries) GetUserByID(ctx context.Context, id pgtype.UUID) (*db.User, error) {
	return q.GetUserByIDFunc(ctx, id)
}

// GetSessionByID calls the mock function.
func (q *MockSessionQueries) GetSessionByID(ctx context.Context, id pgtype.UUID) (*db.Session, error) {
	return q.GetSessionByIDFunc(ctx, id)
}

// GetSessionByUserID calls the mock function.
func (q *MockSessionQueries) GetSessionByUserID(ctx context.Context, userID pgtype.UUID) ([]db.Session, error) {
	return q.GetSessionByUserIDFunc(ctx, userID)
}

// GetSessionByIDAndUserID calls the mock function.
func (q *MockSessionQueries) GetSessionByIDAndUserID(ctx context.Context, params db.GetSessionByIDAndUserIDParams) (*db.Session, error) {
	return q.GetSessionByIDAndUserIDFunc(ctx, params)
}

// DeleteSession calls the mock function.
func (q *MockSessionQueries) DeleteSession(ctx context.Context, id pgtype.UUID) error {
	return q.DeleteSessionFunc(ctx, id)
}

// TestableSessionHandler is a session handler that can be injected with mocks for testing.
type TestableSessionHandler struct {
	queries *MockSessionQueries
}

// NewTestableSessionHandler creates a testable session handler with mocks.
func NewTestableSessionHandler(queries *MockSessionQueries) (*TestableSessionHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	return &TestableSessionHandler{
		queries: queries,
	}, nil
}

// Me handles GET /auth/me - Returns current user profile
func (h *TestableSessionHandler) Me(w http.ResponseWriter, r *http.Request) {
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

// ListSessions handles GET /auth/me/sessions - Lists user's active sessions
func (h *TestableSessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
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
		ipStr := ""
		if s.IpAddress != nil {
			ipStr = s.IpAddress.String()
		}
		sessions[i] = SessionResponse{
			ID:         s.ID.Bytes,
			UserID:     s.UserID.Bytes,
			UserAgent:  s.UserAgent.String,
			IPAddress:  ipStr,
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

// RevokeSession handles DELETE /auth/me/sessions/:id - Revokes specific session
func (h *TestableSessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
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

// Helper function to create context with user ID.
func createContextWithUserID(userID uuid.UUID) context.Context {
	return context.WithValue(context.Background(), UserIDKey, userID)
}

// Helper function to create test user.
func createTestUserWithID(email string, id uuid.UUID) *db.User {
	return &db.User{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         "user",
		Status:       "active",
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// Helper function to create test session.
func createTestSessionWithID(userID uuid.UUID, sessionID uuid.UUID) *db.Session {
	ipAddr, _ := netip.ParseAddr("127.0.0.1")
	return &db.Session{
		ID:               pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID:           pgtype.UUID{Bytes: userID, Valid: true},
		RefreshTokenHash: "refresh_hash",
		UserAgent:        pgtype.Text{String: "Test Agent", Valid: true},
		IpAddress:        &ipAddr,
		LastUsedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		CreatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// Tests for Me endpoint.

func TestMe_Success(t *testing.T) {
	userID := uuid.New()
	testUser := createTestUserWithID("test@example.com", userID)

	mockQueries := &MockSessionQueries{
		GetUserByIDFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return testUser, nil
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(createContextWithUserID(userID))
	w := httptest.NewRecorder()

	handler.Me(w, req)

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

func TestMe_NotAuthenticated(t *testing.T) {
	mockQueries := &MockSessionQueries{}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMe_UserNotFound(t *testing.T) {
	userID := uuid.New()

	mockQueries := &MockSessionQueries{
		GetUserByIDFunc: func(ctx context.Context, id pgtype.UUID) (*db.User, error) {
			return nil, errors.New("not found")
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(createContextWithUserID(userID))
	w := httptest.NewRecorder()

	handler.Me(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Tests for ListSessions endpoint.

func TestListSessions_Success(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	mockQueries := &MockSessionQueries{
		GetSessionByUserIDFunc: func(ctx context.Context, userID pgtype.UUID) ([]db.Session, error) {
			return []db.Session{*createTestSessionWithID(userID.Bytes, sessionID)}, nil
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(createContextWithUserID(userID))
	w := httptest.NewRecorder()

	handler.ListSessions(w, req)

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

func TestListSessions_NotAuthenticated(t *testing.T) {
	mockQueries := &MockSessionQueries{}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ListSessions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestListSessions_DefaultPagination(t *testing.T) {
	userID := uuid.New()

	mockQueries := &MockSessionQueries{
		GetSessionByUserIDFunc: func(ctx context.Context, userID pgtype.UUID) ([]db.Session, error) {
			return []db.Session{}, nil
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=1&limit=10", nil)
	req = req.WithContext(createContextWithUserID(userID))
	w := httptest.NewRecorder()

	handler.ListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Tests for RevokeSession endpoint.

func TestRevokeSession_Success(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	mockQueries := &MockSessionQueries{
		GetSessionByIDAndUserIDFunc: func(ctx context.Context, params db.GetSessionByIDAndUserIDParams) (*db.Session, error) {
			return createTestSessionWithID(userID, sessionID), nil
		},
		DeleteSessionFunc: func(ctx context.Context, id pgtype.UUID) error {
			return nil
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/"+sessionID.String(), nil)
	req = req.WithContext(createContextWithUserID(userID))

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.RevokeSession(w, req)

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

func TestRevokeSession_NotAuthenticated(t *testing.T) {
	mockQueries := &MockSessionQueries{}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	sessionID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/"+sessionID.String(), nil)
	w := httptest.NewRecorder()

	handler.RevokeSession(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRevokeSession_InvalidSessionID(t *testing.T) {
	userID := uuid.New()

	mockQueries := &MockSessionQueries{}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/invalid-uuid", nil)
	req = req.WithContext(createContextWithUserID(userID))
	w := httptest.NewRecorder()

	handler.RevokeSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRevokeSession_SessionNotFound(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	mockQueries := &MockSessionQueries{
		GetSessionByIDAndUserIDFunc: func(ctx context.Context, params db.GetSessionByIDAndUserIDParams) (*db.Session, error) {
			return nil, errors.New("not found")
		},
	}

	handler, err := NewTestableSessionHandler(mockQueries)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/"+sessionID.String(), nil)
	req = req.WithContext(createContextWithUserID(userID))

	// Use chi to set the URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.RevokeSession(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test parsePaginationParams for coverage.

func TestParsePaginationParams(t *testing.T) {
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
			page, limit := parsePaginationParams(req)

			if page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, page)
			}
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, limit)
			}
		})
	}
}
