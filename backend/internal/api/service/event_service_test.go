package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
)

// testTime is a fixed timestamp for consistent test comparisons
var testTime = time.Date(2024, 4, 9, 12, 0, 0, 0, time.UTC)

// MockEventPublisher is a mock implementation of EventPublisher for testing.
type MockEventPublisher struct {
	PublishedEvents []PublishedEvent
	ErrToReturn     error
}

type PublishedEvent struct {
	Subject       string
	CorrelationID string
	AgentID       string
	Source        string
	Payload       []byte
}

func (m *MockEventPublisher) Publish(ctx context.Context, subject, correlationID, agentID, cycleID, source string, payload []byte) error {
	if m.ErrToReturn != nil {
		return m.ErrToReturn
	}
	m.PublishedEvents = append(m.PublishedEvents, PublishedEvent{
		Subject:       subject,
		CorrelationID: correlationID,
		AgentID:       agentID,
		Source:        source,
		Payload:       payload,
	})
	return nil
}

func TestNewEventService(t *testing.T) {
	t.Run("creates service with nil publisher (stub mode)", func(t *testing.T) {
		svc := NewEventService(nil, nil)
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if svc.enabled {
			t.Error("expected stub mode (enabled=false)")
		}
	})

	t.Run("creates service with publisher", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if !svc.enabled {
			t.Error("expected enabled=true")
		}
	})
}

func TestPublishLoginEvent(t *testing.T) {
	t.Run("publishes login event in stub mode", func(t *testing.T) {
		svc := NewEventService(nil, nil)
		event := model.LoginEvent{
			UserID:    uuid.New(),
			Email:     "test@example.com",
			Timestamp: testTime,
			IPAddress: "192.168.1.1",
			UserAgent: "Mozilla/5.0",
		}

		err := svc.PublishLoginEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("publishes login event via publisher", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		event := model.LoginEvent{
			UserID:    userID,
			Email:     "test@example.com",
			Timestamp: testTime,
			IPAddress: "192.168.1.1",
			UserAgent: "Mozilla/5.0",
		}

		err := svc.PublishLoginEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		evt := mock.PublishedEvents[0]
		if evt.Subject != "ace.auth.login.event" {
			t.Errorf("expected subject 'ace.auth.login.event', got '%s'", evt.Subject)
		}
		if evt.AgentID != userID.String() {
			t.Errorf("expected agentID '%s', got '%s'", userID.String(), evt.AgentID)
		}
	})

	t.Run("handles publisher error", func(t *testing.T) {
		mock := &MockEventPublisher{ErrToReturn: errors.New("publish failed")}
		svc := NewEventService(mock, nil)

		event := model.LoginEvent{
			UserID:    uuid.New(),
			Email:     "test@example.com",
			Timestamp: testTime,
		}

		err := svc.PublishLoginEvent(context.Background(), event)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPublishLogoutEvent(t *testing.T) {
	t.Run("publishes logout event in stub mode", func(t *testing.T) {
		svc := NewEventService(nil, nil)
		event := model.LogoutEvent{
			UserID:    uuid.New(),
			SessionID: "session-123",
			Timestamp: testTime,
		}

		err := svc.PublishLogoutEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("publishes logout event via publisher", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		event := model.LogoutEvent{
			UserID:    userID,
			SessionID: "session-123",
			Timestamp: testTime,
		}

		err := svc.PublishLogoutEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.logout.event" {
			t.Errorf("expected subject 'ace.auth.logout.event', got '%s'", mock.PublishedEvents[0].Subject)
		}
	})
}

func TestPublishFailedLoginEvent(t *testing.T) {
	t.Run("publishes failed login event", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		event := model.FailedLoginEvent{
			Email:     "test@example.com",
			Timestamp: testTime,
			IPAddress: "192.168.1.1",
			Reason:    "invalid_password",
		}

		err := svc.PublishFailedLoginEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.failed_login.event" {
			t.Errorf("expected subject 'ace.auth.failed_login.event', got '%s'", mock.PublishedEvents[0].Subject)
		}

		// Failed login should have empty agentID (no user yet)
		if mock.PublishedEvents[0].AgentID != "" {
			t.Errorf("expected empty agentID, got '%s'", mock.PublishedEvents[0].AgentID)
		}
	})
}

func TestPublishPasswordChangeEvent(t *testing.T) {
	t.Run("publishes password change event", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		event := model.PasswordChangeEvent{
			UserID:    userID,
			Timestamp: testTime,
		}

		err := svc.PublishPasswordChangeEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.password_change.event" {
			t.Errorf("expected subject 'ace.auth.password_change.event', got '%s'", mock.PublishedEvents[0].Subject)
		}
	})
}

func TestPublishRoleChangeEvent(t *testing.T) {
	t.Run("publishes role change event", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		changedBy := uuid.New()
		event := model.RoleChangeEvent{
			UserID:    userID,
			OldRole:   "user",
			NewRole:   "admin",
			ChangedBy: changedBy,
			Timestamp: testTime,
		}

		err := svc.PublishRoleChangeEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.role_change.event" {
			t.Errorf("expected subject 'ace.auth.role_change.event', got '%s'", mock.PublishedEvents[0].Subject)
		}
	})
}

func TestPublishAccountSuspendedEvent(t *testing.T) {
	t.Run("publishes account suspended event", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		suspendedBy := uuid.New()
		event := model.AccountSuspendedEvent{
			UserID:      userID,
			SuspendedBy: suspendedBy,
			Reason:      "violation of terms",
			Timestamp:   testTime,
		}

		err := svc.PublishAccountSuspendedEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.account_suspended.event" {
			t.Errorf("expected subject 'ace.auth.account_suspended.event', got '%s'", mock.PublishedEvents[0].Subject)
		}
	})
}

func TestPublishAccountDeletedEvent(t *testing.T) {
	t.Run("publishes account deleted event", func(t *testing.T) {
		mock := &MockEventPublisher{}
		svc := NewEventService(mock, nil)

		userID := uuid.New()
		deletedBy := uuid.New()
		event := model.AccountDeletedEvent{
			UserID:    userID,
			DeletedBy: deletedBy,
			Timestamp: testTime,
		}

		err := svc.PublishAccountDeletedEvent(context.Background(), event)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mock.PublishedEvents) != 1 {
			t.Fatalf("expected 1 event, got %d", len(mock.PublishedEvents))
		}

		if mock.PublishedEvents[0].Subject != "ace.auth.account_deleted.event" {
			t.Errorf("expected subject 'ace.auth.account_deleted.event', got '%s'", mock.PublishedEvents[0].Subject)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("NewLoginEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		event := NewLoginEvent(userID, "test@example.com", "192.168.1.1", "Mozilla/5.0")

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.Email != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%s'", event.Email)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewLogoutEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		event := NewLogoutEvent(userID, "session-123")

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.SessionID != "session-123" {
			t.Errorf("expected sessionID 'session-123', got '%s'", event.SessionID)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewFailedLoginEvent creates event with timestamp", func(t *testing.T) {
		event := NewFailedLoginEvent("test@example.com", "192.168.1.1", "invalid_password")

		if event.Email != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%s'", event.Email)
		}
		if event.Reason != "invalid_password" {
			t.Errorf("expected reason 'invalid_password', got '%s'", event.Reason)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewPasswordChangeEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		event := NewPasswordChangeEvent(userID)

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewRoleChangeEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		changedBy := uuid.New()
		event := NewRoleChangeEvent(userID, "user", "admin", changedBy)

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.OldRole != "user" {
			t.Errorf("expected oldRole 'user', got '%s'", event.OldRole)
		}
		if event.NewRole != "admin" {
			t.Errorf("expected newRole 'admin', got '%s'", event.NewRole)
		}
		if event.ChangedBy != changedBy {
			t.Errorf("expected changedBy %s, got %s", changedBy, event.ChangedBy)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewAccountSuspendedEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		suspendedBy := uuid.New()
		event := NewAccountSuspendedEvent(userID, suspendedBy, "violation of terms")

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.SuspendedBy != suspendedBy {
			t.Errorf("expected suspendedBy %s, got %s", suspendedBy, event.SuspendedBy)
		}
		if event.Reason != "violation of terms" {
			t.Errorf("expected reason 'violation of terms', got '%s'", event.Reason)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NewAccountDeletedEvent creates event with timestamp", func(t *testing.T) {
		userID := uuid.New()
		deletedBy := uuid.New()
		event := NewAccountDeletedEvent(userID, deletedBy)

		if event.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, event.UserID)
		}
		if event.DeletedBy != deletedBy {
			t.Errorf("expected deletedBy %s, got %s", deletedBy, event.DeletedBy)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})
}
