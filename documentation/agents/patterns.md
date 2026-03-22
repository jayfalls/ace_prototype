# Agent Patterns

**FSD Requirement**: FR-6.3

---

## Role

Quick reference for agents generating database code. Constraint-first format — constraints are listed first, patterns second.

---

## Naming Conventions Summary

| Object | Convention | Example |
|--------|-----------|---------|
| Tables | `snake_case`, plural | `agents`, `memory_nodes` |
| Columns | `snake_case`, singular | `created_at`, `user_id` |
| Indexes | `idx_{table}_{columns}` | `idx_agents_user_id` |
| Unique constraints | `uq_{table}_{columns}` | `uq_agents_name` |
| Foreign keys | `fk_{table}_{column}` | `fk_usage_events_agent_id` |
| Triggers | `set_{table}_updated_at` | `set_agents_updated_at` |

---

## Data Type Defaults

| Use Case | Type | Default |
|----------|------|---------|
| Primary key | `UUID` | `gen_random_uuid()` |
| Timestamp | `TIMESTAMPTZ` | `NOW()` |
| Status/enum | `VARCHAR(50)` | explicit default |
| Name/label | `VARCHAR(255)` | — |
| Description | `TEXT` | — |
| JSON data | `JSONB` | — |
| Boolean | `BOOLEAN` | `DEFAULT false` |
| Money | `DECIMAL(10,6)` | — |
| Count/duration | `BIGINT` | — |

---

## ALWAYS Rules

1. **ALWAYS** use `gen_random_uuid()` for new table primary keys
2. **ALWAYS** include `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` in all tables
3. **ALWAYS** include `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` in mutable tables
4. **ALWAYS** create `set_{table}_updated_at` trigger for mutable tables
5. **ALWAYS** include `agent_id UUID NOT NULL` in agent-attributed tables
6. **ALWAYS** create `idx_{table}_agent_id` on agent-attributed tables
7. **ALWAYS** use snake_case for all SQL identifiers
8. **ALWAYS** include `down` function in migrations (even if no-op for data migrations)
9. **ALWAYS** use `TIMESTAMPTZ` — never `TIMESTAMP`
10. **ALWAYS** run `sqlc generate` after modifying `.sql` query files
11. **ALWAYS** commit generated SQLC files alongside `.sql` changes

---

## NEVER Rules

1. **NEVER** hand-edit files in `internal/repository/generated/`
2. **NEVER** use `CREATE TABLE IF NOT EXISTS` (Goose manages existence)
3. **NEVER** use `CREATE INDEX IF NOT EXISTS` (same reason)
4. **NEVER** use numeric filename prefixes (`001_`) — use timestamps
5. **NEVER** use bare `up`/`down` function names — use descriptive names
6. **NEVER** use `timestamp without time zone`
7. **NEVER** use `VARCHAR` without length constraint (except `TEXT`)
8. **NEVER** skip the `down` function in migrations
9. **NEVER** run `goose down` in production without explicit approval
10. **NEVER** use `DROP TABLE` without a backup or rollback plan

---

## Error Handling Patterns

### PostgreSQL Errors → API Response

| SQLState | Error | API Status | API Code |
|----------|-------|------------|----------|
| `23505` | Unique violation | 409 | `conflict` |
| `23503` | Foreign key violation | 404 | `not_found` |
| `23502` | Not null violation | 400 | `bad_request` |
| `23514` | Check constraint | 400 | `bad_request` |
| `40P01` | Deadlock | 409 | `conflict` (retry) |

### Response Pattern

```go
response.Success(w, data)         // 200
response.Created(w, data)         // 201
response.BadRequest(w, code, msg) // 400
response.ValidationError(w, err)  // 400 + field details
response.NotFound(w, msg)         // 404
response.InternalError(w, msg)    // 500
```

---

## Transaction Pattern

```go
tx, err := pool.BeginTx(ctx, nil)
if err != nil { return err }
defer tx.Rollback()

qtx := queries.WithTx(tx)
// run queries with qtx...

return tx.Commit()
```

---

## Decision Trees

### Creating a new table?

```
Include id (UUID, gen_random_uuid())
  → Include agent_id (UUID NOT NULL) if agent-attributed
    → Include created_at, updated_at (TIMESTAMPTZ)
      → Add trigger set_{table}_updated_at
        → Add indexes for foreign keys
          → Create migration with up + down
            → Create SQLC queries
              → Run sqlc generate
                → Test migration locally
                  → Commit
```

### Modifying an existing column?

```
Is it a rename?
  YES → Use expand-contract (add new, backfill, migrate code, drop old)
  NO  → Is it a type change?
          YES → Expand-contract with validation
          NO  → Direct ALTER with default for NOT NULL
```

---

## References

- Full conventions: `documentation/database-design/conventions.md`
- Error codes: `documentation/api/errors.md`
- SQLC workflow: `documentation/database-design/sqlc.md`
- Migration patterns: `documentation/database-design/migrations.md`
- API reference: `documentation/agents/api-reference.md`
