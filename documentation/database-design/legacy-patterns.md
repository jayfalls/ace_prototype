# Legacy Patterns

**FSD Requirement**: FR-5.1

---

## Overview

This document catalogs legacy patterns identified in the existing codebase, their modern equivalents, and migration complexity. As the codebase currently has a single migration, the list is short — it will grow as more legacy patterns are discovered.

---

## Legacy Pattern Catalog

### 1. Numeric Filename Prefix

| Aspect | Detail |
|--------|--------|
| **Legacy** | `001_create_usage_events.go` |
| **Location** | `backend/shared/telemetry/migrations/` |
| **Modern** | `20260321000000_create_usage_events.go` |
| **Complexity** | Trivial |
| **Status** | **Fixed** — renamed in PR-0 |

The original migration used a numeric prefix (`001_`) instead of the documented standard timestamp prefix (`YYYYMMDDHHMMSS_`). This has been corrected.

**Why it matters**: Numeric prefixes cause merge conflicts when multiple developers create migrations simultaneously. Timestamp prefixes ensure unique, chronologically ordered filenames.

### 2. Bare `up`/`down` Function Names

| Aspect | Detail |
|--------|--------|
| **Legacy** | `up(tx *sql.Tx) error`, `down(tx *sql.Tx) error` |
| **Location** | `backend/shared/telemetry/migrations/` |
| **Modern** | `upCreateUsageEvents(tx *sql.Tx) error`, `downCreateUsageEvents(tx *sql.Tx) error` |
| **Complexity** | Trivial |
| **Status** | **Fixed** — renamed in PR-0 |

Generic `up`/`down` function names make it difficult to identify which migration a function belongs to in code search and stack traces. Descriptive names (`upCreateUsageEvents`) improve readability.

### 3. CREATE TABLE IF NOT EXISTS

| Aspect | Detail |
|--------|--------|
| **Legacy** | `CREATE TABLE IF NOT EXISTS usage_events (...)` |
| **Location** | `backend/shared/telemetry/migrations/` |
| **Modern** | `CREATE TABLE usage_events (...)` |
| **Complexity** | Trivial |
| **Status** | **Fixed** — changed in PR-0 |

Goose manages migration state — it knows which migrations have been applied. Using `IF NOT EXISTS` masks potential issues (e.g., a migration running when the table already exists from a different source). Plain `CREATE TABLE` will fail loudly if the table exists, which is the correct behavior.

The same applies to `CREATE INDEX IF NOT EXISTS` → `CREATE INDEX`.

### 4. `log.Fatalf` in Migrate Function

| Aspect | Detail |
|--------|--------|
| **Legacy** | `log.Fatalf("failed to run migrations: %v", err)` |
| **Location** | `backend/services/api/cmd/main.go` |
| **Modern** | `return fmt.Errorf("failed to run migrations: %w", err)` |
| **Complexity** | Trivial |
| **Status** | **Fixed** — changed in PR-0 |

`log.Fatalf` calls `os.Exit(1)` immediately, preventing proper cleanup (closing connections, flushing logs). Returning an error allows the caller to handle it gracefully.

---

## Pattern Summary

| # | Legacy Pattern | Modern Equivalent | Complexity | Status |
|---|---------------|-------------------|------------|--------|
| 1 | Numeric prefix (`001_`) | Timestamp prefix (`YYYYMMDDHHMMSS_`) | Trivial | Fixed |
| 2 | Bare `up`/`down` names | Descriptive names (`upCreateUsageEvents`) | Trivial | Fixed |
| 3 | `CREATE TABLE IF NOT EXISTS` | `CREATE TABLE` | Trivial | Fixed |
| 4 | `log.Fatalf` in `migrate` | `return fmt.Errorf` | Trivial | Fixed |

---

## Migration Complexity Definitions

| Level | Description | Risk |
|-------|-------------|------|
| Trivial | Naming/convention change, no schema impact | Zero — safe to apply immediately |
| Moderate | Requires data backfill or dual-write period | Low-Medium — needs testing |
| Complex | Breaking schema change, requires expand-contract | Medium-High — needs coordination |

---

## Notes

- All identified legacy patterns have been addressed in PR-0 (Phase 0: Code Cleanup)
- Future legacy patterns will be added here as the codebase grows
- Each pattern entry includes: current implementation, modern equivalent, location, complexity, and backward compatibility considerations
- Patterns are sorted by migration complexity (trivial first)
