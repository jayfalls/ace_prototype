package busses

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NATSBus implements the Bus interface using NATS
type NATSBus struct {
	nc         *nats.Conn
	agentID    uuid.UUID
	subs       map[layers.LayerType]*nats.Subscription
	mu         sync.RWMutex
	messageCh  chan layers.Message
}

// NewNATSBus creates a new NATS bus
func NewNATSBus(nc *nats.Conn, agentID uuid.UUID) *NATSBus {
	return &NATSBus{
		nc:        nc,
		agentID:   agentID,
		subs:     make(map[layers.LayerType]*nats.Subscription),
		messageCh: make(chan layers.Message, 100),
	}
}

// Publish sends a message to the bus
func (b *NATSBus) Publish(ctx context.Context, msg layers.Message) error {
	if b.nc == nil {
		return fmt.Errorf("NATS connection not established")
	}

	subject := fmt.Sprintf("ace.%s.%s", b.agentID, msg.SourceLayer.String())
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return b.nc.Publish(subject, data)
}

// Subscribe registers a handler for messages to a specific layer
func (b *NATSBus) Subscribe(ctx context.Context, layerType layers.LayerType, handler func(layers.Message) error) error {
	if b.nc == nil {
		return fmt.Errorf("NATS connection not established")
	}

	subject := fmt.Sprintf("ace.%s.%s", b.agentID, layerType.String())
	sub, err := b.nc.Subscribe(subject, func(msg *nats.Msg) {
		var m layers.Message
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			return
		}
		handler(m)
	})
	if err != nil {
		return err
	}

	b.mu.Lock()
	b.subs[layerType] = sub
	b.mu.Unlock()

	return nil
}

// Unsubscribe removes a subscription
func (b *NATSBus) Unsubscribe(layerType layers.LayerType) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, ok := b.subs[layerType]
	if !ok {
		return nil
	}

	err := sub.Unsubscribe()
	delete(b.subs, layerType)
	return err
}

// Close closes all subscriptions and connection
func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		sub.Unsubscribe()
	}
	b.subs = make(map[layers.LayerType]*nats.Subscription)

	if b.nc != nil {
		b.nc.Close()
	}
	return nil
}

// MockBus provides an in-memory implementation for testing
type MockBus struct {
	mu          sync.RWMutex
	subs        map[layers.LayerType][]func(layers.Message) error
	published   []layers.Message
	messageCh   chan layers.Message
}

// NewMockBus creates an in-memory bus
func NewMockBus() *MockBus {
	return &MockBus{
		subs:      make(map[layers.LayerType][]func(layers.Message) error),
		published: make([]layers.Message, 0),
		messageCh: make(chan layers.Message, 100),
	}
}

// Publish sends a message
func (b *MockBus) Publish(ctx context.Context, msg layers.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.published = append(b.published, msg)

	// Notify subscribers
	handlers, ok := b.subs[msg.TargetLayer]
	if ok {
		for _, h := range handlers {
			go h(msg)
		}
	}

	return nil
}

// Subscribe registers a handler
func (b *MockBus) Subscribe(ctx context.Context, layerType layers.LayerType, handler func(layers.Message) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subs[layerType] = append(b.subs[layerType], handler)
	return nil
}

// Unsubscribe removes a handler
func (b *MockBus) Unsubscribe(layerType layers.LayerType) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subs, layerType)
	return nil
}

// GetPublished returns all published messages
func (b *MockBus) GetPublished() []layers.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]layers.Message, len(b.published))
	copy(result, b.published)
	return result
}

// Clear resets the mock bus
func (b *MockBus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.published = make([]layers.Message, 0)
}

// MessageBusConfig holds NATS configuration
type MessageBusConfig struct {
	URL      string
	User     string
	Password string
}

// Connect establishes connection to NATS
func Connect(ctx context.Context, config MessageBusConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("ace-framework"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(5),
	}

	if config.User != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.User, config.Password))
	}

	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return nc, nil
}
