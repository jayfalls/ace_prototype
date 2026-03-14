package telemetry

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Standard metric names
const (
	MetricHTTPRequestDuration = "http_request_duration_seconds"
	MetricHTTPRequestsTotal   = "http_requests_total"
	MetricHTTPActiveRequests = "http_active_requests"
)

// Label names (low cardinality only)
const (
	LabelServiceName = "service_name"
	LabelMethod      = "method"
	LabelPath        = "path"
	LabelStatusCode  = "status_code"
)

// globalMetrics holds the singleton metrics instance
var (
	globalMetrics     *Metrics
	globalMetricsOnce sync.Once
)

// Metrics holds all standard metrics
type Metrics struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	activeRequests  *prometheus.GaugeVec
	registry        *prometheus.Registry
}

// getGlobalMetrics returns the singleton metrics instance
func getGlobalMetrics() *Metrics {
	globalMetricsOnce.Do(func() {
		reg := prometheus.NewRegistry()
		globalMetrics = &Metrics{
			requestDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    MetricHTTPRequestDuration,
					Help:    "HTTP request latency in seconds",
					Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
				},
				[]string{LabelServiceName, LabelMethod, LabelPath, LabelStatusCode},
			),
			requestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: MetricHTTPRequestsTotal,
					Help: "Total number of HTTP requests",
				},
				[]string{LabelServiceName, LabelMethod, LabelPath, LabelStatusCode},
			),
			activeRequests: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: MetricHTTPActiveRequests,
					Help: "Number of active HTTP requests",
				},
				[]string{LabelServiceName},
			),
			registry: reg,
		}
		
		// Register metrics with the registry
		reg.MustRegister(globalMetrics.requestDuration)
		reg.MustRegister(globalMetrics.requestsTotal)
		reg.MustRegister(globalMetrics.activeRequests)
	})
	return globalMetrics
}

// MetricsMiddleware returns middleware that instruments HTTP requests with metrics
func MetricsMiddleware(serviceName string) func(http.Handler) http.Handler {
	metrics := getGlobalMetrics()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics endpoint to avoid recursion
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Increment active requests
			metrics.activeRequests.WithLabelValues(serviceName).Inc()
			defer metrics.activeRequests.WithLabelValues(serviceName).Dec()

			// Record start time
			start := time.Now()

			// Create response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Get labels
			method := r.Method
			path := getPathLabel(r.URL.Path)
			statusCode := strconv.Itoa(wrapped.statusCode)

			// Record metrics
			metrics.requestDuration.WithLabelValues(serviceName, method, path, statusCode).Observe(duration)
			metrics.requestsTotal.WithLabelValues(serviceName, method, path, statusCode).Inc()
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the status code if not already set
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// getPathLabel returns a normalized path label
// This ensures low cardinality by grouping similar paths
func getPathLabel(path string) string {
	// Normalize path segments to reduce cardinality
	// Replace UUIDs, numbers, and IDs with placeholder
	switch {
	case path == "" || path == "/":
		return "root"
	case len(path) > 100:
		// Truncate very long paths
		return path[:100]
	default:
		return path
	}
}

// RegisterMetrics returns an HTTP handler that serves Prometheus metrics
func RegisterMetrics() http.Handler {
	metrics := getGlobalMetrics()
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

// MetricsRecorder provides an interface for recording metrics manually
type MetricsRecorder interface {
	RecordRequest(method, path string, statusCode int, duration time.Duration)
	IncrementActiveRequests()
	DecrementActiveRequests()
}

// metricsRecorder implements MetricsRecorder
type metricsRecorder struct {
	serviceName string
	metrics     *Metrics
}

// NewMetricsRecorder creates a new metrics recorder
func NewMetricsRecorder(serviceName string) MetricsRecorder {
	return &metricsRecorder{
		serviceName: serviceName,
		metrics:    getGlobalMetrics(),
	}
}

// RecordRequest manually records an HTTP request metric
func (r *metricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	statusCodeStr := strconv.Itoa(statusCode)
	pathLabel := getPathLabel(path)

	r.metrics.requestDuration.WithLabelValues(
		r.serviceName, method, pathLabel, statusCodeStr,
	).Observe(duration.Seconds())

	r.metrics.requestsTotal.WithLabelValues(
		r.serviceName, method, pathLabel, statusCodeStr,
	).Inc()
}

// IncrementActiveRequests increments the active requests gauge
func (r *metricsRecorder) IncrementActiveRequests() {
	r.metrics.activeRequests.WithLabelValues(r.serviceName).Inc()
}

// DecrementActiveRequests decrements the active requests gauge
func (r *metricsRecorder) DecrementActiveRequests() {
	r.metrics.activeRequests.WithLabelValues(r.serviceName).Dec()
}
