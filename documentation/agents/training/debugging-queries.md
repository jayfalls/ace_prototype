# Training: Debugging Database Queries

**FSD Requirement**: FR-6.4

---

## Role

Investigation approach for agents encountering database query issues: slow queries, incorrect results, connection errors, and migration failures.

---

## Prerequisites

- Understanding of conventions in `documentation/database-design/conventions.md`
- Understanding of SQLC workflow in `documentation/database-design/sqlc.md`
- Understanding of connection pooling in `documentation/database-design/connection-pooling.md`

---

## Investigation Workflow

### Step 1: Identify the Error Category

| Category | Symptoms | Starting Point |
|----------|----------|----------------|
| Query syntax | SQLC generation fails, SQL error in logs | Check `.sql` annotation syntax |
| Type mismatch | `Scan` errors, `cannot assign` errors | Compare SQLC param types to column types |
| Slow query | Timeouts, high latency | Check indexes, `EXPLAIN ANALYZE` |
| Connection | `too many connections`, `connection refused` | Check pool config, DB health |
| Migration | `goose up` fails, schema drift | Check migration order, conflicts |

### Step 2: Check SQLC Annotation Syntax

Common annotation errors:

```sql
-- WRONG: missing return type
-- name: GetEvent SELECT * FROM usage_events WHERE id = $1;

-- CORRECT:
-- name: GetEvent :one
SELECT * FROM usage_events WHERE id = $1;
```

Valid return types: `:one`, `:many`, `:exec`, `:execrows`

### Step 3: Verify Parameter Binding

```sql
-- WRONG: parameter count mismatch
-- name: CreateEvent :one
INSERT INTO usage_events (agent_id, service_name) VALUES ($1);

-- CORRECT:
-- name: CreateEvent :one
INSERT INTO usage_events (agent_id, service_name) VALUES ($1, $2)
RETURNING *;
```

### Step 4: Diagnose Slow Queries

```sql
-- Check query plan
EXPLAIN ANALYZE
SELECT * FROM usage_events WHERE agent_id = $1 ORDER BY timestamp DESC;
```

Look for:
- `Seq Scan` on large tables (missing index)
- `Sort` with high cost (missing index for ORDER BY)
- `Nested Loop` with high row estimates (join strategy)

### Step 5: Check Index Coverage

Reference `documentation/database-design/indexes.md` and verify the query uses available indexes:

```sql
-- This query needs idx_usage_events_agent_id
SELECT * FROM usage_events WHERE agent_id = $1;

-- This query needs a composite index on (agent_id, timestamp)
SELECT * FROM usage_events
WHERE agent_id = $1
ORDER BY timestamp DESC;
```

### Step 6: Verify Migration State

```bash
# Check current migration status
goose -dir backend/shared/telemetry/migrations status

# Check if migration ran
goose -dir backend/shared/telemetry/migrations up
```

---

## Debugging Tools

### Go Test with Verbose Output

```bash
go test -v -run TestSpecificFunction ./...
```

### SQLC Generation Debugging

```bash
# Verify sqlc config
sqlc generate -f backend/services/api/sqlc.yaml

# Check for parse errors in .sql files
sqlc vet -f backend/services/api/sqlc.yaml
```

### Database Connection Check

```go
// In handler or health check
err := pool.Ping(ctx)
if err != nil {
    log.Printf("DB connection failed: %v", err)
}
```

---

## Common Issues & Fixes

| Issue | Cause | Fix |
|-------|-------|-----|
| `Scan: converting NULL` | Nullable column scanned to non-nullable type | Use `pgtype.*` types for nullable columns |
| `too many connections` | Pool exhaustion | Check `MaxOpenConns`, connection leaks |
| `relation does not exist` | Migration not run | Run `goose up` |
| `column does not exist` | Schema drift | Compare migration DDL to query |
| Slow `OFFSET` queries | Linear scan | Switch to cursor-based pagination |
| `deadlock detected` | Concurrent transactions | Retry with backoff, review lock order |

---

## Conventions Checklist

When fixing query issues, verify:

- [ ] SQLC annotations correct (`:one`, `:many`, `:exec`, `:execrows`)
- [ ] Parameter count matches placeholders ($1, $2, ...)
- [ ] Column types match Go types (or use `pgtype.*` for nullable)
- [ ] Indexes exist for WHERE and ORDER BY columns
- [ ] Migration state is consistent (`goose status`)
- [ ] Connection pool settings appropriate for load
- [ ] No hand-edited SQLC generated files
