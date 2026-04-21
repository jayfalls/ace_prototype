package realtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"ace/internal/api/model"
	"ace/internal/telemetry"
)

// maxTopicsPerClient is the max topics per connection (used when config not available).
const maxTopicsPerClient = WS_MAX_SUBSCRIPTIONS

// Hub manages all active WebSocket clients and routes NATS events to them.
type Hub struct {
	mu sync.RWMutex
	// clients maps userID → active connections for that user
	clients map[string][]*Client

	topics *TopicReg
	nats   *nats.Conn
	buffer *SeqBuffer
	logger *zap.Logger
	meter  metric.Meter

	// Metrics instruments
	connectionsActive    metric.Int64UpDownCounter
	messagesSent         metric.Int64Counter
	messagesReceived     metric.Int64Counter
	wsErrors             metric.Int64Counter
	pollRequests         metric.Int64Counter
	pollEventsDelivered  metric.Int64Counter
	bufferReplayEvents   metric.Int64Counter
	bufferResyncRequired metric.Int64Counter

	// Usage publisher for connection time events
	usagePublisher *telemetry.UsagePublisher
}

// NewHub creates a Hub wired to the given NATS connection.
func NewHub(natsConn *nats.Conn, logger *zap.Logger, meter metric.Meter) *Hub {
	h := &Hub{
		clients: make(map[string][]*Client),
		nats:    natsConn,
		buffer:  NewSeqBuffer(DefaultSeqBufferConfig()),
		logger:  logger,
		meter:   meter,
	}
	h.topics = NewTopicReg(natsConn, h.dispatchNATSEvent, logger)

	// Initialize metrics instruments
	h.connectionsActive, _ = meter.Int64UpDownCounter(MetricWSConnectionsActive)
	h.messagesSent, _ = meter.Int64Counter(MetricWSMessagesSent)
	h.messagesReceived, _ = meter.Int64Counter(MetricWSMessagesReceived)
	h.wsErrors, _ = meter.Int64Counter(MetricWSErrors)
	h.pollRequests, _ = meter.Int64Counter(MetricPollRequests)
	h.pollEventsDelivered, _ = meter.Int64Counter(MetricPollEventsDelivered)
	h.bufferReplayEvents, _ = meter.Int64Counter(MetricBufferReplayEvents)
	h.bufferResyncRequired, _ = meter.Int64Counter(MetricBufferResyncRequired)

	return h
}

// SetUsagePublisher sets the usage event publisher for connection time tracking.
func (h *Hub) SetUsagePublisher(publisher *telemetry.UsagePublisher) {
	h.usagePublisher = publisher
}

// Register adds a client to the hub and sends auth_ok.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c.userID] = append(h.clients[c.userID], c)
	h.mu.Unlock()

	c.Send(NewAuthOkMessage(c.id))
	h.logger.Info("client registered",
		zap.String("connection_id", c.id),
		zap.String("user_id", c.userID),
		zap.String("role", string(c.role)),
	)

	// Increment active connections metric
	h.connectionsActive.Add(context.Background(), 1)
}

// Unregister removes a client, cleans up its topic subscriptions, and closes its send channel.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	conns := h.clients[c.userID]
	filtered := conns[:0]
	for _, conn := range conns {
		if conn != c {
			filtered = append(filtered, conn)
		}
	}
	if len(filtered) == 0 {
		delete(h.clients, c.userID)
	} else {
		h.clients[c.userID] = filtered
	}
	h.mu.Unlock()

	durationMs := time.Since(c.connectedAt).Milliseconds()
	topicCount := len(c.topics)

	// Remove all topic refs held by this client.
	for topic := range c.topics {
		if err := h.topics.Remove(topic); err != nil {
			h.logger.Warn("topic remove on unregister",
				zap.String("topic", topic),
				zap.Error(err),
			)
		}
	}

	close(c.send)
	h.logger.Info("client unregistered",
		zap.String("connection_id", c.id),
		zap.String("user_id", c.userID),
		zap.Int64("duration_ms", durationMs),
		zap.Int("topics_count", topicCount),
	)

	// Decrement active connections metric
	h.connectionsActive.Add(context.Background(), -1)

	// Emit usage event for connection time
	if h.usagePublisher != nil {
		go func() {
			ctx := context.Background()
			_ = h.usagePublisher.Publish(ctx, telemetry.UsageEvent{
				OperationType: telemetry.OperationTypeNATSPublish,
				ResourceType:  telemetry.ResourceTypeMessaging,
				DurationMs:    durationMs,
				Metadata: map[string]string{
					"user_id":       c.userID,
					"connection_id": c.id,
					"transport":     "websocket",
					"topics_count":  fmt.Sprintf("%d", topicCount),
				},
			})
		}()
	}
}

// Subscribe adds topics to a client and creates NATS subscriptions as needed.
func (h *Hub) Subscribe(c *Client, topics []string) {
	if len(c.topics)+len(topics) > maxTopicsPerClient {
		c.Send(NewErrorMessage(fmt.Sprintf("max %d topics per connection", maxTopicsPerClient)))
		return
	}

	var added []string
	for _, topic := range topics {
		if err := ValidateTopic(topic); err != nil {
			c.Send(NewErrorMessage(fmt.Sprintf("invalid topic %q", topic)))
			continue
		}
		if !h.isAuthorized(c.userID, c.role, topic) {
			c.Send(NewErrorMessage(fmt.Sprintf("not authorized for topic %q", topic)))
			continue
		}
		if _, already := c.topics[topic]; already {
			added = append(added, topic)
			continue
		}
		if err := h.topics.Add(topic); err != nil {
			c.Send(NewErrorMessage(fmt.Sprintf("cannot subscribe to %q: %s", topic, err)))
			continue
		}
		c.topics[topic] = struct{}{}
		added = append(added, topic)
	}

	if len(added) > 0 {
		c.Send(NewSubscribedMessage(added))
	}
}

// Unsubscribe removes topics from a client and cleans up NATS subscriptions.
func (h *Hub) Unsubscribe(c *Client, topics []string) {
	var removed []string
	for _, topic := range topics {
		if _, ok := c.topics[topic]; !ok {
			continue
		}
		delete(c.topics, topic)
		if err := h.topics.Remove(topic); err != nil {
			h.logger.Warn("topic remove on unsubscribe",
				zap.String("topic", topic),
				zap.Error(err),
			)
		}
		removed = append(removed, topic)
	}
	if len(removed) > 0 {
		c.Send(NewUnsubscribedMessage(removed))
	}
}

// Replay sends buffered events for a topic since sinceSeq to the client.
func (h *Hub) Replay(c *Client, topic string, sinceSeq uint64) {
	if !h.isAuthorized(c.userID, c.role, topic) {
		c.Send(NewErrorMessage(fmt.Sprintf("not authorized for topic %q", topic)))
		return
	}
	entries, err := h.buffer.Replay(topic, sinceSeq)
	if err != nil {
		c.Send(NewResyncRequiredMessage([]string{topic}))
		h.bufferResyncRequired.Add(context.Background(), 1)
		return
	}
	for _, e := range entries {
		c.Send(NewEventMessage(topic, e.Seq, "replay", e.Data))
		h.bufferReplayEvents.Add(context.Background(), 1)
	}
}

// RefreshAuth updates the client's token. Currently a no-op placeholder for Slice 4 JWT validation.
func (h *Hub) RefreshAuth(c *Client, _ string) {
	// Token refresh logic wired in Slice 4 when tokenService is available.
	c.Send(NewAuthOkMessage(c.id))
}

// PollEntry is a buffered event with its topic attached, for polling responses.
type PollEntry struct {
	Topic string
	SeqEntry
}

// PollResult is the return value of PollEvents.
type PollResult struct {
	Events         []PollEntry
	ResyncRequired []string
}

// PollEvents returns buffered events for the requested topics since sinceSeq.
// Topics where the buffer was exceeded are returned in the resync list.
func (h *Hub) PollEvents(userID string, role model.UserRole, topics []string, sinceSeq uint64) PollResult {
	var result PollResult
	for _, topic := range topics {
		if !h.isAuthorized(userID, role, topic) {
			continue
		}
		entries, err := h.buffer.Replay(topic, sinceSeq)
		if err != nil {
			result.ResyncRequired = append(result.ResyncRequired, topic)
			continue
		}
		for _, e := range entries {
			result.Events = append(result.Events, PollEntry{Topic: topic, SeqEntry: e})
		}
	}
	return result
}

// dispatchNATSEvent is the callback called by TopicReg when a NATS message arrives.
// It sequences the event and fans out to all authorized subscribers.
func (h *Hub) dispatchNATSEvent(topic string, data []byte) {
	seq := h.buffer.GetLastSeq(topic) + 1
	h.buffer.Append(topic, seq, data)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for _, c := range conns {
			if _, subscribed := c.topics[topic]; !subscribed {
				continue
			}
			if !h.isAuthorized(c.userID, c.role, topic) {
				continue
			}
			c.Send(NewEventMessage(topic, seq, "event", data))
		}
	}
}

// Close drains all client connections and unsubscribes all NATS subscriptions.
func (h *Hub) Close() error {
	h.mu.Lock()
	// Snapshot clients to close outside the lock.
	var all []*Client
	for _, conns := range h.clients {
		all = append(all, conns...)
	}
	h.mu.Unlock()

	for _, c := range all {
		c.conn.Close(1000, "server shutdown")
	}

	return h.topics.Close()
}

// isAuthorized returns true if the user may receive events for the given topic.
// Admins see everything. Regular users may only see their own agent/usage topics.
func (h *Hub) isAuthorized(userID string, role model.UserRole, topic string) bool {
	if role == model.RoleAdmin {
		return true
	}
	// system:health is visible to all authenticated users.
	if topic == "system:health" {
		return true
	}
	parts := strings.SplitN(topic, ":", 3)
	if len(parts) < 2 {
		return false
	}
	resourceID := parts[1]
	return resourceID == userID
}

// IsRunning returns true if the hub has active clients.
func (h *Hub) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients) > 0 || h.topics != nil
}

// NATSConnected returns true if the NATS connection is alive.
func (h *Hub) NATSConnected() bool {
	if h.nats == nil {
		return false
	}
	return !h.nats.IsClosed()
}
