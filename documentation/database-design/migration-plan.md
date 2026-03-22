# Migration Plan

**FSD Requirements**: FR-5.2, FR-5.3

---

## Overview

This document defines a four-phase plan for migrating existing implementations to documented standards. Each phase is ordered by risk level, from non-breaking changes to breaking changes.

---

## Phase 1: Non-Breaking Changes (Low Risk)

**Scope**: Naming conventions, comments, documentation — no schema impact.

| Change | Tables Affected | Migration Steps | Rollback |
|--------|----------------|-----------------|----------|
| Rename indexes to `idx_{table}_{columns}` convention | usage_events | `DROP INDEX old_name; CREATE INDEX idx_{table}_{columns} ON {table}({columns});` | Reverse the rename |
| Add `COMMENT ON TABLE/COLUMN` statements | usage_events | `COMMENT ON TABLE usage_events IS '...'` | `COMMENT ON TABLE usage_events IS NULL` |
| Add documentation files | — | Create markdown files (already done in this PR) | Delete files |

### Testing Requirements

- Verify index renames don't break existing queries
- Verify comments don't affect query performance

### Dependencies

None — these changes are purely cosmetic.

---

## Phase 2: Low-Risk Changes (Low-Medium Risk)

**Scope**: Add constraints, standardize types on existing columns.

| Change | Tables Affected | Migration Steps | Rollback |
|--------|----------------|-----------------|----------|
| Add `NOT NULL` constraint to `created_at` | usage_events | `ALTER TABLE usage_events ALTER COLUMN created_at SET NOT NULL` | `ALTER TABLE usage_events ALTER COLUMN created_at DROP NOT NULL` |
| Add `updated_at` column | usage_events | `ALTER TABLE usage_events ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | `ALTER TABLE usage_events DROP COLUMN updated_at` |
| Add `update_updated_at()` trigger | usage_events | Create function + trigger | `DROP TRIGGER` + `DROP FUNCTION` |
| Add foreign key constraints (when agents table exists) | usage_events | `ALTER TABLE usage_events ADD CONSTRAINT fk_usage_events_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id)` | `ALTER TABLE usage_events DROP CONSTRAINT fk_usage_events_agent_id` |

### Testing Requirements

- Backfill `updated_at` for existing rows before adding NOT NULL
- Verify foreign key references exist before adding constraint
- Test with production-like data volume

### Dependencies

- Phase 1 completed (comments and naming done)

---

## Phase 3: Moderate Changes (Medium Risk)

**Scope**: Column renames with backward compatibility views.

| Change | Tables Affected | Migration Steps | Rollback |
|--------|----------------|-----------------|----------|
| Rename column (if needed) | varies | Expand-contract pattern (see below) | Revert to old column name |

### Expand-Contract Pattern

For column renames, use the 4-phase expand-contract pattern:

```
Phase 1: Expand    → Add new column alongside old
Phase 2: Migrate   → Dual-write to both columns; backfill old data to new
Phase 3: Shift     → Update app code to read from new column only
Phase 4: Contract  → Drop old column after all consumers migrated
```

### Database View for Backward Compatibility

Create a view that maps the old column name to the new:

```sql
CREATE VIEW usage_events_compat AS
SELECT
    id,
    timestamp,
    agent_id,
    new_column_name AS old_column_name,  -- backward compat alias
    created_at,
    updated_at
FROM usage_events;
```

### Testing Requirements

- Validate data consistency between old and new columns
- Performance regression testing
- Application integration testing

### Dependencies

- Phase 2 completed (constraints added)

---

## Phase 4: Breaking Changes (High Risk)

**Scope**: Table restructures, data migrations, PK type changes.

| Change | Tables Affected | Migration Steps | Rollback |
|--------|----------------|-----------------|----------|
| Split table (if needed) | varies | Create new table + migrate data + update FKs | Drop new table, restore old structure |
| Merge tables (if needed) | varies | Create merged table + migrate data | Restore original tables from backup |
| Data type change (e.g., VARCHAR → ENUM) | varies | Add new column, backfill, drop old | Restore from backup |

### Safety Requirements

- Pre-migration backup is mandatory
- Test in staging with production-like data
- Coordinated deployment with dependent services
- Document rollback plan with data loss assessment

### Dependencies

- Phase 3 completed (expand-contract done)

---

## Refactoring Guidelines

### Renaming Columns Safely

1. Add new column: `ALTER TABLE {table} ADD COLUMN new_name TYPE`
2. Backfill: `UPDATE {table} SET new_name = old_name`
3. Dual-write: Application writes to both columns
4. Switch reads: Update app to read from new column
5. Drop old: `ALTER TABLE {table} DROP COLUMN old_name`

### Changing Data Types Safely

1. Add new column with target type
2. Backfill with type conversion (with validation)
3. Add CHECK constraint to validate conversion
4. Switch reads to new column
5. Drop old column

### Adding Constraints to Existing Data

1. Check for violations: `SELECT * FROM {table} WHERE NOT (constraint)`
2. Fix violations
3. Add constraint: `ALTER TABLE {table} ADD CONSTRAINT ...`

---

## PR Requirements for Schema Changes

- Migration file follows naming conventions
- `down` function is implemented and tested
- Documentation is updated (schema docs, indexes, etc.)
- Schema dump validation passes
- Rollback plan is documented

---

## Success Criteria Per Phase

| Phase | Criteria |
|-------|----------|
| 1 | All indexes follow `idx_{table}_{columns}` naming; all tables have comments |
| 2 | All mutable tables have `updated_at` with triggers; all FKs enforced |
| 3 | No column renames needed; expand-contract pattern documented and tested |
| 4 | All tables follow documented standards; no legacy patterns remain |

---

## Notes

- The current codebase has minimal legacy debt (one table, already cleaned up in PR-0)
- This plan will expand as new tables and patterns are introduced
- Each phase should be completed in a separate PR
- Rollback plans must be tested before proceeding to the next phase
