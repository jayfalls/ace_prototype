# Migration and Rollback

<!--
Intent: Define database schema changes and rollback procedures.
Scope: All migrations needed, rollback strategies, and data migration scripts.
Used by: AI agents to safely modify the database schema and recover from failures.

NOTE: Remove this comment block in the final document
-->

## Overview
[Summary of database changes for this feature]

## Migrations

### Migration 1: [Name]
**Direction**: [UP/DOWN]
**Description**: [What this migration does]

```sql
-- Up Migration
-- Description: [What this does]

-- Create table
CREATE TABLE [table_name] (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    [column_1] [type] [constraints],
    [column_2] [type] [constraints],
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX [index_name] ON [table_name]([column]);

-- Add foreign keys
ALTER TABLE [table_name] 
ADD CONSTRAINT [constraint_name] 
FOREIGN KEY ([column]) REFERENCES [referenced_table]([column]);
```

```sql
-- Down Migration
-- Description: [How to revert]

DROP TABLE [table_name];
```

### Migration 2: [Name]
**Direction**: [UP/DOWN]
**Description**: [What this migration does]

## Data Migration
[If data needs to be migrated from existing structure]

```sql
-- Data migration script
INSERT INTO [new_table] ([columns])
SELECT [columns] 
FROM [old_table]
WHERE [conditions];
```

## Rollback Strategy

### Primary Rollback
| Step | Action | Command |
|------|--------|---------|
| 1 | [Action] | [Command] |
| 2 | [Action] | [Command] |

### Automatic Rollback
- **Tool**: [Alembic/Django/Manual]
- **Command**: [Command to rollback]

### Manual Rollback Procedures
[If automatic rollback is not available]

## Pre-Migration Checklist
- [ ] Backup database
- [ ] Test migration on staging
- [ ] Verify sufficient disk space
- [ ] Check for locks
- [ ] Notify users of downtime (if applicable)

## Post-Migration Checklist
- [ ] Verify data integrity
- [ ] Run application tests
- [ ] Check logs for errors
- [ ] Verify performance
- [ ] Update documentation

## Migration Dependencies
| Migration | Depends On |
|-----------|-----------|
| [Migration 1] | [Migration] |

## Rollback Dependencies
| Migration | Must Be Rolled Back With |
|-----------|-------------------------|
| [Migration 1] | [Migration] |