# Database Design

Comprehensive database design, patterns, and API/DB documentation for the ACE Framework.

## Documents

- [problem_space.md](problem_space.md) - Problem space exploration
- [bsd.md](bsd.md) - Business Specification
- [user_stories.md](user_stories.md) - User Stories
- [fsd.md](fsd.md) - Functional Specification Document
- [research.md](research.md) - Technology research and recommendations
- [architecture.md](architecture.md) - Technical architecture
- [api.md](api.md) - API/DB documentation specification
- [migration_and_rollback.md](migration_and_rollback.md) - Migration documentation specification
- [implementation.md](implementation.md) - Implementation plan (24 PRs, all executed)

## Documentation Artifacts (Produced)

### Database Design
- `documentation/database-design/conventions.md` — Naming conventions & data type standards
- `documentation/database-design/schema/usage/usage_events.md` — Schema documentation
- `documentation/database-design/erd/master.md` — Mermaid ERD diagrams
- `documentation/database-design/indexes.md` — Index strategy & catalog
- `documentation/database-design/query-patterns/usage.md` — Query patterns
- `documentation/database-design/sqlc.md` — SQLC workflow documentation
- `documentation/database-design/connection-pooling.md` — Connection pooling config
- `documentation/database-design/transactions.md` — Transaction patterns
- `documentation/database-design/query-helpers.md` — Query helpers catalog
- `documentation/database-design/migrations.md` — Migration strategy, rollback, testing, versioning
- `documentation/database-design/legacy-patterns.md` — Legacy patterns & modern equivalents
- `documentation/database-design/migration-plan.md` — Four-phase migration plan
- `documentation/database-design/compatibility.md` — Backward compatibility patterns

### API Documentation
- `documentation/api/openapi.yaml` — OpenAPI 3.1.0 specification
- `documentation/api/endpoint-map.md` — Endpoint-to-DB mapping
- `documentation/api/errors.md` — Error code catalog & DB error mapping

### Agent Integration
- `documentation/agents/api-reference.md` — AGENTS.md-style API reference
- `documentation/agents/schema-generation.md` — SQLC/Goose generation workflows for agents
- `documentation/agents/patterns.md` — Constraint-first quick reference
- `documentation/agents/training/adding-a-table.md` — Step-by-step table creation guide
- `documentation/agents/config-updates.md` — Agent configuration updates

### Testing
- `tests/agent-integration/docs_test.go` — Integration test stubs

### Tooling
- `scripts/docs-gen/` — Documentation generation orchestrator
- `scripts/schema-doc-gen/` — Schema extraction from pg_catalog
- `scripts/erd-gen/` — Mermaid ERD generation
- `scripts/validate-docs/` — Schema-doc drift detection
- `scripts/openapi-gen/` — OpenAPI generation (placeholder)

## Scope

This unit covers:
- Database schema design and documentation
- Naming conventions and data type patterns
- Query patterns and SQLC usage
- Indexing strategy and query optimization
- Pagination patterns and connection pooling
- API documentation (OpenAPI specification)
- Migration strategies and rollback patterns
- ERD diagrams and relationship documentation
- Agent integration and training materials

## Relationship to Core Infrastructure

This unit both defines standards/patterns AND documents existing implementations from the core-infra unit.

## Status

**COMPLETE** — All 24 implementation PRs executed and merged.
