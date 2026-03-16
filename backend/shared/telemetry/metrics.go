package telemetry

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric"
)

// Standard metric names
const (
	MetricHTTPRequestDuration = "http_request_duration_seconds"
	MetricHTTPRequestsTotal   = "http_requests_total"
	MetricHTTPActiveRequests  = "http_active_requests"

	// NATS metrics
	MetricNATSMessagesPublished = "nats_messages_published_total"
	MetricNATSMessagesConsumed  = "nats_messages_consumed_total"
)

// Label names (low cardinality only)
const (
	LabelServiceName = "service_name"
	LabelMethod      = "method"
	LabelPath        = "path"
	LabelStatusCode  = "status_code"
	LabelNATSService = "nats_service" // Service name for NATS metrics
	LabelNATSSubject = "nats_subject" // Subject pattern (e.g., "ace.usage.event")
)

// UUID regex pattern (matches standard UUID format)
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Numeric pattern (matches pure numeric segments)
var numericPattern = regexp.MustCompile(`^[0-9]+$`)

// Alphanumeric ID pattern (matches alphanumeric strings 8+ chars, common in REST APIs)
var alphanumericIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8,}$`)

// globalMetrics holds the singleton metrics instance
var (
	globalMetrics     *Metrics
	globalMetricsOnce sync.Once
)

// Metrics holds all standard metrics
type Metrics struct {
	requestDuration       *prometheus.HistogramVec
	requestsTotal         *prometheus.CounterVec
	activeRequests        *prometheus.GaugeVec
	natsMessagesPublished *prometheus.CounterVec
	natsMessagesConsumed  *prometheus.CounterVec
	registry              *prometheus.Registry
	meter                 metric.Meter // OTel meter for potential future use
}

// getGlobalMetrics returns the singleton metrics instance
func getGlobalMetrics() *Metrics {
	globalMetricsOnce.Do(func() {
		globalMetrics = newMetrics(nil)
	})
	return globalMetrics
}

// NewMetrics creates a new Metrics instance with its own registry
// This allows for test isolation and multiple metric sets if needed
func NewMetrics() *Metrics {
	return newMetrics(prometheus.NewRegistry())
}

// newMetrics creates and registers the standard metrics using a provided registry
func newMetrics(reg *prometheus.Registry) *Metrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}

	metrics := &Metrics{
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
		natsMessagesPublished: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricNATSMessagesPublished,
				Help: "Total number of NATS messages published",
			},
			[]string{LabelNATSService, LabelNATSSubject},
		),
		natsMessagesConsumed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricNATSMessagesConsumed,
				Help: "Total number of NATS messages consumed",
			},
			[]string{LabelNATSService, LabelNATSSubject},
		),
		registry: reg,
		meter:    nil,
	}

	// Register metrics with the registry
	reg.MustRegister(metrics.requestDuration)
	reg.MustRegister(metrics.requestsTotal)
	reg.MustRegister(metrics.activeRequests)
	reg.MustRegister(metrics.natsMessagesPublished)
	reg.MustRegister(metrics.natsMessagesConsumed)

	return metrics
}

// Registry returns the Prometheus registry for this Metrics instance
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// NATS helper methods for recording message counts

// RecordNATSPublish records a NATS message publish
func (m *Metrics) RecordNATSPublish(serviceName, subject string) {
	m.natsMessagesPublished.WithLabelValues(serviceName, normalizeNATSSubject(subject)).Inc()
}

// RecordNATSConsume records a NATS message consumption
func (m *Metrics) RecordNATSConsume(serviceName, subject string) {
	m.natsMessagesConsumed.WithLabelValues(serviceName, normalizeNATSSubject(subject)).Inc()
}

// normalizeNATSSubject normalizes NATS subject patterns for low cardinality
// Returns the subject pattern with dynamic segments replaced by placeholders
func normalizeNATSSubject(subject string) string {
	// For now, return the subject as-is since NATS subjects are typically
	// well-structured and not high cardinality like HTTP paths
	// In the future, we could add pattern matching here if needed
	if len(subject) > 100 {
		return subject[:100]
	}
	return subject
}

// SetMeter sets the OTel meter for the metrics (optional, for future integration)
func (m *Metrics) SetMeter(meter metric.Meter) {
	m.meter = meter
}

// MetricsMiddleware returns middleware that instruments HTTP requests with metrics
func MetricsMiddleware(serviceName string) func(http.Handler) http.Handler {
	metrics := getGlobalMetrics()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics and health endpoints to avoid recursion
			// Health check paths: /health, /health/live, /health/ready
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" ||
				r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" {
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

// isUUID checks if a string is a UUID
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// isNumeric checks if a string is purely numeric
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// isAlphanumericID checks if a string looks like an alphanumeric ID (e.g., nanoid)
func isAlphanumericID(s string) bool {
	return alphanumericIDPattern.MatchString(s)
}

// getPathLabel returns a normalized path label
// This ensures low cardinality by replacing dynamic segments with placeholders
func getPathLabel(path string) string {
	// Handle empty and root paths
	if path == "" || path == "/" {
		return "root"
	}

	// Truncate very long paths
	if len(path) > 100 {
		path = path[:100]
	}

	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue // Skip empty segments
		}
		// Replace UUIDs, numeric IDs, and alphanumeric IDs with placeholder
		if isUUID(seg) || isNumeric(seg) || isAlphanumericID(seg) {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
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
		metrics:     getGlobalMetrics(),
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
