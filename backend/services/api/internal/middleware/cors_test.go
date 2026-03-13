// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	// Test with allowed origins
	allowedOrigins := []string{"http://localhost:5173", "https://example.com"}

	// Create test handler
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	handler := CORS(allowedOrigins)(nextHandler)

	// Test request with allowed origin
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify
	if !nextCalled {
		t.Error("Expected next handler to be called")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("Expected Access-Control-Allow-Origin header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	// Test with allowed origins
	allowedOrigins := []string{"http://localhost:5173"}

	// Create test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	handler := CORS(allowedOrigins)(nextHandler)

	// Test request with disallowed origin
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify - should not set CORS headers for disallowed origin
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no Access-Control-Allow-Origin header for disallowed origin")
	}
}

func TestCORSWildcard(t *testing.T) {
	// Test with wildcard
	allowedOrigins := []string{"*"}

	// Create test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	handler := CORS(allowedOrigins)(nextHandler)

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify - should set wildcard
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin *, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	// Test with allowed origins
	allowedOrigins := []string{"http://localhost:5173"}

	// Create test handler
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	// Create middleware
	handler := CORS(allowedOrigins)(nextHandler)

	// Test OPTIONS preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify - should return 204 No Content
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
	if nextCalled {
		t.Error("Expected next handler NOT to be called for preflight")
	}
}

func TestCORSAllowedMethods(t *testing.T) {
	allowedOrigins := []string{"*"}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(allowedOrigins)(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify allowed methods header
	expectedMethods := "GET, POST, PUT, DELETE, OPTIONS, PATCH"
	if w.Header().Get("Access-Control-Allow-Methods") != expectedMethods {
		t.Errorf("Expected %s, got %s", expectedMethods, w.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSAllowedHeaders(t *testing.T) {
	allowedOrigins := []string{"*"}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(allowedOrigins)(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify allowed headers header
	expectedHeaders := "Content-Type, Authorization, X-Requested-With, X-Request-ID"
	if w.Header().Get("Access-Control-Allow-Headers") != expectedHeaders {
		t.Errorf("Expected %s, got %s", expectedHeaders, w.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSExposeHeaders(t *testing.T) {
	allowedOrigins := []string{"*"}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(allowedOrigins)(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify expose headers header
	expectedHeaders := "Content-Length, Content-Type, X-Request-ID"
	if w.Header().Get("Access-Control-Expose-Headers") != expectedHeaders {
		t.Errorf("Expected %s, got %s", expectedHeaders, w.Header().Get("Access-Control-Expose-Headers"))
	}
}
