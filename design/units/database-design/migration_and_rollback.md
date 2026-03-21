# Migration and Rollback Documentation Specification

## Overview

This document defines the approach and specification for creating migration and rollback documentation (FA-4). It is **not** the actual migrations — it is the plan for how `documentation/database-design/migrations.md` will be structured, generated, and maintained.

The migration documentation covers four functional requirements from the FSD:
- **FR-4.1**: Migration Strategy Documentation — Goose workflow, file naming, forward-only strategy
- **FR-4.2**: Rollback Patterns — Pre-deploy/post-deploy/data migration rollback, safety checks, decision tree
- **FR-4.3**: Migration Testing Patterns — Unit/integration/rollback testing, CI/CD integration
- **FR-4.4**: Schema Versioning Approach — goose_db_version table, version numbering, deployment sequence

Plus FA-5 standardization artifacts that depend on migration documentation:
- **FR-5.1**: Legacy Pattern Documentation → `legacy-patterns.md`
- **FR-5.2**: Migration Plan → `migration-plan.md`
- **FR-5.3**: Refactoring Guidelines → section within `migration-plan.md`
- **FR-5.4**: Backward Compatibility Framework → `compatibility.md`

---

## Source Artifacts

The documentation is generated from these existing code artifacts:

| Source | Location | Produces |
|--------|----------|----------|
| Goose migration files | `backend/services/api/migrations/*.go` | Migration workflow docs, migration catalog |
| Existing migration (telemetry) | `backend/shared/telemetry/migrations/001_create_usage_events.go` | Reference example for migration docs |
| Goose configuration | `backend/services/api/cmd/main.go` (`migrate` function) | Migration execution workflow |
| SQLC config | `backend/services/api/sqlc.yaml` | Schema source reference |
| Migration table | `goose_db_version` (runtime) | Versioning documentation |

---

## Current Migration Inventory

### Existing Migration: `001_create_usage_events.go`

Located at `backend/shared/telemetry/migrations/001_create_usage_events.go`. This is the reference example for documenting the Goose migration pattern:

| Aspect | Implementation |
|--------|---------------|
| Package | `migrations` |
| Registration | `init()` → `goose.AddMigration(up, down)` |
| Up action | Creates `usage_events` table with 6 indexes |
| Down action | `DROP TABLE IF EXISTS usage_events` |
| Transaction | Wrapped in transaction by default (Goose behavior) |
| Naming | `001_create_usage_events.go` (NOTE: uses numeric prefix, not timestamp — legacy pattern) |

This migration demonstrates the pattern but uses a legacy naming convention (numeric prefix) instead of the documented standard (`YYYYMMDDHHMMSS_description.go`). This is a legacy pattern to be documented in `legacy-patterns.md` (FR-5.1).

### Goose Configuration

From `backend/services/api/cmd/main.go` (`migrate` function):

| Setting | Value |
|---------|-------|
| Table name | `schema_migrations` |
| Driver | `pgx` (via `database/sql`) |
| Migration directory | `migrations` |
| Execution | `goose.Up(sqlDB, "migrations")` |

---

## Migration Documentation Strategy (FR-4.1)

Output: `documentation/database-design/migrations.md`.

### Documentation Structure

The migrations.md file follows this organization:

```
documentation/database-design/migrations.md
├── Migration Framework          ← Goose v3 overview
├── File Naming Convention       ← YYYYMMDDHHMMSS_description.go
├── Migration Template           ← Go function skeleton
├── Forward-Only Strategy        ← Why and when
├── Migration Workflow           ← Step-by-step process
├── Goose Command Reference      ← goose up/down/status
├── Migration Catalog            ← Table of all migrations
├── Rollback Patterns            ← Scenarios and decision tree (FR-4.2)
├── Migration Testing            ← Testing patterns (FR-4.3)
└── Schema Versioning            ← goose_db_version, deployment (FR-4.4)
```

### Migration Framework Documentation

Document the core Goose framework facts:

| Aspect | Standard | Example |
|--------|----------|---------|
| Tool | Goose v3 | `github.com/pressly/goose/v3` |
| Language | Go functions | Never SQL files |
| Registration | `init()` function | Auto-discovered by Goose |
| Signature | `goose.AddMigration(upFunc, downFunc)` | Standard pattern |
| Transaction | Automatic (default) | Wraps `up`/`down` in transaction |
| Package | `migrations` | All files in same package |

### File Naming Convention

Standard: `YYYYMMDDHHMMSS_description.go`

| Component | Format | Example |
|-----------|--------|---------|
| Timestamp | `YYYYMMDDHHMMSS` | `20260315143000` |
| Separator | `_` (underscore) | — |
| Description | `snake_case` | `create_agents` |
| Extension | `.go` | — |
| Full example | — | `20260315143000_create_agents.go` |

The timestamp ensures chronological ordering and prevents merge conflicts when multiple developers create migrations simultaneously.

### Migration Template

Document the standard migration file skeleton (from `design/README.md`):

```go
// backend/services/api/migrations/YYYYMMDDHHMMSS_description.go
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

### Forward-Only Strategy (FR-4.1)

Document the forward-first approach:

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

### Migration Workflow

Document the step-by-step workflow:

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

### Goose Command Reference

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

## Rollback Documentation Strategy (FR-4.2)

Section within `documentation/database-design/migrations.md`.

### Rollback Scenarios

Three scenarios documented with increasing risk:

#### Scenario 1: Pre-Deployment Rollback (Safe)

| Aspect | Detail |
|--------|--------|
| When | Migration written but not deployed to production |
| Action | Revert Git commit. Do not deploy. |
| Risk | Low — no production data affected |
| Safety checks | None beyond normal code review |

#### Scenario 2: Post-Deployment Rollback (Risky)

| Aspect | Detail |
|--------|--------|
| When | Migration applied to production; app code may depend on new schema |
| Action | Compensating migration or expand-contract rollback |
| Risk | High — new data may exist in new schema |
| Safety checks | 1. Verify no dependent deployments reference new schema |
| | 2. Check if new data written to new columns/tables |
| | 3. Backup affected data before rollback |
| | 4. Test rollback in staging with production-like data |
| | 5. Coordinate with frontend/API teams |

#### Scenario 3: Data Migration Rollback (Very Risky)

| Aspect | Detail |
|--------|--------|
| When | Migration included data transformation (backfill, column rename with data copy) |
| Action | Point-in-time recovery from pre-migration backup, or compensating data migration |
| Risk | Very high — data loss possible |
| Safety checks | 1. Pre-migration backup is mandatory |
| | 2. Verify backup integrity before starting rollback |
| | 3. Test data restoration in staging |
| | 4. Plan for data written between migration and rollback (may be lost) |

### Rollback Decision Tree

The documentation includes this decision tree (from research.md §11.1):

```
Migration failed or causing issues?
│
├─ NOT YET DEPLOYED TO PRODUCTION?
│  └─ YES → Revert Git commit. Do not deploy. (SAFE)
│
├─ DEPLOYED TO PRODUCTION?
│  │
│  ├─ Is it an ADDITIVE change? (new column, new table, new index)
│  │  ├─ NO DATA written to new schema yet?
│  │  │  └─ YES → Run compensating migration (DROP column/table/index) (MODERATE RISK)
│  │  │
│  │  └─ DATA EXISTS in new schema?
│  │     └─ YES → Forward fix preferred. Keep new schema, fix application code. (PREFER FORWARD FIX)
│  │
│  ├─ Is it a DESTRUCTIVE change? (DROP column, DROP table, type change)
│  │  ├─ Pre-migration backup EXISTS?
│  │  │  └─ YES → Point-in-time recovery from backup (HIGH RISK — data loss possible)
│  │  │
│  │  └─ NO backup?
│  │     └─ Forward fix mandatory. Cannot safely rollback. (FORWARD FIX ONLY)
│  │
│  └─ Is it a RENAME or RESTRUCTURE?
│     ├─ Expand-contract phase 1-3 (old schema still exists)?
│     │  └─ YES → Switch reads/writes back to old schema via feature flag (LOW RISK)
│     │
│     └─ Expand-contract phase 4 (old schema dropped)?
│        └─ Restore from backup or forward fix (HIGH RISK)
```

### `down` Function Requirements

Document requirements for all `down` functions:

1. **Must restore schema to previous state** — reverse all changes made in `up`
2. **Must handle data loss gracefully** — document that `down` may destroy data
3. **Must be tested before deployment** — run `up` then `down` in test DB, verify schema state
4. **Must be idempotent** — use `IF EXISTS` / `IF NOT EXISTS` for safety
5. **Should NOT be used in production for data-destructive operations** — prefer compensating migrations

### Pre-Migration Checklist

Document a checklist for all migration deployments:

- [ ] Backup database (or verify automated backup exists)
- [ ] Test migration on staging environment
- [ ] Verify sufficient disk space
- [ ] Check for long-running transactions that may block
- [ ] Notify users of downtime (if applicable)
- [ ] Verify rollback plan is documented and tested
- [ ] Confirm `down` function works correctly

### Post-Migration Checklist

- [ ] Verify data integrity (spot-check critical tables)
- [ ] Run application tests against new schema
- [ ] Check logs for errors
- [ ] Verify query performance (check slow query log)
- [ ] Update schema documentation if needed

---

## Migration Testing Documentation (FR-4.3)

Section within `documentation/database-design/migrations.md`.

### Testing Approach

Three levels of migration testing:

| Level | What It Tests | How | When |
|-------|--------------|-----|------|
| Unit | `up` and `down` functions execute without error | Run against test DB | Pre-commit, CI |
| Integration | Migration produces correct schema state | Compare schema after migration | CI |
| Rollback | `up` then `down` returns to previous schema state | Schema diff before/after | CI |

### Unit Test Pattern

Each migration file should have a corresponding test file:

```go
// migrations/YYYYMMDDHHMMSS_description_test.go
package migrations

import (
    "testing"
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
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

    // Run up first
    if err := upDescription(tx); err != nil {
        t.Fatalf("up failed: %v", err)
    }

    // Rollback
    if err := downDescription(tx); err != nil {
        t.Fatalf("down failed: %v", err)
    }
}
```

### Integration Test Pattern

Verify the full migration sequence produces the expected schema:

```go
func TestMigrationSequence(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Run all migrations
    runAllMigrations(t, db)

    // Verify expected tables exist
    tables := queryTables(t, db)
    expected := []string{"agents", "usage_events", "memory_nodes"}
    for _, table := range expected {
        if !contains(tables, table) {
            t.Errorf("expected table %s not found", table)
        }
    }
}
```

### CI/CD Integration

| Check | When | Tool |
|-------|------|------|
| Migration syntax | Pre-commit | `go vet`, Go compiler |
| Naming convention | Pre-commit | Custom lint script |
| `down` function exists | Pre-commit | Custom lint script |
| Unit tests pass | CI | `go test ./migrations/...` |
| Integration tests pass | CI | `go test` with test DB |
| Schema dump matches | CI | Schema dump comparison |

### Schema Dump Validation Pattern

From research recommendation:

```
1. Run all migrations on test database
2. Dump resulting schema: pg_dump --schema-only > schema.sql
3. Compare with expected schema (committed to repo)
4. Fail CI if drift detected
```

This provides drift detection without additional tool dependencies (research.md §11, Approach 3).

---

## Schema Versioning Documentation (FR-4.4)

Section within `documentation/database-design/migrations.md`.

### Version Tracking

| Aspect | Documentation |
|--------|--------------|
| Tracking table | `goose_db_version` (created by Goose automatically) |
| Version ID | Numeric (derived from timestamp in filename) |
| Ordering | Chronological by timestamp |
| State | Applied or pending |

### The `goose_db_version` Table

Document the Goose-managed version tracking table:

| Column | Type | Purpose |
|--------|------|---------|
| `version_id` | BIGINT | Migration version (from filename timestamp) |
| `is_applied` | BOOLEAN | Whether migration has been applied |
| `tstamp` | TIMESTAMP | When the migration was applied |

### Deployment Sequence

Document the standard deployment process:

```
1. Pre-deploy: goose status
   → List pending migrations
   → Verify no unexpected migrations

2. Deploy: goose up
   → Apply all pending migrations in order
   → Each migration runs in its own transaction

3. Post-deploy: Verify
   → Check goose_db_version for applied state
   → Run application health check (/health/ready)
   → Spot-check critical tables
```

### Concurrent Migration Handling

Document how Goose handles concurrent deployments:

- Goose uses database-level locking (`advisory lock` or table lock)
- Only one `goose up` can run at a time
- Second invocation blocks until first completes
- Safe for multi-instance deployments (K8s pods)

### Edge Cases

| Scenario | Handling |
|----------|----------|
| Skipped version | Goose applies all pending in order — skipped version will be applied |
| Out-of-order files | Goose sorts by timestamp — file order matters |
| Failed migration | Goose stops at failure point; subsequent migrations not applied |
| Manual schema change | Not tracked by Goose — use schema dump validation to detect |

---

## Legacy Migration Plan (FA-5)

This section documents how existing patterns will be migrated to documented standards. The actual migration plan is produced in `documentation/database-design/migration-plan.md` (FR-5.2).

### Legacy Patterns to Migrate

From the existing codebase:

| Legacy Pattern | Location | Modern Equivalent | Complexity |
|---------------|----------|-------------------|------------|
| Numeric filename prefix (`001_create_usage_events.go`) | `backend/shared/telemetry/migrations/` | Timestamp prefix (`YYYYMMDDHHMMSS_description.go`) | Trivial (rename on next migration) |
| Bare `up`/`down` function names | `001_create_usage_events.go` | Descriptive names (`upCreateUsageEvents`) | Trivial |
| `CREATE TABLE IF NOT EXISTS` | `001_create_usage_events.go` | `CREATE TABLE` (Goose manages existence) | Trivial |
| `log.Fatalf` in `migrate` function | `backend/services/api/cmd/main.go` | `return fmt.Errorf(...)` with caller handling | Trivial (acceptable for main function) |

### Phased Migration Plan (FR-5.2)

| Phase | Scope | Risk | Examples |
|-------|-------|------|----------|
| 1: Non-breaking | Index renames, comments, naming | Low | Rename index to `idx_{table}_{columns}` convention |
| 2: Low-risk | Add constraints, standardize types | Low-Medium | Add `NOT NULL` to existing nullable columns |
| 3: Moderate | Column renames with views | Medium | Rename column, create backward-compat view |
| 4: Breaking | Table restructures, data migrations | High | Split table, merge tables, change PK type |

### Backward Compatibility (FR-5.4)

Document the expand-contract pattern for breaking changes:

```
Phase 1: Expand    → Add new column/table alongside old
Phase 2: Migrate   → Dual-write to both; backfill data
Phase 3: Shift     → Update app code to read from new only
Phase 4: Contract  → Drop old column/table after all consumers migrated
```

This pattern is documented in `documentation/database-design/compatibility.md`.

### Documentation Artifacts for FA-5

| Artifact | FSD Requirement | Depends On |
|----------|----------------|------------|
| `legacy-patterns.md` | FR-5.1 | Schema docs, Conventions |
| `migration-plan.md` | FR-5.2, FR-5.3 | Legacy patterns, Conventions |
| `compatibility.md` | FR-5.4 | Migration plan, API contracts |

---

## Makefile Integration

Per `design/README.md`: "All operations go through the Makefile."

```makefile
# Included in the docs target (architecture.md §Makefile Integration)
docs:
	go run ./scripts/docs-gen/
```

The `docs-gen` script includes migration documentation generation:
- Scans `backend/services/api/migrations/*.go` for migration files
- Extracts migration names, timestamps, and descriptions
- Generates migration catalog table in `migrations.md`
- Validates naming convention compliance
- Validates `down` function existence

---

## Constraints

1. **Goose Go functions only** — never SQL files (design/README.md)
2. **init() registration** — each file registers via `goose.AddMigration(up, down)` (design/README.md)
3. **Timestamp naming** — `YYYYMMDDHHMMSS_description.go` (FR-4.1)
4. **Forward-only strategy** — `down` functions for testing, not production rollback (FR-4.1)
5. **SQLC generated files are never hand-edited** — migrations change schema, then `sqlc generate` (design/README.md)
6. **Version control** — migration docs in same PR as migration code (NFR-4)
7. **All operations through Makefile** — `make docs` for generation (design/README.md)
