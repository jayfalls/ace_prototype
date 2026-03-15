package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// Note: goose runs migrations within a transaction by default.
// The up function creates the table and indexes in a single transaction,
// which ensures atomicity - either all changes succeed or none do.

func init() {
	goose.AddMigration(up, down)
}

func up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS usage_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			timestamp TIMESTAMPTZ NOT NULL,
			agent_id UUID NOT NULL,
			cycle_id UUID NOT NULL,
			session_id UUID NOT NULL,
			service_name VARCHAR(255) NOT NULL,
			operation_type VARCHAR(50) NOT NULL,
			resource_type VARCHAR(50) NOT NULL,
			cost_usd DECIMAL(10, 6),
			duration_ms BIGINT,
			token_count BIGINT,
			metadata JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_usage_events_agent_id ON usage_events(agent_id);
		CREATE INDEX IF NOT EXISTS idx_usage_events_cycle_id ON usage_events(cycle_id);
		CREATE INDEX IF NOT EXISTS idx_usage_events_session_id ON usage_events(session_id);
		CREATE INDEX IF NOT EXISTS idx_usage_events_timestamp ON usage_events(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_usage_events_operation_type ON usage_events(operation_type);
		CREATE INDEX IF NOT EXISTS idx_usage_events_service_name ON usage_events(service_name);
	`)
	return err
}

func down(tx *sql.Tx) error {
	_, err := tx.Exec("DROP TABLE IF EXISTS usage_events")
	return err
}
