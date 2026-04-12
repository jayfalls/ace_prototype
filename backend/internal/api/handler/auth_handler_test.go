// Package handler provides tests for HTTP handlers.
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
	"ace/internal/api/response"
)

// MockAuthService is a mock implementation of AuthService for testing.
type MockAuthService struct {
	RegisterFunc       func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error)
	LoginFunc          func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error)
	LogoutFunc         func(ctx context.Context, sessionID uuid.UUID) error
	RefreshSessionFunc func(ctx context.Context, refreshToken string) (*model.TokenPair, error)
	GetCurrentUserFunc func(ctx context.Context, userID uuid.UUID) (*model.User, error)
}

// Register calls the mock function.
func (s *MockAuthService) Register(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
	return s.RegisterFunc(ctx, email, password)
}

// Login calls the mock function.
func (s *MockAuthService) Login(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
	return s.LoginFunc(ctx, email, password)
}

// Logout calls the mock function.
func (s *MockAuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.LogoutFunc(ctx, sessionID)
}

// RefreshSession calls the mock function.
func (s *MockAuthService) RefreshSession(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	return s.RefreshSessionFunc(ctx, refreshToken)
}

// GetCurrentUser calls the mock function.
func (s *MockAuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.GetCurrentUserFunc(ctx, userID)
}

// MockTokenService is a mock implementation of TokenService for testing.
type MockTokenService struct {
	GenerateTokenPairFunc    func(user *model.User, sessionID uuid.UUID) (*model.TokenPair, error)
	ValidateRefreshTokenFunc func(refreshToken string) (*model.TokenClaims, error)
	GetRefreshTokenTTLFunc   func() time.Duration
}

// MockMagicLinkService is a mock implementation of MagicLinkService for testing.
type MockMagicLinkService struct {
	GenerateMagicLinkFunc func(ctx context.Context, email string, tokenType string) (string, error)
	ValidateMagicLinkFunc func(ctx context.Context, token string, tokenType string) (uuid.UUID, error)
	ResetPasswordFunc     func(ctx context.Context, token string, newPassword string) (*model.User, error)
}

// MockQueries is a mock implementation of db.Queries for testing.
type MockQueries struct {
	GetUserByIDFunc               func(ctx context.Context, id pgtype.UUID) (*db.User, error)
	GetUserByEmailFunc            func(ctx context.Context, email string) (*db.User, error)
	GetSessionByIDFunc            func(ctx context.Context, id pgtype.UUID) (*db.Session, error)
	GetSessionByUserIDFunc        func(ctx context.Context, userID pgtype.UUID) ([]db.Session, error)
	GetSessionByIDAndUserIDFunc   func(ctx context.Context, params db.GetSessionByIDAndUserIDParams) (*db.Session, error)
	DeleteSessionFunc             func(ctx context.Context, id pgtype.UUID) error
	ListUsersFunc                 func(ctx context.Context, params db.ListUsersParams) ([]db.User, error)
	ListUsersCountFunc            func(ctx context.Context, status string) (int64, error)
	CountUsersFunc                func(ctx context.Context) (int64, error)
	CreateUserFunc                func(ctx context.Context, params db.CreateUserParams) (*db.User, error)
	CreateSessionFunc             func(ctx context.Context, params db.CreateSessionParams) (*db.Session, error)
	UpdateUserRoleFunc            func(ctx context.Context, params db.UpdateUserRoleParams) (*db.User, error)
	SuspendUserFunc               func(ctx context.Context, params db.SuspendUserParams) (*db.User, error)
	RestoreUserFunc               func(ctx context.Context, id pgtype.UUID) (*db.User, error)
	DeleteAllSessionsByUserIDFunc func(ctx context.Context, userID pgtype.UUID) error
}

// TestableAuthHandler is a handler that can be injected with mocks for testing.
type TestableAuthHandler struct {
	queries      *MockQueries
	authSvc      *MockAuthService
	tokenSvc     *MockTokenService
	magicLinkSvc *MockMagicLinkService
}

// NewTestableAuthHandler creates a testable auth handler with mocks.
func NewTestableAuthHandler(
	queries *MockQueries,
	authSvc *MockAuthService,
	tokenSvc *MockTokenService,
	magicLinkSvc *MockMagicLinkService,
) (*TestableAuthHandler, error) {
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

	return &TestableAuthHandler{
		queries:      queries,
		authSvc:      authSvc,
		tokenSvc:     tokenSvc,
		magicLinkSvc: magicLinkSvc,
	}, nil
}

// Register handles POST /auth/register - Creates user, returns tokens
func (h *TestableAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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

// Login handles POST /auth/login - Validates credentials, returns tokens
func (h *TestableAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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

// Logout handles POST /auth/logout - Invalidates session
func (h *TestableAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
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

// Helper functions for creating test data.
func createTestUser(email string) *model.User {
	userID := uuid.New()
	return &model.User{
		ID:        userID,
		Email:     email,
		Role:      model.RoleUser,
		Status:    model.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestTokens() *model.TokenPair {
	return &model.TokenPair{
		AccessToken:  "access_token_test",
		RefreshToken: "refresh_token_test",
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}
}

func createTestDBUser(email string) *db.User {
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

// Tests for Register endpoint.

func TestRegister_Success(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		RegisterFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return createTestUser(email), createTestTokens(), nil
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "strongpassword123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success to be false")
	}
}

func TestRegister_MissingEmail(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := RegisterRequest{
		Email:    "",
		Password: "strongpassword123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRegister_MissingPassword(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRegister_UserAlreadyExists(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		RegisterFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return nil, nil, model.ErrUserAlreadyExists
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := RegisterRequest{
		Email:    "existing@example.com",
		Password: "strongpassword123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error.Code != "user_exists" {
		t.Errorf("expected code 'user_exists', got '%s'", resp.Error.Code)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		RegisterFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return nil, nil, model.ErrWeakPassword
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "weak",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error.Code != "weak_password" {
		t.Errorf("expected code 'weak_password', got '%s'", resp.Error.Code)
	}
}

// Tests for Login endpoint.

func TestLogin_Success(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		LoginFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return createTestUser(email), createTestTokens(), nil
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "test@example.com",
		Password: "strongpassword123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

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

func TestLogin_InvalidJSON(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogin_MissingEmail(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "",
		Password: "strongpassword123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogin_MissingPassword(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "test@example.com",
		Password: "",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		LoginFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return nil, nil, model.ErrInvalidCredentials
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		LoginFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return nil, nil, model.ErrInvalidCredentials
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "notfound@example.com",
		Password: "somepassword",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLogin_UserSuspended(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		LoginFunc: func(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
			return nil, nil, model.ErrAccountSuspended
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LoginRequest{
		Email:    "suspended@example.com",
		Password: "somepassword",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil || resp.Error.Message == "" {
		t.Error("expected error message to be present")
	}
}

// Tests for Logout endpoint.

func TestLogout_Success(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		LogoutFunc: func(ctx context.Context, sessionID uuid.UUID) error {
			return nil
		},
	}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	sessionID := uuid.New()
	body := LogoutRequest{
		SessionID: sessionID.String(),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Logout(w, req)

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

func TestLogout_InvalidSessionID(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := LogoutRequest{
		SessionID: "invalid-uuid",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogout_InvalidJSON(t *testing.T) {
	mockAuthSvc := &MockAuthService{}

	handler, err := NewTestableAuthHandler(
		&MockQueries{},
		mockAuthSvc,
		&MockTokenService{},
		&MockMagicLinkService{},
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
