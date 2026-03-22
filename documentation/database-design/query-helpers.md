# Query Helpers

**FSD Requirement**: FR-3.5

---

## Overview

This document catalogs reusable query helper functions and patterns. This is an **initial stub** — the codebase currently has one table (`usage_events`) and no SQLC queries. This document will expand as query helpers are implemented.

---

## Trigger Functions

### update_updated_at()

A standard PostgreSQL trigger function that automatically sets `updated_at` to `NOW()` on row update:

```sql
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

**Usage**: Add a trigger to any table with an `updated_at` column:

```sql
CREATE TRIGGER set_{table}_updated_at
    BEFORE UPDATE ON {table}
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
```

**Note**: This function should be created in a dedicated migration when the first table requires it.

---

## Proposed Patterns

The following patterns are proposed for implementation as the codebase grows:

### Pagination Helpers

#### Cursor-Based Pagination

A helper function or SQLC pattern for cursor-based pagination:

```sql
-- Base pattern (adapt per table)
SELECT * FROM {table}
WHERE (sort_column, id) < ($1, $2)
ORDER BY sort_column DESC, id DESC
LIMIT $3;
```

**Go helper proposal**:
```go
type Cursor struct {
    Value interface{}
    ID    uuid.UUID
    Limit int32
}
```

#### Offset-Based Pagination

For small datasets or admin dashboards:

```sql
SELECT * FROM {table}
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
```

**Go helper proposal**:
```go
type PageParams struct {
    Limit  int32
    Offset int32
}
```

### Filtering Helpers

#### Dynamic Filter Builder

A Go helper for building dynamic WHERE clauses:

```go
type FilterBuilder struct {
    conditions []string
    args       []interface{}
    argIndex   int
}

func (f *FilterBuilder) Add(column string, value interface{}) {
    f.argIndex++
    f.conditions = append(f.conditions, fmt.Sprintf("%s = $%d", column, f.argIndex))
    f.args = append(f.args, value)
}

func (f *FilterBuilder) Where() string {
    if len(f.conditions) == 0 {
        return ""
    }
    return "WHERE " + strings.Join(f.conditions, " AND ")
}
```

### Sorting Helpers

#### Safe Sort Builder

Prevent SQL injection in ORDER BY clauses:

```go
var allowedSortColumns = map[string]string{
    "created_at": "created_at",
    "name":       "name",
    "status":     "status",
}

func SafeOrderBy(column, direction string) string {
    col, ok := allowedSortColumns[column]
    if !ok {
        col = "created_at"
    }
    dir := "DESC"
    if strings.ToUpper(direction) == "ASC" {
        dir = "ASC"
    }
    return fmt.Sprintf("ORDER BY %s %s", col, dir)
}
```

---

## Gap Analysis

| Pattern | Status | Priority | Notes |
|---------|--------|----------|-------|
| `update_updated_at()` trigger | Not implemented | High | Needed when `updated_at` columns are added |
| Cursor pagination | Not implemented | High | Needed for list endpoints |
| Offset pagination | Not implemented | Medium | Needed for admin dashboards |
| Dynamic filter builder | Not implemented | Medium | Needed for search/filter endpoints |
| Safe sort builder | Not implemented | Medium | Needed for sortable list endpoints |
| Batch insert helper | Not implemented | Low | Needed for high-volume data ingestion |
| Upsert helper | Not implemented | Low | Needed for idempotent operations |

---

## Notes

- This document starts as a stub and expands as the codebase grows
- New helpers should be cataloged here with function signatures and usage examples
- Each helper should integrate with SQLC-generated code where possible
- Helpers should follow the naming conventions in `conventions.md`
