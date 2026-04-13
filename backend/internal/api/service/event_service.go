// Package service provides authentication and authorization services.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"ace/internal/api/model"
)

// EventPublisher is an interface for publishing events to NATS.
// This allows for mocking in tests.
type EventPublisher interface {
	Publish(ctx context.Context, subject, correlationID, agentID, cycleID, source string, payload []byte) error
}

// EventService handles publishing authentication events to NATS.
type EventService struct {
	publisher EventPublisher
	enabled   bool
	logger    *zap.Logger
}

// NewEventService creates a new event service.
// If publisher is nil, events will be logged but not published (development mode).
func NewEventService(publisher EventPublisher, logger *zap.Logger) *EventService {
	if publisher == nil {
		if logger != nil {
			logger.Warn("no publisher configured, running in stub mode - events will be logged only")
		}
		return &EventService{
			publisher: nil,
			enabled:   false,
			logger:    logger,
		}
	}

	return &EventService{
		publisher: publisher,
		enabled:   true,
		logger:    logger,
	}
}

// PublishLoginEvent publishes a successful login event.
func (s *EventService) PublishLoginEvent(ctx context.Context, event model.LoginEvent) error {
	subject := "ace.auth.login.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal login event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// PublishLogoutEvent publishes a logout event.
func (s *EventService) PublishLogoutEvent(ctx context.Context, event model.LogoutEvent) error {
	subject := "ace.auth.logout.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal logout event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// PublishFailedLoginEvent publishes a failed login attempt event.
func (s *EventService) PublishFailedLoginEvent(ctx context.Context, event model.FailedLoginEvent) error {
	subject := "ace.auth.failed_login.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal failed login event: %w", err)
	}

	return s.publish(ctx, subject, "", payload)
}

// PublishPasswordChangeEvent publishes a password change event.
func (s *EventService) PublishPasswordChangeEvent(ctx context.Context, event model.PasswordChangeEvent) error {
	subject := "ace.auth.password_change.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal password change event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// PublishRoleChangeEvent publishes a role change event.
func (s *EventService) PublishRoleChangeEvent(ctx context.Context, event model.RoleChangeEvent) error {
	subject := "ace.auth.role_change.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal role change event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// PublishAccountSuspendedEvent publishes an account suspension event.
func (s *EventService) PublishAccountSuspendedEvent(ctx context.Context, event model.AccountSuspendedEvent) error {
	subject := "ace.auth.account_suspended.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal account suspended event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// PublishAccountDeletedEvent publishes an account deletion event.
func (s *EventService) PublishAccountDeletedEvent(ctx context.Context, event model.AccountDeletedEvent) error {
	subject := "ace.auth.account_deleted.event"
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal account deleted event: %w", err)
	}

	return s.publish(ctx, subject, event.UserID.String(), payload)
}

// publish sends an event to the message broker or logs it in stub mode.
func (s *EventService) publish(ctx context.Context, subject, agentID string, payload []byte) error {
	if !s.enabled || s.publisher == nil {
		if s.logger != nil {
			s.logger.Debug("stub event",
				zap.String("subject", subject),
				zap.String("agent_id", agentID),
				zap.String("payload", string(payload)),
			)
		}
		return nil
	}

	correlationID := uuid.New().String()
	err := s.publisher.Publish(ctx, subject, correlationID, agentID, "", "auth-service", payload)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to publish event",
				zap.String("subject", subject),
				zap.Error(err),
			)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("event published",
			zap.String("subject", subject),
			zap.String("correlation_id", correlationID),
			zap.String("agent_id", agentID),
		)
	}
	return nil
}

// Helper functions to create events with defaults

// NewLoginEvent creates a new login event with the current timestamp.
func NewLoginEvent(userID uuid.UUID, email, ipAddress, userAgent string) model.LoginEvent {
	return model.LoginEvent{
		UserID:    userID,
		Email:     email,
		Timestamp: time.Now(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}
}

// NewLogoutEvent creates a new logout event with the current timestamp.
func NewLogoutEvent(userID uuid.UUID, sessionID string) model.LogoutEvent {
	return model.LogoutEvent{
		UserID:    userID,
		SessionID: sessionID,
		Timestamp: time.Now(),
	}
}

// NewFailedLoginEvent creates a new failed login event with the current timestamp.
func NewFailedLoginEvent(email, ipAddress, reason string) model.FailedLoginEvent {
	return model.FailedLoginEvent{
		Email:     email,
		Timestamp: time.Now(),
		IPAddress: ipAddress,
		Reason:    reason,
	}
}

// NewPasswordChangeEvent creates a new password change event with the current timestamp.
func NewPasswordChangeEvent(userID uuid.UUID) model.PasswordChangeEvent {
	return model.PasswordChangeEvent{
		UserID:    userID,
		Timestamp: time.Now(),
	}
}

// NewRoleChangeEvent creates a new role change event with the current timestamp.
func NewRoleChangeEvent(userID uuid.UUID, oldRole, newRole string, changedBy uuid.UUID) model.RoleChangeEvent {
	return model.RoleChangeEvent{
		UserID:    userID,
		OldRole:   oldRole,
		NewRole:   newRole,
		ChangedBy: changedBy,
		Timestamp: time.Now(),
	}
}

// NewAccountSuspendedEvent creates a new account suspended event with the current timestamp.
func NewAccountSuspendedEvent(userID, suspendedBy uuid.UUID, reason string) model.AccountSuspendedEvent {
	return model.AccountSuspendedEvent{
		UserID:      userID,
		SuspendedBy: suspendedBy,
		Reason:      reason,
		Timestamp:   time.Now(),
	}
}

// NewAccountDeletedEvent creates a new account deleted event with the current timestamp.
func NewAccountDeletedEvent(userID, deletedBy uuid.UUID) model.AccountDeletedEvent {
	return model.AccountDeletedEvent{
		UserID:    userID,
		DeletedBy: deletedBy,
		Timestamp: time.Now(),
	}
}
