// Package service provides authentication and authorization services.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
)

// AuthService handles user authentication operations.
// It orchestrates PIN verification, token generation, and session management.
type AuthService struct {
	queries  *db.Queries
	tokenSvc *TokenService
}

// NewAuthService creates a new auth service with the given dependencies.
func NewAuthService(
	queries *db.Queries,
	tokenSvc *TokenService,
) (*AuthService, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}
	if tokenSvc == nil {
		return nil, errors.New("token service is required")
	}

	return &AuthService{
		queries:  queries,
		tokenSvc: tokenSvc,
	}, nil
}

// RegisterWithPIN registers a new user with username and PIN (OS-style login).
// First user automatically becomes admin.
func (s *AuthService) RegisterWithPIN(ctx context.Context, username, pin string) (*model.User, *model.TokenPair, error) {
	if ctx == nil {
		return nil, nil, errors.New("context is required")
	}
	if username == "" {
		return nil, nil, errors.New("username is required")
	}
	if pin == "" {
		return nil, nil, errors.New("PIN is required")
	}

	// Validate PIN
	if err := ValidatePIN(pin); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", model.ErrWeakPassword, err)
	}

	// Hash PIN
	pinHash, err := HashPIN(pin)
	if err != nil {
		return nil, nil, fmt.Errorf("hash PIN: %w", err)
	}

	// Check if username already exists
	existingUser, err := s.queries.GetUserByUsername(ctx, username)
	log.Printf("[DEBUG] GetUserByUsername returned: user=%v, err=%v, err==nil:%v, errors.Is(err,sql.ErrNoRows):%v",
		existingUser != nil, err, err == nil, errors.Is(err, sql.ErrNoRows))
	if err == nil {
		log.Printf("[DEBUG] User exists, returning error")
		return nil, nil, model.ErrUserAlreadyExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[DEBUG] Database error: %v", err)
		return nil, nil, fmt.Errorf("check existing user: %w", err)
	}
	log.Printf("[DEBUG] User not found, continuing with registration")

	// Check if this is the first user (make them admin)
	isFirstUser := false
	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		log.Printf("[DEBUG] CountUsers error: %v", err)
		return nil, nil, fmt.Errorf("count users: %w", err)
	}
	log.Printf("[DEBUG] User count: %d, isFirstUser: %v", count, count == 0)
	if count == 0 {
		isFirstUser = true
	}

	// Create user in database
	now := time.Now().Format(time.RFC3339)
	role := string(model.RoleUser)
	if isFirstUser {
		role = string(model.RoleAdmin)
	}

	dbUser, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: "", // Empty for PIN-based auth
		PinHash:      sql.NullString{String: pinHash, Valid: true},
		Role:         role,
		Status:       string(model.StatusActive),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		log.Printf("[DEBUG] CreateUser error: %v", err)
		return nil, nil, fmt.Errorf("create user: %w", err)
	}
	log.Printf("[DEBUG] User created: %s", dbUser.ID)

	// Convert to model user
	user := s.userToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		log.Printf("[DEBUG] Generate tokens error: %v", err)
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}
	log.Printf("[DEBUG] Tokens generated successfully")

	// Return user without sensitive data
	return s.userWithoutSensitive(user), tokens, nil
}

// LoginWithPIN authenticates a user with username and PIN (OS-style login).
func (s *AuthService) LoginWithPIN(ctx context.Context, username, pin string) (*model.User, *model.TokenPair, error) {
	if ctx == nil {
		return nil, nil, errors.New("context is required")
	}
	if username == "" {
		return nil, nil, errors.New("username is required")
	}
	if pin == "" {
		return nil, nil, errors.New("PIN is required")
	}

	// Get user by username
	dbUser, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, model.ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	// Check if user is suspended
	if dbUser.Status == string(model.StatusSuspended) {
		return nil, nil, model.ErrAccountSuspended
	}

	// Check if user is deleted
	if dbUser.DeletedAt.Valid {
		return nil, nil, model.ErrInvalidCredentials
	}

	// Verify PIN
	if !dbUser.PinHash.Valid || dbUser.PinHash.String == "" {
		return nil, nil, model.ErrInvalidCredentials
	}

	valid, err := VerifyPIN(dbUser.PinHash.String, pin)
	if err != nil {
		return nil, nil, fmt.Errorf("verify PIN: %w", err)
	}
	if !valid {
		return nil, nil, model.ErrInvalidCredentials
	}

	// Convert to model user
	user := s.userToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Return user without sensitive data
	return s.userWithoutSensitive(user), tokens, nil
}

// Logout deletes a session, logging the user out.
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if sessionID == uuid.Nil {
		return errors.New("session ID is required")
	}

	// Delete session from database
	err := s.queries.DeleteSession(ctx, sessionID.String())
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// RefreshSession validates a refresh token and generates a new token pair.
// It also updates the session's last used timestamp.
func (s *AuthService) RefreshSession(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	// Validate refresh token
	tokenData, err := s.tokenSvc.ValidateRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			return nil, model.ErrTokenExpired
		}
		return nil, model.ErrRefreshTokenInvalid
	}

	// Get session to verify it's still valid
	refreshTokenHash := HashRefreshToken(refreshToken)
	now := time.Now().Format(time.RFC3339)
	session, err := s.queries.GetSessionByRefreshTokenHash(ctx, db.GetSessionByRefreshTokenHashParams{
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        now,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Update session last used timestamp
	_, err = s.queries.UpdateSessionLastUsed(ctx, db.UpdateSessionLastUsedParams{
		LastUsedAt: time.Now().Format(time.RFC3339),
		ID:         session.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	// Get user to generate new tokens
	dbUser, err := s.queries.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Check if user is still active
	if dbUser.Status == string(model.StatusSuspended) {
		return nil, model.ErrAccountSuspended
	}
	if dbUser.DeletedAt.Valid {
		return nil, model.ErrInvalidCredentials
	}

	// Convert to model user
	user := s.userToModel(dbUser)

	// Generate new token pair using the session ID from token data
	sessionID := tokenData.SessionID
	if sessionID == uuid.Nil {
		sessionID, _ = uuid.Parse(session.ID)
	}

	tokens, err := s.tokenSvc.GenerateTokenPair(s.userWithoutSensitive(user), sessionID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, nil
}

// GetCurrentUser retrieves a user by their ID.
// Returns the user without the sensitive data.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if userID == uuid.Nil {
		return nil, errors.New("user ID is required")
	}

	dbUser, err := s.queries.GetUserByID(ctx, userID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Check if user is deleted
	if dbUser.DeletedAt.Valid {
		return nil, model.ErrUserNotFound
	}

	// Convert and return user without sensitive data
	user := s.userToModel(dbUser)
	return s.userWithoutSensitive(user), nil
}

// ListUsersForLogin returns public user information for the login screen.
// Only returns active users with their ID, username, and role.
func (s *AuthService) ListUsersForLogin(ctx context.Context) ([]model.UserListItem, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	dbUsers, err := s.queries.ListUsers(ctx, db.ListUsersParams{
		Column1: nil,
		Limit:   100, // Max users to show on login screen
		Offset:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]model.UserListItem, 0, len(dbUsers))
	for _, u := range dbUsers {
		if u.Status != string(model.StatusActive) {
			continue
		}
		id, _ := uuid.Parse(u.ID)
		createdAt, _ := time.Parse(time.RFC3339, u.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, u.UpdatedAt)
		users = append(users, model.UserListItem{
			ID:        id,
			Username:  u.Username,
			Role:      model.UserRole(u.Role),
			Status:    model.UserStatus(u.Status),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return users, nil
}

// CountUsers returns the total number of users.
func (s *AuthService) CountUsers(ctx context.Context) (int64, error) {
	if ctx == nil {
		return 0, errors.New("context is required")
	}
	return s.queries.CountUsers(ctx)
}

// generateTokensForUser creates a session and generates token pair for a user.
func (s *AuthService) generateTokensForUser(ctx context.Context, user *model.User) (*model.TokenPair, error) {
	now := time.Now().Format(time.RFC3339)
	expiresAt := time.Now().Add(s.tokenSvc.GetRefreshTokenTTL())

	// Create session FIRST with empty refresh token hash
	session, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:               uuid.New().String(),
		UserID:           user.ID.String(),
		RefreshTokenHash: "",
		UserAgent:        sql.NullString{},
		IpAddress:        sql.NullString{Valid: false},
		ExpiresAt:        expiresAt.Format(time.RFC3339),
		CreatedAt:        now,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Generate tokens with real session ID
	sessionUUID, _ := uuid.Parse(session.ID)
	tokens, err := s.tokenSvc.GenerateTokenPair(s.userWithoutSensitive(user), sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("generate token pair: %w", err)
	}

	// Update session with refresh token hash
	refreshTokenHash := HashRefreshToken(tokens.RefreshToken)
	_, err = s.queries.UpdateSessionRefreshTokenHash(ctx, db.UpdateSessionRefreshTokenHashParams{
		RefreshTokenHash: refreshTokenHash,
		ID:               session.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("update session refresh token hash: %w", err)
	}

	return tokens, nil
}

// userWithoutSensitive returns a copy of the user without password hash and PIN hash.
func (s *AuthService) userWithoutSensitive(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:              user.ID,
		Username:        user.Username,
		PasswordHash:    nil,
		PinHash:         nil,
		Role:            user.Role,
		Status:          user.Status,
		SuspendedAt:     user.SuspendedAt,
		SuspendedReason: user.SuspendedReason,
		DeletedAt:       user.DeletedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

// userToModel converts a *User (from db) to model.User.
func (s *AuthService) userToModel(row *db.User) *model.User {
	if row == nil {
		return nil
	}

	userID, _ := uuid.Parse(row.ID)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	var pinHash *string
	if row.PinHash.Valid {
		pinHash = &row.PinHash.String
	}

	user := &model.User{
		ID:           userID,
		Username:     row.Username,
		PasswordHash: &row.PasswordHash,
		PinHash:      pinHash,
		Role:         model.UserRole(row.Role),
		Status:       model.UserStatus(row.Status),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	if row.SuspendedAt.Valid {
		suspendedAt, _ := time.Parse(time.RFC3339, row.SuspendedAt.String)
		user.SuspendedAt = &suspendedAt
	}

	if row.SuspendedReason.Valid {
		user.SuspendedReason = &row.SuspendedReason.String
	}

	if row.DeletedAt.Valid {
		deletedAt, _ := time.Parse(time.RFC3339, row.DeletedAt.String)
		user.DeletedAt = &deletedAt
	}

	return user
}
