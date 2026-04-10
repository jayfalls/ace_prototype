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

// UserStatus represents the current status of a user account.
type UserStatus string

const (
	StatusPending   UserStatus = "pending"
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
)

// User represents a user in the system.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    *string    `json:"-"` // Never exposed in JSON
	Role            UserRole   `json:"role"`
	Status          UserStatus `json:"status"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsActive checks if the user account is active.
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// IsSuspended checks if the user account is suspended.
func (u *User) IsSuspended() bool {
	return u.Status == StatusSuspended
}

// IsDeleted checks if the user account is soft-deleted.
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// CanLogin checks if the user can log in based on status.
func (u *User) CanLogin() bool {
	return u.Status == StatusActive && u.DeletedAt == nil
}
