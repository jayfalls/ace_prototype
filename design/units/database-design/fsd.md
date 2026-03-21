# Functional Specification Document

## Overview

The Database Design Documentation & API/DB Specification unit produces authoritative documentation for the ACE Framework's data layer. It is a documentation system, not a runtime service. The deliverables are markdown files, OpenAPI specs, ERD diagrams, and agent integration guidelines that live in version control alongside the codebase.

This FSD defines six functional areas, their data flows, acceptance criteria, and traceability to user stories.

---

## Functional Areas

| ID | Area | Description |
|----|------|-------------|
| FA-1 | Database Design Documentation | Schema docs, ERDs, index strategies, query patterns |
| FA-2 | API/DB Documentation | OpenAPI spec, endpoint-to-query mapping, error codes |
| FA-3 | Pattern Documentation | Naming conventions, SQLC patterns, connection pooling, transactions |
| FA-4 | Migration & Schema Management | Goose migration strategies, rollback patterns, testing |
| FA-5 | Standardization & Adoption | Legacy migration plan, refactoring guidelines, phased rollout |
| FA-6 | Agent Integration | Agent-facing docs, schema-aware generation guidelines, agent config updates |

---

## FA-1: Database Design Documentation

### FR-1.1: Schema Documentation

**Description:** Produce complete documentation for every existing PostgreSQL table, including columns, types, constraints, defaults, and purpose.

**Inputs:** Existing migrations, SQLC query files, live database schema.

**Outputs:** `documentation/database-design/schema/` — one markdown file per table.

**Behavior:**
1. Enumerate all tables from migrations and live schema.
2. For each table, document:
   - Table name and purpose
   - Column definitions (name, type, nullable, default, constraints)
   - Primary key strategy (UUID gen_random_uuid() default)
   - Foreign key relationships with target tables
   - Indexes (name, columns, type, partial conditions)
   - Triggers (e.g., `update_updated_at`)
   - Soft-delete columns where applicable
3. Cross-reference with SQLC query files to document which queries operate on each table.

**Acceptance Criteria:**
- 100% of existing tables documented
- Every column has type, nullability, and default documented
- Foreign keys reference target table and column
- Documentation matches live schema (verified by automated validation script):
  - Script extracts schema from migrations using SQLC introspection
  - Script compares extracted schema against documentation markdown files
  - CI/CD pipeline runs validation on every PR touching schema or documentation
  - Discrepancies fail the build and require resolution before merge

**Traceability:** US-Schema-1 (View complete schema documentation)

---

### FR-1.2: Entity-Relationship Diagrams

**Description:** Produce ERD diagrams showing entity relationships across the schema.

**Inputs:** Schema documentation (FR-1.1).

**Outputs:** `documentation/database-design/erd/` — text-based ERD descriptions and Mermaid/PlantUML diagrams.

**Behavior:**
1. Identify entity groups (agents, memory, tools, messaging, usage, system).
2. For each group, produce:
   - Text-based relationship description
   - Visual diagram (Mermaid syntax for version-control compatibility)
   - Cardinality annotations (1:1, 1:N, M:N)
   - Cascade and delete behavior notes
3. Produce a master ERD showing all groups and cross-group relationships.

**Acceptance Criteria:**
- All major entity groups have diagrams (major groups defined as: agents, memory, tools, messaging, usage, system groups)
- Foreign key relationships visually represented
- Cascade delete patterns documented
- Soft delete patterns marked where applicable

**Traceability:** US-Schema-2 (Understand table relationships)

---

### FR-1.3: Index Strategy Documentation

**Description:** Document all indexes, their purpose, and performance rationale.

**Inputs:** Schema documentation, migration files.

**Outputs:** Section within each table doc + `documentation/database-design/indexes.md` summary.

**Behavior:**
1. Catalog all indexes by table.
2. Classify each index: B-tree (default), partial, composite, GIN/GiST.
3. Document the query pattern each index supports.
4. Identify missing indexes based on common query patterns from SQLC files.
5. Provide recommendations for index additions or removals.

**Acceptance Criteria:**
- Every index documented with type and rationale
- Common query patterns mapped to supporting indexes
- Missing index recommendations included

**Traceability:** US-Schema-1 (View complete schema documentation)

---

### FR-1.4: Query Pattern Library

**Description:** Document reusable query patterns for common database operations.

**Inputs:** SQLC query files, existing Go repository code.

**Outputs:** `documentation/database-design/query-patterns/` — markdown files by pattern category.

**Behavior:**
1. Extract and categorize query patterns:
   - CRUD operations (Create, Read, Update, Delete)
   - Filtering (WHERE clauses, dynamic filters)
   - Pagination (cursor-based, offset-based)
   - Joins (INNER, LEFT, subqueries)
   - Aggregations (GROUP BY, HAVING, window functions)
2. For each pattern, provide:
   - SQL example
   - SQLC annotation
   - Go repository usage
   - Performance considerations
3. Document PostgreSQL-specific features in use (JSONB, arrays, CTEs, etc.).

**Acceptance Criteria:**
- CRUD patterns documented with examples
- Filtering patterns cover dynamic and static filters
- Pagination patterns documented with cursor and offset variants, including:
  - Performance trade-offs between cursor-based and offset-based pagination
  - Performance implications for large datasets (offset degradation, cursor efficiency)
  - Use case guidance for when to use each approach
  - Index requirements for efficient pagination
- Join patterns documented with common use cases

**Traceability:** US-Schema-3 (Find query patterns for common operations)

---

## FA-2: API/DB Documentation

### FR-2.1: OpenAPI Specification

**Description:** Produce a complete OpenAPI 3.x specification for all API endpoints.

**Inputs:** Chi router registrations, handler code, request/response types.

**Outputs:** `documentation/api/openapi.yaml` — machine-readable OpenAPI spec.

**Behavior:**
1. Enumerate all routes from Chi router.
2. For each endpoint, document:
   - HTTP method and path
   - Request parameters (path, query, header)
   - Request body schema with validation rules
   - Response body schema for success and error cases
   - Authentication requirements
   - Example requests and responses
3. Define reusable schema components for common types (pagination envelope, error envelope).
4. Document the standard response envelope:
   ```json
   { "success": true, "data": {...} }
   { "success": false, "error": { "code": "...", "message": "...", "details": [...] } }
   ```

**Acceptance Criteria:**
- 100% of endpoints documented
- Request/response examples for every endpoint
- Error codes documented (400, 401, 403, 404, 409, 500)
- Auth requirements specified per endpoint

**Traceability:** US-API-1 (View complete API specification), US-API-3 (Frontend developer builds feature against API)

---

### FR-2.2: Endpoint-to-Database Mapping

**Description:** Document how each API endpoint translates to database operations.

**Inputs:** Handler code, service layer, SQLC queries.

**Outputs:** `documentation/api/endpoint-map.md` — endpoint-to-query mapping.

**Behavior:**
1. For each endpoint, trace the call chain:
   - Handler → Service → Repository → SQLC query
2. Document:
   - Which SQLC queries are invoked
   - Data transformations applied
   - Transaction boundaries
   - Authorization checks on data access
3. Include SQLC query file locations and generated Go function names.

**Acceptance Criteria:**
- Every endpoint maps to its database queries
- Transaction boundaries documented
- Data transformation steps listed
- SQLC file references included

**Traceability:** US-API-2 (Understand endpoint-to-database mapping)

---

### FR-2.3: SQLC Workflow Documentation

**Description:** Document the SQLC code generation workflow and query organization.

**Inputs:** `sqlc.yaml` config, `.sql` query files, generated Go code.

**Outputs:** `documentation/database-design/sqlc.md`.

**Behavior:**
1. Document `sqlc.yaml` configuration and options.
2. Explain query organization (one file per domain, naming conventions).
3. Document SQLC annotations:
   - `-- name: FunctionName :one/:many/:exec`
   - Parameter binding patterns
   - Return type generation
4. Provide workflow: edit `.sql` → run `sqlc generate` → use generated Go code.
5. Document common patterns: named queries, batch operations, JSONB handling.

**Acceptance Criteria:**
- SQLC configuration documented
- Query organization conventions explained
- Annotation syntax documented with examples
- Generation workflow clearly described

**Traceability:** US-API-2 (Understand endpoint-to-database mapping)

---

### FR-2.4: Error Code Documentation

**Description:** Document all API error codes and their handling patterns.

**Inputs:** Handler code, middleware, response package.

**Outputs:** Section within OpenAPI spec + `documentation/api/errors.md`.

**Behavior:**
1. Catalog all error codes used across handlers.
2. For each code, document:
   - HTTP status code
   - Error code string
   - When it occurs
   - Client handling recommendation
3. Document the `response.ValidationError` format for struct validation failures.
4. Map database errors to API errors (unique violation → 409, foreign key → 404, etc.).

**Acceptance Criteria:**
- All error codes cataloged
- Database-to-API error mapping documented
- Validation error format specified
- Client handling guidance provided

**Traceability:** US-API-1 (View complete API specification), US-API-3 (Frontend developer builds feature against API)

---

## FA-3: Pattern Documentation

### FR-3.1: Naming Conventions

**Description:** Establish and document naming conventions for all database objects.

**Inputs:** Existing schema, migration files, SQLC queries.

**Outputs:** `documentation/database-design/conventions.md`.

**Behavior:**
1. Document conventions for:
   - **Tables:** snake_case, plural nouns (e.g., `agents`, `memory_nodes`)
   - **Columns:** snake_case, descriptive (e.g., `created_at`, `user_id`, `is_active`)
   - **Indexes:** `idx_{table}_{columns}` (e.g., `idx_agents_user_id`)
   - **Constraints:** `{type}_{table}_{columns}` (e.g., `fk_agents_user_id`, `uq_agents_name`)
   - **Triggers:** `set_{table}_updated_at` pattern
   - **Functions:** snake_case, verb-noun (e.g., `update_updated_at`)
2. Document exceptions and legacy naming with migration path.
3. Provide a linter or validation script for enforcing conventions on new migrations.

**Acceptance Criteria:**
- All object types have documented conventions
- Examples provided for each convention
- Legacy exceptions documented with modern equivalents
- Validation tooling specified

**Traceability:** US-Pattern-1 (Follow naming conventions)

---

### FR-3.2: Data Type Conventions

**Description:** Document standard data type usage for common field patterns.

**Inputs:** Existing schema, PostgreSQL documentation.

**Outputs:** Section within `documentation/database-design/conventions.md`.

**Behavior:**
1. Define standard types for:
   - Primary keys: `UUID DEFAULT gen_random_uuid()`
   - Timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`
   - Status fields: `VARCHAR(50) NOT NULL DEFAULT '...'`
   - Names/labels: `VARCHAR(255) NOT NULL`
   - Descriptions/text: `TEXT`
   - JSON data: `JSONB` (with indexing strategy)
   - Boolean flags: `BOOLEAN NOT NULL DEFAULT false`
   - Numeric amounts: `DECIMAL` for money, `INTEGER` for counts
2. Document PostgreSQL-specific features and when to use them:
   - JSONB for flexible schema data
   - Arrays for simple lists
   - CTEs for complex queries
   - Partial indexes for filtered queries
3. Provide decision matrix for type selection.

**Acceptance Criteria:**
- Standard types defined for common patterns
- PostgreSQL-specific features documented
- Decision guidance provided
- Examples for each type

**Traceability:** US-Pattern-2 (Understand data type conventions)

---

### FR-3.3: Connection Pooling Configuration

**Description:** Document connection pool settings, tuning, and best practices.

**Inputs:** `database/sql` configuration, production metrics.

**Outputs:** `documentation/database-design/connection-pooling.md`.

**Behavior:**
1. Document pool parameters:
   - `MaxOpenConns` — recommended values by environment
   - `MaxIdleConns` — recommended values
   - `ConnMaxLifetime` — recommended values
   - `ConnMaxIdleTime` — recommended values
2. Provide tuning guidance:
   - Relationship to PostgreSQL `max_connections`
   - Monitoring pool exhaustion
   - Connection leak detection
3. Document environment-specific settings (dev vs. prod).
4. Include health check pattern (`db.Ping()` in `/health/ready`).

**Acceptance Criteria:**
- All pool parameters documented with recommendations
- Tuning guidance provided
- Environment-specific configurations specified
- Health check pattern documented

**Traceability:** US-Pattern-4 (Configure connection pooling)

---

### FR-3.4: Transaction Patterns

**Description:** Document transaction usage patterns, isolation levels, and error handling.

**Inputs:** Existing repository code, PostgreSQL documentation.

**Outputs:** `documentation/database-design/transactions.md`.

**Behavior:**
1. Document transaction patterns:
   - `db.BeginTx()` with context propagation
   - Deferred rollback with `defer tx.Rollback()`
   - Commit after successful operations
2. Document isolation levels:
   - `READ COMMITTED` (default, recommended for most operations)
   - `REPEATABLE READ` for consistent reads
   - `SERIALIZABLE` for critical sections
3. Document error handling:
   - Deadlock detection and retry
   - Constraint violation handling
   - Context cancellation during transactions
4. Provide examples for common transaction patterns.

**Acceptance Criteria:**
- Transaction lifecycle documented
- Isolation levels explained with use cases
- Error handling patterns documented
- Code examples provided

**Traceability:** US-Pattern-5 (Understand transaction patterns)

---

### FR-3.5: Reusable Query Helpers

**Description:** Document and catalog reusable query helper functions.

**Inputs:** Existing repository code, shared packages.

**Outputs:** `documentation/database-design/query-helpers.md`.

**Behavior:**
1. Catalog existing helper functions (pagination, filtering, sorting).
2. For each helper, document:
   - Function signature
   - Usage pattern
   - SQLC integration
   - Example usage
3. Identify gaps and propose new helpers for common patterns.
4. Document the `update_updated_at()` trigger function.

**Acceptance Criteria:**
- Existing helpers cataloged with signatures
- Usage examples provided
- Gaps identified with proposals
- Trigger function documented

**Traceability:** US-Pattern-3 (Use reusable query helpers)

---

## FA-4: Migration & Schema Management

### FR-4.1: Migration Strategy Documentation

**Description:** Document the Goose migration workflow and strategies.

**Inputs:** Existing migration files, Goose documentation.

**Outputs:** `documentation/database-design/migrations.md`.

**Behavior:**
1. Document the migration framework:
   - Goose v3 with Go migration functions
   - `init()` registration pattern
   - `goose.AddMigration(up, down)` pattern
2. Document the file naming convention: `YYYYMMDDHHMMSS_description.go`
3. Document forward-only strategy:
   - Why forward-only (simplicity, safety)
   - When reversals are needed
   - How to handle breaking changes
4. Document the migration workflow:
   - Create migration file → implement `up` → implement `down` → test → commit
   - `goose up` / `goose down` / `goose status` commands

**Acceptance Criteria:**
- Goose workflow documented
- File naming convention specified
- Forward-only strategy explained
- Command reference included

**Traceability:** US-Migration-1 (Plan forward-only migration), US-Migration-3 (Understand migration versioning)

---

### FR-4.2: Rollback Patterns

**Description:** Document safe rollback patterns and considerations.

**Inputs:** Migration strategy docs, production incident history.

**Outputs:** Section within `documentation/database-design/migrations.md`.

**Behavior:**
1. Document rollback scenarios:
   - Pre-deployment rollback (safe)
   - Post-deployment rollback (risky)
   - Data migration rollback (requires data backup)
2. Document safety checks:
   - Verify no dependent deployments
   - Backup affected data
   - Test rollback in staging
3. Document the `down` function requirements:
   - Must restore schema to previous state
   - Must handle data loss gracefully
   - Must be tested before deployment
4. Provide rollback decision tree.

**Acceptance Criteria:**
- Rollback scenarios documented
- Safety checks specified
- `down` function requirements clear
- Decision tree provided

**Traceability:** US-Migration-1 (Plan forward-only migration), US-Migration-2 (Test migration safely)

---

### FR-4.3: Migration Testing Patterns

**Description:** Document how to test migrations safely before deployment.

**Inputs:** Testing infrastructure, CI/CD pipeline.

**Outputs:** Section within `documentation/database-design/migrations.md`.

**Behavior:**
1. Document testing approach:
   - Unit test: verify `up` and `down` functions execute without error
   - Integration test: run migration against test database, verify schema state
   - Rollback test: run `up` then `down`, verify schema returns to previous state
2. Document the test database setup:
   - Fresh database per test suite
   - Run all migrations from scratch
   - Verify final schema state
3. Document CI/CD integration:
   - Pre-commit hook for migration validation
   - Automated testing on PR
4. Document migration validation patterns:
   - Check for missing `down` functions
   - Check for naming convention compliance
   - Check for dangerous operations (DROP TABLE without backup)

**Acceptance Criteria:**
- Testing approach documented
- Test database setup specified
- CI/CD integration documented
- Validation patterns defined

**Traceability:** US-Migration-2 (Test migration safely)

---

### FR-4.4: Schema Versioning Approach

**Description:** Document how schema versions are tracked and managed.

**Inputs:** Goose behavior, deployment process.

**Outputs:** Section within `documentation/database-design/migrations.md`.

**Behavior:**
1. Document the `goose_db_version` table and its role.
2. Document version numbering: timestamp-based file naming.
3. Document the deployment sequence:
   - `goose status` to check pending migrations
   - `goose up` to apply migrations
   - Verify schema state
4. Document handling of:
   - Concurrent migrations (locking)
   - Skipped versions
   - Out-of-order migrations

**Acceptance Criteria:**
- Version tracking mechanism documented
- Deployment sequence clear
- Edge cases documented
- Concurrent migration handling specified

**Traceability:** US-Migration-1 (Plan forward-only migration), US-Migration-3 (Understand migration versioning)

---

## FA-5: Standardization & Adoption

### FR-5.1: Legacy Pattern Documentation

**Description:** Document existing legacy patterns and their modern equivalents.

**Inputs:** Existing codebase, schema diffs.

**Outputs:** `documentation/database-design/legacy-patterns.md`.

**Behavior:**
1. Identify legacy patterns:
   - Non-standard naming
   - Missing constraints
   - Inconsistent data types
   - Hardcoded values
2. For each legacy pattern, document:
   - Current implementation
   - Modern equivalent
   - Migration complexity (trivial, moderate, complex)
   - Backward compatibility considerations
3. Provide a catalog sorted by migration complexity.

**Acceptance Criteria:**
- All legacy patterns identified
- Modern equivalents documented
- Migration complexity assessed
- Backward compatibility noted

**Traceability:** US-Standard-2 (Understand legacy patterns)

---

### FR-5.2: Migration Plan

**Description:** Produce a phased plan for migrating existing implementations to documented standards.

**Inputs:** Legacy pattern catalog, codebase analysis.

**Outputs:** `documentation/database-design/migration-plan.md`.

**Behavior:**
1. Group migrations by:
   - Phase 1: Non-breaking changes (renaming indexes, adding comments)
   - Phase 2: Low-risk changes (adding constraints, standardizing types)
   - Phase 3: Moderate changes (column renames with views)
   - Phase 4: Breaking changes (table restructures, data migrations)
2. For each phase, document:
   - Scope (tables affected)
   - Migration steps
   - Testing requirements
   - Rollback plan
   - Dependencies on other phases
3. Define success criteria per phase.

**Acceptance Criteria:**
- Phased plan with clear scope
- Migration steps per phase
- Testing requirements specified
- Rollback plans included

**Traceability:** US-Standard-1 (Migrate existing implementation)

---

### FR-5.3: Refactoring Guidelines

**Description:** Document guidelines for refactoring existing code to follow new standards.

**Inputs:** Coding standards, pattern documentation.

**Outputs:** Section within `documentation/database-design/migration-plan.md`.

**Behavior:**
1. Document refactoring patterns:
   - How to rename columns with backward compatibility (views, aliases)
   - How to change data types safely
   - How to add constraints to existing data
   - How to split/merge tables
2. Document testing requirements:
   - Before/after data validation
   - Performance regression testing
   - Application integration testing
3. Document the review process:
   - PR requirements for schema changes
   - Required reviewers (database-aware team members)
   - Documentation update requirements

**Acceptance Criteria:**
- Refactoring patterns documented
- Testing requirements clear
- Review process specified
- Examples provided

**Traceability:** US-Standard-1 (Migrate existing implementation), US-Standard-2 (Understand legacy patterns)

---

### FR-5.4: Backward Compatibility Framework

**Description:** Define rules for maintaining backward compatibility during migrations.

**Inputs:** API contracts, frontend dependencies.

**Outputs:** `documentation/database-design/compatibility.md`.

**Behavior:**
1. Define compatibility rules:
   - API response schema stability during migration
   - Database view creation for renamed columns
   - Gradual deprecation of old patterns
2. Document breaking change process:
   - Deprecation notice period
   - Migration path for consumers
   - Rollback capability
3. Document versioning strategy for API-impacting schema changes.

**Acceptance Criteria:**
- Compatibility rules defined
- Breaking change process documented
- Versioning strategy specified
- Deprecation timeline defined

**Traceability:** US-Standard-1 (Migrate existing implementation)

---

## FA-6: Agent Integration

### FR-6.1: Agent API Documentation Reference

**Description:** Create documentation that opencode agents can use to understand and reference API specifications.

**Inputs:** OpenAPI spec, agent context.

**Outputs:** `documentation/agents/api-reference.md`.

**Behavior:**
1. Structure documentation for agent consumption:
   - Machine-parseable endpoint summaries
   - Request/response schema in agent-friendly format
   - Code generation templates for each endpoint
2. Document how agents should:
   - Load and parse OpenAPI spec
   - Generate request types from schemas
   - Generate response handlers
   - Follow the standard envelope pattern
3. Provide example agent prompts for common API tasks.

**Acceptance Criteria:**
- Agent-friendly API reference created
- Code generation templates included
- Example prompts documented
- Parsing instructions clear

**Traceability:** US-Agent-1 (Reference API documentation)

---

### FR-6.2: Schema-Aware Code Generation Guidelines

**Description:** Document how agents should generate database code using schema awareness.

**Inputs:** Schema documentation, SQLC patterns, Goose patterns.

**Outputs:** `documentation/agents/schema-generation.md`.

**Behavior:**
1. Document agent workflows for:
   - Generating SQLC queries from schema
   - Creating Goose migrations from requirements
   - Generating repository code from SQLC output
   - Following naming conventions in generated code
2. Provide prompt templates for:
   - "Create a migration for table X with columns Y"
   - "Generate SQLC queries for CRUD on table X"
   - "Create repository methods for entity X"
3. Document validation steps agents should perform:
   - Schema consistency checks
   - Naming convention compliance
   - Migration safety checks

**Acceptance Criteria:**
- Agent workflows documented
- Prompt templates provided
- Validation steps specified
- Examples included

**Traceability:** US-Agent-2 (Use schema-aware code generation)

---

### FR-6.3: Agent Pattern Guidelines

**Description:** Document database patterns specifically for agent code generation.

**Inputs:** All pattern documentation, agent capabilities.

**Outputs:** `documentation/agents/patterns.md`.

**Behavior:**
1. Create agent-specific version of pattern docs:
   - Condensed naming conventions
   - Quick reference for data types
   - Transaction pattern templates
   - Error handling checklists
2. Document agent-specific constraints:
   - Never hand-edit SQLC generated files
   - Always include `down` function in migrations
   - Always use `gen_random_uuid()` for new tables
   - Always include `created_at` and `updated_at` columns
3. Provide decision trees for common agent tasks.

**Acceptance Criteria:**
- Agent-specific pattern docs created
- Constraints documented
- Decision trees included
- Quick reference provided

**Traceability:** US-Agent-3 (Follow database patterns)

---

### FR-6.4: Agent Training Materials

**Description:** Produce training materials for agents working with database patterns.

**Inputs:** All documentation, agent interaction history.

**Outputs:** `documentation/agents/training/` — scenario-based guides.

**Behavior:**
1. Create scenario guides:
   - "Adding a new table" — step-by-step agent workflow
   - "Adding a column to existing table" — migration approach
   - "Creating a new API endpoint with database access" — full workflow
   - "Debugging a database query" — investigation approach
2. Each guide includes:
   - Required inputs
   - Decision points
   - Validation steps
   - Common mistakes to avoid
3. Document agent learning feedback loops.

**Acceptance Criteria:**
- Scenario guides for common tasks
- Step-by-step workflows
- Common mistakes documented
- Feedback loop described

**Traceability:** US-Agent-1 (Reference API documentation), US-Agent-2 (Use schema-aware code generation), US-Agent-3 (Follow database patterns)

---

### FR-6.5: Update Opencode Agent Configurations

**Description:** Update opencode agent configurations and references to actively use the new API documentation. This requirement addresses the BSD's explicit scope item: "Update opencode agents to reference and utilize API documentation."

**Inputs:** Agent configuration files, agent documentation (FR-6.1 through FR-6.4), OpenAPI spec.

**Outputs:** Updated agent tool configurations, documentation references, integration tests.

**Behavior:**
1. Update agent tool configurations to reference new documentation:
   - Add API documentation paths to agent tool context directories
   - Configure documentation lookup tools to use new paths
   - Update agent prompt templates to reference documented patterns
2. Update agent documentation references:
   - Ensure agent initialization files point to `documentation/agents/` directory
   - Update agent context injection scripts to load relevant documentation
   - Configure agent memory/context to prioritize documented patterns over inferred patterns
3. Add integration tests:
   - Verify agents can load and reference API documentation
   - Verify agents generate code following documented patterns
   - Verify agent outputs comply with naming conventions and schema awareness
4. Document the update process:
   - Steps to configure new agents to use documentation
   - Validation checklist for agent configuration
   - Troubleshooting guide for documentation access issues

**Acceptance Criteria:**
- Agent configurations updated to reference `documentation/agents/` paths
- Agent tool contexts include API and pattern documentation
- Integration tests verify agents can access and use documentation
- Agent prompt templates updated to reference documented patterns
- Update process documented for future agent onboarding

**Traceability:** US-Agent-1 (Reference API documentation), US-Agent-2 (Use schema-aware code generation), US-Agent-3 (Follow database patterns)

---

## Non-Functional Requirements

### NFR-1: Documentation Freshness
- Documentation must be updated within 1 sprint of code changes
- Automated checks flag stale documentation (>30 days without review)
- Enforcement mechanisms:
  - Pre-commit hook validates documentation freshness when schema changes are detected
  - CI/CD pipeline checks for documentation updates in PRs touching database-related files
  - Automated diff comparison between schema and documentation on every PR
  - Documentation staleness alerts via CI/CD failure when schema drift is detected

### NFR-2: Accessibility
- Documentation must be readable by backend, frontend, and DevOps team members
- No assumed knowledge of internal tooling beyond basic PostgreSQL and Go

### NFR-3: Discoverability
- Documentation must be searchable via file naming and index files
- Cross-references link related documents
- README files in each documentation directory

### NFR-4: Version Control
- All documentation lives in the repository alongside code
- Documentation changes in the same PR as related code changes
- Review required for documentation changes

### NFR-5: Machine Readability
- OpenAPI spec must be valid YAML parseable by standard tools
- Schema documentation must be extractable by scripts
- Agent documentation must be structured for context injection

---

## Traceability Matrix

| Requirement ID | User Story | Feature Area | Priority |
|---------------|------------|--------------|----------|
| FR-1.1 | US-Schema-1 | FA-1: Schema Documentation | Must |
| FR-1.2 | US-Schema-2 | FA-1: ERD Diagrams | Must |
| FR-1.3 | US-Schema-1 | FA-1: Index Strategy | Must |
| FR-1.4 | US-Schema-3 | FA-1: Query Pattern Library | Must |
| FR-2.1 | US-API-1, US-API-3 | FA-2: OpenAPI Spec | Must |
| FR-2.2 | US-API-2 | FA-2: Endpoint Mapping | Must |
| FR-2.3 | US-API-2 | FA-2: SQLC Workflow | Must |
| FR-2.4 | US-API-1, US-API-3 | FA-2: Error Codes | Must |
| FR-3.1 | US-Pattern-1 | FA-3: Naming Conventions | Must |
| FR-3.2 | US-Pattern-2 | FA-3: Data Types | Must |
| FR-3.3 | US-Pattern-4 | FA-3: Connection Pooling | Should |
| FR-3.4 | US-Pattern-5 | FA-3: Transactions | Should |
| FR-3.5 | US-Pattern-3 | FA-3: Query Helpers | Should |
| FR-4.1 | US-Migration-1, US-Migration-3 | FA-4: Migration Strategy | Must |
| FR-4.2 | US-Migration-1, US-Migration-2 | FA-4: Rollback Patterns | Must |
| FR-4.3 | US-Migration-2 | FA-4: Migration Testing | Should |
| FR-4.4 | US-Migration-1, US-Migration-3 | FA-4: Schema Versioning | Must |
| FR-5.1 | US-Standard-2 | FA-5: Legacy Patterns | Should |
| FR-5.2 | US-Standard-1 | FA-5: Migration Plan | Must |
| FR-5.3 | US-Standard-1, US-Standard-2 | FA-5: Refactoring Guidelines | Must |
| FR-5.4 | US-Standard-1 | FA-5: Compatibility | Must |
| FR-6.1 | US-Agent-1 | FA-6: Agent API Reference | Must |
| FR-6.2 | US-Agent-2 | FA-6: Schema Generation | Must |
| FR-6.3 | US-Agent-3 | FA-6: Agent Patterns | Should |
| FR-6.4 | US-Agent-1, US-Agent-2, US-Agent-3 | FA-6: Agent Training | Should |
| FR-6.5 | US-Agent-1, US-Agent-2, US-Agent-3 | FA-6: Agent Config Updates | Must |

---

## Output Artifacts Summary

| Artifact | Location | Format |
|----------|----------|--------|
| Table Schema Docs | `documentation/database-design/schema/` | Markdown (1 per table) |
| ERD Diagrams | `documentation/database-design/erd/` | Mermaid + Markdown |
| Index Strategy | `documentation/database-design/indexes.md` | Markdown |
| Query Patterns | `documentation/database-design/query-patterns/` | Markdown (by category) |
| OpenAPI Spec | `documentation/api/openapi.yaml` | YAML |
| Endpoint Map | `documentation/api/endpoint-map.md` | Markdown |
| SQLC Docs | `documentation/database-design/sqlc.md` | Markdown |
| Error Codes | `documentation/api/errors.md` | Markdown |
| Conventions | `documentation/database-design/conventions.md` | Markdown |
| Connection Pooling | `documentation/database-design/connection-pooling.md` | Markdown |
| Transactions | `documentation/database-design/transactions.md` | Markdown |
| Query Helpers | `documentation/database-design/query-helpers.md` | Markdown |
| Migrations | `documentation/database-design/migrations.md` | Markdown |
| Legacy Patterns | `documentation/database-design/legacy-patterns.md` | Markdown |
| Migration Plan | `documentation/database-design/migration-plan.md` | Markdown |
| Compatibility | `documentation/database-design/compatibility.md` | Markdown |
| Agent API Reference | `documentation/agents/api-reference.md` | Markdown |
| Agent Schema Guide | `documentation/agents/schema-generation.md` | Markdown |
| Agent Patterns | `documentation/agents/patterns.md` | Markdown |
| Agent Training | `documentation/agents/training/` | Markdown (by scenario) |
| Agent Config Updates | `documentation/agents/config-updates.md` | Markdown |
| Agent Integration Tests | `tests/agent-integration/` | Go test files |

---

## Dependencies

| Dependency | Type | Impact |
|-----------|------|--------|
| core-infra unit | Source | Existing schema to document |
| PostgreSQL | Runtime | Features to reference |
| SQLC | Tooling | Code generation patterns to document |
| Goose | Tooling | Migration patterns to document |
| Chi router | Source | API routes to document |

---

## Acceptance Criteria Summary

| Criterion | Metric | Target |
|-----------|--------|--------|
| Schema coverage | Tables documented / Total | 100% |
| ERD completeness | Entity groups mapped | All major groups |
| API documentation | Endpoints documented / Total | 100% with examples |
| Naming convention adoption | New code following conventions | 100% of new migrations |
| Query patterns | Patterns documented | CRUD, filtering, pagination, joins |
| Performance docs | Strategies documented | Indexing, pooling, pagination |
| Migration docs | Patterns documented | Forward, rollback, versioning |
| SQLC coverage | Workflows documented | Organization, generation, helpers |
| Legacy migration | Code refactored to standards | 100% aligned |
| Agent integration | Agents trained on docs | 100% can reference API docs |
| Agent configuration | Agents updated to use docs | Configs verified, integration tests pass |
