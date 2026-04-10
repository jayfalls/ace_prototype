# Migration and Rollback: Users-Auth Unit

<!--
Intent: Define database schema changes and rollback procedures for the users-auth unit.
Scope: All migrations needed for users, sessions, auth_tokens, and resource_permissions tables.
Used by: AI agents to safely modify the database schema and recover from failures.
-->

---

## Overview

This migration creates all auth-related tables for the users-auth unit. The tables are created in dependency order to ensure referential integrity:

1. **users** — Core user table (base table)
2. **sessions** — Session tracking with refresh tokens
3. **auth_tokens** — Magic link tokens for login, verification, password reset
4. **resource_permissions** — Resource-level permissions

All migrations follow Goose v3 conventions with Go migration functions. Timestamps use `YYYYMMDDHHMMSS` format.

---

## Migrations

### Migration 1: Create Users Table

**Direction**: UP → DOWN  
**Description**: Creates the core users table with email, password_hash, role, and status fields.

```go
// migrations/20240401000001_create_users.go
package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upCreateUsers, downCreateUsers)
}

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
	`)
	return err
}

func downCreateUsers(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP TRIGGER IF EXISTS set_users_updated_at ON users;
		DROP INDEX IF EXISTS idx_users_email;
		DROP INDEX IF EXISTS idx_users_status;
		DROP TABLE IF EXISTS users;
	`)
	return err
}
```

---

### Migration 2: Create Sessions Table

**Direction**: UP → DOWN  
**Description**: Creates the sessions table for JWT refresh token tracking.

```go
// migrations/20240401000002_create_sessions.go
package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upCreateSessions, downCreateSessions)
}

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
	`)
	return err
}

func downCreateSessions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_sessions_user_id;
		DROP INDEX IF EXISTS idx_sessions_expires_at;
		DROP TABLE IF EXISTS sessions;
	`)
	return err
}
```

---

### Migration 3: Create Auth Tokens Table

**Direction**: UP → DOWN  
**Description**: Creates the auth_tokens table for magic link tokens (login, verification, password_reset).

```go
// migrations/20240401000003_create_auth_tokens.go
package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upCreateAuthTokens, downCreateAuthTokens)
}

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
	`)
	return err
}

func downCreateAuthTokens(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_auth_tokens_user_id;
		DROP INDEX IF EXISTS idx_auth_tokens_token_hash;
		DROP INDEX IF EXISTS idx_auth_tokens_expires_at;
		DROP TABLE IF EXISTS auth_tokens;
	`)
	return err
}
```

---

### Migration 4: Create Resource Permissions Table

**Direction**: UP → DOWN  
**Description**: Creates the resource_permissions table for granular resource-level access control.

```go
// migrations/20240401000004_create_resource_permissions.go
package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upCreateResourcePermissions, downCreateResourcePermissions)
}

func upCreateResourcePermissions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_permissions (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			resource_type  VARCHAR(50) NOT NULL,
			resource_id    UUID NOT NULL,
			permission_level VARCHAR(20) NOT NULL 
			                CHECK (permission_level IN ('view', 'use', 'admin')),
			granted_by     UUID REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			
			UNIQUE(user_id, resource_type, resource_id)
		);

		CREATE INDEX idx_resource_permissions_user_id ON resource_permissions(user_id);
		CREATE INDEX idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);
	`)
	return err
}

func downCreateResourcePermissions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_resource_permissions_user_id;
		DROP INDEX IF EXISTS idx_resource_permissions_resource;
		DROP TABLE IF EXISTS resource_permissions;
	`)
	return err
}
```

---

## Rollback Strategy

### Primary Rollback

Each migration includes a corresponding `down` function that reverses the `up` migration. The rollback order must follow the reverse dependency order:

| Step | Action | Command |
|------|--------|---------|
| 1 | Rollback resource_permissions | `goose down` (runs downCreateResourcePermissions) |
| 2 | Rollback auth_tokens | `goose down` (runs downCreateAuthTokens) |
| 3 | Rollback sessions | `goose down` (runs downCreateSessions) |
| 4 | Rollback users | `goose down` (runs downCreateUsers) |

### Automatic Rollback

- **Tool**: Goose v3
- **Command**: `goose down` — rolls back the most recent migration
- **Command**: `goose down-to N` — rolls back to a specific version

### Manual Rollback Procedures

If automatic rollback fails:

1. **Identify failed migration**: Check `goose_db_version` table
2. **Restore from backup**: Point-in-time recovery from pre-migration backup
3. **Verify schema**: Run schema dump comparison to confirm restoration

### Rollback Decision Tree

```
Migration failed?
├─ NOT YET DEPLOYED TO PRODUCTION?
│  └─ YES → Revert Git commit. Do not deploy. (SAFE)
├─ DEPLOYED TO PRODUCTION?
│  ├─ ADDITIVE change? (new table)
│  │  ├─ NO DATA written yet? → Run goose down (MODERATE)
│  │  └─ DATA EXISTS? → Forward fix preferred (PREFER FORWARD FIX)
│  └─ DESTRUCTIVE change? 
│     ├─ Backup EXISTS? → Point-in-time recovery (HIGH RISK)
│     └─ NO backup? → Forward fix mandatory (FORWARD FIX ONLY)
```

---

## Pre-Migration Checklist

- [ ] Backup database (or verify automated backup exists)
- [ ] Verify PostgreSQL is available and accessible
- [ ] Test all migrations on staging environment
- [ ] Confirm rollback plan is documented and tested
- [ ] Verify `down` functions work correctly
- [ ] Check for existing users table in target database (prevent conflicts)
- [ ] Ensure `update_updated_at()` trigger function exists in database

---

## Post-Migration Checklist

- [ ] Verify all tables exist with correct columns
- [ ] Run `SELECT * FROM users LIMIT 1` to confirm table is queryable
- [ ] Run application tests against new schema
- [ ] Check logs for migration errors
- [ ] Verify indexes are created and usable
- [ ] Test single-user mode seed data (first user = admin, status='active')

---

## Migration Dependencies

| Migration | Depends On | Description |
|-----------|-------------|-------------|
| create_sessions | create_users | sessions.user_id REFERENCES users(id) |
| create_auth_tokens | create_users | auth_tokens.user_id REFERENCES users(id) |
| create_resource_permissions | create_users | resource_permissions.user_id REFERENCES users(id) |
| create_resource_permissions | create_users | resource_permissions.granted_by REFERENCES users(id) |

---

## Rollback Dependencies

| Migration | Must Be Rolled Back With |
|-----------|--------------------------|
| create_users | All dependent tables (sessions, auth_tokens, resource_permissions) must be dropped first |

---

## Single-User Mode Seed Data

After migration completes in single-user mode deployment:

```go
// internal/migration/seed_single_user.go
package migration

func SeedSingleUser(db *sql.DB) error {
	// Only create first user if no users exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // User already exists
	}

	// Create first admin user
	_, err = db.Exec(`
		INSERT INTO users (email, password_hash, role, status)
		VALUES ($1, $2, 'admin', 'active')
	`, "admin@example.com", "$2a$10$placeholder_hash") // Replace with actual hash
	return err
}
```

**Note**: The seed data script should be run as a separate deployment step after migrations complete. Password must be set via secure means (environment variable or initial setup flow).

---

## Schema Versioning

| Aspect | Detail |
|--------|--------|
| Tracking table | `goose_db_version` (created by Goose automatically) |
| Version ID | Numeric (20240401000001, 20240401000002, etc.) |
| Ordering | Chronological by timestamp |
| State | Applied or pending |

---

## Deployment Sequence

```
1. Pre-deploy: goose status
   → List pending migrations
   → Verify no unexpected migrations

2. Deploy: goose up
   → Apply all 4 migrations in order
   → Each migration runs in its own transaction

3. Post-deploy: Verify
   → Check goose_db_version for applied state
   → Run SELECT COUNT(*) FROM users
   → Verify all tables exist
```