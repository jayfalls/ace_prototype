package caching

import (
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// Sentinel Errors
// =============================================================================

var (
	// ErrCacheMiss is returned when the key does not exist in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ErrBackendUnavailable is returned when the cache backend is unavailable.
	ErrBackendUnavailable = errors.New("cache backend unavailable")

	// ErrAgentIDMissing is returned when agentId is required but not provided.
	ErrAgentIDMissing = errors.New("agentID is required")

	// ErrInvalidKey is returned when the key format is invalid.
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrTTLExpired is returned when the TTL has expired.
	ErrTTLExpired = errors.New("TTL expired")

	// ErrVersionMismatch is returned when the version does not match.
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrStampedeLock is returned when the stampede lock cannot be acquired.
	ErrStampedeLock = errors.New("stampede lock acquisition failed")

	// ErrFetchFailed is returned when the fetch function fails.
	ErrFetchFailed = errors.New("fetch function failed")

	// ErrWarmingTimeout is returned when cache warming exceeds the deadline.
	ErrWarmingTimeout = errors.New("cache warming timeout")

	// ErrMaxSizeExceeded is returned when the value exceeds the maximum size.
	ErrMaxSizeExceeded = errors.New("value exceeds maximum size")

	// ErrSerializationFailed is returned when serialization/deserialization fails.
	ErrSerializationFailed = errors.New("serialization failed")

	// ErrNATSDisconnected is returned when NATS is disconnected.
	ErrNATSDisconnected = errors.New("NATS disconnected")

	// ErrPatternInvalid is returned when the pattern is invalid.
	ErrPatternInvalid = errors.New("invalid pattern")

	// ErrTagNotFound is returned when the tag does not exist.
	ErrTagNotFound = errors.New("tag not found")
)

// =============================================================================
// Error Codes
// =============================================================================

const (
	ErrCodeCacheMiss          = "CACHE_MISS"
	ErrCodeBackendUnavailable = "BACKEND_UNAVAILABLE"
	ErrCodeAgentIDMissing     = "AGENT_ID_MISSING"
	ErrCodeInvalidKey         = "INVALID_KEY"
	ErrCodeTTLExpired         = "TTL_EXPIRED"
	ErrCodeVersionMismatch    = "VERSION_MISMATCH"
	ErrCodeStampedeLock       = "STAMPEDE_LOCK"
	ErrCodeFetchFailed        = "FETCH_FAILED"
	ErrCodeWarmingTimeout     = "WARMING_TIMEOUT"
	ErrCodeMaxSizeExceeded    = "MAX_SIZE_EXCEEDED"
	ErrCodeSerialization      = "SERIALIZATION_FAILED"
	ErrCodeNATSDisconnected   = "NATS_DISCONNECTED"
	ErrCodePatternInvalid     = "PATTERN_INVALID"
	ErrCodeTagNotFound        = "TAG_NOT_FOUND"
)

// =============================================================================
// Error Types with Context
// =============================================================================

// CacheError represents an error with additional context.
type CacheError struct {
	Code    string
	Message string
	Err     error
}

// Error returns the error message.
func (e *CacheError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *CacheError) Unwrap() error {
	return e.Err
}

// Is checks if the target error matches this error.
func (e *CacheError) Is(target error) bool {
	if ce, ok := target.(*CacheError); ok {
		return e.Code == ce.Code
	}
	return errors.Is(e.Err, target)
}

// As checks if the error can be cast to the target type.
func (e *CacheError) As(target any) bool {
	if ce, ok := target.(*CacheError); ok {
		*ce = *e
		return true
	}
	return errors.As(e.Err, target)
}

// NewCacheError creates a new CacheError with the given code, message, and error.
func NewCacheError(code, message string, err error) *CacheError {
	return &CacheError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// =============================================================================
// Error Wrappers
// =============================================================================

// BackendUnavailableError wraps a backend unavailability error.
func BackendUnavailableError(err error) error {
	if err == nil {
		return nil
	}
	return &CacheError{
		Code:    ErrCodeBackendUnavailable,
		Message: "cache backend is unavailable",
		Err:     err,
	}
}

// AgentIDMissingError returns an agentID missing error.
func AgentIDMissingError() error {
	return &CacheError{
		Code:    ErrCodeAgentIDMissing,
		Message: "agentID is required for cache operations",
		Err:     nil,
	}
}

// InvalidKeyError wraps an invalid key error.
func InvalidKeyError(key string, reason string) error {
	return &CacheError{
		Code:    ErrCodeInvalidKey,
		Message: fmt.Sprintf("invalid key '%s': %s", key, reason),
		Err:     nil,
	}
}

// VersionMismatchError returns a version mismatch error.
func VersionMismatchError(expected, actual string) error {
	return &CacheError{
		Code:    ErrCodeVersionMismatch,
		Message: fmt.Sprintf("version mismatch: expected '%s', got '%s'", expected, actual),
		Err:     nil,
	}
}

// MaxSizeExceededError returns a max size exceeded error.
func MaxSizeExceededError(size, maxSize int64) error {
	return &CacheError{
		Code:    ErrCodeMaxSizeExceeded,
		Message: fmt.Sprintf("value size %d exceeds maximum %d", size, maxSize),
		Err:     nil,
	}
}

// FetchFailedError wraps a fetch function error.
func FetchFailedError(err error) error {
	if err == nil {
		return nil
	}
	return &CacheError{
		Code:    ErrCodeFetchFailed,
		Message: "fetch function failed",
		Err:     err,
	}
}

// WarmingTimeoutError returns a warming timeout error.
func WarmingTimeoutError(namespace string, elapsed time.Duration) error {
	return &CacheError{
		Code:    ErrCodeWarmingTimeout,
		Message: fmt.Sprintf("warming namespace '%s' timed out after %v", namespace, elapsed),
		Err:     nil,
	}
}

// PatternInvalidError returns an invalid pattern error.
func PatternInvalidError(pattern, reason string) error {
	return &CacheError{
		Code:    ErrCodePatternInvalid,
		Message: fmt.Sprintf("invalid pattern '%s': %s", pattern, reason),
		Err:     nil,
	}
}

// TagNotFoundError returns a tag not found error.
func TagNotFoundError(tag string) error {
	return &CacheError{
		Code:    ErrCodeTagNotFound,
		Message: fmt.Sprintf("tag '%s' not found", tag),
		Err:     nil,
	}
}
