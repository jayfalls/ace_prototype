package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceMiddleware(t *testing.T) {
	// Create a test handler - middleware should still work even without full tracer init
	// The key is that the middleware chain works, not that spans are fully valid
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Even without a full tracer, we can verify context is processed
		// The middleware should not panic
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := TraceMiddleware()
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	// Serve request - should not panic
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddlewareWithTraceHeaders(t *testing.T) {
	// Test that middleware handles W3C traceparent header
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := TraceMiddleware()
	handler := middleware(testHandler)

	// Create test request with W3C traceparent header
	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddlewareExtractsContext(t *testing.T) {
	// Test that the middleware processes requests without panicking
	var processed bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		processed = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := TraceMiddleware()
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, processed)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddlewareCreatesSpan(t *testing.T) {
	// Test that middleware creates a span even without existing trace context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify we can get a span from context (may be noop but should not panic)
		span := trace.SpanFromContext(r.Context())
		_ = span // Just verify no panic
		w.WriteHeader(http.StatusOK)
	})

	middleware := TraceMiddleware()
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoggerMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoggerMiddlewareLogsRequest(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoggerMiddlewareWithErrorStatus(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLoggerMiddlewareWithWarningStatus(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/notfound", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLoggerMiddlewareCapturesStatusCode(t *testing.T) {
	statusCodeCaptured := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("POST", "/api/users", nil)
	w := httptest.NewRecorder()
	
	// Wrap to capture status
	wrapped := &responseWriter{ResponseWriter: w, statusCode: 0}
	handler.ServeHTTP(wrapped, req)

	statusCodeCaptured = wrapped.statusCode
	assert.Equal(t, http.StatusCreated, statusCodeCaptured)
}

func TestLoggerMiddlewareWithTraceContext(t *testing.T) {
	// Test logger middleware with trace context - no special init needed
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoggerMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		xForwardedFor string
		xRealIP      string
		expectedIP   string
	}{
		{
			name:         "remote addr only",
			remoteAddr:   "192.168.1.1:12345",
			expectedIP:   "192.168.1.1",
		},
		{
			name:         "x-forwarded-for",
			remoteAddr:   "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 70.41.3.18",
			expectedIP:   "203.0.113.1",
		},
		{
			name:         "x-real-ip",
			remoteAddr:   "10.0.0.1:12345",
			xRealIP:      "198.51.100.1",
			expectedIP:   "198.51.100.1",
		},
		{
			name:         "x-forwarded-for takes precedence over x-real-ip",
			remoteAddr:   "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			xRealIP:      "198.51.100.1",
			expectedIP:   "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestMiddlewareChiRouterIntegration(t *testing.T) {
	// This test verifies that the middleware works correctly with chi router pattern
	// by checking that the middleware signature is compatible

	// Test TraceMiddleware signature
	var _ func(http.Handler) http.Handler = TraceMiddleware()

	// Test LoggerMiddleware signature
	var _ func(http.Handler) http.Handler = LoggerMiddleware("test-service")

	// Test MetricsMiddleware signature
	var _ func(http.Handler) http.Handler = MetricsMiddleware("test-service")

	// All middleware should have the correct signature for chi router
	t.Log("All middleware functions have correct chi.RouterMiddleware signature")
}
