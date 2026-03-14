package messaging

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
)

// DefaultRequestTimeout is the default timeout for request-reply operations.
const DefaultRequestTimeout = 30 * time.Second

// Publish sends a message without waiting for response.
// It creates an envelope with correlation ID, agent ID, cycle ID, and source service,
// then sets envelope fields as NATS headers before publishing.
func Publish(client Client, subject, correlationID, agentID, cycleID, sourceService string, payload []byte) error {
	env := NewEnvelope(
		correlationID, // preserve correlation chain
		agentID,
		cycleID,
		sourceService,
	)

	headers := make(nats.Header)
	SetHeadersToMsg(headers, env)

	return client.Publish(subject, payload, headers)
}

// PublishWithSubject sends a message using a Subject type with variadic formatting.
// This is a convenience function that handles subject interpolation.
func PublishWithSubject(client Client, subject Subject, correlationID, agentID, cycleID, sourceService string, payload []byte, args ...interface{}) error {
	formattedSubject := subject.Format(args...)
	return Publish(client, formattedSubject, correlationID, agentID, cycleID, sourceService, payload)
}

// RequestReply sends a message and waits for response with timeout.
// It creates an envelope with correlation ID propagation to maintain the chain,
// then sends the request and returns the response data.
func RequestReply(client Client, subject, correlationID, agentID, cycleID, sourceService string, payload []byte, timeout time.Duration) ([]byte, error) {
	env := NewEnvelope(
		correlationID, // preserve correlation chain
		agentID,
		cycleID,
		sourceService,
	)

	headers := make(nats.Header)
	SetHeadersToMsg(headers, env)

	reply, err := client.Request(subject, payload, timeout)
	if err != nil {
		return nil, err
	}

	return reply.Data, nil
}

// RequestReplyWithSubject sends a request-reply using a Subject type with variadic formatting.
func RequestReplyWithSubject(client Client, subject Subject, correlationID, agentID, cycleID, sourceService string, payload []byte, timeout time.Duration, args ...interface{}) ([]byte, error) {
	formattedSubject := subject.Format(args...)
	return RequestReply(client, formattedSubject, correlationID, agentID, cycleID, sourceService, payload, timeout)
}

// RequestReplyDefault sends a request with the default timeout.
func RequestReplyDefault(client Client, subject, correlationID, agentID, cycleID, sourceService string, payload []byte) ([]byte, error) {
	return RequestReply(client, subject, correlationID, agentID, cycleID, sourceService, payload, DefaultRequestTimeout)
}

// SetHeadersToMsg sets envelope fields as NATS headers on a message.
// This is a convenience function that works with a header map instead of nats.Msg.
func SetHeadersToMsg(headers nats.Header, env *Envelope) {
	if headers == nil || env == nil {
		return
	}

	headers.Set(HeaderMessageID, env.MessageID)

	if env.CorrelationID != "" {
		headers.Set(HeaderCorrelationID, env.CorrelationID)
	}

	if env.AgentID != "" {
		headers.Set(HeaderAgentID, env.AgentID)
	}

	if env.CycleID != "" {
		headers.Set(HeaderCycleID, env.CycleID)
	}

	headers.Set(HeaderSourceService, env.SourceService)
	headers.Set(HeaderTimestamp, env.Timestamp.Format(time.RFC3339Nano))
	headers.Set(HeaderSchemaVersion, env.SchemaVersion)
}

// Subscribe creates a simple subscription with a handler.
func Subscribe(client Client, subject string, handler func(*nats.Msg) error) (Subscription, error) {
	wrappedHandler := func(msg *nats.Msg) {
		if err := handler(msg); err != nil {
			// Negative ack the message if handler returns error
			msg.Nak()
		} else {
			msg.Ack()
		}
	}

	return client.Subscribe(subject, wrappedHandler)
}

// SubscribeWithEnvelope creates a subscription that parses the envelope from headers.
// The handler receives both the parsed envelope and the raw message data.
func SubscribeWithEnvelope(client Client, subject string, handler func(*Envelope, []byte) error) (Subscription, error) {
	wrappedHandler := func(msg *nats.Msg) {
		env, err := EnvelopeFromHeaders(msg)
		if err != nil {
			msg.Nak()
			return
		}

		if err := handler(env, msg.Data); err != nil {
			msg.Nak()
		} else {
			msg.Ack()
		}
	}

	return client.Subscribe(subject, wrappedHandler)
}

// SubscribeToStream creates a JetStream push consumer subscription.
// It sets up a durable consumer with the given stream, consumer name, and subject.
func SubscribeToStream(ctx context.Context, client Client, stream, consumer, subject string, handler func(*nats.Msg) error) error {
	wrappedHandler := func(msg *nats.Msg) {
		if err := handler(msg); err != nil {
			msg.Nak()
		} else {
			msg.Ack()
		}
	}

	return client.SubscribeToStream(ctx, stream, consumer, subject, wrappedHandler)
}

// SubscribeToStreamWithEnvelope creates a JetStream subscription that parses the envelope.
func SubscribeToStreamWithEnvelope(ctx context.Context, client Client, stream, consumer, subject string, handler func(*Envelope, []byte) error) error {
	wrappedHandler := func(msg *nats.Msg) {
		env, err := EnvelopeFromHeaders(msg)
		if err != nil {
			msg.Nak()
			return
		}

		if err := handler(env, msg.Data); err != nil {
			msg.Nak()
		} else {
			msg.Ack()
		}
	}

	return client.SubscribeToStream(ctx, stream, consumer, subject, wrappedHandler)
}

// SubscribeToStreamWithSubject creates a JetStream subscription using a Subject type.
func SubscribeToStreamWithSubject(ctx context.Context, client Client, stream, consumer string, subject Subject, handler func(*nats.Msg) error, args ...interface{}) error {
	formattedSubject := subject.Format(args...)
	return SubscribeToStream(ctx, client, stream, consumer, formattedSubject, handler)
}

// SubscribeToStreamWithEnvelopeAndSubject creates a JetStream subscription with envelope parsing using a Subject type.
func SubscribeToStreamWithEnvelopeAndSubject(ctx context.Context, client Client, stream, consumer string, subject Subject, handler func(*Envelope, []byte) error, args ...interface{}) error {
	formattedSubject := subject.Format(args...)
	return SubscribeToStreamWithEnvelope(ctx, client, stream, consumer, formattedSubject, handler)
}

// StreamSubscriptionConfig holds configuration for stream subscriptions.
type StreamSubscriptionConfig struct {
	Stream        string
	Consumer      string
	Subject       string
	Handler       func(*nats.Msg) error
	AutoAck       bool
	MaxAckWait    time.Duration
	MaxDeliver    int
}

// SubscribeToStreamWithConfig creates a JetStream subscription with detailed configuration.
func SubscribeToStreamWithConfig(ctx context.Context, client Client, cfg StreamSubscriptionConfig) error {
	wrappedHandler := func(msg *nats.Msg) {
		if cfg.AutoAck {
			if err := cfg.Handler(msg); err != nil {
				msg.Nak()
			} else {
				msg.Ack()
			}
		} else {
			// Don't auto-ack, caller is responsible
			_ = cfg.Handler(msg)
		}
	}

	return client.SubscribeToStream(ctx, cfg.Stream, cfg.Consumer, cfg.Subject, wrappedHandler)
}

// CreateRequestEnvelope creates an envelope for request-reply pattern.
// It uses the incoming message's correlation ID if available, or generates a new one.
func CreateRequestEnvelope(incoming *nats.Msg, agentID, cycleID, sourceService string) *Envelope {
	var correlationID string

	if incoming != nil {
		// Try to get correlation ID from incoming message
		if env, err := EnvelopeFromHeaders(incoming); err == nil && env != nil {
			correlationID = env.CorrelationID
		}
		// Fall back to header directly
		if correlationID == "" && incoming.Header != nil {
			correlationID = incoming.Header.Get(HeaderCorrelationID)
		}
	}

	// Generate new correlation ID if not found
	if correlationID == "" {
		correlationID = GenerateMessageID()
	}

	return NewEnvelope(
		correlationID,
		agentID,
		cycleID,
		sourceService,
	)
}

// ReplyTo sends a reply message in response to an incoming request.
// It preserves the correlation ID from the incoming message.
func ReplyTo(client Client, incoming *nats.Msg, payload []byte) error {
	if incoming == nil {
		return &MessagingError{
			Code:    "INVALID_MESSAGE",
			Message: "incoming message is nil",
		}
	}

	// Get correlation ID from incoming message
	correlationID := ""
	if incoming.Header != nil {
		correlationID = incoming.Header.Get(HeaderCorrelationID)
	}

	// Extract source service from original message or use a default
	sourceService := "unknown"
	if incoming.Header != nil {
		if ss := incoming.Header.Get(HeaderSourceService); ss != "" {
			sourceService = ss + "-response"
		}
	}

	env := NewEnvelope(
		correlationID,
		"", // No agent ID for replies
		"", // No cycle ID for replies
		sourceService,
	)

	headers := make(nats.Header)
	SetHeadersToMsg(headers, env)

	// Reply to the reply subject if available
	replySubject := incoming.Reply
	if replySubject == "" {
		return &MessagingError{
			Code:    "INVALID_MESSAGE",
			Message: "no reply subject in incoming message",
		}
	}

	return client.Publish(replySubject, payload, headers)
}

// ForwardMessage forwards a received message to a new subject while preserving envelope.
func ForwardMessage(client Client, incoming *nats.Msg, newSubject string) error {
	if incoming == nil {
		return &MessagingError{
			Code:    "INVALID_MESSAGE",
			Message: "incoming message is nil",
		}
	}

	env, err := EnvelopeFromHeaders(incoming)
	if err != nil {
		return err
	}

	headers := make(nats.Header)
	SetHeadersToMsg(headers, env)

	return client.Publish(newSubject, incoming.Data, headers)
}
