package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthEventType represents the type of authentication event.
type AuthEventType string

const (
	EventLogin            AuthEventType = "login"
	EventLogout           AuthEventType = "logout"
	EventFailedLogin      AuthEventType = "failed_login"
	EventAccountLocked    AuthEventType = "account_locked"
	EventPasswordChange   AuthEventType = "password_change"
	EventUserRegistered   AuthEventType = "user_registered"
	EventRoleChange       AuthEventType = "role_change"
	EventAccountSuspended AuthEventType = "account_suspended"
	EventAccountRestored  AuthEventType = "account_restored"
	EventAccountDeleted   AuthEventType = "account_deleted"
	EventTokenRevoked     AuthEventType = "token_revoked"
)

// AuthEvent represents a general authentication event.
type AuthEvent struct {
	EventID   uuid.UUID         `json:"event_id"`
	EventType AuthEventType     `json:"event_type"`
	UserID    uuid.UUID         `json:"user_id"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  AuthEventMetadata `json:"metadata"`
}

// AuthEventMetadata contains additional context for auth events.
type AuthEventMetadata struct {
	IPAddress   string `json:"ip_address,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
	Provider    string `json:"provider,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Role        string `json:"role,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Attempts    int    `json:"attempts,omitempty"`
	OldRole     string `json:"old_role,omitempty"`
	ChangedBy   string `json:"changed_by,omitempty"`
	SuspendedBy string `json:"suspended_by,omitempty"`
	DeletedBy   string `json:"deleted_by,omitempty"`
}

// LoginEvent represents a successful login event.
type LoginEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// LogoutEvent represents a logout event.
type LogoutEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
}

// FailedLoginEvent represents a failed login attempt.
type FailedLoginEvent struct {
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	IPAddress string    `json:"ip_address,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// PasswordChangeEvent represents a password change event.
type PasswordChangeEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}

// RoleChangeEvent represents a role change event.
type RoleChangeEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	OldRole   string    `json:"old_role"`
	NewRole   string    `json:"new_role"`
	ChangedBy uuid.UUID `json:"changed_by"`
	Timestamp time.Time `json:"timestamp"`
}

// AccountSuspendedEvent represents an account suspension event.
type AccountSuspendedEvent struct {
	UserID      uuid.UUID `json:"user_id"`
	SuspendedBy uuid.UUID `json:"suspended_by"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
}

// AccountDeletedEvent represents an account deletion event.
type AccountDeletedEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	DeletedBy uuid.UUID `json:"deleted_by"`
	Timestamp time.Time `json:"timestamp"`
}
