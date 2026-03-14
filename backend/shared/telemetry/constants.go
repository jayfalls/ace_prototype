package telemetry

// Trace context header keys
const (
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"
	BaggageHeader     = "baggage"
)

// Default OTLP endpoints
const (
	DefaultOTLPGRPCEndpoint     = "localhost:4317"
	DefaultOTLPHTTPEndpoint     = "localhost:4318"
	DefaultPrometheusEndpoint   = ":8888"
)
