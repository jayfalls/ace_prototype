package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateUsageEvents creates the usage_events table for tracking API and service usage.
// The table stores per-operation usage data including cost, duration, and token counts,
// linked to agents, cycles, and sessions for attribution and billing.
//
// Goose runs migrations within a transaction by default, so the table and all
// indexes are created atomically — either all succeed or none do.
func upCreateUsageEvents(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE usage_events (
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
		CREATE INDEX idx_usage_events_agent_id ON usage_events(agent_id);
		CREATE INDEX idx_usage_events_cycle_id ON usage_events(cycle_id);
		CREATE INDEX idx_usage_events_session_id ON usage_events(session_id);
		CREATE INDEX idx_usage_events_timestamp ON usage_events(timestamp DESC);
		CREATE INDEX idx_usage_events_operation_type ON usage_events(operation_type);
		CREATE INDEX idx_usage_events_service_name ON usage_events(service_name);
	`)
	return err
}

func init() {
	goose.AddMigration(upCreateUsageEvents, downCreateUsageEvents)
}

// downCreateUsageEvents drops the usage_events table and all associated indexes.
func downCreateUsageEvents(tx *sql.Tx) error {
	_, err := tx.Exec("DROP TABLE IF EXISTS usage_events")
	return err
}
