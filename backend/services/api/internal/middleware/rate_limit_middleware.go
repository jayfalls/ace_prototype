// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality using an in-memory map.
// This is a simple implementation that can be replaced with Valkey later.
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*clientWindow
	config   RateLimitConfig
}

// RateLimitConfig holds the configuration for rate limiting.
type RateLimitConfig struct {
	MaxRequests    int           // Maximum requests allowed per window
	WindowDuration time.Duration // Time window for rate limiting
}

// clientWindow tracks request counts for a specific client within a time window.
type clientWindow struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a new RateLimiter with the given configuration.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string]*clientWindow),
		config:   config,
	}
}

// LimitByIP checks if the IP address has exceeded the rate limit.
// Returns true if allowed, false if limit exceeded.
// The key should be the client's IP address.
func (rl *RateLimiter) LimitByIP(key string, maxRequests int, window time.Duration) (bool, error) {
	return rl.checkLimit(key, maxRequests, window)
}

// LimitByEmail checks if the email has exceeded the rate limit.
// Returns true if allowed, false if limit exceeded.
// The key should be the user's email address.
func (rl *RateLimiter) LimitByEmail(email string, maxRequests int, window time.Duration) (bool, error) {
	return rl.checkLimit(email, maxRequests, window)
}

// checkLimit is the internal method that performs the rate limit check.
func (rl *RateLimiter) checkLimit(key string, maxRequests int, window time.Duration) (bool, error) {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.requests[key]
	if !exists {
		// First request from this client
		rl.requests[key] = &clientWindow{
			count:       1,
			windowStart: now,
		}
		return true, nil
	}

	// Check if the window has expired
	if now.Sub(client.windowStart) >= window {
		// Reset the window
		client.count = 1
		client.windowStart = now
		return true, nil
	}

	// Increment the count and check against limit
	client.count++

	if client.count > maxRequests {
		return false, nil
	}

	return true, nil
}

// GetRemainingRequests returns the number of remaining requests for a key.
// Returns -1 if the key is not found.
func (rl *RateLimiter) GetRemainingRequests(key string, maxRequests int, window time.Duration) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	client, exists := rl.requests[key]
	if !exists {
		return maxRequests
	}

	now := time.Now()
	if now.Sub(client.windowStart) >= window {
		return maxRequests
	}

	remaining := maxRequests - client.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetResetTime returns the time when the rate limit will reset for a key.
// Returns zero time if the key is not found.
func (rl *RateLimiter) GetResetTime(key string, window time.Duration) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	client, exists := rl.requests[key]
	if !exists {
		return time.Time{}
	}

	return client.windowStart.Add(window)
}

// Middleware returns a rate limiting middleware with the specified configuration.
// It adds rate limit headers to responses:
//   - X-RateLimit-Limit: Maximum requests allowed
//   - X-RateLimit-Remaining: Remaining requests in the current window
//   - X-RateLimit-Reset: Unix timestamp when the rate limit resets
//
// Returns 429 Too Many Requests when the limit is exceeded.
func (rl *RateLimiter) Middleware(maxRequests int, window time.Duration, keyFunc func(*http.Request) string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				// If no key, allow the request
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := rl.checkLimit(key, maxRequests, window)
			if err != nil {
				// On error, allow the request but log
				next.ServeHTTP(w, r)
				return
			}

			remaining := rl.GetRemainingRequests(key, maxRequests, window)
			resetTime := rl.GetResetTime(key, window)

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			if !resetTime.IsZero() {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
			}

			if !allowed {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByIP returns a middleware that rate limits by client IP address.
func (rl *RateLimiter) RateLimitByIP(maxRequests int, window time.Duration) func(next http.Handler) http.Handler {
	return rl.Middleware(maxRequests, window, func(r *http.Request) string {
		// Get client IP, preferring X-Forwarded-For if behind a proxy
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}
		// Take the first IP if there are multiple
		if idx := len(ip) - 1; idx > 0 {
			if commaIdx := indexOf(ip, ","); commaIdx > 0 {
				ip = ip[:commaIdx]
			}
		}
		return ip
	})
}

// RateLimitByEmail returns a middleware that rate limits by user email.
// This middleware expects the email to be in the request context under UserEmailKey.
// Use this after the auth middleware.
func (rl *RateLimiter) RateLimitByEmail(maxRequests int, window time.Duration) func(next http.Handler) http.Handler {
	return rl.Middleware(maxRequests, window, func(r *http.Request) string {
		return GetUserEmailFromContext(r.Context())
	})
}

// indexOf returns the index of the first occurrence of sep in s, or -1 if not found.
func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
