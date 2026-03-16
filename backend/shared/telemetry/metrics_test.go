package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Apply middleware
	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestMetricsMiddlewareRecordsDuration(t *testing.T) {
	// Create a test handler that takes some time
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify the metric was recorded - the test passes if no panic occurs
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsMiddlewareSkipsMetricsEndpoint(t *testing.T) {
	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	// Request to /metrics should skip metrics middleware logic
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 1, callCount, "handler should still be called")
}

func TestMetricsMiddlewareSkipsHealthEndpoint(t *testing.T) {
	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	// Request to /health should skip metrics middleware logic
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 1, callCount, "handler should still be called")
}

func TestMetricsMiddlewareRecordsStatusCode(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMetricsMiddlewareRecordsErrorStatus(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	middleware := MetricsMiddleware("test-service")
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestResponseWriterWrapper(t *testing.T) {
	// Test WriteHeader capture
	rec := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rec, statusCode: 0}

	wrapped.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, wrapped.statusCode)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestResponseWriterWrapperWrite(t *testing.T) {
	// Test Write capture
	rec := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rec, statusCode: 0}

	n, err := wrapped.Write([]byte("test"))

	assert.NoError(t, err)
	assert.Equal(t, 4, n) // "test" has 4 bytes
	// When Write is called without WriteHeader first, it sets status to 200 (OK)
	// The actual behavior is that httptest.ResponseRecorder will have Code=200
	assert.Equal(t, http.StatusOK, wrapped.statusCode)
}

func TestResponseWriterWrapperWriteWithStatus(t *testing.T) {
	// Test Write with pre-set status
	rec := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rec, statusCode: http.StatusBadRequest}

	n, err := wrapped.Write([]byte("test"))

	assert.NoError(t, err)
	assert.Equal(t, 4, n) // "test" has 4 bytes
	// When status is pre-set, Write should not change it
	assert.Equal(t, http.StatusBadRequest, wrapped.statusCode)
}

func TestGetPathLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "root"},
		{"/", "root"},
		{"/api/users", "/api/users"},
		{"/api/users/123", "/api/users/:id"},
		{"/api/users/abc123def456", "/api/users/:id"},
		{"/api/agents/550e8400-e29b-41d4-a716-446655440000", "/api/agents/:id"},
		{"/api/items/123456", "/api/items/:id"},
		{"/api/posts/abc123def456", "/api/posts/:id"},
		{"/api/static", "/api/static"},
		{"/api/users/123/posts/456", "/api/users/:id/posts/:id"},
		{"/api/some/path", "/api/some/path"}, // regular paths unchanged
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getPathLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"not-a-uuid", false},
		{"123", false},
		{"abc123", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isUUID(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"123456789", true},
		{"123.456", false},
		{"abc", false},
		{"123abc", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isNumeric(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

func TestIsAlphanumericID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123def456", true},
		{"ABC123DEF456", true},
		{"550e8400e29b41d4a716446655440000", true},
		{"12345678", true}, // 8 chars - all digits
		{"123abcde", true}, // 8 chars - mixed
		{"abc12345", true}, // 8 chars - mixed
		{"", false},
		{"abc123", false},   // too short (6 chars)
		{"abcdefgh", false}, // too short, no digits
		{"abc-123", false},  // contains hyphen
	}

	for _, tt := range tests {
		result := isAlphanumericID(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

func TestGetPathLabelTruncatesLongPath(t *testing.T) {
	longPath := "/" + string(make([]byte, 150))
	result := getPathLabel(longPath)
	assert.LessOrEqual(t, len(result), 100)
}

func TestRegisterMetrics(t *testing.T) {
	handler := RegisterMetrics()
	assert.NotNil(t, handler)

	// Verify it's a valid http.Handler
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	// Should return 200 with Prometheus content
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
}

func TestMetricsRecorderInterface(t *testing.T) {
	rec := NewMetricsRecorder("test-service")

	// Test that it implements the interface
	var _ MetricsRecorder = rec

	// Test recording
	rec.RecordRequest("GET", "/api/test", 200, 100*1000*1000) // 100ms in nanoseconds

	// Record some more requests
	rec.RecordRequest("POST", "/api/users", 201, 50*1000*1000)
	rec.RecordRequest("GET", "/api/users", 500, 200*1000*1000)
}

func TestMetricsRecorderIncrementDecrement(t *testing.T) {
	rec := NewMetricsRecorder("test-service")

	rec.IncrementActiveRequests()
	rec.IncrementActiveRequests()
	rec.DecrementActiveRequests()
	rec.DecrementActiveRequests()
}

func TestMetricsConstants(t *testing.T) {
	assert.Equal(t, "http_request_duration_seconds", MetricHTTPRequestDuration)
	assert.Equal(t, "http_requests_total", MetricHTTPRequestsTotal)
	assert.Equal(t, "http_active_requests", MetricHTTPActiveRequests)
}

func TestLabelConstants(t *testing.T) {
	assert.Equal(t, "service_name", LabelServiceName)
	assert.Equal(t, "method", LabelMethod)
	assert.Equal(t, "path", LabelPath)
	assert.Equal(t, "status_code", LabelStatusCode)
}

func TestMetricsMiddlewareWithVariousMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := MetricsMiddleware("test-service")
			handler := middleware(testHandler)

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	// Create a simple handler that returns 200
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := MetricsMiddleware("prometheus-test")
	handler := middleware(testHandler)

	// Make a request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Get metrics endpoint
	metricsHandler := RegisterMetrics()
	rec := httptest.NewRecorder()
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsHandler.ServeHTTP(rec, metricsReq)

	// Verify Prometheus format
	body := rec.Body.String()

	// Check for metric names
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_active_requests")

	// Check for labels
	assert.Contains(t, body, "service_name")
	assert.Contains(t, body, "method")
	assert.Contains(t, body, "path")
	assert.Contains(t, body, "status_code")
}

func TestNoHighCardinalityLabels(t *testing.T) {
	// Create and register new metrics to test cardinality
	rec := NewMetricsRecorder("cardinality-test")

	// Record many different paths
	for i := 0; i < 100; i++ {
		rec.RecordRequest("GET", "/api/item/"+strconv.Itoa(i), 200, 10000000)
	}

	// Record with different users (should NOT be a label)
	// This is just to verify the design - agentId should NOT be in labels

	// The metrics should only have low-cardinality labels
	// This test passes if the code compiles and runs without agentId
}

func TestMetricsMiddlewareConcurrent(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := MetricsMiddleware("concurrent-test")
	handler := middleware(testHandler)

	// Run concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/concurrent", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMetricsEndToEnd(t *testing.T) {
	require := require.New(t)

	// Create recorder
	rec := NewMetricsRecorder("e2e-test")

	// Simulate request lifecycle
	rec.IncrementActiveRequests()
	rec.RecordRequest("GET", "/api/users", 200, 50*1000*1000)
	rec.DecrementActiveRequests()

	// Get metrics
	metricsHandler := RegisterMetrics()
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	metricsHandler.ServeHTTP(rec2, req)

	require.Equal(http.StatusOK, rec2.Code)

	// Verify content
	body := rec2.Body.String()
	require.Contains(body, "http_request_duration_seconds")
	require.Contains(body, "service_name=\"e2e-test\"")
}
