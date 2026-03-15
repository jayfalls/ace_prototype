package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

// UsageConsumer consumes usage events from NATS and persists them to PostgreSQL
type UsageConsumer struct {
	sub  *nats.Subscription
	pool *pgxpool.Pool
	nc   *nats.Conn
}

// NewUsageConsumer creates a new usage event consumer
func NewUsageConsumer(nc *nats.Conn, pool *pgxpool.Pool) *UsageConsumer {
	return &UsageConsumer{
		pool: pool,
		nc:   nc,
	}
}

// Start begins consuming messages from the NATS usage event subject
func (c *UsageConsumer) Start(ctx context.Context) error {
	if c.nc == nil {
		return ErrNATSNotConnected
	}

	if c.pool == nil {
		return ErrDatabaseNotConnected
	}

	sub, err := c.nc.Subscribe(SubjectUsageEvent, c.handleMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", SubjectUsageEvent, err)
	}

	c.sub = sub

	// Start a goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

// handleMessage processes a single NATS message containing a usage event
func (c *UsageConsumer) handleMessage(msg *nats.Msg) {
	// Extract trace context from the message
	ctx := ExtractTraceContext(context.Background(), msg)

	// Parse the usage event from JSON
	var event UsageEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		// Log error but don't crash - malformed messages shouldn't stop the consumer
		// In production, this would go to a dead letter queue
		return
	}

	// Insert into database
	if err := c.insertUsageEvent(ctx, event); err != nil {
		// Log error - in production, implement retry logic or dead letter queue
		return
	}

	// Acknowledge the message
	msg.Ack()
}

// insertUsageEvent inserts a usage event into the PostgreSQL database
func (c *UsageConsumer) insertUsageEvent(ctx context.Context, event UsageEvent) error {
	if c.pool == nil {
		return ErrDatabaseNotConnected
	}

	// Convert map to JSONB
	metadataJSON := "{}"
	if event.Metadata != nil && len(event.Metadata) > 0 {
		metadataBytes, err := json.Marshal(event.Metadata)
		if err != nil {
			metadataJSON = "{}"
		} else {
			metadataJSON = string(metadataBytes)
		}
	}

	// Use timestamp from event or current time if not set
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	// Insert the usage event
	_, err := c.pool.Exec(ctx, `
		INSERT INTO usage_events (
			timestamp,
			agent_id,
			cycle_id,
			session_id,
			service_name,
			operation_type,
			resource_type,
			cost_usd,
			duration_ms,
			token_count,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		timestamp,
		event.AgentID,
		event.CycleID,
		event.SessionID,
		event.ServiceName,
		event.OperationType,
		event.ResourceType,
		event.CostUSD,
		event.DurationMs,
		event.TokenCount,
		metadataJSON,
	)

	return err
}

// Stop stops the consumer and closes the subscription
func (c *UsageConsumer) Stop() {
	if c.sub != nil {
		c.sub.Unsubscribe()
		c.sub = nil
	}
}

// ErrDatabaseNotConnected is returned when the database pool is not available
var ErrDatabaseNotConnected = fmt.Errorf("database: connection pool not available")
