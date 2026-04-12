// Package service provides authentication and authorization services.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ace/api/internal/model"
	db "ace/api/internal/repository/generated"
)

// AuthService handles user authentication operations.
// It orchestrates password verification, token generation, and session management.
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

// Register registers a new user with email and password.
// It validates password strength, hashes the password, creates the user,
// and generates an initial token pair.
func (s *AuthService) Register(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
	if ctx == nil {
		return nil, nil, errors.New("context is required")
	}
	if email == "" {
		return nil, nil, errors.New("email is required")
	}
	if password == "" {
		return nil, nil, errors.New("password is required")
	}

	// Validate password strength first
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", model.ErrWeakPassword, err)
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, fmt.Errorf("check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, nil, model.ErrUserAlreadyExists
	}

	// Create user in database
	dbUser, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	// Convert to model user
	user := s.dbUserToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Return user without password hash
	return s.userWithoutPassword(user), tokens, nil
}

// Login authenticates a user with email and password.
// It verifies the credentials and generates a new token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*model.User, *model.TokenPair, error) {
	if ctx == nil {
		return nil, nil, errors.New("context is required")
	}
	if email == "" {
		return nil, nil, errors.New("email is required")
	}
	if password == "" {
		return nil, nil, errors.New("password is required")
	}

	// Get user by email
	dbUser, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	// Verify password
	valid, err := VerifyPassword(dbUser.PasswordHash, password)
	if err != nil {
		return nil, nil, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return nil, nil, model.ErrInvalidCredentials
	}

	// Convert to model user
	user := s.dbUserToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Return user without password hash
	return s.userWithoutPassword(user), tokens, nil
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
	err := s.queries.DeleteSession(ctx, pgtype.UUID{Bytes: sessionID, Valid: true})
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
	session, err := s.queries.GetSessionByRefreshTokenHash(ctx, refreshTokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Update session last used timestamp
	_, err = s.queries.UpdateSessionLastUsed(ctx, session.ID)
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
	user := s.dbUserToModel(dbUser)

	// Generate new token pair using the session ID from token data
	sessionID := tokenData.SessionID
	if sessionID == uuid.Nil {
		sessionID = session.ID.Bytes
	}

	tokens, err := s.tokenSvc.GenerateTokenPair(s.userWithoutPassword(user), sessionID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, nil
}

// GetCurrentUser retrieves a user by their ID.
// Returns the user without the password hash.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if userID == uuid.Nil {
		return nil, errors.New("user ID is required")
	}

	dbUser, err := s.queries.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Check if user is deleted
	if dbUser.DeletedAt.Valid {
		return nil, model.ErrUserNotFound
	}

	// Convert and return user without password hash
	user := s.dbUserToModel(dbUser)
	return s.userWithoutPassword(user), nil
}

// generateTokensForUser creates a session and generates token pair for a user.
func (s *AuthService) generateTokensForUser(ctx context.Context, user *model.User) (*model.TokenPair, error) {
	// Generate token pair first to get the refresh token for session
	// Use nil UUID as placeholder - will regenerate after getting actual session ID
	tokens, err := s.tokenSvc.GenerateTokenPair(user, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("generate token pair: %w", err)
	}

	// Create session in database
	refreshTokenHash := HashRefreshToken(tokens.RefreshToken)
	expiresAt := time.Now().Add(s.tokenSvc.GetRefreshTokenTTL())

	session, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		UserID:           pgtype.UUID{Bytes: user.ID, Valid: true},
		RefreshTokenHash: refreshTokenHash,
		UserAgent:        pgtype.Text{Valid: false},
		IpAddress:        nil,
		ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Regenerate tokens with actual session ID
	tokens, err = s.tokenSvc.GenerateTokenPair(s.userWithoutPassword(user), session.ID.Bytes)
	if err != nil {
		return nil, fmt.Errorf("regenerate token pair: %w", err)
	}

	// Update session last used timestamp
	_, err = s.queries.UpdateSessionLastUsed(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	return tokens, nil
}

// userWithoutPassword returns a copy of the user without the password hash.
func (s *AuthService) userWithoutPassword(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:              user.ID,
		Email:           user.Email,
		PasswordHash:    nil,
		Role:            user.Role,
		Status:          user.Status,
		SuspendedAt:     user.SuspendedAt,
		SuspendedReason: user.SuspendedReason,
		DeletedAt:       user.DeletedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

// dbUserToModel converts a database user to a model user.
func (s *AuthService) dbUserToModel(dbUser *db.User) *model.User {
	if dbUser == nil {
		return nil
	}

	user := &model.User{
		ID:           dbUser.ID.Bytes,
		Email:        dbUser.Email,
		PasswordHash: &dbUser.PasswordHash,
		Role:         model.UserRole(dbUser.Role),
		Status:       model.UserStatus(dbUser.Status),
		CreatedAt:    dbUser.CreatedAt.Time,
		UpdatedAt:    dbUser.UpdatedAt.Time,
	}

	if dbUser.SuspendedAt.Valid {
		user.SuspendedAt = &dbUser.SuspendedAt.Time
	}

	if dbUser.SuspendedReason.Valid {
		user.SuspendedReason = &dbUser.SuspendedReason.String
	}

	if dbUser.DeletedAt.Valid {
		user.DeletedAt = &dbUser.DeletedAt.Time
	}

	return user
}
