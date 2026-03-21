# Implementation Plan — Database Design Documentation Unit

## Overview

This is the step-by-step execution plan for producing all documentation artifacts defined by the database-design unit. It is **not** a plan for building runtime code — it is a plan for creating 22+ documentation files across `documentation/database-design/`, `documentation/api/`, `documentation/agents/`, and `tests/agent-integration/`, preceded by a code cleanup phase to bring existing code in line with documented standards.

Each micro-PR produces one or more artifacts, is independently reviewable, and includes verification criteria. Tasks are ordered by the dependency graph defined in `architecture.md`.

---

## Phase 0: Code Cleanup & Migration

Before producing documentation, migrate the existing codebase to the new documented standards. This ensures the code matches the conventions documented in subsequent phases.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 0.1 | Migrate existing migration file to new naming/function conventions | None |
| 0.2 | Fix `migrate` function error handling in `main.go` | None |
| 0.3 | Add documentation comments to existing migration and `migrate` function | 0.1, 0.2 |

### PR-0: Code Cleanup & Migration

**Branch**: `feature/docs-migrate-existing-code`
**Artifacts**: Modified `backend/shared/telemetry/migrations/001_create_usage_events.go`, `backend/services/api/cmd/main.go`

**Task 0.1 — Migrate existing migration file:**
- Rename `backend/shared/telemetry/migrations/001_create_usage_events.go` → `backend/shared/telemetry/migrations/20260321000000_create_usage_events.go` (timestamp prefix per FR-4.1)
- Rename `up` → `upCreateUsageEvents`, `down` → `downCreateUsageEvents` (descriptive names)
- Change `CREATE TABLE IF NOT EXISTS` → `CREATE TABLE` (Goose manages table existence)
- Change `CREATE INDEX IF NOT EXISTS` → `CREATE INDEX` (same reason)
- Update `goose.AddMigration(upCreateUsageEvents, downCreateUsageEvents)` to use new function names

**Task 0.2 — Fix `migrate` function error handling:**
- In `backend/services/api/cmd/main.go`, change `log.Fatalf` calls in `migrate` function to `return fmt.Errorf` pattern
- Update `migrate` signature to return `error`
- Handle returned error in `main()` with proper logging and shutdown

**Task 0.3 — Add documentation comments:**
- Add Go doc comment to the migration file explaining `usage_events` table purpose
- Add `COMMENT ON TABLE usage_events IS '...'` in the migration up function
- Document the `migrate` function purpose and its role in the startup sequence

**Verification**:
- `make test` passes
- Migration runs against fresh database without errors
- `goose up` / `goose down` roundtrip works with new function names
- No behavioral changes — only naming and error handling improvements

---

## Phase 1: Tooling Setup

Sets up the documentation generation pipeline (`make docs` target) before any artifacts are produced. This enables automated validation from the start.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 0.1 | Create `scripts/docs-gen/main.go` — entry point that orchestrates all doc generation | None |
| 0.2 | Create `scripts/schema-doc-gen/main.go` — extracts schema from `pg_catalog`, outputs markdown | None |
| 0.3 | Create `scripts/erd-gen/main.go` — generates Mermaid ERD from FK metadata | 0.2 |
| 0.4 | Create `scripts/openapi-gen/` — runs Annot8 to generate `openapi.yaml` | None |
| 0.5 | Create `scripts/validate-docs/main.go` — compares live schema vs docs, fails on drift | 0.2 |
| 0.6 | Add `docs` target to Makefile — wraps `go run ./scripts/docs-gen/` | 0.1 |

**Note**: Task 0.2 is the most complex script in the tooling — it must handle `pg_catalog` extraction with proper error handling, connection management, and formatted markdown output.

### PR-1: Tooling Foundation

**Branch**: `feature/docs-gen-tooling`
**Artifacts**: `scripts/`, Makefile change
**Verification**:
- `make docs` runs without error (even if no docs exist yet)
- `scripts/schema-doc-gen/` connects to DB and extracts table list
- `scripts/validate-docs/` compares schema state against empty docs dir (reports missing docs, not crashes)

---

## Phase 2: Foundation Documents (Batch 1)

These three documents have no dependencies on each other and can be produced in parallel. They form the foundation for all subsequent documentation.

### PR-2: Conventions Document

**Branch**: `feature/docs-conventions`
**FSD Requirements**: FR-3.1, FR-3.2
**Artifacts**: `documentation/database-design/conventions.md`

**Contents**:
- Table naming: `snake_case`, plural nouns (`agents`, `memory_nodes`)
- Column naming: `snake_case`, descriptive (`created_at`, `user_id`, `is_active`)
- Index naming: `idx_{table}_{columns}` (`idx_agents_user_id`)
- Constraint naming: `{type}_{table}_{columns}` (`fk_agents_user_id`, `uq_agents_name`)
- Trigger naming: `set_{table}_updated_at`
- Standard types: `UUID DEFAULT gen_random_uuid()`, `TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `VARCHAR(50)`, `VARCHAR(255)`, `TEXT`, `JSONB`, `BOOLEAN NOT NULL DEFAULT false`, `DECIMAL` for money
- PostgreSQL features guidance: JSONB vs normalized, arrays, CTEs, partial indexes

**Verification**:
- File exists at `documentation/database-design/conventions.md`
- All naming conventions documented with examples
- Standard data types table present
- Referenced by `architecture.md` generation order

---

### PR-3: Schema Documentation (per-entity)

**Branch**: `feature/docs-schema`
**FSD Requirements**: FR-1.1
**Artifacts**: `documentation/database-design/schema/{agents,memory,tools,messaging,usage,system}/` — one markdown file per table

**Note**: The FSD specifies a flat structure (`schema/{table}.md`). This implementation uses entity-group subdirectories (`schema/{agents,memory,...}/{table}.md`) as a design improvement — better organization as the number of tables grows, and aligns with the entity groupings in `architecture.md`.

**Contents per table file**:
- Table name and purpose
- Column definitions (name, type, nullable, default, constraints)
- Primary key strategy
- Foreign key relationships
- Indexes (name, columns, type)
- Triggers
- Soft-delete columns where applicable
- Cross-referenced SQLC queries

**Current tables to document** (from existing migrations):
- `usage_events` (from `backend/shared/telemetry/migrations/001_create_usage_events.go`)

As new migrations are added, new table docs are generated by `scripts/schema-doc-gen/`.

**Verification**:
- Directory structure matches entity groupings from `architecture.md`
- `usage_events.md` matches the actual migration DDL
- Column types match PostgreSQL type names
- Indexes match migration CREATE INDEX statements
- `make docs` validates schema docs against live DB without discrepancies

---

### PR-4: OpenAPI Specification (initial)

**Branch**: `feature/docs-openapi`
**FSD Requirements**: FR-2.1
**Artifacts**: `documentation/api/openapi.yaml`

**Contents**:
- OpenAPI 3.1.0 header and server config
- Paths for all current endpoints (from `backend/services/api/cmd/main.go`):
  - `GET /` — root
  - `GET /health/live` — liveness probe
  - `GET /health/ready` — readiness probe
  - `GET /health/exporters` — exporter health
  - `GET /metrics` — Prometheus metrics
  - `POST /examples/` — create example
  - `GET /examples/{id}` — get example
- Components: `APIResponse`, `APIError`, `FieldError`, request/response schemas
- Security scheme: `BearerAuth` (JWT placeholder, all endpoints currently public)
- Example request/response for each endpoint

Generated by `scripts/openapi-gen/` using Annot8 annotations on handler code.

**Verification**:
- Valid YAML parseable by `swagger-cli validate` or `redocly lint`
- All 7 current endpoints present
- Response envelope schema matches `response.APIResponse` struct
- `make docs` passes OpenAPI validation

---

## Phase 3: Derived Documents (Batch 2)

These documents depend on Phase 2 foundations. They can be produced in parallel once Phase 2 is complete.

### PR-5: ERD Diagrams

**Branch**: `feature/docs-erd`
**FSD Requirements**: FR-1.2
**Artifacts**: `documentation/database-design/erd/{agents,memory,tools,messaging,usage,system}.md` + `erd/master.md`

**Contents per ERD file**:
- Mermaid `erDiagram` syntax showing tables and relationships
- Text-based relationship description
- Cardinality annotations (1:1, 1:N)
- Cascade delete and soft delete patterns

**Master ERD**: Shows all entity groups and cross-group relationships.

**Verification**:
- All 6 entity group ERD files exist
- `master.md` shows cross-group FK relationships
- Mermaid syntax valid (verified by `mmdc` in `make docs`)
- FK relationships match schema docs

---

### PR-6: Index Strategy

**Branch**: `feature/docs-indexes`
**FSD Requirements**: FR-1.3
**Artifacts**: `documentation/database-design/indexes.md`

**Contents**:
- Index catalog by table (name, columns, type, rationale)
- Classification: B-tree, partial, composite, GIN/GiST
- Query pattern each index supports
- Missing index recommendations based on SQLC query patterns

**Verification**:
- Every index from migrations documented
- Index types classified
- Query pattern → index mapping present

---

### PR-7: Query Patterns

**Branch**: `feature/docs-query-patterns`
**FSD Requirements**: FR-1.4
**Artifacts**: `documentation/database-design/query-patterns/{agents,memory,tools,messaging,usage}.md`

**Contents per category**:
- CRUD operations with SQLC annotation examples
- Filtering patterns (WHERE clauses, dynamic filters)
- Pagination patterns (cursor-based vs offset-based with performance trade-offs)
- Join patterns (INNER, LEFT)
- Aggregation patterns (GROUP BY, window functions)
- PostgreSQL-specific features (JSONB, arrays, CTEs)

**Verification**:
- All 5 category files exist
- CRUD patterns include SQL + SQLC annotation + Go usage
- Pagination section covers both cursor and offset with index requirements
- `usage.md` references the actual `usage_events` table queries

---

### PR-8: SQLC Workflow

**Branch**: `feature/docs-sqlc`
**FSD Requirements**: FR-2.3
**Artifacts**: `documentation/database-design/sqlc.md`

**Contents**:
- `sqlc.yaml` configuration reference (from `backend/services/api/sqlc.yaml`)
- Query file organization convention (one `.sql` per domain)
- Annotation syntax: `-- name: FunctionName :one/:many/:exec/:execrows`
- Parameter binding patterns
- Generation workflow: edit `.sql` → `sqlc generate` → use generated Go code
- Generated output files: `db.go`, `models.go`, `querier.go`, `*.sql.go`
- Note on `emit_interface: false` behavior
- Common patterns: named queries, batch operations, JSONB handling

**Verification**:
- Config table matches actual `sqlc.yaml`
- Annotation examples are syntactically correct
- Generation workflow steps are correct
- Generated file descriptions match actual output

---

### PR-9: Connection Pooling

**Branch**: `feature/docs-connection-pooling`
**FSD Requirements**: FR-3.3
**Artifacts**: `documentation/database-design/connection-pooling.md`

**Contents**:
- Pool parameters: `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`
- Recommended values by environment (dev vs prod)
- Relationship to PostgreSQL `max_connections`
- Connection leak detection patterns
- Health check pattern (`db.Ping()` in `/health/ready`)
- `pgxpool` configuration (from `backend/services/api/internal/repository/db.go`)

**Verification**:
- All pool parameters documented with recommendations
- Environment-specific configs specified
- Health check pattern references existing `/health/ready` endpoint

---

### PR-10: Transaction Patterns

**Branch**: `feature/docs-transactions`
**FSD Requirements**: FR-3.4
**Artifacts**: `documentation/database-design/transactions.md`

**Contents**:
- Transaction lifecycle: `db.BeginTx()` → deferred rollback → commit
- Isolation levels: `READ COMMITTED` (default), `REPEATABLE READ`, `SERIALIZABLE`
- Error handling: deadlock detection/retry, constraint violations, context cancellation
- Code examples for common patterns

**Verification**:
- Transaction lifecycle documented with Go code examples
- Isolation levels explained with use cases
- Error handling patterns documented

---

### PR-11: Query Helpers

**Branch**: `feature/docs-query-helpers`
**FSD Requirements**: FR-3.5
**Artifacts**: `documentation/database-design/query-helpers.md`

**Note**: The codebase currently has only one migration and no SQLC queries. This PR will produce an initial stub documenting the `update_updated_at()` trigger function and proposed patterns (pagination, filtering, sorting). The document expands as the codebase grows and query helpers are implemented.

**Contents**:
- Catalog of existing helper functions (pagination, filtering, sorting)
- Function signatures and usage patterns
- `update_updated_at()` trigger function documentation
- SQLC integration notes
- Gap analysis with proposed new helpers

**Verification**:
- Existing helpers cataloged
- Trigger function documented
- Usage examples provided

---

### PR-12: Migration Documentation

**Branch**: `feature/docs-migrations`
**FSD Requirements**: FR-4.1, FR-4.2, FR-4.3, FR-4.4
**Artifacts**: `documentation/database-design/migrations.md`

**Contents**:
- Goose v3 framework reference (`init()`, `goose.AddMigration(up, down)`)
- File naming convention: `YYYYMMDDHHMMSS_description.go`
- Migration template (Go function skeleton)
- Forward-only strategy rationale
- Migration workflow: create → implement → test → commit
- Goose command reference (`goose up`, `goose down`, `goose status`)
- Rollback scenarios (pre-deploy, post-deploy, data migration) with decision tree
- `down` function requirements (idempotent, tested, data-loss aware)
- Pre/post-migration checklists
- Testing patterns: unit (up/down execute), integration (schema state), rollback (up+down roundtrip)
- CI/CD integration: pre-commit hooks, automated testing, schema dump validation
- Schema versioning: `goose_db_version` table, deployment sequence, concurrent migration handling

**Verification**:
- All FR-4.1 through FR-4.4 sections present
- Rollback decision tree matches research.md §11.1
- Testing patterns include Go code examples
- Migration template matches design/README.md pattern
- Legacy naming note references `001_create_usage_events.go` numeric prefix

---

## Phase 4: Cross-Reference Documents (Batch 3)

These documents synthesize information from Phase 2 and Phase 3 artifacts.

### PR-13: Endpoint-to-DB Mapping

**Branch**: `feature/docs-endpoint-map`
**FSD Requirements**: FR-2.2
**Artifacts**: `documentation/api/endpoint-map.md`

**Contents**:
- Call chain pattern: Handler → Service → Repository → SQLC
- Mapping table: endpoint, handler, service, repository, SQLC query, transaction
- Data transformation documentation (request JSON → Go struct → SQLC params → SQL)
- SQLC file references and generated Go function names

Current mappings (from codebase):
- `POST /examples/` → `ExampleHandler.Create` (inline, no DB)
- `GET /examples/{id}` → `ExampleHandler.Get` (inline, no DB)
- `GET /health/ready` → `HealthHandler.Ready` (direct `pool.Ping()`)

**Verification**:
- All current endpoints mapped
- Call chain Mermaid diagram present
- SQLC references point to actual files

---

### PR-14: Error Codes

**Branch**: `feature/docs-errors`
**FSD Requirements**: FR-2.4
**Artifacts**: `documentation/api/errors.md`

**Contents**:
- Error code catalog with HTTP status mappings (from `response.go`)
- Database-to-API error mapping (PostgreSQL SQLState → API error code)
- Validation error format (`response.ValidationError`)
- Client handling guidance per error code

**Verification**:
- All `response.*` helper functions documented with error codes
- DB error mapping covers unique violation, FK violation, not null, check constraint
- Validation error format matches `FieldError` struct

---

### PR-15: Legacy Patterns

**Branch**: `feature/docs-legacy-patterns`
**FSD Requirements**: FR-5.1
**Artifacts**: `documentation/database-design/legacy-patterns.md`

**Contents**:
- Legacy pattern catalog with migration complexity scoring
- Current implementation → modern equivalent for each pattern
- Backward compatibility considerations
- Identified legacy patterns from codebase:
  - Numeric filename prefix (`001_create_usage_events.go`) → timestamp prefix
  - Bare `up`/`down` function names → descriptive names
  - `CREATE TABLE IF NOT EXISTS` → `CREATE TABLE` (Goose manages existence)

**Verification**:
- All legacy patterns identified
- Modern equivalents documented
- Migration complexity assessed per pattern

---

### PR-16: Agent API Reference

**Branch**: `feature/docs-agent-api-ref`
**FSD Requirements**: FR-6.1
**Artifacts**: `documentation/agents/api-reference.md`

**Contents**:
- Machine-parseable endpoint summaries in AGENTS.md format
- Role: what this helps the agent do
- Project knowledge: key API facts
- Commands: how to run/test endpoints
- Boundaries: NEVER/ALWAYS constraints for API code generation
- Example code generation templates

**Verification**:
- Follows AGENTS.md structure (role, knowledge, commands, style, boundaries, examples, validation)
- References OpenAPI spec endpoints
- Code templates include agentId attribution

---

### PR-17: Agent Schema Generation Guide

**Branch**: `feature/docs-agent-schema-gen`
**FSD Requirements**: FR-6.2
**Artifacts**: `documentation/agents/schema-generation.md`

**Contents**:
- SQLC/Goose generation workflows for agents
- Schema-aware code generation patterns
- Migration file template with agentId inclusion
- SQLC query template with annotation syntax
- AgentId attribution: always include `agent_id UUID NOT NULL` + FK + index

**Verification**:
- Follows AGENTS.md structure
- Migration template matches design/README.md pattern
- SQLC template matches actual sqlc.yaml config
- agentId pattern documented

---

### PR-18: Agent Patterns

**Branch**: `feature/docs-agent-patterns`
**FSD Requirements**: FR-6.3
**Artifacts**: `documentation/agents/patterns.md`

**Contents**:
- Constraint-first quick reference
- Naming conventions summary
- Data type defaults
- ALWAYS/NEVER rules for database code
- Error handling patterns
- Reference to conventions.md, errors.md

**Verification**:
- Follows AGENTS.md structure
- Constraints match conventions.md
- Error codes match errors.md

---

## Phase 5: Derivative Documents (Batch 4)

These depend on Phase 4 outputs and must be sequential.

### PR-19: Migration Plan

**Branch**: `feature/docs-migration-plan`
**FSD Requirements**: FR-5.2, FR-5.3
**Artifacts**: `documentation/database-design/migration-plan.md`

**Contents**:
- Four-phase migration plan:
  - Phase 1: Non-breaking (index renames, comments) — Low risk
  - Phase 2: Low-risk (add constraints, standardize types) — Low-Medium risk
  - Phase 3: Moderate (column renames with views) — Medium risk
  - Phase 4: Breaking (table restructures, data migrations) — High risk
- Per-phase: scope, migration steps, testing requirements, rollback plan, dependencies
- Refactoring guidelines: rename columns, change types, add constraints, split/merge tables
- PR requirements for schema changes

**Verification**:
- All 4 phases defined with scope
- Each phase has testing and rollback plans
- Refactoring patterns documented with examples
- Depends on legacy-patterns.md (PR-15) and conventions.md (PR-2)

---

### PR-20: Backward Compatibility

**Branch**: `feature/docs-compatibility`
**FSD Requirements**: FR-5.4
**Artifacts**: `documentation/database-design/compatibility.md`

**Contents**:
- Compatibility rules (API response stability during migration)
- Database view creation for renamed columns (with SQL examples)
- Expand-contract pattern (4 phases with code examples)
- Breaking change process: deprecation notice period, migration path, rollback capability
- Versioning strategy for API-impacting schema changes

**Verification**:
- Expand-contract pattern documented with 4 phases
- View compatibility examples present
- Breaking change process defined
- Depends on migration-plan.md (PR-19)

---

## Phase 6: Agent Training & Integration (Batch 5)

Final phase — depends on all previous phases.

### PR-21: Agent Training Materials

**Branch**: `feature/docs-agent-training`
**FSD Requirements**: FR-6.4
**Artifacts**: `documentation/agents/training/{adding-a-table.md,adding-a-column.md,new-endpoint.md,debugging-queries.md}`

**Contents per scenario**:
- `adding-a-table.md`: Step-by-step guide for creating a new table (migration → SQLC → handler → test)
- `adding-a-column.md`: Safe column addition with backward compatibility
- `new-endpoint.md`: Full workflow (handler → service → repository → SQLC → OpenAPI annotation)
- `debugging-queries.md`: Investigation approach for query issues

Each follows AGENTS.md structure with explicit steps, code templates, and validation checks.

**Verification**:
- All 4 scenario files exist
- Each follows AGENTS.md structure
- Code templates reference actual patterns from conventions.md, sqlc.md
- agentId inclusion covered in all scenarios

---

### PR-22: Agent Config Updates

**Branch**: `feature/docs-agent-config`
**FSD Requirements**: FR-6.5
**Artifacts**: `documentation/agents/config-updates.md`

**Contents**:
- Process for adding documentation paths to agent tool context directories
- Agent configuration YAML template
- Integration test specifications
- Rollback procedure for config changes

**Verification**:
- Config YAML template present
- Integration test specs defined
- References all agent doc paths

---

### PR-23: Agent Integration Tests

**Branch**: `feature/docs-agent-tests`
**FSD Requirements**: FR-6.5
**Artifacts**: `tests/agent-integration/` — Go test files

**Contents**:
- `TestLoadAPIDocs` — asserts `documentation/api/openapi.yaml` is parseable and valid YAML
- `TestAgentPatternCompliance` — asserts a generated migration includes `agent_id UUID NOT NULL` with FK to `agents` table and index `idx_{table}_agent_id`
- `TestSchemaDocsExist` — asserts all entity-group directories (`schema/agents/`, `schema/memory/`, etc.) contain at least one markdown file
- `TestNamingConventions` — asserts generated table/column names follow `snake_case` convention
- `TestSQLCAnnotations` — asserts generated SQLC queries use correct annotation syntax (`-- name: FunctionName :one/:many/:exec`)

**Verification**:
- Test files compile and run: `go test ./tests/agent-integration/...`
- `TestLoadAPIDocs` passes against current `openapi.yaml`
- `TestAgentPatternCompliance` validates against sample migration output
- `TestSchemaDocsExist` passes with current schema doc directories

---

## Implementation Checklist

- [ ] **Phase 0: Code Cleanup & Migration**
  - [ ] PR-0: Migrate `001_create_usage_events.go` to timestamp naming, fix `migrate` error handling

- [ ] **Phase 1: Tooling**
  - [ ] PR-1: `scripts/docs-gen/`, `scripts/schema-doc-gen/`, `scripts/erd-gen/`, `scripts/openapi-gen/`, `scripts/validate-docs/`, Makefile `docs` target

- [ ] **Phase 2: Foundations**
  - [ ] PR-2: `conventions.md`
  - [ ] PR-3: `schema/{entity}/` (per-table docs)
  - [ ] PR-4: `openapi.yaml`

- [ ] **Phase 3: Derived**
  - [ ] PR-5: `erd/` (6 entity groups + master)
  - [ ] PR-6: `indexes.md`
  - [ ] PR-7: `query-patterns/` (5 categories)
  - [ ] PR-8: `sqlc.md`
  - [ ] PR-9: `connection-pooling.md`
  - [ ] PR-10: `transactions.md`
  - [ ] PR-11: `query-helpers.md`
  - [ ] PR-12: `migrations.md`

- [ ] **Phase 4: Cross-Reference**
  - [ ] PR-13: `endpoint-map.md`
  - [ ] PR-14: `errors.md`
  - [ ] PR-15: `legacy-patterns.md`
  - [ ] PR-16: `agents/api-reference.md`
  - [ ] PR-17: `agents/schema-generation.md`
  - [ ] PR-18: `agents/patterns.md`

- [ ] **Phase 5: Derivative**
  - [ ] PR-19: `migration-plan.md`
  - [ ] PR-20: `compatibility.md`

- [ ] **Phase 6: Agent Integration**
  - [ ] PR-21: `agents/training/` (4 scenario files)
  - [ ] PR-22: `agents/config-updates.md`
  - [ ] PR-23: `tests/agent-integration/`

---

## PR Summary Table

| PR | Branch | Artifacts | FSD Reqs | Phase |
|----|--------|-----------|----------|-------|
| 0 | `feature/docs-migrate-existing-code` | Modified migration, `main.go` | FR-4.1 | 0 |
| 1 | `feature/docs-gen-tooling` | `scripts/`, Makefile | — | 1 |
| 2 | `feature/docs-conventions` | `conventions.md` | FR-3.1, FR-3.2 | 2 |
| 3 | `feature/docs-schema` | `schema/{entity}/` | FR-1.1 | 2 |
| 4 | `feature/docs-openapi` | `openapi.yaml` | FR-2.1 | 2 |
| 5 | `feature/docs-erd` | `erd/` (7 files) | FR-1.2 | 3 |
| 6 | `feature/docs-indexes` | `indexes.md` | FR-1.3 | 3 |
| 7 | `feature/docs-query-patterns` | `query-patterns/` (5 files) | FR-1.4 | 3 |
| 8 | `feature/docs-sqlc` | `sqlc.md` | FR-2.3 | 3 |
| 9 | `feature/docs-connection-pooling` | `connection-pooling.md` | FR-3.3 | 3 |
| 10 | `feature/docs-transactions` | `transactions.md` | FR-3.4 | 3 |
| 11 | `feature/docs-query-helpers` | `query-helpers.md` | FR-3.5 | 3 |
| 12 | `feature/docs-migrations` | `migrations.md` | FR-4.1–4.4 | 3 |
| 13 | `feature/docs-endpoint-map` | `endpoint-map.md` | FR-2.2 | 4 |
| 14 | `feature/docs-errors` | `errors.md` | FR-2.4 | 4 |
| 15 | `feature/docs-legacy-patterns` | `legacy-patterns.md` | FR-5.1 | 4 |
| 16 | `feature/docs-agent-api-ref` | `agents/api-reference.md` | FR-6.1 | 4 |
| 17 | `feature/docs-agent-schema-gen` | `agents/schema-generation.md` | FR-6.2 | 4 |
| 18 | `feature/docs-agent-patterns` | `agents/patterns.md` | FR-6.3 | 4 |
| 19 | `feature/docs-migration-plan` | `migration-plan.md` | FR-5.2, FR-5.3 | 5 |
| 20 | `feature/docs-compatibility` | `compatibility.md` | FR-5.4 | 5 |
| 21 | `feature/docs-agent-training` | `agents/training/` (4 files) | FR-6.4 | 6 |
| 22 | `feature/docs-agent-config` | `agents/config-updates.md` | FR-6.5 | 6 |
| 23 | `feature/docs-agent-tests` | `tests/agent-integration/` | FR-6.5 | 6 |

---

## Constraints

1. **Makefile entry point**: All generation runs through `make docs` (design/README.md)
2. **Version control**: All docs live alongside code, changes in same PR as related code (NFR-4)
3. **One document per PR**: Each PR produces focused, independently reviewable output (AGENTS.md)
4. **Validation mandatory**: `make docs` must validate all generated artifacts (architecture.md §Validation Flow)
5. **SQLC generated files are never hand-edited**: Docs reference generated code but never modify it (design/README.md)
6. **AgentId attribution**: All agent-facing docs include agentId constraints (design/README.md)
7. **Text-based output only**: All artifacts are markdown, YAML, or Mermaid (architecture.md)
