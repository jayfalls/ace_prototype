package realtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"ace/internal/messaging"
)

// natsClientAdapter wraps *nats.Conn to implement messaging.Client interface
// for use with SubscribeWithEnvelope.
type natsClientAdapter struct {
	conn *nats.Conn
	reg  *TopicReg
}

// Subscribe implements messaging.Client.Subscribe
func (a *natsClientAdapter) Subscribe(subject string, handler messaging.MsgHandler) (messaging.Subscription, error) {
	sub, err := a.conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg)
	})
	if err != nil {
		return nil, err
	}
	return &subscriptionAdapter{sub: sub, reg: a.reg, subject: subject}, nil
}

// Publish implements messaging.Client.Publish
func (a *natsClientAdapter) Publish(subject string, data []byte, headers nats.Header) error {
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  headers,
	}
	return a.conn.PublishMsg(msg)
}

// Request implements messaging.Client.Request
func (a *natsClientAdapter) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return a.conn.Request(subject, data, timeout)
}

// SubscribeToStream implements messaging.Client.SubscribeToStream (not used in TopicReg but required by interface)
func (a *natsClientAdapter) SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler messaging.MsgHandler) error {
	return fmt.Errorf("not implemented")
}

// HealthCheck implements messaging.Client.HealthCheck (not used in TopicReg but required by interface)
func (a *natsClientAdapter) HealthCheck() error {
	return nil
}

// Drain implements messaging.Client.Drain (not used in TopicReg but required by interface)
func (a *natsClientAdapter) Drain() error {
	return nil
}

// Close implements messaging.Client.Close (not used in TopicReg but required by interface)
func (a *natsClientAdapter) Close() {
}

// subscriptionAdapter wraps *nats.Subscription to implement messaging.Subscription
type subscriptionAdapter struct {
	sub     *nats.Subscription
	reg     *TopicReg
	subject string
}

func (s *subscriptionAdapter) Unsubscribe() error {
	return s.sub.Unsubscribe()
}

// TopicReg manages NATS subscriptions with reference counting.
// Multiple clients can subscribe to the same topic, sharing a single
// NATS subscription until all clients unsubscribe.
type TopicReg struct {
	mu sync.Mutex

	// refs tracks reference counts per topic
	refs map[string]int

	// subs tracks active NATS subscriptions per topic
	subs map[string]*nats.Subscription

	// topicToSubject maps public topic strings to NATS subject patterns
	topicToSubject map[string]string

	// subjectToTopic maps NATS subjects back to public topics (reverse lookup)
	subjectToTopic map[string]string

	// nats is the NATS connection
	nats *nats.Conn

	// natsClient is the adapter for the NATS connection
	natsClient *natsClientAdapter

	// dispatch is the callback for NATS events (topic string, data []byte)
	dispatch func(topic string, data []byte)

	// logger for structured logging
	logger *zap.Logger
}

// NewTopicReg creates a new TopicReg with the given NATS connection,
// dispatch callback, and logger.
func NewTopicReg(natsConn *nats.Conn, dispatch func(topic string, data []byte), logger *zap.Logger) *TopicReg {
	topicToSubject := map[string]string{
		"agent:{id}:status": "ace.engine.{id}.layer.>",
		"agent:{id}:logs":   "ace.engine.{id}.loop.>",
		"agent:{id}:cycles": "ace.engine.{id}.layer.6.output",
		"system:health":     "ace.system.health.>",
		"usage:{id}":        "ace.usage.{id}.>",
	}

	subjectToTopic := make(map[string]string)
	for topic, subject := range topicToSubject {
		subjectToTopic[subject] = topic
	}

	topicReg := &TopicReg{
		refs:           make(map[string]int),
		subs:           make(map[string]*nats.Subscription),
		topicToSubject: topicToSubject,
		subjectToTopic: subjectToTopic,
		nats:           natsConn,
		dispatch:       dispatch,
		logger:         logger,
	}
	topicReg.natsClient = &natsClientAdapter{conn: natsConn, reg: topicReg}

	return topicReg
}

// Add subscribes to a topic, creating a NATS subscription if this is the first
// reference. Returns an error if the topic format is invalid.
func (t *TopicReg) Add(topic string) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}

	subject, err := t.topicToSubjectFunc(topic)
	if err != nil {
		return fmt.Errorf("unsupported topic: %s", topic)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.refs[topic]++

	if t.refs[topic] == 1 {
		// First reference - create NATS subscription
		sub, err := t.subscribeNATS(subject, topic)
		if err != nil {
			t.refs[topic]--
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		t.subs[topic] = sub
		t.logger.Info("NATS subscription created",
			zap.String("topic", topic),
			zap.String("subject", subject),
		)
	}

	return nil
}

// Remove unsubscribes from a topic when the reference count reaches zero.
func (t *TopicReg) Remove(topic string) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.refs[topic] == 0 {
		return fmt.Errorf("topic %q is not subscribed", topic)
	}

	t.refs[topic]--

	if t.refs[topic] == 0 {
		// Last reference - unsubscribe from NATS
		if err := t.unsubscribeNATS(topic); err != nil {
			t.logger.Warn("failed to unsubscribe from NATS",
				zap.String("topic", topic),
				zap.Error(err),
			)
		}
		delete(t.refs, topic)
		delete(t.subs, topic)
		t.logger.Info("NATS subscription removed",
			zap.String("topic", topic),
		)
	}

	return nil
}

// NATSToTopic converts a NATS subject back to a public topic string.
// Returns empty string if the subject is not mapped.
func (t *TopicReg) NATSToTopic(subject string) string {
	// Direct lookup in subjectToTopic map
	if topic, ok := t.subjectToTopic[subject]; ok {
		return topic
	}

	// Try pattern matching for wildcard subjects
	// e.g., "ace.engine.abc.layer.>" matches "agent:{id}:status"
	for _, pattern := range []string{
		"ace.engine.%s.layer.>",
		"ace.engine.%s.loop.>",
		"ace.system.health.>",
		"ace.usage.%s.>",
	} {
		if strings.HasPrefix(subject, strings.TrimSuffix(pattern, ".>")+".") ||
			subject == strings.TrimSuffix(pattern, ".>") {
			// Try to extract the ID from the subject
			topic, found := t.matchSubjectToTopic(subject, pattern)
			if found {
				return topic
			}
		}
	}

	return ""
}

// topicToSubjectFunc converts a public topic to a NATS subject pattern.
// Returns an error if the topic format is not supported.
func (t *TopicReg) topicToSubjectFunc(topic string) (string, error) {
	// Direct mapping
	if subject, ok := t.topicToSubject[topic]; ok {
		return subject, nil
	}

	// Pattern-based mapping for agent topics
	// agent:{id}:status -> ace.engine.{id}.layer.>
	// agent:{id}:logs -> ace.engine.{id}.loop.>
	// agent:{id}:cycles -> ace.engine.{id}.layer.6.output
	if strings.HasPrefix(topic, "agent:") {
		parts := strings.Split(topic, ":")
		if len(parts) == 3 {
			id := parts[1]
			subType := parts[2]

			switch subType {
			case "status":
				return fmt.Sprintf("ace.engine.%s.layer.>", id), nil
			case "logs":
				return fmt.Sprintf("ace.engine.%s.loop.>", id), nil
			case "cycles":
				return fmt.Sprintf("ace.engine.%s.layer.6.output", id), nil
			}
		}
	}

	// system:health -> ace.system.health.>
	if topic == "system:health" {
		return "ace.system.health.>", nil
	}

	// usage:{id} -> ace.usage.{id}.>
	if strings.HasPrefix(topic, "usage:") {
		parts := strings.Split(topic, ":")
		if len(parts) == 2 {
			return fmt.Sprintf("ace.usage.%s.>", parts[1]), nil
		}
	}

	return "", fmt.Errorf("unsupported topic format: %s", topic)
}

// matchSubjectToTopic attempts to match a NATS subject against known patterns
// to extract the public topic.
func (t *TopicReg) matchSubjectToTopic(subject, pattern string) (string, bool) {
	switch pattern {
	case "ace.engine.%s.layer.>":
		// e.g., ace.engine.abc123.layer.> -> agent:abc123:status
		prefix := "ace.engine."
		suffix := ".layer.>"
		if strings.HasSuffix(subject, suffix) && strings.Contains(subject, prefix) {
			id := strings.TrimSuffix(strings.TrimPrefix(subject, prefix), ".layer.>")
			if id != "" {
				return fmt.Sprintf("agent:%s:status", id), true
			}
		}
	case "ace.engine.%s.loop.>":
		// e.g., ace.engine.abc123.loop.> -> agent:abc123:logs
		prefix := "ace.engine."
		suffix := ".loop.>"
		if strings.HasSuffix(subject, suffix) && strings.Contains(subject, prefix) {
			id := strings.TrimSuffix(strings.TrimPrefix(subject, prefix), ".loop.>")
			if id != "" {
				return fmt.Sprintf("agent:%s:logs", id), true
			}
		}
	case "ace.system.health.>":
		if strings.HasPrefix(subject, "ace.system.health.") {
			return "system:health", true
		}
	case "ace.usage.%s.>":
		// e.g., ace.usage.user123.> -> usage:user123
		prefix := "ace.usage."
		suffix := ".>"
		if strings.HasSuffix(subject, suffix) && strings.HasPrefix(subject, prefix) {
			id := strings.TrimSuffix(strings.TrimPrefix(subject, prefix), ".>")
			if id != "" {
				return fmt.Sprintf("usage:%s", id), true
			}
		}
	}
	return "", false
}

// subscribeNATS creates a NATS subscription for the given subject.
// Returns nil without error when nats is nil (test-only path).
func (t *TopicReg) subscribeNATS(subject, topic string) (*nats.Subscription, error) {
	if t.nats == nil {
		return nil, nil
	}
	sub, err := messaging.SubscribeWithEnvelope(t.natsClient, subject, func(env *messaging.Envelope, data []byte) error {
		if t.dispatch != nil {
			t.dispatch(topic, data)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if sa, ok := sub.(*subscriptionAdapter); ok {
		return sa.sub, nil
	}
	return nil, fmt.Errorf("unexpected subscription type")
}

// unsubscribeNATS removes the NATS subscription for a topic.
func (t *TopicReg) unsubscribeNATS(topic string) error {
	sub, ok := t.subs[topic]
	if !ok {
		return nil
	}

	if sub != nil {
		if err := sub.Unsubscribe(); err != nil {
			return fmt.Errorf("unsubscribe failed: %w", err)
		}
	}

	return nil
}

// RefCount returns the current reference count for a topic.
func (t *TopicReg) RefCount(topic string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.refs[topic]
}

// IsSubscribed returns true if the topic has an active NATS subscription.
func (t *TopicReg) IsSubscribed(topic string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.subs[topic]
	return ok
}

// Close unsubscribes from all active NATS subscriptions.
func (t *TopicReg) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lastErr error
	for topic, sub := range t.subs {
		if sub != nil {
			if err := sub.Unsubscribe(); err != nil {
				t.logger.Error("failed to unsubscribe",
					zap.String("topic", topic),
					zap.Error(err),
				)
				lastErr = err
			}
		}
	}

	t.refs = make(map[string]int)
	t.subs = make(map[string]*nats.Subscription)

	return lastErr
}
