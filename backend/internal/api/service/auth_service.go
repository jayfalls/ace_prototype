// Package service provides authentication and authorization services.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
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
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, nil, model.ErrUserAlreadyExists
	}

	// Create user in database
	now := time.Now().Format(time.RFC3339)
	dbUser, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         string(model.RoleUser),
		Status:       string(model.StatusActive),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	// Convert to model user
	user := s.createUserRowToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Return user without password hash
	return s.userWithoutSensitive(user), tokens, nil
}

// RegisterWithPIN registers a new user with username and PIN (OS-style login).
// First user automatically becomes admin.
func (s *AuthService) RegisterWithPIN(ctx context.Context, username, pin, email string) (*model.User, *model.TokenPair, error) {
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
	existingUser, err := s.queries.GetUserByUsername(ctx, sql.NullString{String: username, Valid: true})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, nil, model.ErrUserAlreadyExists
	}

	// Check if this is the first user (make them admin)
	isFirstUser := false
	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count users: %w", err)
	}
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
		Email:        email,
		Username:     sql.NullString{String: username, Valid: true},
		PasswordHash: "", // Empty for PIN-based auth
		PinHash:      sql.NullString{String: pinHash, Valid: true},
		Role:         role,
		Status:       string(model.StatusActive),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	// Convert to model user
	user := s.createUserRowToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Return user without sensitive data
	return s.userWithoutSensitive(user), tokens, nil
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

	// Verify password
	valid, err := VerifyPassword(dbUser.PasswordHash, password)
	if err != nil {
		return nil, nil, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return nil, nil, model.ErrInvalidCredentials
	}

	// Convert to model user
	user := s.getUserByEmailRowToModel(dbUser)

	// Generate token pair
	tokens, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

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
	dbUser, err := s.queries.GetUserByUsername(ctx, sql.NullString{String: username, Valid: true})
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
	user := s.getUserByUsernameRowToModel(dbUser)

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
	user := s.getUserByIDRowToModel(dbUser)

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
	user := s.getUserByIDRowToModel(dbUser)
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
		if u.Status != string(model.StatusActive) || !u.Username.Valid {
			continue
		}
		id, _ := uuid.Parse(u.ID)
		createdAt, _ := time.Parse(time.RFC3339, u.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, u.UpdatedAt)
		users = append(users, model.UserListItem{
			ID:        id,
			Username:  u.Username.String,
			Email:     u.Email,
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
	// Generate token pair first to get the refresh token for session
	// Use nil UUID as placeholder - will regenerate after getting actual session ID
	tokens, err := s.tokenSvc.GenerateTokenPair(user, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("generate token pair: %w", err)
	}

	// Create session in database
	refreshTokenHash := HashRefreshToken(tokens.RefreshToken)
	expiresAt := time.Now().Add(s.tokenSvc.GetRefreshTokenTTL())
	now := time.Now().Format(time.RFC3339)

	session, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:               uuid.New().String(),
		UserID:           user.ID.String(),
		RefreshTokenHash: refreshTokenHash,
		UserAgent:        sql.NullString{},
		IpAddress:        sql.NullString{Valid: false},
		ExpiresAt:        expiresAt.Format(time.RFC3339),
		CreatedAt:        now,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Regenerate tokens with actual session ID
	sessionUUID, _ := uuid.Parse(session.ID)
	tokens, err = s.tokenSvc.GenerateTokenPair(s.userWithoutSensitive(user), sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("regenerate token pair: %w", err)
	}

	// Update session last used timestamp
	_, err = s.queries.UpdateSessionLastUsed(ctx, db.UpdateSessionLastUsedParams{
		LastUsedAt: now,
		ID:         session.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("update session: %w", err)
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
		Email:           user.Email,
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

// createUserRowToModel converts a CreateUserRow to model.User.
func (s *AuthService) createUserRowToModel(row *db.CreateUserRow) *model.User {
	if row == nil {
		return nil
	}

	userID, _ := uuid.Parse(row.ID)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	var username string
	if row.Username.Valid {
		username = row.Username.String
	}

	var pinHash *string
	if row.PinHash.Valid {
		pinHash = &row.PinHash.String
	}

	user := &model.User{
		ID:           userID,
		Email:        row.Email,
		Username:     username,
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

// getUserByEmailRowToModel converts a GetUserByEmailRow to model.User.
func (s *AuthService) getUserByEmailRowToModel(row *db.GetUserByEmailRow) *model.User {
	if row == nil {
		return nil
	}

	userID, _ := uuid.Parse(row.ID)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	var username string
	if row.Username.Valid {
		username = row.Username.String
	}

	var pinHash *string
	if row.PinHash.Valid {
		pinHash = &row.PinHash.String
	}

	user := &model.User{
		ID:           userID,
		Email:        row.Email,
		Username:     username,
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

// getUserByUsernameRowToModel converts a GetUserByUsernameRow to model.User.
func (s *AuthService) getUserByUsernameRowToModel(row *db.GetUserByUsernameRow) *model.User {
	if row == nil {
		return nil
	}

	userID, _ := uuid.Parse(row.ID)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	var username string
	if row.Username.Valid {
		username = row.Username.String
	}

	var pinHash *string
	if row.PinHash.Valid {
		pinHash = &row.PinHash.String
	}

	user := &model.User{
		ID:           userID,
		Email:        row.Email,
		Username:     username,
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

// getUserByIDRowToModel converts a GetUserByIDRow to model.User.
func (s *AuthService) getUserByIDRowToModel(row *db.GetUserByIDRow) *model.User {
	if row == nil {
		return nil
	}

	userID, _ := uuid.Parse(row.ID)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	var username string
	if row.Username.Valid {
		username = row.Username.String
	}

	var pinHash *string
	if row.PinHash.Valid {
		pinHash = &row.PinHash.String
	}

	user := &model.User{
		ID:           userID,
		Email:        row.Email,
		Username:     username,
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
