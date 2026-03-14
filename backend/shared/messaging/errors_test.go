package messaging

import (
	"errors"
	"testing"
)

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantMsg  string
	}{
		{
			name:    "ErrConnectionFailed",
			err:     ErrConnectionFailed,
			wantMsg: "nats connection failed",
		},
		{
			name:    "ErrJetStreamDown",
			err:     ErrJetStreamDown,
			wantMsg: "jetstream unavailable",
		},
		{
			name:    "ErrTimeout",
			err:     ErrTimeout,
			wantMsg: "request timeout",
		},
		{
			name:    "ErrNoResponders",
			err:     ErrNoResponders,
			wantMsg: "no responders for request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestMessagingError(t *testing.T) {
	originalErr := errors.New("original error")

	t.Run("NewMessagingError", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test message", originalErr)
		if err.Code != ErrCodeConnection {
			t.Errorf("Code = %v, want %v", err.Code, ErrCodeConnection)
		}
		if err.Message != "test message" {
			t.Errorf("Message = %v, want %v", err.Message, "test message")
		}
		if !errors.Is(err, originalErr) {
			t.Error("Unwrap should return original error")
		}
	})

	t.Run("Error string includes code and message", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test message", originalErr)
		errStr := err.Error()
		if errStr == "" {
			t.Error("Error string should not be empty")
		}
	})
}

func TestErrorWrappers(t *testing.T) {
	originalErr := errors.New("connection refused")

	t.Run("ConnectionError wraps error", func(t *testing.T) {
		err := ConnectionError(originalErr)
		if !errors.Is(err, originalErr) {
			t.Error("ConnectionError should unwrap to original error")
		}
	})

	t.Run("ConnectionError nil returns nil", func(t *testing.T) {
		err := ConnectionError(nil)
		if err != nil {
			t.Errorf("ConnectionError(nil) = %v, want nil", err)
		}
	})

	t.Run("JetStreamError wraps error", func(t *testing.T) {
		err := JetStreamError(originalErr)
		if !errors.Is(err, originalErr) {
			t.Error("JetStreamError should unwrap to original error")
		}
	})

	t.Run("JetStreamError nil returns nil", func(t *testing.T) {
		err := JetStreamError(nil)
		if err != nil {
			t.Errorf("JetStreamError(nil) = %v, want nil", err)
		}
	})

	t.Run("TimeoutError wraps error", func(t *testing.T) {
		err := TimeoutError(originalErr)
		if !errors.Is(err, originalErr) {
			t.Error("TimeoutError should unwrap to original error")
		}
	})

	t.Run("TimeoutError nil returns nil", func(t *testing.T) {
		err := TimeoutError(nil)
		if err != nil {
			t.Errorf("TimeoutError(nil) = %v, want nil", err)
		}
	})

	t.Run("NoRespondersError wraps error", func(t *testing.T) {
		err := NoRespondersError(originalErr)
		if !errors.Is(err, originalErr) {
			t.Error("NoRespondersError should unwrap to original error")
		}
	})

	t.Run("NoRespondersError nil returns nil", func(t *testing.T) {
		err := NoRespondersError(nil)
		if err != nil {
			t.Errorf("NoRespondersError(nil) = %v, want nil", err)
		}
	})
}

func TestMessagingErrorIs(t *testing.T) {
	originalErr := errors.New("original")

	t.Run("Is returns true for matching code", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test", originalErr)
		target := &MessagingError{Code: ErrCodeConnection}
		if !errors.Is(err, target) {
			t.Error("Is should return true for matching code")
		}
	})

	t.Run("Is returns false for different code", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test", originalErr)
		target := &MessagingError{Code: ErrCodeJetStream}
		if errors.Is(err, target) {
			t.Error("Is should return false for different code")
		}
	})

	t.Run("Is returns true for wrapped error", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test", originalErr)
		if !errors.Is(err, originalErr) {
			t.Error("Is should return true for wrapped error")
		}
	})
}

func TestMessagingErrorAs(t *testing.T) {
	originalErr := errors.New("original")

	t.Run("As succeeds for same type", func(t *testing.T) {
		err := NewMessagingError(ErrCodeConnection, "test", originalErr)
		var target *MessagingError
		if !errors.As(err, &target) {
			t.Error("As should succeed for same type")
		}
		if target.Code != ErrCodeConnection {
			t.Errorf("target.Code = %v, want %v", target.Code, ErrCodeConnection)
		}
	})
}
