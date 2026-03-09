package services

import (
	"context"
	"errors"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidInput     = errors.New("invalid input")
)

type UserService struct {
	queries interface {
		CreateUser(ctx context.Context, arg models.CreateUserParams) (models.User, error)
		GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
		GetUserByEmail(ctx context.Context, email string) (models.User, error)
		GetUserByUsername(ctx context.Context, username string) (models.User, error)
		ListUsers(ctx context.Context, limit, offset int32) ([]models.User, error)
		UpdateUser(ctx context.Context, arg models.UpdateUserParams) (models.User, error)
		DeleteUser(ctx context.Context, id uuid.UUID) error
	}
}

func NewUserService(q interface {
	CreateUser(ctx context.Context, arg models.CreateUserParams) (models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	ListUsers(ctx context.Context, limit, offset int32) ([]models.User, error)
	UpdateUser(ctx context.Context, arg models.UpdateUserParams) (models.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}) *UserService {
	return &UserService{queries: q}
}

type CreateUserInput struct {
	Email       string
	Username    string
	Password    string
	Role        string
	PasswordHash string
}

func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (models.User, error) {
	// Check if email already exists
	existing, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err == nil && existing.ID != uuid.Nil {
		return models.User{}, ErrUserExists
	}

	// Check if username already exists
	existing, err = s.queries.GetUserByUsername(ctx, input.Username)
	if err == nil && existing.ID != uuid.Nil {
		return models.User{}, ErrUserExists
	}

	role := input.Role
	if role == "" {
		role = "user"
	}

	return s.queries.CreateUser(ctx, models.CreateUserParams{
		Email:        input.Email,
		Username:     input.Username,
		PasswordHash: input.PasswordHash,
		Role:         role,
	})
}

func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		return models.User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return models.User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *UserService) ListUsers(ctx context.Context, limit, offset int32) ([]models.User, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.queries.ListUsers(ctx, limit, offset)
}

func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, email, username, role string) (models.User, error) {
	return s.queries.UpdateUser(ctx, models.UpdateUserParams{
		ID:       id,
		Email:    email,
		Username: username,
		Role:     role,
	})
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteUser(ctx, id)
}
