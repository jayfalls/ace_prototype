package telemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HealthCheck returns an error if the OTLP exporter connection is down
func HealthCheck() error {
	if globalTraceProvider == nil {
		return ErrTracerNotInitialized
	}

	// Create a context with timeout to check exporter connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to force a flush of any pending spans
	// If the exporter is down, this will fail with a timeout or connection error
	if err := globalTraceProvider.ForceFlush(ctx); err != nil {
		return errors.Join(ErrExporterConnectionDown, err)
	}

	return nil
}

// ExtractHTTP extracts trace context from HTTP headers
func ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	// Get the global propagator
	propagator := otel.GetTextMapPropagator()

	// Create a MapCarrier from the HTTP headers
	carrier := make(propagation.MapCarrier)
	for key, values := range headers {
		if len(values) > 0 {
			carrier[key] = values[0]
		}
	}

	// Extract the trace context from HTTP headers
	return propagator.Extract(ctx, carrier)
}

// ExtractHTTPWithHeaders extracts trace context from a map of header strings
func ExtractHTTPWithHeaders(ctx context.Context, headers map[string][]string) context.Context {
	propagator := otel.GetTextMapPropagator()
	carrier := make(propagation.MapCarrier)
	for key, values := range headers {
		if len(values) > 0 {
			carrier[key] = values[0]
		}
	}
	return propagator.Extract(ctx, carrier)
}

// SpanFromContext returns the span from the context if it exists
func SpanFromContext(ctx context.Context) (trace.Span, bool) {
	return trace.SpanFromContext(ctx), trace.SpanFromContext(ctx).SpanContext().IsValid()
}

// AddSpanAttributes adds standard span attributes to a span
func AddSpanAttributes(span trace.Span, attrs SpanAttributes) {
	if attrs.AgentID != "" {
		span.SetAttributes(attribute.String("agent_id", attrs.AgentID))
	}
	if attrs.CycleID != "" {
		span.SetAttributes(attribute.String("cycle_id", attrs.CycleID))
	}
	if attrs.ServiceName != "" {
		span.SetAttributes(attribute.String("service_name", attrs.ServiceName))
	}
}
