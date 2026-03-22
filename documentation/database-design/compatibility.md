# Backward Compatibility

**FSD Requirement**: FR-5.4

---

## Overview

This document defines rules for maintaining backward compatibility during database migrations and API changes. It covers the expand-contract pattern, database views, breaking change processes, and API versioning strategy.

---

## Compatibility Rules

### API Response Schema Stability

- API response schemas must remain stable during database migrations
- If a column is renamed, the API must continue returning the old field name until consumers are updated
- Use database views or Go struct aliases to maintain the mapping

### Database Views for Renamed Columns

When renaming a column, create a view that exposes the old column name:

```sql
-- Step 1: Add new column
ALTER TABLE usage_events ADD COLUMN event_timestamp TIMESTAMPTZ;

-- Step 2: Backfill
UPDATE usage_events SET event_timestamp = timestamp;

-- Step 3: Create compatibility view
CREATE VIEW usage_events_compat AS
SELECT
    id,
    event_timestamp AS timestamp,  -- old name alias
    agent_id,
    cycle_id,
    session_id,
    service_name,
    operation_type,
    resource_type,
    cost_usd,
    duration_ms,
    token_count,
    metadata,
    created_at
FROM usage_events;

-- Step 4: Update consumers to use new column name
-- Step 5: Drop view after all consumers migrated
DROP VIEW usage_events_compat;
```

### Gradual Deprecation

- Mark old patterns as deprecated in documentation
- Add logging when old patterns are used
- Set a deprecation deadline (minimum 1 sprint)
- Remove old patterns only after all consumers have migrated

---

## Expand-Contract Pattern

The 4-phase expand-contract pattern is the standard approach for breaking schema changes:

### Phase 1: Expand

Add the new column/table alongside the old:

```sql
-- Add new column
ALTER TABLE agents ADD COLUMN display_name VARCHAR(255);

-- Or create new table
CREATE TABLE agent_v2 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- new schema --
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**No application changes yet.** The new schema exists but is unused.

### Phase 2: Migrate

Dual-write to both old and new, backfill existing data:

```sql
-- Backfill existing data
UPDATE agents SET display_name = name WHERE display_name IS NULL;

-- Application writes to both columns on create/update
```

**Application changes**: Write to both old and new. Read from old only.

### Phase 3: Shift

Update application code to read from new, stop writing to old:

```sql
-- Application reads from display_name
-- Application writes to display_name only
-- Old column still exists but is not written to
```

**Application changes**: Read from new. Write to new only. Old column is dormant.

### Phase 4: Contract

Drop old column/table after all consumers have migrated:

```sql
-- Drop old column
ALTER TABLE agents DROP COLUMN name;

-- Or drop old table
DROP TABLE agent_v1;
```

**Prerequisites**: All consumers migrated, deprecation period elapsed, no reads from old column.

---

## Breaking Change Process

### Before Making a Breaking Change

1. **Document the change** — What is changing, why, and what consumers are affected
2. **Identify consumers** — List all code, queries, and external systems that reference the old schema
3. **Set deprecation timeline** — Minimum 1 sprint notice before the breaking change
4. **Create migration path** — Document how consumers should update their code

### Deprecation Notice Format

```markdown
## Deprecation Notice

**What**: Column `agents.name` will be renamed to `agents.display_name`
**When**: After [DATE] (minimum 1 sprint from notice)
**Impact**: All code reading `agents.name` must be updated to `agents.display_name`
**Migration**: See expand-contract pattern in compatibility.md
**Rollback**: View `agents_compat` provides backward compatibility during transition
```

### Rollback Capability

- Every breaking change must have a documented rollback plan
- Rollback must be testable in staging before production
- Data migration rollbacks require pre-migration backups

---

## API Versioning Strategy

### Current Approach: No Versioning (Single Version)

The ACE API currently uses a single version with no URL prefix or header-based versioning. This is appropriate for the current stage.

### Future Versioning (When Needed)

If breaking API changes are required:

| Strategy | Format | When to Use |
|----------|--------|-------------|
| URL prefix | `/v1/agents`, `/v2/agents` | Major version changes |
| Header | `Accept-Version: v2` | Minor version changes |
| Deprecation header | `Deprecation: true` + `Sunset: <date>` | All deprecations |

### Versioning Rules

1. **Never break the current version** without a deprecation period
2. **Document all version differences** in the OpenAPI spec
3. **Support at most 2 versions simultaneously** (current + deprecated)
4. **Set sunset dates** for deprecated versions

---

## Compatibility Checklist

Before deploying a breaking schema change:

- [ ] Expand-contract pattern is in the correct phase
- [ ] Backward compatibility view exists (if column rename)
- [ ] All consumers identified and notified
- [ ] Deprecation period elapsed (minimum 1 sprint)
- [ ] Pre-migration backup created
- [ ] Rollback plan tested in staging
- [ ] Documentation updated
- [ ] API response schema unchanged (or versioned)

---

## Notes

- The current codebase has minimal compatibility concerns (one table)
- As the codebase grows, these patterns become increasingly important
- The expand-contract pattern requires coordination between multiple PRs
- Always prefer forward fixes over rollbacks when possible
