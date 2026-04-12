package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateAuthTokens creates the auth_tokens table for magic link tokens.
func upCreateAuthTokens(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE auth_tokens (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_type VARCHAR(30) NOT NULL 
			            CHECK (token_type IN ('login', 'verification', 'password_reset')),
			token_hash VARCHAR(255) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at   TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
		CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens(token_hash);
		CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);

		COMMENT ON TABLE auth_tokens IS 'Auth tokens for magic links and password reset.';
		COMMENT ON COLUMN auth_tokens.user_id IS 'Reference to the user this token is for.';
		COMMENT ON COLUMN auth_tokens.token_type IS 'Type: login, verification, or password_reset.';
		COMMENT ON COLUMN auth_tokens.token_hash IS 'SHA256 hash of the token value.';
		COMMENT ON COLUMN auth_tokens.expires_at IS 'When this token expires.';
		COMMENT ON COLUMN auth_tokens.used_at IS 'When this token was used (null if unused).';
	`)
	return err
}

// downCreateAuthTokens drops the auth_tokens table.
func downCreateAuthTokens(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_auth_tokens_user_id;
		DROP INDEX IF EXISTS idx_auth_tokens_token_hash;
		DROP INDEX IF EXISTS idx_auth_tokens_expires_at;
		DROP TABLE IF EXISTS auth_tokens;
	`)
	return err
}

func init() {
	goose.AddMigration(upCreateAuthTokens, downCreateAuthTokens)
}
