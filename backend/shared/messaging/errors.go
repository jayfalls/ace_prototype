package messaging

import (
	"errors"
	"fmt"
)

// Predefined error types for NATS messaging operations.

var (
	// ErrConnectionFailed is returned when the NATS connection cannot be established.
	ErrConnectionFailed = errors.New("nats connection failed")

	// ErrJetStreamDown is returned when JetStream is unavailable.
	ErrJetStreamDown = errors.New("jetstream unavailable")

	// ErrTimeout is returned when a request times out.
	ErrTimeout = errors.New("request timeout")

	// ErrNoResponders is returned when there are no responders for a request.
	ErrNoResponders = errors.New("no responders for request")
)

// Error types with additional context for better error handling.

const (
	// ErrCodeConnection is the error code for connection failures.
	ErrCodeConnection = "CONNECTION_FAILED"

	// ErrCodeJetStream is the error code for JetStream failures.
	ErrCodeJetStream = "JETSTREAM_DOWN"

	// ErrCodeTimeout is the error code for timeout errors.
	ErrCodeTimeout = "TIMEOUT"

	// ErrCodeNoResponders is the error code for no responders errors.
	ErrCodeNoResponders = "NO_RESPONDERS"
)

// MessagingError represents an error with additional context.
type MessagingError struct {
	Code    string
	Message string
	Err     error
}

// Error returns the error message.
func (e *MessagingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *MessagingError) Unwrap() error {
	return e.Err
}

// NewMessagingError creates a new MessagingError with the given code, message, and error.
func NewMessagingError(code, message string, err error) *MessagingError {
	return &MessagingError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ConnectionError wraps a connection error with context.
func ConnectionError(err error) error {
	if err == nil {
		return nil
	}
	return &MessagingError{
		Code:    ErrCodeConnection,
		Message: "failed to connect to NATS server",
		Err:     err,
	}
}

// JetStreamError wraps a JetStream error with context.
func JetStreamError(err error) error {
	if err == nil {
		return nil
	}
	return &MessagingError{
		Code:    ErrCodeJetStream,
		Message: "JetStream operation failed",
		Err:     err,
	}
}

// TimeoutError wraps a timeout error with context.
func TimeoutError(err error) error {
	if err == nil {
		return nil
	}
	return &MessagingError{
		Code:    ErrCodeTimeout,
		Message: "operation timed out",
		Err:     err,
	}
}

// NoRespondersError wraps a no responders error with context.
func NoRespondersError(err error) error {
	if err == nil {
		return nil
	}
	return &MessagingError{
		Code:    ErrCodeNoResponders,
		Message: "no responders available",
		Err:     err,
	}
}

// Is checks if the target error matches this error.
func (e *MessagingError) Is(target error) bool {
	if me, ok := target.(*MessagingError); ok {
		return e.Code == me.Code
	}
	return errors.Is(e.Err, target)
}

// As checks if the error can be cast to the target type.
func (e *MessagingError) As(target interface{}) bool {
	if te, ok := target.(*MessagingError); ok {
		*te = *e
		return true
	}
	return errors.As(e.Err, target)
}
