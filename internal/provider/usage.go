package provider

import (
	"context"
	"database/sql"
)

// usage_events is the per-call accounting table (spec 9.2): tokens come only
// from provider-reported usage — a response without usage stores NULL, never
// an estimate.
func migrateUsage(db *sql.DB) {
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS usage_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			call_id TEXT NOT NULL,
			layer TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			provider TEXT NOT NULL,
			billing_path TEXT NOT NULL,
			prompt_tokens INTEGER,
			cached_tokens INTEGER,
			completion_tokens INTEGER,
			retries INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			cost REAL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_usage_events_created ON usage_events(created_at);
		CREATE TABLE IF NOT EXISTS models_catalogue (
			id TEXT NOT NULL,
			provider TEXT NOT NULL,
			source_url TEXT NOT NULL DEFAULT '',
			price_in REAL NOT NULL DEFAULT 0,
			price_out REAL NOT NULL DEFAULT 0,
			price_cached REAL NOT NULL DEFAULT 0,
			context INTEGER NOT NULL DEFAULT 0,
			cutoff TEXT NOT NULL DEFAULT '',
			aa_index REAL NOT NULL DEFAULT 0,
			removed_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (id, provider)
		);
	`)
}

// recordUsage writes one usage_events row. tokIn/tokCached/tokOut are
// pointers: nil = the provider reported no usage (NULL, never estimated).
func recordUsage(ctx context.Context, db *sql.DB, e usageEvent) {
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `INSERT INTO usage_events
		(call_id, layer, role, model, provider, billing_path, prompt_tokens, cached_tokens, completion_tokens, retries, duration_ms, cost)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.callID, e.layer, e.role, e.model, e.group, e.billing,
		e.tokIn, e.tokCached, e.tokOut, e.retries, e.durationMs, e.cost)
}

type usageEvent struct {
	callID, layer, role, model, group, billing string
	tokIn, tokCached, tokOut                   *int64
	retries                                    int
	durationMs                                 int64
	cost                                       *float64
}

// layerKey carries the emitting ACE layer (L1..L6) through ctx into
// usage_events.layer.
type layerKey struct{}

// WithLayer tags a context with the emitting layer code.
func WithLayer(ctx context.Context, layer string) context.Context {
	return context.WithValue(ctx, layerKey{}, layer)
}

// LayerFrom reads the layer tag ("" when untagged).
func LayerFrom(ctx context.Context) string {
	if v, ok := ctx.Value(layerKey{}).(string); ok {
		return v
	}
	return ""
}

// usageCost prices a call from catalogue rates: in×price_in + out×price_out
// + cached×price_cached (spec 9.2).
func usageCost(ctx context.Context, db *sql.DB, group, model string, in, cached, out int64) *float64 {
	var pin, pout, pcached float64
	if db.QueryRowContext(ctx, `SELECT price_in, price_out, price_cached FROM models_catalogue WHERE id=? AND provider=? AND removed_at=''`,
		model, group).Scan(&pin, &pout, &pcached) != nil {
		return nil // no catalogue row → cost unknown, never estimated
	}
	c := float64(in)*pin + float64(out)*pout + float64(cached)*pcached
	return &c
}
