# Transaction Patterns

**FSD Requirement**: FR-3.4

---

## Overview

This document covers transaction usage patterns, isolation levels, and error handling for the ACE Framework's PostgreSQL data layer.

---

## Transaction Lifecycle

### Standard Pattern: BeginTx + defer Rollback

The standard Go/PostgreSQL transaction pattern:

```go
func (r *AgentRepo) CreateWithConfig(ctx context.Context, agent Agent, config AgentConfig) error {
    tx, err := r.pool.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback() // safe no-op after Commit

    // Create agent
    if err := r.queries.WithTx(tx).CreateAgent(ctx, agent); err != nil {
        return fmt.Errorf("create agent: %w", err)
    }

    // Create config
    if err := r.queries.WithTx(tx).CreateAgentConfig(ctx, config); err != nil {
        return fmt.Errorf("create config: %w", err)
    }

    return tx.Commit()
}
```

**Key points**:
- `defer tx.Rollback()` is called immediately after `BeginTx`
- If `Commit()` succeeds, `Rollback()` becomes a no-op (deferred call has no effect)
- If any operation fails, the function returns early and `Rollback()` executes automatically
- `context.Context` propagates cancellation into the transaction

### Transaction Helper Pattern

For operations requiring multiple repository calls, wrap the lifecycle in a helper:

```go
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    if err := fn(tx); err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

**Usage**:
```go
err := WithTransaction(ctx, pool, func(tx pgx.Tx) error {
    q := queries.WithTx(tx)
    if err := q.CreateAgent(ctx, agent); err != nil {
        return err
    }
    return q.CreateAgentConfig(ctx, config)
})
```

---

## Isolation Levels

### READ COMMITTED (Default)

```go
tx, err := pool.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelReadCommitted,
})
```

- **Default** PostgreSQL isolation level
- Each statement sees a consistent snapshot at statement start
- Suitable for most operations
- No phantom reads within a single statement

**Use when**: General CRUD operations, most business logic.

### REPEATABLE READ

```go
tx, err := pool.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelRepeatableRead,
})
```

- All statements in the transaction see the same snapshot (taken at first statement)
- Prevents non-repeatable reads and phantom reads
- Serialization errors possible on concurrent writes to the same rows

**Use when**: Multi-step reads that must be consistent (e.g., read-modify-write on multiple tables).

### SERIALIZABLE

```go
tx, err := pool.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
})
```

- Full serializability — transactions behave as if executed one at a time
- PostgreSQL uses SSI (Serializable Snapshot Isolation) — may raise serialization errors
- **Requires retry logic** for serialization failures

**Use when**: Critical sections (financial operations, inventory management, concurrent booking).

---

## Error Handling

### Deadlock Detection

PostgreSQL detects deadlocks automatically and aborts one transaction with SQLState `40P01`.

```go
func retryOnDeadlock(ctx context.Context, maxRetries int, fn func() error) error {
    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
            // Deadlock detected, retry
            if attempt < maxRetries {
                time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
                continue
            }
        }

        return err
    }
    return fmt.Errorf("max retries exceeded")
}
```

**Deadlock prevention**:
- Acquire locks in consistent order across all transactions
- Keep transactions short
- Ensure proper indexing on filter columns

### Constraint Violations

| SQLState | Constraint Type | API Response |
|----------|----------------|--------------|
| `23505` | Unique violation | 409 Conflict |
| `23503` | Foreign key violation | 404 Not Found |
| `23502` | Not null violation | 400 Bad Request |
| `23514` | Check constraint | 400 Bad Request |

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    switch pgErr.Code {
    case "23505":
        return nil, fmt.Errorf("conflict: %s", pgErr.Detail)
    case "23503":
        return nil, fmt.Errorf("not found: referenced resource does not exist")
    default:
        return nil, fmt.Errorf("database error: %w", err)
    }
}
```

### Context Cancellation

When a context is cancelled (client disconnect, timeout), the transaction is rolled back automatically:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

tx, err := pool.BeginTx(ctx, nil)
// If ctx is cancelled:
// - BeginTx returns context.Canceled
// - Any query on tx returns context.Canceled
// - Rollback is handled by defer
```

### Serialization Errors (SERIALIZABLE)

When using SERIALIZABLE isolation, serialization failures (SQLState `40001`) require retry:

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "40001" {
    // Serialization failure — retry the entire transaction
    return retryTransaction(ctx, pool, fn)
}
```

---

## SQLC Transaction Integration

Use `WithTx()` to run SQLC-generated queries within a transaction:

```go
tx, err := pool.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

qtx := queries.WithTx(tx)

// All queries through qtx participate in the transaction
agent, err := qtx.CreateAgent(ctx, params)
config, err := qtx.CreateAgentConfig(ctx, configParams)

return tx.Commit()
```

---

## Best Practices

1. **Always defer Rollback** immediately after BeginTx
2. **Keep transactions short** — minimize lock duration
3. **Use READ COMMITTED** unless you need stronger guarantees
4. **Retry deadlocks** — they are expected under concurrent load
5. **Handle context cancellation** — let the context propagate through the transaction
6. **Never swallow errors** — always check and return transaction errors
7. **Use explicit transaction options** when using non-default isolation levels
