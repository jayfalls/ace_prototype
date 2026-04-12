package telemetry

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestNATSCarrier_Set(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}

	carrier := NATSCarrier{msg: msg}
	carrier.Set("test-key", "test-value")

	assert.Equal(t, "test-value", msg.Header.Get("test-key"))
}

func TestNATSCarrier_SetNilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	carrier.Set("test-key", "test-value")
}

func TestNATSCarrier_SetNilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	carrier.Set("test-key", "test-value")
}

func TestNATSCarrier_Get(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}
	msg.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	msg.Header.Set("tracestate", "vendor=custom")

	carrier := NATSCarrier{msg: msg}

	assert.Equal(t, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", carrier.Get("traceparent"))
	assert.Equal(t, "vendor=custom", carrier.Get("tracestate"))
	assert.Equal(t, "", carrier.Get("nonexistent"))
}

func TestNATSCarrier_GetNilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	assert.Equal(t, "", carrier.Get("test-key"))
}

func TestNATSCarrier_GetNilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	assert.Equal(t, "", carrier.Get("test-key"))
}

func TestNATSCarrier_Keys(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}
	msg.Header.Set("key1", "value1")
	msg.Header.Set("key2", "value2")
	msg.Header.Set("key3", "value3")

	carrier := NATSCarrier{msg: msg}
	keys := carrier.Keys()

	assert.Len(t, keys, 3)
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	assert.True(t, keySet["key1"])
	assert.True(t, keySet["key2"])
	assert.True(t, keySet["key3"])
}

func TestNATSCarrier_KeysNilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	assert.Nil(t, carrier.Keys())
}

func TestNATSCarrier_KeysNilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	assert.Nil(t, carrier.Keys())
}

func TestInjectTraceContext(t *testing.T) {
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, _ := Init(ctx, config)

	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}

	if telemetry != nil {
		ctx, span := telemetry.Tracer.Start(ctx, "test-inject-span")
		defer span.End()

		InjectTraceContext(ctx, msg)

		traceparent := msg.Header.Get("traceparent")
		assert.NotEmpty(t, traceparent)
		assert.Contains(t, traceparent, "00-")

		_ = telemetry.Shutdown(ctx)
	} else {
		t.Log("Telemetry init returned nil, skipping inject verification")
	}
}

func TestExtractTraceContext(t *testing.T) {
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, _ := Init(ctx, config)

	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}
	msg.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	msg.Header.Set("tracestate", "vendor=custom")
	msg.Header.Set("baggage", "key1=value1,key2=value2")

	newCtx := ExtractTraceContext(ctx, msg)

	span := trace.SpanFromContext(newCtx)
	assert.True(t, span.SpanContext().IsValid())

	if telemetry != nil {
		_ = telemetry.Shutdown(ctx)
	}
}

func TestExtractTraceContextNilMsg(t *testing.T) {
	ctx := context.Background()
	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nil,
	}

	newCtx := ExtractTraceContext(ctx, msg)
	assert.NotNil(t, newCtx)
}

func TestExtractTraceContextEmptyHeaders(t *testing.T) {
	ctx := context.Background()

	msg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}

	newCtx := ExtractTraceContext(ctx, msg)
	assert.NotNil(t, newCtx)
}

func TestInjectAndExtractRoundTrip(t *testing.T) {
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)
	if err != nil || telemetry == nil {
		t.Skip("Telemetry initialization failed, skipping round-trip test")
	}

	originalMsg := &nats.Msg{
		Subject: "test.subject",
		Header:  nats.Header{},
	}

	ctx, span := telemetry.Tracer.Start(ctx, "round-trip-test")
	defer span.End()

	originalSpanContext := span.SpanContext()
	traceID := originalSpanContext.TraceID().String()

	InjectTraceContext(ctx, originalMsg)

	receivedMsg := &nats.Msg{
		Subject: "received.subject",
		Header:  originalMsg.Header,
	}

	extractedCtx := ExtractTraceContext(context.Background(), receivedMsg)

	extractedSpan := trace.SpanFromContext(extractedCtx)
	extractedTraceID := extractedSpan.SpanContext().TraceID().String()

	assert.Equal(t, traceID, extractedTraceID)

	_ = telemetry.Shutdown(ctx)
}
