package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

// Class is the failure taxonomy (spec 9.2): only transient and timeout retry.
type Class string

const (
	ClassTransient Class = "transient" // 5xx, 408, 429 w/o quota body, network
	ClassQuota     Class = "quota"     // 429 with quota/daily body
	ClassAuth      Class = "auth"      // 401/403
	ClassInvalid   Class = "invalid"   // 400/404/422
	ClassTimeout   Class = "timeout"
)

// Classified is an error carrying its failure class, HTTP status and the
// provider's reason string once retries are exhausted (spec 9.2).
type Classified struct {
	Class  Class
	Status int
	Reason string
	Err    error
}

func (e *Classified) Error() string {
	return fmt.Sprintf("%s (HTTP %d): %s", e.Class, e.Status, e.Reason)
}
func (e *Classified) Unwrap() error { return e.Err }

// Classify maps an error + optional HTTP status onto the taxonomy. status 0
// means a transport-level failure (no response).
func Classify(err error, status int) Class {
	if err == nil && status == 0 {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeoutErr(err) {
		return ClassTimeout
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	lower := strings.ToLower(reason)
	switch {
	case status == 401 || status == 403:
		return ClassAuth
	case status == 400 || status == 404 || status == 422:
		return ClassInvalid
	case status == 429:
		if strings.Contains(lower, "quota") || strings.Contains(lower, "daily") || strings.Contains(lower, "limit") {
			return ClassQuota
		}
		return ClassTransient
	case status == 408:
		return ClassTimeout
	case status >= 500:
		return ClassTransient
	case status == 0:
		return ClassTransient // network error with no response
	}
	return ClassTransient
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline")
}

// retryable reports the spec rule: only transient and timeout retry.
func retryable(c Class) bool { return c == ClassTransient || c == ClassTimeout }

// backoff returns the full-jitter delay for attempt n (0-based): base<<n
// capped at cap (spec: base 1 s, cap 8 s).
func backoff(attempt int, base, cap float64) time.Duration {
	if base <= 0 {
		base = 1
	}
	if cap <= 0 {
		cap = 8
	}
	d := base * float64(int(1)<<attempt)
	if d > cap {
		d = cap
	}
	return time.Duration(rand.Float64() * d * float64(time.Second))
}

// statusOf extracts the HTTP status embedded in a call error — the wire
// adapters format it as "HTTP <code>"; zero when absent (transport failure).
func statusOf(err error) int {
	if err == nil {
		return 0
	}
	var ce *Classified
	if errors.As(err, &ce) {
		return ce.Status
	}
	s := err.Error()
	i := strings.Index(s, "HTTP ")
	if i < 0 {
		return 0
	}
	var code int
	if _, err := fmt.Sscanf(s[i+5:], "%d", &code); err == nil {
		return code
	}
	return 0
}
