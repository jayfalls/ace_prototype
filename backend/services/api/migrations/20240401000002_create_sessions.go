package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateSessions creates the sessions table for JWT refresh token tracking.
func upCreateSessions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE sessions (
			id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			refresh_token_hash VARCHAR(255) NOT NULL,
			user_agent          VARCHAR(512),
			ip_address          INET,
			last_used_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at          TIMESTAMPTZ NOT NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_sessions_user_id ON sessions(user_id);
		CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

		COMMENT ON TABLE sessions IS 'Active user sessions with refresh tokens.';
		COMMENT ON COLUMN sessions.user_id IS 'Reference to the user who owns this session.';
		COMMENT ON COLUMN sessions.refresh_token_hash IS 'SHA256 hash of the refresh token.';
		COMMENT ON COLUMN sessions.user_agent IS 'Client user agent string.';
		COMMENT ON COLUMN sessions.ip_address IS 'Client IP address.';
		COMMENT ON COLUMN sessions.last_used_at IS 'When the session was last used.';
		COMMENT ON COLUMN sessions.expires_at IS 'When the session expires.';
	`)
	return err
}

// downCreateSessions drops the sessions table.
func downCreateSessions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_sessions_user_id;
		DROP INDEX IF EXISTS idx_sessions_expires_at;
		DROP TABLE IF EXISTS sessions;
	`)
	return err
}

func init() {
	goose.AddMigration(upCreateSessions, downCreateSessions)
}
