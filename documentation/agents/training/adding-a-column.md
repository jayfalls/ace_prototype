# Training: Adding a Column to an Existing Table

**FSD Requirement**: FR-6.4

---

## Role

Guide for agents adding a new column to an existing table with backward compatibility.

---

## Prerequisites

- Understanding of conventions in `documentation/database-design/conventions.md`
- Understanding of migration patterns in `documentation/database-design/migrations.md`
- Understanding of backward compatibility in `documentation/database-design/compatibility.md`

---

## Step-by-Step Workflow

### Step 1: Determine Column Requirements

- Column name (must be `snake_case`)
- Data type (use standard types from `conventions.md`)
- Nullable or NOT NULL
- Default value (required for NOT NULL columns added to existing tables)
- Index requirements

### Step 2: Create the Migration

```bash
TIMESTAMP=$(date +%Y%m%d%H%M%S)
goose -dir backend/shared/telemetry/migrations create add_{column_name}_to_{table_name} go
```

Or create manually:

```
backend/shared/telemetry/migrations/{YYYYMMDDHHMMSS}_add_{column_name}_to_{table_name}.go
```

### Step 3: Implement the Migration

```go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upAdd{ColumnName}To{TableName}, downAdd{ColumnName}To{TableName})
}

func upAdd{ColumnName}To{TableName}(tx *sql.Tx) error {
    _, err := tx.Exec(`
        ALTER TABLE {table_name}
        ADD COLUMN {column_name} {TYPE} {NULLABILITY} {DEFAULT};
    `)
    return err
}

func downAdd{ColumnName}To{TableName}(tx *sql.Tx) error {
    _, err := tx.Exec(`
        ALTER TABLE {table_name}
        DROP COLUMN IF EXISTS {column_name};
    `)
    return err
}
```

### Step 4: Update SQLC Queries

If the new column should appear in queries, update the relevant `.sql` file and regenerate:

```bash
sqlc generate -f backend/services/api/sqlc.yaml
```

### Step 5: Update Schema Documentation

Update `documentation/database-design/schema/{group}/{table_name}.md` with the new column definition.

---

## Backward Compatibility Rules

- **Nullable columns**: Always safe to add. Existing queries continue to work.
- **NOT NULL columns**: Must include a `DEFAULT` value. Existing rows get the default.
- **New indexes**: Create concurrently in production: `CREATE INDEX CONCURRENTLY ...`
- **Dropping columns**: Never drop in the same migration as adding. Use expand-contract pattern.

---

## Conventions Checklist

Before committing, verify:

- [ ] Column name is `snake_case`, singular
- [ ] Type matches standard types in `conventions.md`
- [ ] NOT NULL columns have a DEFAULT
- [ ] Migration has descriptive `up`/`down` function names
- [ ] Migration file uses timestamp prefix
- [ ] SQLC regenerated if queries changed
- [ ] Schema documentation updated

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Adding NOT NULL without DEFAULT | Provide a DEFAULT value or make column nullable |
| Dropping column in same PR as add | Use expand-contract: add in PR1, migrate data in PR2, drop in PR3 |
| Forgetting `sqlc generate` | Run after every `.sql` change |
| Missing `down` function | Always implement `down` — `DROP COLUMN IF EXISTS` |
