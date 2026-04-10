package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateUsers creates the users table with email, password_hash, role, and status fields.
func upCreateUsers(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE users (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email            VARCHAR(255) NOT NULL UNIQUE,
			password_hash    VARCHAR(255) NOT NULL,
			role             VARCHAR(20) NOT NULL DEFAULT 'user' 
			                CHECK (role IN ('admin', 'user', 'viewer')),
			status           VARCHAR(30) NOT NULL DEFAULT 'pending' 
			                CHECK (status IN ('pending', 'active', 'suspended')),
			suspended_at    TIMESTAMPTZ,
			suspended_reason VARCHAR(255),
			deleted_at      TIMESTAMPTZ,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TRIGGER set_users_updated_at
			BEFORE UPDATE ON users
			FOR EACH ROW EXECUTE FUNCTION update_updated_at();

		CREATE INDEX idx_users_email ON users(email);
		CREATE INDEX idx_users_status ON users(status) WHERE deleted_at IS NULL;

		COMMENT ON TABLE users IS 'User accounts with authentication credentials and roles.';
		COMMENT ON COLUMN users.email IS 'User email address (unique identifier).';
		COMMENT ON COLUMN users.password_hash IS 'Argon2id hash of the user password.';
		COMMENT ON COLUMN users.role IS 'System role: admin, user, or viewer.';
		COMMENT ON COLUMN users.status IS 'Account status: pending, active, or suspended.';
		COMMENT ON COLUMN users.suspended_at IS 'When the account was suspended (if applicable).';
		COMMENT ON COLUMN users.suspended_reason IS 'Reason for suspension.';
		COMMENT ON COLUMN users.deleted_at IS 'Soft delete timestamp (NULL if active).';
	`)
	return err
}

// downCreateUsers drops the users table and associated objects.
func downCreateUsers(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP TRIGGER IF EXISTS set_users_updated_at ON users;
		DROP INDEX IF EXISTS idx_users_email;
		DROP INDEX IF EXISTS idx_users_status;
		DROP TABLE IF EXISTS users;
	`)
	return err
}

func init() {
	goose.AddMigration(upCreateUsers, downCreateUsers)
}
