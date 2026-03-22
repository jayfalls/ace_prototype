# Database Documentation

Reference documentation for the ACE Framework's data layer.

## For Humans

### Reading the docs

All documentation lives alongside code in `documentation/`:

| Directory | Contents |
|-----------|----------|
| `documentation/database-design/` | Schema, conventions, indexes, migrations, query patterns |
| `documentation/api/` | OpenAPI spec, endpoint mapping, error codes |
| `documentation/agents/` | Agent integration guides and templates |

Start with `documentation/database-design/conventions.md` for naming standards, then read the schema docs for the table you're working with.

### Making database changes

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

3. **Read the conventions**: `documentation/database-design/conventions.md` has naming rules, data type defaults, and index naming patterns.

4. **Check the migration plan**: `documentation/database-design/migration-plan.md` shows the phased approach for standardizing existing code.

### Adding a new API endpoint

1. Read `documentation/api/openapi.yaml` for existing endpoint structure
2. Follow the Handler → Service → Repository → SQLC pattern (`documentation/api/endpoint-map.md`)
3. Use the response envelope from `documentation/api/errors.md`
4. Add `@Security BearerAuth` annotation if the endpoint requires auth

## For Agents

### Agent context files

When working on database code, agents should read these files:

| When doing... | Read these |
|--------------|------------|
| Creating migrations | `documentation/database-design/migrations.md`, `documentation/database-design/conventions.md` |
| Writing SQLC queries | `documentation/database-design/sqlc.md`, `documentation/database-design/query-patterns/{domain}.md` |
| Adding API endpoints | `documentation/api/openapi.yaml`, `documentation/api/endpoint-map.md`, `documentation/api/errors.md` |
| Any database code | `documentation/agents/patterns.md` (constraint-first quick reference) |
| Schema awareness | `documentation/database-design/schema/{group}/{table}.md` |
| AgentId integration | `documentation/agents/schema-generation.md` |

### Key constraints (from `documentation/agents/patterns.md`)

- **ALWAYS** include `agent_id UUID NOT NULL` on new tables
- **ALWAYS** use `snake_case` for tables, columns, indexes
- **NEVER** hand-edit SQLC generated files
- **NEVER** use `CREATE TABLE IF NOT EXISTS` (Goose manages existence)
- **NEVER** use `interface{}` or `any` in new code

### Training scenarios

Step-by-step guides for common tasks:
- `documentation/agents/training/adding-a-table.md` — Full walkthrough: migration → SQLC → handler → test

### Agent configuration

See `documentation/agents/config-updates.md` for how to wire these docs into agent tool configurations.
