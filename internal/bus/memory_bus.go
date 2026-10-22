package bus

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"

	"ace/internal/core"
	"github.com/google/uuid"
)

type MemoryBus struct {
	db     *sql.DB
	telem  core.TelemetrySink
	mu     sync.RWMutex
	nextID atomic.Uint64
	subs   map[uint64]func(core.BusEnvelope)
}

func NewMemoryBus(db *sql.DB, telem core.TelemetrySink) (*MemoryBus, error) {
	b := &MemoryBus{
		db:    db,
		telem: telem,
		subs:  make(map[uint64]func(core.BusEnvelope)),
	}
	if err := b.migrate(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *MemoryBus) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS bus_events (
		id TEXT PRIMARY KEY,
		cycle_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		topic TEXT NOT NULL,
		direction TEXT NOT NULL,
		from_layer INTEGER NOT NULL,
		to_layer INTEGER NOT NULL,
		kind TEXT NOT NULL,
		summary TEXT NOT NULL,
		payload TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_bus_events_created ON bus_events(created_at DESC);
	`
	_, err := b.db.Exec(schema)
	return err
}

func (b *MemoryBus) Publish(ctx context.Context, env core.BusEnvelope) error {
	if env.ID == "" {
		env.ID = "bus_" + uuid.NewString()[:12]
	}
	if env.CreatedAt.IsZero() {
		env.CreatedAt = time.Now().UTC()
	}

	_, err := b.db.ExecContext(ctx, `
		INSERT INTO bus_events (
			id, cycle_id, agent_id, topic, direction, from_layer, to_layer, kind, summary, payload, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		env.ID, env.CycleID, env.AgentID, env.Topic, string(env.Direction),
		env.FromLayer, env.ToLayer, env.Kind, env.Summary, env.Payload, env.CreatedAt,
	)
	if err != nil {
		return err
	}

	if b.telem != nil {
		b.telem.EmitLive("bus.envelope", env)
	}

	b.mu.RLock()
	handlers := make([]func(core.BusEnvelope), 0, len(b.subs))
	for _, h := range b.subs {
		handlers = append(handlers, h)
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		h(env)
	}
	return nil
}

func (b *MemoryBus) Subscribe(handler func(core.BusEnvelope)) func() {
	id := b.nextID.Add(1)
	b.mu.Lock()
	b.subs[id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

func (b *MemoryBus) Recent(limit int) ([]core.BusEnvelope, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	rows, err := b.db.Query(`
		SELECT id, cycle_id, agent_id, topic, direction, from_layer, to_layer, kind, summary, payload, created_at
		FROM bus_events
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.BusEnvelope
	for rows.Next() {
		var e core.BusEnvelope
		var dir string
		if err := rows.Scan(
			&e.ID, &e.CycleID, &e.AgentID, &e.Topic, &dir,
			&e.FromLayer, &e.ToLayer, &e.Kind, &e.Summary, &e.Payload, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		e.Direction = core.BusDirection(dir)
		out = append(out, e)
	}
	return out, rows.Err()
}
