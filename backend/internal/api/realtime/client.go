package realtime

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"ace/internal/api/model"
)

// clientMetrics holds metrics instruments for client operations.
type clientMetrics struct {
	messagesSent     metric.Int64Counter
	messagesReceived metric.Int64Counter
	wsErrors         metric.Int64Counter
}

// rateLimiter tracks message timestamps for rate limiting per connection.
type rateLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	limit      int
	window     time.Duration
}

// newRateLimiter creates a rate limiter with the given limit per window.
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		timestamps: make([]time.Time, 0, limit),
		limit:      limit,
		window:     window,
	}
}

// Allow returns true if a new message is allowed under the rate limit.
func (rl *rateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove timestamps outside the window
	var valid []time.Time
	for _, ts := range rl.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	rl.timestamps = valid

	if len(rl.timestamps) >= rl.limit {
		return false
	}

	rl.timestamps = append(rl.timestamps, now)
	return true
}

// Client represents a single WebSocket connection.
type Client struct {
	id          string
	userID      string
	role        model.UserRole
	conn        *websocket.Conn
	topics      map[string]struct{}
	send        chan []byte
	hub         *Hub
	connectedAt time.Time
	done        chan struct{}

	// Rate limiting
	msgRateLimiter *rateLimiter

	// Metrics
	metrics *clientMetrics
}

// NewClient creates a new Client for the given WebSocket connection.
func NewClient(conn *websocket.Conn, userID string, role model.UserRole, hub *Hub) *Client {
	return &Client{
		id:             uuid.New().String(),
		userID:         userID,
		role:           role,
		conn:           conn,
		topics:         make(map[string]struct{}),
		send:           make(chan []byte, WS_SEND_CHANNEL_SIZE),
		hub:            hub,
		connectedAt:    time.Now(),
		done:           make(chan struct{}),
		msgRateLimiter: newRateLimiter(WS_RATE_LIMIT, time.Second),
	}
}

// SetMetrics sets the metrics instruments for this client.
func (c *Client) SetMetrics(m *clientMetrics) {
	c.metrics = m
}

// Send marshals msg to JSON and queues it for delivery. Non-blocking: drops if channel full.
func (c *Client) Send(msg ServerMessage) {
	data := msg.Marshal()
	select {
	case c.send <- data:
		if c.metrics != nil {
			c.metrics.messagesSent.Add(context.Background(), 1, metric.WithAttributes(
				attribute.String("user_id", c.userID),
				attribute.String("connection_id", c.id),
			))
		}
	default:
		c.hub.logger.Warn("send channel full, dropping message",
			zap.String("connection_id", c.id),
			zap.String("user_id", c.userID),
			zap.String("msg_type", string(msg.Type)),
		)
	}
}

// AllowMessage checks if a message from this client is within rate limits.
// Returns true if allowed, false if rate limited.
func (c *Client) AllowMessage() bool {
	if c.msgRateLimiter == nil {
		return true
	}
	return c.msgRateLimiter.Allow()
}

// writePump reads from the send channel and writes to the WebSocket until done.
func (c *Client) writePump(ctx context.Context) {
	defer close(c.done)
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
				c.hub.logger.Debug("write error",
					zap.String("connection_id", c.id),
					zap.String("user_id", c.userID),
					zap.Error(err),
				)
				if c.metrics != nil {
					c.metrics.wsErrors.Add(ctx, 1, metric.WithAttributes(
						attribute.String("user_id", c.userID),
						attribute.String("connection_id", c.id),
					))
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// readPump reads messages from the WebSocket and dispatches them until the connection closes.
func (c *Client) readPump(ctx context.Context) {
	defer c.hub.Unregister(c)
	for {
		var msg ClientMessage
		if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
			if ctx.Err() == nil {
				c.hub.logger.Debug("read error",
					zap.String("connection_id", c.id),
					zap.String("user_id", c.userID),
					zap.Error(err),
				)
				if c.metrics != nil {
					c.metrics.wsErrors.Add(ctx, 1, metric.WithAttributes(
						attribute.String("user_id", c.userID),
						attribute.String("connection_id", c.id),
					))
				}
			}
			return
		}

		// Check rate limit
		if !c.AllowMessage() {
			c.hub.logger.Warn("rate limit exceeded",
				zap.String("connection_id", c.id),
				zap.String("user_id", c.userID),
			)
			if c.metrics != nil {
				c.metrics.wsErrors.Add(ctx, 1, metric.WithAttributes(
					attribute.String("user_id", c.userID),
					attribute.String("connection_id", c.id),
					attribute.String("error_type", "rate_limit"),
				))
			}
			continue
		}

		// Record message received
		if c.metrics != nil {
			c.metrics.messagesReceived.Add(ctx, 1, metric.WithAttributes(
				attribute.String("user_id", c.userID),
				attribute.String("connection_id", c.id),
			))
		}

		c.handleMessage(ctx, msg)
	}
}

// handleMessage routes an incoming client message to the appropriate Hub operation.
func (c *Client) handleMessage(ctx context.Context, msg ClientMessage) {
	switch msg.Type {
	case ClientMessageSubscribe:
		c.hub.Subscribe(c, msg.Topics)
	case ClientMessageUnsubscribe:
		c.hub.Unsubscribe(c, msg.Topics)
	case ClientMessageReplay:
		c.hub.Replay(c, msg.Topic, msg.SinceSeq)
	case ClientMessagePing:
		c.Send(NewPongMessage())
	case ClientMessageAuth:
		// Token refresh on existing connection — no reconnect needed.
		// Hub re-validates; on failure it closes the connection.
		c.hub.RefreshAuth(c, msg.Token)
	}
}
