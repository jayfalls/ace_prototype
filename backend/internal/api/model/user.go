package model

import (
	"time"

	"github.com/google/uuid"
)

// UserRole represents the role of a user in the system.
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleUser   UserRole = "user"
	RoleViewer UserRole = "viewer"
)

// UserStatus represents the current status of a user's account.
type UserStatus string

const (
	StatusPending   UserStatus = "pending"
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
)

// User represents an authenticated user in the system.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	PasswordHash    *string    `json:"-"`
	PinHash         *string    `json:"-"`
	Role            UserRole   `json:"role"`
	Status          UserStatus `json:"status"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsActive returns true if the user is active.
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// IsSuspended returns true if the user is suspended.
func (u *User) IsSuspended() bool {
	return u.Status == StatusSuspended
}

// IsDeleted returns true if the user has been soft-deleted.
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// CanLogin returns true if the user can log in (active and not deleted/suspended).
func (u *User) CanLogin() bool {
	return u.IsActive() && !u.IsDeleted() && !u.IsSuspended()
}

// UserListItem represents a user in the login screen list.
type UserListItem struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	Role      UserRole   `json:"role"`
	Status    UserStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
