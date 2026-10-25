// Package contextsys is the spec-9.5 context system: composable
// byte-budgeted stages, a SQLite problem-space map, VOI-ranked closure
// items, per-layer input contracts, overflow-to-file, tiered compaction,
// turn recaps and an injection log. Context assembly previously lived in
// internal/cognitive/context_build.go with no compaction, recap or log.
package contextsys

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// InjectionEvent is one injected context block — constitution, memory,
// config or model change, compaction — with source, size and time (spec).
type InjectionEvent struct {
	Source string    `json:"source"` // e.g. "constitution", "memory.goals", "config.reload", "compact.micro"
	Kind   string    `json:"kind"`   // block | control | compaction
	Bytes  int       `json:"bytes"`
	At     time.Time `json:"at"`
	Notice string    `json:"notice,omitempty"` // in-band text for the model (control-plane changes)
}

// InjectionLog records every injected block. Control-plane events carry a
// Notice the model sees as an in-band message on the next prompt.
type InjectionLog struct {
	db     *sql.DB
	mu     sync.Mutex
	events []InjectionEvent // in-memory tail for notices + tests
}

// NewInjectionLog binds the log to the engine DB (may be nil in tests).
func NewInjectionLog(db *sql.DB) *InjectionLog {
	return &InjectionLog{db: db}
}

// Migrate creates the injection_log table.
func MigrateInjectionLog(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS injection_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		kind TEXT NOT NULL,
		bytes INTEGER NOT NULL,
		notice TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`)
	return err
}

// Record appends an event. Control events are also drained as notices.
func (l *InjectionLog) Record(ctx context.Context, ev InjectionEvent) {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	l.mu.Lock()
	l.events = append(l.events, ev)
	if len(l.events) > 500 {
		l.events = l.events[len(l.events)-500:]
	}
	l.mu.Unlock()
	if l.db != nil {
		_, _ = l.db.ExecContext(ctx,
			`INSERT INTO injection_log (source, kind, bytes, notice, created_at) VALUES (?,?,?,?,?)`,
			ev.Source, ev.Kind, ev.Bytes, ev.Notice, ev.At.Format(tsFmt))
	}
}

// DrainNotices returns pending control-plane notices (compaction results,
// config/model changes) and clears them — they reach the model once.
func (l *InjectionLog) DrainNotices() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	var keep []InjectionEvent
	for _, ev := range l.events {
		if ev.Kind == "control" && ev.Notice != "" {
			out = append(out, ev.Notice)
		} else {
			keep = append(keep, ev)
		}
	}
	l.events = keep
	return out
}

// Tail returns the last n recorded events (newest last) for inspection.
func (l *InjectionLog) Tail(n int) []InjectionEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > len(l.events) {
		n = len(l.events)
	}
	out := make([]InjectionEvent, n)
	copy(out, l.events[len(l.events)-n:])
	return out
}
