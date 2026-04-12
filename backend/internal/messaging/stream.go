package messaging

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamConfig holds JetStream stream configuration.
type StreamConfig struct {
	// Name is the stream name.
	Name string
	// Description is the stream description.
	Description string
	// Subjects is the list of subjects the stream will receive.
	Subjects []string
	// Retention is the retention policy.
	Retention nats.RetentionPolicy
	// MaxBytes is the maximum bytes allowed in the stream.
	MaxBytes int64
	// MaxAge is the maximum age of messages in the stream.
	MaxAge time.Duration
	// Storage is the storage backend.
	Storage nats.StorageType
	// Replicas is the number of replicas.
	Replicas int
}

// StreamConfigs defines the stream configurations for ACE.
var StreamConfigs = []StreamConfig{
	{
		Name:        "COGNITIVE",
		Description: "Cognitive engine messages",
		Subjects: []string{
			"ace.engine.>",
			"ace.memory.>",
			"ace.tools.>",
			"ace.senses.>",
			"ace.llm.request",
			"ace.llm.response",
		},
		Retention: nats.LimitsPolicy,
		MaxBytes:  1 * 1024 * 1024 * 1024, // 1GB
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
		Replicas:  1,
	},
	{
		Name:        "USAGE",
		Description: "LLM usage events",
		Subjects: []string{
			"ace.usage.>",
		},
		Retention: nats.LimitsPolicy,
		MaxBytes:  100 * 1024 * 1024,   // 100MB
		MaxAge:    30 * 24 * time.Hour, // 30 days
		Storage:   nats.FileStorage,
		Replicas:  1,
	},
	{
		Name:        "SYSTEM",
		Description: "System events",
		Subjects: []string{
			"ace.system.>",
		},
		Retention: nats.WorkQueuePolicy,
		MaxBytes:  10 * 1024 * 1024, // 10MB
		Storage:   nats.MemoryStorage,
		Replicas:  1,
	},
}

// EnsureStreams creates or updates all configured streams idempotently.
// js should be a JetStreamManager (e.g., *nats.Conn or nats.JetStreamContext).
func EnsureStreams(ctx context.Context, js nats.JetStreamManager) error {
	for _, cfg := range StreamConfigs {
		streamCfg := &nats.StreamConfig{
			Name:        cfg.Name,
			Description: cfg.Description,
			Subjects:    cfg.Subjects,
			Retention:   cfg.Retention,
			MaxBytes:    cfg.MaxBytes,
			MaxAge:      cfg.MaxAge,
			Storage:     cfg.Storage,
			Replicas:    cfg.Replicas,
		}

		// Try to add or update the stream (idempotent operation)
		_, err := js.AddStream(streamCfg, nats.Context(ctx))
		if err != nil {
			return &MessagingError{
				Code:    "STREAM_CREATE_FAILED",
				Message: "failed to create stream " + cfg.Name + ": " + err.Error(),
				Err:     err,
			}
		}
	}
	return nil
}

// ConsumerConfig holds JetStream consumer configuration.
type ConsumerConfig struct {
	// Stream is the stream name.
	Stream string
	// Consumer is the consumer name.
	Consumer string
	// Durable is the durable consumer name.
	Durable string
	// DeliverSubject is the delivery subject for push consumer.
	DeliverSubject string
	// FilterSubject is the subject to filter messages.
	FilterSubject string
	// DeliverPolicy is the delivery policy.
	DeliverPolicy nats.DeliverPolicy
	// AckPolicy is the acknowledgment policy.
	AckPolicy nats.AckPolicy
	// AckWait is the acknowledgment wait time.
	AckWait time.Duration
	// MaxDeliver is the maximum delivery attempts.
	MaxDeliver int
	// QueueGroup is the queue group name for distributed delivery.
	QueueGroup string
}

// DefaultConsumerConfig returns default consumer configuration.
func DefaultConsumerConfig(stream, consumer, filterSubject string) ConsumerConfig {
	return ConsumerConfig{
		Stream:         stream,
		Consumer:       consumer,
		Durable:        consumer,
		DeliverSubject: consumer,
		FilterSubject:  filterSubject,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        30 * time.Second,
		MaxDeliver:     3,
		QueueGroup:     "",
	}
}

// CreateConsumer creates a durable consumer on a stream.
func CreateConsumer(ctx context.Context, js nats.JetStreamManager, cfg ConsumerConfig) error {
	consumerCfg := &nats.ConsumerConfig{
		Durable:        cfg.Durable,
		DeliverSubject: cfg.DeliverSubject,
		FilterSubject:  cfg.FilterSubject,
		DeliverPolicy:  cfg.DeliverPolicy,
		AckPolicy:      cfg.AckPolicy,
		AckWait:        cfg.AckWait,
		MaxDeliver:     cfg.MaxDeliver,
	}

	_, err := js.AddConsumer(cfg.Stream, consumerCfg, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "CONSUMER_CREATE_FAILED",
			Message: "failed to create consumer " + cfg.Consumer + ": " + err.Error(),
			Err:     err,
		}
	}
	return nil
}

// CreateConsumerWithQueueGroup creates a consumer with a queue group for horizontal scaling.
func CreateConsumerWithQueueGroup(ctx context.Context, js nats.JetStreamManager, cfg ConsumerConfig, queueGroup string) error {
	consumerCfg := &nats.ConsumerConfig{
		Durable:        cfg.Durable,
		DeliverSubject: cfg.DeliverSubject,
		FilterSubject:  cfg.FilterSubject,
		DeliverPolicy:  cfg.DeliverPolicy,
		AckPolicy:      cfg.AckPolicy,
		AckWait:        cfg.AckWait,
		MaxDeliver:     cfg.MaxDeliver,
		DeliverGroup:   queueGroup,
	}

	_, err := js.AddConsumer(cfg.Stream, consumerCfg, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "CONSUMER_CREATE_FAILED",
			Message: "failed to create consumer with queue group: " + err.Error(),
			Err:     err,
		}
	}
	return nil
}

// DLQConfig holds Dead Letter Queue configuration.
type DLQConfig struct {
	// StreamName is the main stream name.
	StreamName string
	// ConsumerName is the consumer name.
	ConsumerName string
	// FilterSubject is the subject to filter.
	FilterSubject string
	// MaxDeliver is the maximum delivery attempts before going to DLQ.
	MaxDeliver int
	// AckWait is the acknowledgment wait time.
	AckWait time.Duration
}

// EnsureDLQStream creates the DLQ stream if it doesn't exist.
func EnsureDLQStream(ctx context.Context, js nats.JetStreamManager) error {
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     "DLQ",
		Subjects: []string{"dlq.>"},
		Storage:  nats.FileStorage,
	}, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "DLQ_CREATE_FAILED",
			Message: "failed to create DLQ stream: " + err.Error(),
			Err:     err,
		}
	}
	return nil
}

// CreateConsumerWithDLQ creates a consumer that forwards failed messages to a DLQ.
func CreateConsumerWithDLQ(ctx context.Context, js nats.JetStreamManager, cfg DLQConfig) error {
	// First ensure the DLQ stream exists
	if err := EnsureDLQStream(ctx, js); err != nil {
		return err
	}

	// Create consumer on the main stream
	consumerCfg := &nats.ConsumerConfig{
		Durable:        cfg.ConsumerName,
		DeliverSubject: cfg.ConsumerName,
		FilterSubject:  cfg.FilterSubject,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        cfg.AckWait,
		MaxDeliver:     cfg.MaxDeliver,
	}

	_, err := js.AddConsumer(cfg.StreamName, consumerCfg, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "CONSUMER_CREATE_FAILED",
			Message: "failed to create consumer with DLQ: " + err.Error(),
			Err:     err,
		}
	}

	// Create a consumer on the DLQ stream to receive the dead letters
	dlqSubject := "dlq." + cfg.FilterSubject
	dlqConsumerCfg := &nats.ConsumerConfig{
		Durable:        "dlq-" + cfg.ConsumerName,
		DeliverSubject: dlqSubject,
		FilterSubject:  dlqSubject,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	}

	_, err = js.AddConsumer("DLQ", dlqConsumerCfg, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "DLQ_CONSUMER_CREATE_FAILED",
			Message: "failed to create DLQ consumer: " + err.Error(),
			Err:     err,
		}
	}

	return nil
}

// GetStreamInfo returns information about a stream.
func GetStreamInfo(ctx context.Context, js nats.JetStreamManager, streamName string) (*nats.StreamInfo, error) {
	info, err := js.StreamInfo(streamName, nats.Context(ctx))
	if err != nil {
		return nil, &MessagingError{
			Code:    "STREAM_INFO_FAILED",
			Message: "failed to get stream info: " + err.Error(),
			Err:     err,
		}
	}
	return info, nil
}

// DeleteStream deletes a stream.
func DeleteStream(ctx context.Context, js nats.JetStreamManager, streamName string) error {
	err := js.DeleteStream(streamName, nats.Context(ctx))
	if err != nil {
		return &MessagingError{
			Code:    "STREAM_DELETE_FAILED",
			Message: "failed to delete stream: " + err.Error(),
			Err:     err,
		}
	}
	return nil
}
