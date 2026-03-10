package layers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NATSBus implements the Bus interface using NATS
type NATSBus struct {
	conn       *nats.Conn
	agentID    uuid.UUID
	subs       map[LayerType][]chan Message
	mu         sync.RWMutex
}

// NewNATSBus creates a new NATS-based bus
func NewNATSBus(natsURL string, agentID uuid.UUID) (*NATSBus, error) {
	conn, err := nats.Connect(natsURL)
	if err != nil {
		return nil, err
	}

	bus := &NATSBus{
		conn:    conn,
		agentID: agentID,
		subs:    make(map[LayerType][]chan Message),
	}

	// Subscribe to layer subjects
	for lt := LayerAspirational; lt <= LayerTaskProsecution; lt++ {
		subject := subjectForLayer(agentID, lt)
		sub, err := conn.QueueSubscribe(subject, "ace-workers", func(msg *nats.Msg) {
			var m Message
			if err := json.Unmarshal(msg.Data, &m); err != nil {
				return
			}
			bus.mu.RLock()
			defer bus.mu.RUnlock()
			for _, ch := range bus.subs[lt] {
				select {
				case ch <- m:
				default:
				}
			}
		})
		if err != nil {
			return nil, err
		}
		_ = sub
	}

	return bus, nil
}

// Publish publishes a message to the bus
func (b *NATSBus) Publish(ctx context.Context, msg Message) error {
	msg.SourceLayer = msg.SourceLayer
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	subject := subjectForLayer(b.agentID, msg.SourceLayer)
	return b.conn.Publish(subject, data)
}

// Subscribe subscribes to messages for a specific layer
func (b *NATSBus) Subscribe(ctx context.Context, layerType LayerType, handler func(Message) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, 100)
	b.subs[layerType] = append(b.subs[layerType], ch)

	go func() {
		for msg := range ch {
			_ = handler(msg)
		}
	}()

	return nil
}

// Unsubscribe unsubscribes from a layer
func (b *NATSBus) Unsubscribe(layerType LayerType) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, layerType)
	return nil
}

// Close closes the bus
func (b *NATSBus) Close() {
	if b.conn != nil {
		b.conn.Drain()
	}
}

// subjectForLayer returns the NATS subject for a layer
func subjectForLayer(agentID uuid.UUID, layerType LayerType) string {
	return string(layerType.String()) + "." + agentID.String()
}

// InMemoryBus implements Bus using in-memory channels
type InMemoryBus struct {
	subs map[LayerType][]chan Message
	mu   sync.RWMutex
}

// NewInMemoryBus creates a new in-memory bus
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		subs: make(map[LayerType][]chan Message),
	}
}

// Publish publishes a message
func (b *InMemoryBus) Publish(ctx context.Context, msg Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	chans := b.subs[msg.SourceLayer]
	for _, ch := range chans {
		select {
		case ch <- msg:
		default:
		}
	}
	return nil
}

// Subscribe subscribes to messages for a layer
func (b *InMemoryBus) Subscribe(ctx context.Context, layerType LayerType, handler func(Message) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, 100)
	b.subs[layerType] = append(b.subs[layerType], ch)

	go func() {
		for msg := range ch {
			_ = handler(msg)
		}
	}()

	return nil
}

// Unsubscribe unsubscribes from a layer
func (b *InMemoryBus) Unsubscribe(layerType LayerType) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, layerType)
	return nil
}

// Close closes the bus
func (b *InMemoryBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, chans := range b.subs {
		for _, ch := range chans {
			close(ch)
		}
	}
	b.subs = make(map[LayerType][]chan Message)
}
