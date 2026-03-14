package telemetry

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/propagation"
)

// NATSCarrier implements the TextMapPropagator interface for NATS messages
type NATSCarrier struct {
	msg *nats.Msg
}

// Set sets a header value in the NATS message
func (c NATSCarrier) Set(key, value string) {
	if c.msg.Header == nil {
		c.msg.Header = nats.Header{}
	}
	c.msg.Header.Set(key, value)
}

// Get gets a header value from the NATS message
func (c NATSCarrier) Get(key string) string {
	if c.msg.Header == nil {
		return ""
	}
	return c.msg.Header.Get(key)
}

// Keys returns all header keys from the NATS message
func (c NATSCarrier) Keys() []string {
	if c.msg.Header == nil {
		return nil
	}
	keys := make([]string, 0, len(c.msg.Header))
	for key := range c.msg.Header {
		keys = append(keys, key)
	}
	return keys
}

// globalPropagator is the composite propagator for W3C trace context and baggage
var globalPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

// InjectTraceContext injects the current trace context into a NATS message
func InjectTraceContext(ctx context.Context, msg *nats.Msg) {
	globalPropagator.Inject(ctx, NATSCarrier{msg: msg})
}

// ExtractTraceContext extracts trace context from a NATS message
func ExtractTraceContext(ctx context.Context, msg *nats.Msg) context.Context {
	return globalPropagator.Extract(ctx, NATSCarrier{msg: msg})
}
