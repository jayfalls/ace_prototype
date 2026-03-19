package telemetry

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceMiddleware returns middleware that extracts trace context from HTTP headers
// and adds it to the request context. This enables distributed tracing across
// service boundaries.
//
// The middleware extracts W3C Trace Context headers (traceparent, tracestate)
// and makes them available in the request context for downstream handlers.
//
// Usage with chi router:
//
//	r := chi.NewRouter()
//	r.Use(telemetry.TraceMiddleware())
//	r.Use(telemetry.MetricsMiddleware("my-service"))
//	r.Use(telemetry.LoggerMiddleware("my-service"))
func TraceMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from HTTP headers
			ctx := ExtractHTTP(r.Context(), r.Header)

			// Create a span for the HTTP request using the global tracer provider
			spanName := r.Method + " " + r.URL.Path
			ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, spanName)
			defer span.End()

			// Add span attributes
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.target", r.URL.Path),
			)

			// Update request with context containing trace
			r = r.WithContext(ctx)

			// Process request
			next.ServeHTTP(w, r)

			// Record status code
			if rw, ok := w.(interface{ StatusCode() int }); ok {
				span.SetAttributes(attribute.Int("http.status_code", rw.StatusCode()))
			}
		})
	}
}

// LoggerMiddleware returns middleware that logs HTTP requests using the structured logger.
// It logs request method, path, status code, duration, and client information.
//
// The logger includes:
// - timestamp
// - level (info for success, warn for 4xx, error for 5xx)
// - message
// - service_name
// - http.method
// - http.path
// - http.status_code
// - http.duration_ms
// - client.ip
// - trace_id (if available)
// - span_id (if available)
//
// Usage:
//
//	r := chi.NewRouter()
//	r.Use(telemetry.LoggerMiddleware("my-service"))
func LoggerMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get logger from context or create a basic one
			logger := getLoggerFromContext(r.Context(), serviceName)

			// Capture start time
			start := time.Now()

			// Create response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)
			durationMs := duration.Milliseconds()

			// Get trace context if available
			span := trace.SpanFromContext(r.Context())
			var traceID, spanID string
			if span.SpanContext().IsValid() {
				traceID = span.SpanContext().TraceID().String()
				spanID = span.SpanContext().SpanID().String()
			}

			// Determine log level based on status code
			level := zap.InfoLevel
			switch {
			case wrapped.statusCode >= 500:
				level = zap.ErrorLevel
			case wrapped.statusCode >= 400:
				level = zap.WarnLevel
			}

			// Build log fields
			fields := []zap.Field{
				zap.String("http.method", r.Method),
				zap.String("http.path", r.URL.Path),
				zap.Int("http.status_code", wrapped.statusCode),
				zap.Int64("http.duration_ms", durationMs),
				zap.String("client.ip", getClientIP(r)),
			}

			// Add trace context if available
			if traceID != "" {
				fields = append(fields, zap.String("trace_id", traceID))
			}
			if spanID != "" {
				fields = append(fields, zap.String("span_id", spanID))
			}

			// Log the request
			logger.Log(level, "HTTP request",
				fields...,
			)
		})
	}
}

// loggerCache caches loggers by service name to avoid creating new loggers on every request
var loggerCache = make(map[string]*zap.Logger)
var loggerCacheMu sync.RWMutex

// getLoggerFromContext attempts to get a logger from the context,
// or returns a cached logger for the given service name
func getLoggerFromContext(ctx context.Context, serviceName string) *zap.Logger {
	// Try to get logger from context first (future enhancement)
	// For now, use cached logger or create and cache one
	loggerCacheMu.RLock()
	if logger, ok := loggerCache[serviceName]; ok {
		loggerCacheMu.RUnlock()
		return logger
	}
	loggerCacheMu.RUnlock()

	// Create and cache the logger
	loggerCacheMu.Lock()
	defer loggerCacheMu.Unlock()

	// Double-check after acquiring write lock
	if logger, ok := loggerCache[serviceName]; ok {
		return logger
	}

	if logger, err := NewLogger(serviceName, "dev"); err == nil {
		loggerCache[serviceName] = logger
		return logger
	}

	// Return a no-op logger as last resort
	return zap.NewNop()
}

// getClientIP extracts the client IP from the request,
// handling proxies and load balancers
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (used by proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		for i, ip := range xff {
			if ip == ',' || ip == ' ' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header (common in nginx)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}
	return ip
}
