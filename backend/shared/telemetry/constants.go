package telemetry

// Trace context header keys (W3C standard)
const (
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"
	BaggageHeader     = "baggage"
)

// NATS subject constants
const (
	SubjectUsageEvent = "ace.usage.event"
)
