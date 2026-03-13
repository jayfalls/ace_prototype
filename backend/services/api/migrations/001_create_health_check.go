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
			db VARCHAR(50) NOT NULL DEFAULT 'healthy',
			err TEXT,
			created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create health_check table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_health_check_created ON health_check(created DESC)
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
