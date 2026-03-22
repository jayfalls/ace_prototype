# Migration Documentation

**FSD Requirements**: FR-4.1, FR-4.2, FR-4.3, FR-4.4

---

## Overview

This document covers the Goose v3 migration framework, file conventions, rollback patterns, testing strategies, and schema versioning for the ACE Framework.

---

## Migration Framework

### Goose v3

The ACE Framework uses [Goose v3](https://github.com/pressly/goose) with Go migration functions (never SQL files).

| Aspect | Standard |
|--------|----------|
| Tool | Goose v3 (`github.com/pressly/goose/v3`) |
| Language | Go functions |
| Registration | `init()` function |
| Signature | `goose.AddMigration(upFunc, downFunc)` |
| Transaction | Automatic (wraps `up`/`down` in transaction) |
| Package | `migrations` |

### Migration Template

```go
// migrations/YYYYMMDDHHMMSS_description.go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upDescription, downDescription)
}

func upDescription(tx *sql.Tx) error {
    _, err := tx.Exec(`
        -- Forward migration SQL
    `)
    return err
}

func downDescription(tx *sql.Tx) error {
    _, err := tx.Exec(`
        -- Reverse migration SQL
    `)
    return err
}
```

---

## File Naming Convention

Standard: `YYYYMMDDHHMMSS_description.go`

| Component | Format | Example |
|-----------|--------|---------|
| Timestamp | `YYYYMMDDHHMMSS` | `20260315143000` |
| Separator | `_` | — |
| Description | `snake_case` | `create_agents` |
| Extension | `.go` | — |
| Full example | — | `20260315143000_create_agents.go` |

Generate timestamp: `date +%Y%m%d%H%M%S`

The timestamp ensures chronological ordering and prevents merge conflicts when multiple developers create migrations simultaneously.

---

## Migration Workflow

```
1. Generate timestamp: date +%Y%m%d%H%M%S
2. Create file: migrations/{timestamp}_{description}.go
3. Implement up() function:
   - Write DDL (CREATE TABLE, ALTER TABLE, CREATE INDEX, etc.)
   - Use IF NOT EXISTS / IF EXISTS for idempotency
   - Include all related objects (indexes, constraints, triggers)
4. Implement down() function:
   - Reverse all changes from up()
   - Use IF EXISTS for safety
   - Must restore schema to exact previous state
5. Test locally: goose up && goose down && goose up
6. Run tests: make test
7. Commit: Include migration file in same PR as code changes
```

---

## Forward-Only Strategy

**Principle**: Migrations should be forward-only by default. The `down` function exists for development/testing, not for production rollbacks.

**Rationale**:
- Forward-only migrations are simpler and safer
- Once deployed, new data may depend on the new schema
- Rollback via `down` can cause data loss or inconsistency
- Forward fixes (new migration to correct issues) are preferred

**When to use `down`**:
- Local development (testing migration locally)
- Pre-deployment testing (CI/CD verification)
- Emergency: only when forward fix is impossible and rollback is safe

---

## Goose Command Reference

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `goose status` | Show pending/applied migrations | Before deploy, in CI |
| `goose up` | Apply all pending migrations | Deploy, dev setup |
| `goose up-to VERSION` | Apply up to specific version | Partial deploy |
| `goose down` | Rollback last migration | Dev testing only |
| `goose down-to VERSION` | Rollback to specific version | Emergency rollback |
| `goose create NAME go` | Create new migration file | Development |
| `goose fix` | Fix migration ordering | Legacy cleanup |

---

## Rollback Patterns

### Rollback Scenarios

#### Scenario 1: Pre-Deployment Rollback (Safe)

| Aspect | Detail |
|--------|--------|
| When | Migration written but not deployed to production |
| Action | Revert Git commit. Do not deploy. |
| Risk | Low — no production data affected |

#### Scenario 2: Post-Deployment Rollback (Risky)

| Aspect | Detail |
|--------|--------|
| When | Migration applied to production; app code may depend on new schema |
| Action | Compensating migration or expand-contract rollback |
| Risk | High — new data may exist in new schema |
| Safety checks | Verify no dependent deployments, check for new data, backup affected data, test in staging |

#### Scenario 3: Data Migration Rollback (Very Risky)

| Aspect | Detail |
|--------|--------|
| When | Migration included data transformation |
| Action | Point-in-time recovery from pre-migration backup |
| Risk | Very high — data loss possible |
| Safety checks | Pre-migration backup mandatory, verify backup integrity, test restoration |

### Rollback Decision Tree

```
Migration failed or causing issues?
├─ NOT YET DEPLOYED TO PRODUCTION?
│  └─ YES → Revert Git commit. Do not deploy. (SAFE)
├─ DEPLOYED TO PRODUCTION?
│  ├─ ADDITIVE change? (new column, new table)
│  │  ├─ NO DATA written yet? → Run compensating migration (MODERATE)
│  │  └─ DATA EXISTS? → Forward fix preferred (PREFER FORWARD FIX)
│  ├─ DESTRUCTIVE change? (DROP column, DROP table)
│  │  ├─ Backup EXISTS? → Point-in-time recovery (HIGH RISK)
│  │  └─ NO backup? → Forward fix mandatory (FORWARD FIX ONLY)
│  └─ RENAME or RESTRUCTURE?
│     ├─ Expand-contract in progress? → Switch back via feature flag (LOW RISK)
│     └─ Old schema dropped? → Restore from backup or forward fix (HIGH RISK)
```

### `down` Function Requirements

1. **Must restore schema to previous state** — reverse all changes from `up`
2. **Must handle data loss gracefully** — document that `down` may destroy data
3. **Must be tested before deployment** — run `up` then `down` in test DB
4. **Must be idempotent** — use `IF EXISTS` / `IF NOT EXISTS`
5. **Should NOT be used in production for data-destructive operations** — prefer compensating migrations

### Pre/Post-Migration Checklists

**Pre-migration**:
- [ ] Backup database (or verify automated backup exists)
- [ ] Test migration on staging environment
- [ ] Verify rollback plan is documented and tested
- [ ] Confirm `down` function works correctly

**Post-migration**:
- [ ] Verify data integrity
- [ ] Run application tests against new schema
- [ ] Check logs for errors
- [ ] Update schema documentation

---

## Migration Testing

### Testing Levels

| Level | What It Tests | How | When |
|-------|--------------|-----|------|
| Unit | `up` and `down` functions execute without error | Run against test DB | Pre-commit, CI |
| Integration | Migration produces correct schema state | Compare schema after migration | CI |
| Rollback | `up` then `down` returns to previous schema state | Schema diff before/after | CI |

### Unit Test Pattern

```go
// migrations/YYYYMMDDHHMMSS_description_test.go
package migrations

import (
    "testing"
    "database/sql"
)

func TestUpDescription(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    tx, err := db.Begin()
    if err != nil {
        t.Fatal(err)
    }
    defer tx.Rollback()

    if err := upDescription(tx); err != nil {
        t.Fatalf("up failed: %v", err)
    }
}

func TestDownDescription(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    tx, err := db.Begin()
    if err != nil {
        t.Fatal(err)
    }
    defer tx.Rollback()

    if err := upDescription(tx); err != nil {
        t.Fatalf("up failed: %v", err)
    }

    if err := downDescription(tx); err != nil {
        t.Fatalf("down failed: %v", err)
    }
}
```

### Schema Dump Validation

```
1. Run all migrations on test database
2. Dump resulting schema: pg_dump --schema-only > schema.sql
3. Compare with expected schema (committed to repo)
4. Fail CI if drift detected
```

### CI/CD Integration

| Check | When | Tool |
|-------|------|------|
| Migration syntax | Pre-commit | `go vet`, Go compiler |
| Naming convention | Pre-commit | Custom lint script |
| `down` function exists | Pre-commit | Custom lint script |
| Unit tests pass | CI | `go test ./migrations/...` |
| Schema dump matches | CI | Schema dump comparison |

---

## Schema Versioning

### Version Tracking

| Aspect | Detail |
|--------|--------|
| Tracking table | `goose_db_version` (created by Goose automatically) |
| Version ID | Numeric (derived from timestamp in filename) |
| Ordering | Chronological by timestamp |
| State | Applied or pending |

### goose_db_version Table

| Column | Type | Purpose |
|--------|------|---------|
| `version_id` | BIGINT | Migration version (from filename timestamp) |
| `is_applied` | BOOLEAN | Whether migration has been applied |
| `tstamp` | TIMESTAMP | When the migration was applied |

### Deployment Sequence

```
1. Pre-deploy: goose status
   → List pending migrations
   → Verify no unexpected migrations

2. Deploy: goose up
   → Apply all pending migrations in order
   → Each migration runs in its own transaction

3. Post-deploy: Verify
   → Check goose_db_version for applied state
   → Run /health/ready
   → Spot-check critical tables
```

### Concurrent Migration Handling

- Goose uses database-level locking (advisory lock or table lock)
- Only one `goose up` can run at a time
- Second invocation blocks until first completes
- Safe for multi-instance deployments (K8s pods)

### Edge Cases

| Scenario | Handling |
|----------|----------|
| Skipped version | Goose applies all pending in order |
| Out-of-order files | Goose sorts by timestamp |
| Failed migration | Goose stops; subsequent migrations not applied |
| Manual schema change | Not tracked — use schema dump validation to detect |
