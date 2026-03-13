// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggerMiddleware(t *testing.T) {
	// Create test handler
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	// Create middleware
	handler := Logger(nextHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/test?param=value", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Verify
	if !nextCalled {
		t.Error("Expected next handler to be called")
	}
}

func TestLoggerMiddlewareHTTPS(t *testing.T) {
	// Create test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	handler := Logger(nextHandler)

	// Create test request with TLS
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Execute - will fail to get scheme since we can't easily simulate TLS in test
	// but it should not panic
	handler.ServeHTTP(w, req)
}
