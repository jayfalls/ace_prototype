package telemetry

import (
	"encoding/json"
	"errors"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ErrNATSNotConnected is returned when NATS is not connected
var ErrNATSNotConnected = errors.New("nats: connection not available")

// globalTraceProvider holds the trace provider for health checking
var globalTraceProvider *sdktrace.TracerProvider

// SetGlobalTraceProvider sets the global trace provider for health checking
func SetGlobalTraceProvider(tp *sdktrace.TracerProvider) {
	globalTraceProvider = providerForHealthCheck(tp)
}

// providerForHealthCheck wraps the tracer provider to provide health check functionality
func providerForHealthCheck(tp *sdktrace.TracerProvider) *sdktrace.TracerProvider {
	return tp
}

// ErrTracerNotInitialized is returned when the tracer has not been initialized
var ErrTracerNotInitialized = errors.New("tracer: not initialized")

// ErrExporterConnectionDown is returned when the OTLP exporter cannot connect
var ErrExporterConnectionDown = errors.New("tracer: exporter connection down")

// SpanAttributes defines the mandatory span attributes for agent work
type SpanAttributes struct {
	AgentID     string
	CycleID     string
	ServiceName string
}

// MarshalJSON implements custom JSON marshaling for SpanAttributes
func (s SpanAttributes) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"agent_id":     s.AgentID,
		"cycle_id":     s.CycleID,
		"service_name": s.ServiceName,
	})
}
