// Package tasks is the spec-9.4 time-management layer: durable tasks,
// schedules, checkpoints, monitors, deadlines, child tasks, detached exec
// handles and the budget governor. Everything survives a worker restart —
// Resume re-arms running tasks and monitors.
package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Task states.
const (
	StateQueued   = "QUEUED"
	StateRunning  = "RUNNING"
	StateDone     = "DONE"
	StateFailed   = "FAILED"
	StateCanceled = "CANCELED"
	StateBlocked  = "BLOCKED"
)

// Task is a durable unit of work. Depth is the child-task depth (spec 9.4:
// children may nest only to depth 1).
type Task struct {
	ID             string
	Kind           string
	ParentID       string
	Layer          int
	State          string
	Deadline       time.Time
	IdempotencyKey string
	Progress       float64
	Depth          int
	Payload        string
	HeartbeatAt    time.Time
	CreatedAt      time.Time
}

// Manager is the single shared task manager every layer uses.
type Manager struct {
	db *sql.DB
	// now is injectable for tests (fake clock).
	now func() time.Time
	// providerSems caps concurrent child tasks per provider.
	providerSems map[string]chan struct{}
	// monitors holds the armed Sense conditions (monitor.go).
	monitors map[string]*monitorState
	monMu    sync.Mutex
}

// New returns a Manager bound to the engine DB.
func New(db *sql.DB) *Manager {
	return &Manager{
		db:           db,
		now:          func() time.Time { return time.Now().UTC() },
		providerSems: map[string]chan struct{}{},
	}
}

// Migrate creates the spec-9.4 tables (runs, tasks, schedules, checkpoints).
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			parent_id TEXT NOT NULL DEFAULT '',
			layer INTEGER NOT NULL DEFAULT 6,
			state TEXT NOT NULL,
			deadline TEXT,
			idempotency_key TEXT UNIQUE,
			progress REAL NOT NULL DEFAULT 0,
			depth INTEGER NOT NULL DEFAULT 0,
			payload TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			heartbeat_at TEXT,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			output TEXT NOT NULL DEFAULT '',
			started_at TEXT NOT NULL,
			ended_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_runs_task ON runs(task_id);
		CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			rrule TEXT NOT NULL,
			next_due TEXT NOT NULL,
			last_run_id TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS checkpoints (
			task_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			blob BLOB NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (task_id, seq)
		);`)
	return err
}

const tsFmt = time.RFC3339Nano

// Submit inserts a task. If IdempotencyKey is set and a row already exists
// for it, Submit returns that task instead of duplicating — a side effect
// only runs when its key has no done row (see SideEffectOnce).
func (m *Manager) Submit(ctx context.Context, t Task) (Task, error) {
	if t.ID == "" {
		t.ID = "task_" + uuid.NewString()[:12]
	}
	if t.State == "" {
		t.State = StateQueued
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = m.now()
	}
	if t.IdempotencyKey != "" {
		var existing Task
		if old, err := m.Poll(ctx, t.IdempotencyKey); err == nil && old.ID != "" {
			return old, nil
		}
		var id string
		err := m.db.QueryRowContext(ctx,
			`SELECT id FROM tasks WHERE idempotency_key=?`, t.IdempotencyKey).Scan(&id)
		if err == nil {
			return m.Poll(ctx, id)
		}
		_ = existing
	}
	var deadline *string
	if !t.Deadline.IsZero() {
		s := t.Deadline.Format(tsFmt)
		deadline = &s
	}
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO tasks (id, kind, parent_id, layer, state, deadline, idempotency_key, progress, depth, payload, heartbeat_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Kind, t.ParentID, t.Layer, t.State, deadline, nullIfEmpty(t.IdempotencyKey),
		t.Progress, t.Depth, t.Payload, t.CreatedAt.Format(tsFmt), t.CreatedAt.Format(tsFmt))
	if err != nil {
		return Task{}, err
	}
	return t, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Poll returns the task row for id.
func (m *Manager) Poll(ctx context.Context, id string) (Task, error) {
	var t Task
	var deadline, hb, idem *string
	var created string
	err := m.db.QueryRowContext(ctx,
		`SELECT id, kind, parent_id, layer, state, deadline, idempotency_key, progress, depth, payload, heartbeat_at, created_at
		 FROM tasks WHERE id=?`, id).
		Scan(&t.ID, &t.Kind, &t.ParentID, &t.Layer, &t.State, &deadline, &idem, &t.Progress, &t.Depth, &t.Payload, &hb, &created)
	if err != nil {
		return Task{}, err
	}
	t.CreatedAt, _ = time.Parse(tsFmt, created)
	if deadline != nil {
		t.Deadline, _ = time.Parse(tsFmt, *deadline)
	}
	if hb != nil {
		t.HeartbeatAt, _ = time.Parse(tsFmt, *hb)
	}
	if idem != nil {
		t.IdempotencyKey = *idem
	}
	return t, nil
}

// setState updates state (and heartbeat) on a task.
func (m *Manager) setState(ctx context.Context, id, state string, progress float64, errText string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE tasks SET state=?, progress=?, error=?, heartbeat_at=? WHERE id=?`,
		state, progress, errText, m.now().Format(tsFmt), id)
	return err
}

// Heartbeat bumps progress + heartbeat_at for a running task.
func (m *Manager) Heartbeat(ctx context.Context, id string, progress float64) error {
	return m.setState(ctx, id, StateRunning, progress, "")
}

// Complete marks a task done at progress 1.
func (m *Manager) Complete(ctx context.Context, id string) error {
	return m.setState(ctx, id, StateDone, 1, "")
}

// Fail marks a task failed with the error text.
func (m *Manager) Fail(ctx context.Context, id string, err error) error {
	return m.setState(ctx, id, StateFailed, -1, err.Error())
}

// Cancel marks a task (and its children — the parent kill switch) canceled.
func (m *Manager) Cancel(ctx context.Context, id string) error {
	if err := m.setState(ctx, id, StateCanceled, -1, "canceled"); err != nil {
		return err
	}
	// Kill switch cascades to live children.
	_, err := m.db.ExecContext(ctx,
		`UPDATE tasks SET state=?, error='parent killed' WHERE parent_id=? AND state IN ('QUEUED','RUNNING','BLOCKED')`,
		StateCanceled, id)
	return err
}

// IsChild reports whether the task is a child (depth >= 1).
func (m *Manager) IsChild(ctx context.Context, id string) bool {
	var d int
	return m.db.QueryRowContext(ctx, `SELECT depth FROM tasks WHERE id=?`, id).Scan(&d) == nil && d >= 1
}

// Resume returns every task left live by a crash (QUEUED/RUNNING/BLOCKED) so
// the caller can re-arm it — plus checkpoints for mid-job continuation.
func (m *Manager) Resume(ctx context.Context) ([]Task, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id FROM tasks WHERE state IN ('QUEUED','RUNNING','BLOCKED') ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	// Poll only after the cursor closes — nested queries on one connection
	// starve the cursor on shared in-memory databases.
	var out []Task
	for _, id := range ids {
		if t, err := m.Poll(ctx, id); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

// Checkpoint persists a progress blob for resume-after-kill.
func (m *Manager) Checkpoint(ctx context.Context, taskID string, seq int, blob []byte) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO checkpoints (task_id, seq, blob, created_at) VALUES (?,?,?,?)`,
		taskID, seq, blob, m.now().Format(tsFmt))
	return err
}

// LatestCheckpoint returns the newest checkpoint blob+seq for a task.
func (m *Manager) LatestCheckpoint(ctx context.Context, taskID string) (int, []byte, error) {
	var seq int
	var blob []byte
	err := m.db.QueryRowContext(ctx,
		`SELECT seq, blob FROM checkpoints WHERE task_id=? ORDER BY seq DESC LIMIT 1`, taskID).
		Scan(&seq, &blob)
	return seq, blob, err
}

// SideEffectOnce runs fn only if key has no done row — the spec-9.4
// idempotency rule. Returns ran=false when the key already completed.
func (m *Manager) SideEffectOnce(ctx context.Context, key string, fn func() error) (bool, error) {
	var state string
	err := m.db.QueryRowContext(ctx,
		`SELECT state FROM tasks WHERE idempotency_key=?`, key).Scan(&state)
	if err == nil && state == StateDone {
		return false, nil
	}
	if err := fn(); err != nil {
		return false, err
	}
	mark := Task{ID: "task_se_" + uuid.NewString()[:10], Kind: "side_effect",
		State: StateDone, IdempotencyKey: key, Progress: 1}
	_, _ = m.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO tasks (id, kind, parent_id, layer, state, deadline, idempotency_key, progress, depth, payload, heartbeat_at, created_at)
		 VALUES (?,?,0,6,?,NULL,?,1,0,'',?,?)`,
		mark.ID, mark.Kind, StateDone, key, m.now().Format(tsFmt), m.now().Format(tsFmt))
	return true, nil
}

// startRun opens a run row for a task attempt.
func (m *Manager) startRun(ctx context.Context, taskID string) (string, error) {
	id := "run_" + uuid.NewString()[:12]
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO runs (id, task_id, status, started_at) VALUES (?,?,?,?)`,
		id, taskID, "running", m.now().Format(tsFmt))
	return id, err
}

// endRun closes a run row.
func (m *Manager) endRun(ctx context.Context, runID, status, output string) {
	now := m.now().Format(tsFmt)
	_, _ = m.db.ExecContext(ctx,
		`UPDATE runs SET status=?, output=?, ended_at=? WHERE id=?`, status, output, now, runID)
}

// runCount counts runs for a task (used by tests asserting no duplicate
// side effects).
func (m *Manager) runCount(ctx context.Context, taskID string) int {
	var n int
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE task_id=?`, taskID).Scan(&n)
	return n
}

var errChildDepth = fmt.Errorf("child tasks may not spawn children (depth cap 1)")
