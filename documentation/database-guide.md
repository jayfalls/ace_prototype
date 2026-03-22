# Database Documentation Guide

How to read, write, and maintain the ACE Framework's database documentation.

## Documentation Structure

| Directory | Contents |
|-----------|----------|
| `documentation/database-design/` | Schema, conventions, indexes, migrations, query patterns |
| `documentation/api/` | OpenAPI spec, endpoint mapping, error codes |
| `documentation/agents/` | Agent integration guides and templates |

Start with `documentation/database-design/conventions.md` for naming standards, then read the schema docs for the table you're working with.

## Quick Reference by Task

| Task | Read these |
|------|------------|
| Creating migrations | `documentation/database-design/migrations.md`, `documentation/database-design/conventions.md` |
| Writing SQLC queries | `documentation/database-design/sqlc.md`, `documentation/database-design/query-patterns/{domain}.md` |
| Adding API endpoints | `documentation/api/openapi.yaml`, `documentation/api/endpoint-map.md`, `documentation/api/errors.md` |
| Any database code | `documentation/agents/patterns.md` (constraint-first quick reference) |
| Understanding a table | `documentation/database-design/schema/{group}/{table}.md` |
| AgentId integration | `documentation/agents/schema-generation.md` |

## Making Database Changes

1. **Create a migration**: Follow the template in `documentation/database-design/migrations.md`
   - File naming: `YYYYMMDDHHMMSS_description.go`
   - Use descriptive function names: `upAddColumnX`, not `up`
   - Include `agent_id UUID NOT NULL` + FK + index on new tables

2. **Update docs after migration**: Run `make test` (includes doc validation) or manually:
   ```bash
   cd backend/scripts/schema-doc-gen && go run .     # Regenerate schema docs
   cd backend/scripts/erd-gen && go run .            # Regenerate ERD
   cd backend/scripts/validate-docs && go run .      # Check for drift
   ```

## Adding a New API Endpoint

1. Read `documentation/api/openapi.yaml` for existing endpoint structure
2. Follow the Handler → Service → Repository → SQLC pattern (`documentation/api/endpoint-map.md`)
3. Use the response envelope from `documentation/api/errors.md`
4. Add `@Security BearerAuth` annotation if the endpoint requires auth

## Key Constraints

From `documentation/agents/patterns.md`:

- **ALWAYS** include `agent_id UUID NOT NULL` on new tables
- **ALWAYS** use `snake_case` for tables, columns, indexes
- **NEVER** hand-edit SQLC generated files
- **NEVER** use `CREATE TABLE IF NOT EXISTS` (Goose manages existence)
- **NEVER** use `interface{}` or `any` in new code

## Training Scenarios

Step-by-step guides for common tasks:
- `documentation/agents/training/adding-a-table.md` — Full walkthrough: migration → SQLC → handler → test

## Agent Configuration

See `documentation/agents/config-updates.md` for how to wire these docs into agent tool configurations.
