package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateVersionStamps creates the version_stamps table for cache invalidation by version.
// The table stores the current version string for each cache key, enabling versioned
// invalidation without scanning cache entries.
func upCreateVersionStamps(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE version_stamps (
			key VARCHAR(512) PRIMARY KEY,
			version VARCHAR(255) NOT NULL,
			source_hash VARCHAR(64),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_by VARCHAR(255)
		);

		COMMENT ON TABLE version_stamps IS 'Version stamps for cache invalidation by version.';
		COMMENT ON COLUMN version_stamps.key IS 'Cache key (fully qualified).';
		COMMENT ON COLUMN version_stamps.version IS 'Current version string for the key.';
		COMMENT ON COLUMN version_stamps.source_hash IS 'Hash of the source data that produced this version.';
		COMMENT ON COLUMN version_stamps.updated_at IS 'When this version was last updated.';
		COMMENT ON COLUMN version_stamps.updated_by IS 'Service or agent that updated this version.';
	`)
	return err
}

// downCreateVersionStamps drops the version_stamps table and all associated indexes.
func downCreateVersionStamps(tx *sql.Tx) error {
	_, err := tx.Exec("DROP TABLE IF EXISTS version_stamps")
	return err
}

func init() {
	goose.AddMigration(upCreateVersionStamps, downCreateVersionStamps)
}
