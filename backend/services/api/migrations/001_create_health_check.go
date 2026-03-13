package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddHealthCheck, downAddHealthCheck)
}

func upAddHealthCheck(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE health_check (
			id SERIAL PRIMARY KEY,
			status VARCHAR(50) NOT NULL DEFAULT 'healthy',
			message TEXT,
			checked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create health_check table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO health_check (status, message, checked_at)
		VALUES ('healthy', 'System is operational', NOW())
	`)
	if err != nil {
		return fmt.Errorf("insert initial health check: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_health_check_checked_at ON health_check(checked_at DESC)
	`)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	return nil
}

func downAddHealthCheck(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS health_check")
	if err != nil {
		return fmt.Errorf("drop health_check table: %w", err)
	}
	return nil
}
