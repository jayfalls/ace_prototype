// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	config := RateLimitConfig{
		MaxRequests:    10,
		WindowDuration: time.Minute,
	}
	rl := NewRateLimiter(config)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.requests == nil {
		t.Error("requests map should be initialized")
	}
	if rl.config.MaxRequests != 10 {
		t.Errorf("expected MaxRequests 10, got %d", rl.config.MaxRequests)
	}
}

func TestLimitByIP_AllowsRequestsWithinLimit(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    5,
		WindowDuration: time.Minute,
	})

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		allowed, err := rl.LimitByIP("192.168.1.1", 5, time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestLimitByIP_BlocksRequestsExceedingLimit(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    3,
		WindowDuration: time.Minute,
	})

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		allowed, _ := rl.LimitByIP("192.168.1.1", 3, time.Minute)
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be blocked
	allowed, err := rl.LimitByIP("192.168.1.1", 3, time.Minute)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("4th request should be blocked")
	}
}

func TestLimitByIP_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    2,
		WindowDuration: time.Minute,
	})

	// IP1 reaches limit
	rl.LimitByIP("192.168.1.1", 2, time.Minute)
	rl.LimitByIP("192.168.1.1", 2, time.Minute)
	allowed, _ := rl.LimitByIP("192.168.1.1", 2, time.Minute)
	if allowed {
		t.Error("IP1 should be blocked")
	}

	// IP2 should still be allowed (different IP)
	allowed, _ = rl.LimitByIP("192.168.1.2", 2, time.Minute)
	if !allowed {
		t.Error("IP2 should be allowed")
	}
}

func TestLimitByEmail(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    2,
		WindowDuration: time.Minute,
	})

	// First 2 requests should be allowed
	for i := 0; i < 2; i++ {
		allowed, err := rl.LimitByEmail("user@example.com", 2, time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 3rd request should be blocked
	allowed, err := rl.LimitByEmail("user@example.com", 2, time.Minute)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("3rd request should be blocked")
	}
}

func TestGetRemainingRequests(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    5,
		WindowDuration: time.Minute,
	})

	// Before any requests
	remaining := rl.GetRemainingRequests("192.168.1.1", 5, time.Minute)
	if remaining != 5 {
		t.Errorf("expected 5 remaining, got %d", remaining)
	}

	// After 2 requests
	rl.LimitByIP("192.168.1.1", 5, time.Minute)
	rl.LimitByIP("192.168.1.1", 5, time.Minute)
	remaining = rl.GetRemainingRequests("192.168.1.1", 5, time.Minute)
	if remaining != 3 {
		t.Errorf("expected 3 remaining, got %d", remaining)
	}
}

func TestGetResetTime(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    5,
		WindowDuration: time.Minute,
	})

	// Before any requests
	reset := rl.GetResetTime("192.168.1.1", time.Minute)
	if !reset.IsZero() {
		t.Error("reset time should be zero for unknown key")
	}

	// After a request
	rl.LimitByIP("192.168.1.1", 5, time.Minute)
	reset = rl.GetResetTime("192.168.1.1", time.Minute)
	if reset.IsZero() {
		t.Error("reset time should not be zero after request")
	}

	expectedReset := time.Now().Add(time.Minute)
	if reset.Sub(expectedReset).Abs() > time.Second {
		t.Errorf("reset time should be approximately now + 1 minute")
	}
}

func TestRateLimitByIP_MiddlewareAllowsRequests(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    5,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByIP(5, time.Minute)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRateLimitByIP_MiddlewareBlocksWhenExceeded(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    1,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByIP(1, time.Minute)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("first request should call next handler")
	}

	// Second request should be blocked
	nextCalled = false
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("second request should not call next handler")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

func TestRateLimitByIP_AddsHeaders(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    5,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByIP(5, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	limit := w.Header().Get("X-RateLimit-Limit")
	remaining := w.Header().Get("X-RateLimit-Remaining")
	reset := w.Header().Get("X-RateLimit-Reset")

	if limit == "" {
		t.Error("X-RateLimit-Limit header should be set")
	}
	if remaining == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	}
	if reset == "" {
		t.Error("X-RateLimit-Reset header should be set")
	}
}

func TestRateLimitByEmail_Middleware(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    2,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByEmail(2, time.Minute)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	// Request with email in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), UserEmailKey, "user@example.com")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRateLimitByEmail_BlocksExceeded(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    1,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByEmail(1, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	// First request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), UserEmailKey, "user@example.com")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Second request - should be blocked
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx = context.WithValue(req.Context(), UserEmailKey, "user@example.com")
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

func TestRateLimitWindowExpiration(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    2,
		WindowDuration: 100 * time.Millisecond,
	})

	// Exhaust the limit
	rl.LimitByIP("192.168.1.1", 2, 100*time.Millisecond)
	rl.LimitByIP("192.168.1.1", 2, 100*time.Millisecond)

	// Should be blocked
	allowed, _ := rl.LimitByIP("192.168.1.1", 2, 100*time.Millisecond)
	if allowed {
		t.Error("should be blocked before window expiration")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed after window expiration
	allowed, _ = rl.LimitByIP("192.168.1.1", 2, 100*time.Millisecond)
	if !allowed {
		t.Error("should be allowed after window expiration")
	}
}

func TestRateLimit_NoKeyAllowsRequest(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxRequests:    1,
		WindowDuration: time.Minute,
	})

	middleware := rl.RateLimitByIP(1, time.Minute)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Clear the RemoteAddr to simulate empty key
	req.RemoteAddr = ""
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should still call next handler (no key = allow)
	if !nextCalled {
		t.Error("next handler should be called when no key is available")
	}
}
