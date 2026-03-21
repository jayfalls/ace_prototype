# Research Document

## Topic

Approaches for creating comprehensive database documentation, API specifications, ERD generation, schema documentation, agent integration guidelines, and migration documentation for the ACE Framework's data layer.

---

## Problem Space Summary

The database-design unit produces authoritative documentation for the ACE Framework's data layer — it is a documentation system, not a runtime service. The deliverables are markdown files, OpenAPI specs, ERD diagrams, and agent integration guidelines that live in version control alongside the codebase.

Key challenges:
- **Six functional areas** spanning schema docs (FA-1), API/DB docs (FA-2), pattern docs (FA-3), migration docs (FA-4), standardization (FA-5), and agent integration (FA-6)
- **Documentation freshness**: Documentation must match the live schema and be verified by automated validation (FR-1.1 acceptance criteria)
- **Machine readability**: OpenAPI spec must be valid YAML, agent docs must be structured for context injection (NFR-5)
- **Version control**: All docs live alongside code; changes must be in the same PR as code changes (NFR-4)
- **Agent consumption**: Documentation must be structured for AI agent context injection, not just human readers (FA-6)
- **Mermaid diagrams**: ERDs must be text-based for version-control compatibility (FR-1.2)

This research evaluates approaches across all six functional areas.

---

## 1. Schema Documentation Generation

### Industry Standards

PostgreSQL schema documentation has evolved from GUI tools (pgAdmin, DBeaver) to automated, code-as-documentation approaches. The current best practice (2025-2026) is to:
1. Extract schema metadata from migrations or live database introspection
2. Generate markdown documentation as part of CI/CD
3. Use `COMMENT ON` statements in SQL to embed documentation in the schema itself
4. Validate documentation against live schema on every PR

### Alternative Approaches Evaluated

#### Approach 1: Custom Go Script + pg_catalog Introspection
**Description**: Write a Go script that queries PostgreSQL's `pg_catalog` system tables (pg_class, pg_attribute, pg_type, pg_constraint, pg_index) to extract schema metadata and generates markdown files per table.

**Pros**:
- No external dependencies beyond `database/sql` and `pgx`
- Full control over output format and structure
- Can cross-reference SQLC query files (Go-native)
- Fits the existing Go/PostgreSQL/SQLC stack perfectly
- Can validate documentation freshness by comparing extracted schema to markdown files
- Generates markdown directly (no conversion needed)

**Cons**:
- Must maintain the script as schema evolves
- No built-in visual output (relies on separate ERD tools)
- Initial development time to handle all PostgreSQL object types

**Use Cases**: Primary documentation generation for CI/CD pipeline. Best when you need custom output format and tight integration with the existing Go/SQLC stack.

#### Approach 2: SchemaSpy (Open-Source CLI Tool)
**Description**: Java-based CLI tool that analyzes database metadata and generates comprehensive HTML documentation with visual ERDs. Supports PostgreSQL, requires Java 5+.

**Pros**:
- Free and open-source
- Generates browsable HTML documentation with interactive diagrams
- Detects relationships and anomalies automatically
- Supports Markdown for table comments

**Cons**:
- Requires Java runtime (not in the existing Go stack)
- Output is HTML, not markdown (conflicts with FSD requirements)
- Limited customization of output format
- No direct integration with SQLC or Go tooling
- Less actively maintained than alternatives

**Use Cases**: Supplementary tool for interactive HTML documentation. Not suitable as the primary tool for this unit's markdown-first output requirements.

#### Approach 3: DbSchema (Commercial Tool)
**Description**: Visual database design tool with ERD generation, HTML5 documentation export, Git integration, and schema synchronization.

**Pros**:
- Professional ERD diagrams with interactive HTML5 docs
- Git integration for version-controlled schema designs
- Schema comparison and synchronization
- Supports 70+ databases including PostgreSQL

**Cons**:
- Commercial license required (free version limited to 12 tables)
- GUI-based workflow (not CI/CD friendly)
- Output format doesn't match markdown-first requirement
- Not suitable for automated documentation generation in pipelines

**Use Cases**: Supplementary tool for visual schema exploration and design. Not suitable for automated CI/CD documentation generation.

#### Approach 4: tbls (Go-based Documentation Tool)
**Description**: A Go-based tool that generates database documentation from a live database connection. Outputs markdown, supports PostgreSQL.

**Pros**:
- Written in Go — aligns with the project stack
- Generates markdown output (matches FSD requirements)
- Can be embedded in CI/CD pipelines
- Supports PostgreSQL natively
- Outputs ERD in Mermaid format

**Cons**:
- Limited customization compared to custom scripts
- May not support cross-referencing SQLC query files
- Smaller community than alternatives

**Use Cases**: Alternative to custom scripts when a ready-made solution is preferred.

### Comparison Matrix

| Criteria | Custom Go Script | SchemaSpy | DbSchema | tbls |
|----------|-----------------|-----------|----------|------|
| Output Format | Markdown (custom) | HTML | HTML5/PDF | Markdown |
| CI/CD Integration | Excellent | Good | Poor | Good |
| Go Stack Alignment | Excellent | Poor (Java) | N/A | Excellent |
| SQLC Integration | Custom | None | None | None |
| Customization | Full | Limited | GUI-based | Moderate |
| Validation Support | Custom | Limited | GUI-based | Limited |
| License | N/A | Open-source | Commercial | Open-source |

### Recommendation for Schema Documentation

**Primary: Custom Go Script** — Build a Go script that queries `pg_catalog` for schema metadata and generates per-table markdown files. This provides full control over output format, integrates natively with the Go/SQLC stack, and enables automated validation by comparing extracted schema against documentation files.

**Supplementary: tbls** — Use as a secondary tool for quick schema exploration during development, generating a reference that can be compared against the custom script output.

---

## 2. ERD Generation

### Industry Standards

ERD generation has shifted from GUI tools to text-based, version-controlled approaches. The current standard (2025-2026):
- **Mermaid.js** is the dominant text-based ERD format, natively supported by GitHub, GitLab, VS Code, and major documentation platforms
- ERDs stored as text in version control, rendered by documentation tools
- Automated generation from database schemas using SQL queries or ORM introspection
- CI/CD validation of diagram syntax via Mermaid CLI

### Alternative Approaches Evaluated

#### Approach 1: SQL Query → Mermaid Generator
**Description**: Write SQL queries that extract table definitions, columns, and foreign key constraints from PostgreSQL's system catalogs, outputting valid Mermaid `erDiagram` syntax. Store as `.md` files in version control.

**Pros**:
- Fully automated from live database or migrations
- Text-based output (version-control friendly)
- Native GitHub/GitLab rendering
- Can be generated in CI/CD pipeline
- No external dependencies (pure SQL + script)
- Proven pattern (Cybertec PostgreSQL blog post demonstrates this approach)

**Cons**:
- Limited visual customization compared to GUI tools
- Complex schemas may produce hard-to-read diagrams
- Requires grouping logic for entity clusters

**Use Cases**: Primary ERD generation for all entity groups (agents, memory, tools, messaging, usage, system). Best for version-controlled, automated documentation.

#### Approach 2: Mermaid CLI (mmdc) + Hand-Crafted Diagrams
**Description**: Manually write Mermaid ERD syntax for each entity group, validate with Mermaid CLI (`@mermaid-js/mermaid-cli`), and render to SVG/PNG for distribution.

**Pros**:
- Full control over diagram layout and grouping
- Clean, readable diagrams optimized for communication
- Works with any documentation system
- Can include annotations and notes not derivable from schema

**Cons**:
- Manual effort to maintain diagrams as schema evolves
- Risk of diagrams becoming stale
- Not fully automated

**Use Cases**: Master ERDs and communication diagrams. Use for the master ERD (FR-1.2) that shows all groups and cross-group relationships. Supplement automated SQL-generated diagrams.

#### Approach 3: Atlas Schema Visualization
**Description**: Atlas (by Ariga) can visualize schema from HCL or SQL definitions, generating ERD diagrams as part of its schema-as-code workflow.

**Pros**:
- Integrated with schema migration workflow
- Drift detection built-in
- Declarative schema definitions

**Cons**:
- Requires adopting Atlas as the migration tool (replacing Goose)
- Commercial features behind paywall (extensions, advanced features)
- Not compatible with the existing Goose-based workflow

**Use Cases**: Not suitable for this unit given the existing Goose migration commitment.

### Comparison Matrix

| Criteria | SQL → Mermaid | Mermaid CLI (hand-crafted) | Atlas Viz |
|----------|---------------|---------------------------|-----------|
| Automation | Fully automated | Manual | Semi-automated |
| Version Control | Excellent (text) | Excellent (text) | Good |
| Maintenance Effort | Low (auto-gen) | High (manual) | Low |
| Visual Quality | Good | Excellent | Good |
| GitHub Rendering | Native | Native | N/A |
| Go Stack Alignment | Excellent | N/A | Requires Atlas |
| CI/CD Validation | mmdc CLI | mmdc CLI | Built-in |

### Recommendation for ERD Generation

**Primary: SQL → Mermaid Generator** — Write SQL queries against `pg_catalog` to auto-generate Mermaid ERD syntax for each entity group. This produces version-controlled, automatically-updated diagrams that render natively on GitHub.

**Supplementary: Hand-crafted master ERD** — Manually craft the master ERD (all groups + cross-group relationships) using Mermaid syntax for optimal layout and readability. Use `mmdc` CLI in CI/CD to validate syntax.

---

## 3. OpenAPI Specification Generation

### Industry Standards

OpenAPI generation for Go services has matured significantly. The current approaches (2025-2026):
1. **Code-first (annotation-driven)**: Annotate Go handlers with comments, generate OpenAPI spec from code
2. **Spec-first**: Write OpenAPI spec manually or from external source, generate Go code
3. **Struct-based**: Define API structure in Go structs, generate spec from struct definitions

For Go + Chi router, the leading tools are:
- `swaggo/swag` — Most mature, Go comment annotations → OpenAPI 2.0/3.0
- `go-specgen` (wontaeyang) — Go annotations → OpenAPI 3.x, framework-agnostic, recently active (v1.2.1, March 2026)
- `Annot8` (AxelTahmid) — Annotation-driven OpenAPI 3.1 generator, specifically designed for Chi router + SQLC + pgx/v5 stack
- `go-respec` (Zachacious) — Static analysis approach, no annotations required, infers spec from Go code structure
- `chioas` — Chi router addon that defines API as structs and produces OpenAPI spec

### Alternative Approaches Evaluated

#### Approach 1: Annot8 (Chi-Native Annotation-Driven)
**Description**: Annot8 generates OpenAPI 3.1 specs from Go code annotations. It's specifically designed and optimized for the Chi router with SQLC and pgx/v5 — the exact stack used in ACE Framework.

**Pros**:
- **Purpose-built for Chi + SQLC + pgx/v5** — the exact ACE stack
- Zero configuration, deep type discovery from Go structs
- Extracted from a production app with 340+ models
- Dynamic schema generation from Go types (including SQLC-generated types)
- Supports both runtime generation and static file generation
- OpenAPI 3.1 support (latest standard)
- Actively maintained (v0.3.0, 2025)

**Cons**:
- Relatively new (compared to swaggo)
- Smaller community
- Annotation syntax may need learning

**Use Cases**: Primary OpenAPI generation for the ACE API. Best fit for the Chi + SQLC + pgx stack.

#### Approach 2: swaggo/swag (Industry Standard)
**Description**: The most widely-used Go OpenAPI generator. Uses comment annotations to generate Swagger/OpenAPI specs. Mature ecosystem with http-swagger UI integration.

**Pros**:
- Largest community and ecosystem
- Well-documented, battle-tested
- Supports OpenAPI 2.0 and 3.0
- Interactive Swagger UI integration via http-swagger
- Complex type and nested structure support

**Cons**:
- Originally designed for Gin (Chi support requires adapter)
- OpenAPI 3.0 only (not 3.1)
- Heavier annotation syntax
- No native SQLC type awareness
- Large dependency footprint

**Use Cases**: Alternative if Annot8 proves insufficient. Broad ecosystem support.

#### Approach 3: go-respec (Static Analysis)
**Description**: Static analysis tool that introspects Go source code to generate OpenAPI v3 specs without annotations. Uses a 3-layer approach: explicit metadata API → configuration overrides → smart inference.

**Pros**:
- Zero annotations required (clean codebase)
- Framework-agnostic (works with Chi)
- Smart inference from code structure
- Config overrides for customization
- Multiple releases showing active development (29 releases as of 2025)

**Cons**:
- Newer project (June 2025), experimental status
- Limited SQLC awareness
- May not capture all business logic details
- Relies on inference which can be incomplete

**Use Cases**: Alternative if annotation overhead is unacceptable. Best for maintaining a clean codebase.

#### Approach 4: go-specgen (Annotation-Driven, Framework-Agnostic)
**Description**: Generates OpenAPI 3.x specs from Go code annotations. Framework-agnostic, spec-only (no runtime binding).

**Pros**:
- Supports OpenAPI 3.0, 3.1, and 3.2 (future-proof)
- Framework-agnostic (works with Chi)
- Recent active development (v1.2.1, March 2026)
- Lightweight, spec-only approach

**Cons**:
- Newer project with smaller community
- Requires annotation learning
- No built-in SQLC integration

**Use Cases**: Alternative approach for annotation-driven generation.

#### Approach 5: oaswrap/spec + chiopenapi (Programmatic)
**Description**: Lightweight, framework-agnostic OpenAPI 3.x spec builder. The `chiopenapi` adapter provides Chi-specific integration. Zero dependencies.

**Pros**:
- Zero dependencies, lightweight
- Programmatic spec building in pure Go
- Chi-specific adapter available
- Built-in documentation UI at `/docs`
- CI/CD ready for static generation

**Cons**:
- Requires manual spec definition (no inference)
- More boilerplate code
- No SQLC type awareness

**Use Cases**: Alternative for maximum control over spec definition.

### Comparison Matrix

| Criteria | Annot8 | swaggo/swag | go-respec | go-specgen | oaswrap/spec |
|----------|--------|-------------|-----------|------------|--------------|
| Chi Native | Yes (designed) | Via adapter | Yes | Yes | Via adapter |
| SQLC Awareness | Yes | No | Limited | No | No |
| OpenAPI Version | 3.1 | 2.0/3.0 | 3.0 | 3.0/3.1/3.2 | 3.0.3 |
| Annotation Required | Yes | Yes | No | Yes | No (programmatic) |
| Community Size | Small | Large | Small | Small | Small |
| Active Maintenance | Yes (2025) | Yes | Yes (2025) | Yes (2026) | Yes (2025) |
| Stack Fit (Chi+SQLC) | Excellent | Moderate | Good | Good | Moderate |

### Recommendation for OpenAPI Generation

**Primary: Annot8** — Purpose-built for the Chi + SQLC + pgx/v5 stack used by ACE. Generates OpenAPI 3.1 specs from annotations with zero configuration. Supports both runtime and static file generation for CI/CD integration.

**Fallback: swaggo/swag** — If Annot8 proves insufficient for edge cases, swaggo's mature ecosystem provides a reliable alternative. Use the Chi adapter for router integration.

---

## 4. Agent-Facing Documentation (FA-6)

### Industry Standards

AI agent documentation has emerged as a critical new category in 2025-2026. Key developments:
- **AGENTS.md standard**: Industry-standard file for AI agent instructions, backed by OpenAI, Anthropic, Google, Microsoft, Amazon (Linux Foundation stewardship)
- **Markdown as AI instruction layer**: Markdown has become the lingua franca for agent-facing documentation (Cloudflare's "Markdown for Agents", GitHub Copilot's AGENTS.md support)
- **Structured documentation**: Agents consume structured markdown with clear sections, code examples, and explicit constraints
- **Context injection**: Documentation structured for agent context windows — concise, hierarchical, with code blocks

### Alternative Approaches Evaluated

#### Approach 1: AGENTS.md-Style Structured Markdown
**Description**: Create agent-facing documentation following the AGENTS.md pattern — structured markdown files with clear sections for: project knowledge, commands, code style, patterns, boundaries. Place in `documentation/agents/` directory.

**Pros**:
- Industry standard backed by all major AI labs
- Proven effective (GitHub study of 2,500+ AGENTS.md files)
- Markdown is the optimal format for agent consumption (Cloudflare data shows 4x token efficiency vs HTML)
- Compatible with all major coding agents (Copilot, Claude Code, OpenAI Codex, Cursor)
- Shallow hierarchy (H1 → H2 → H3) matches agent parsing patterns
- Version-controlled alongside code

**Cons**:
- Still evolving (standard proposed August 2025)
- May need updates as agent capabilities evolve
- Token budget constraints require careful curation

**Use Cases**: Primary approach for all agent-facing documentation (FR-6.1 through FR-6.4). Create `documentation/agents/api-reference.md`, `schema-generation.md`, `patterns.md`, and `training/` files following AGENTS.md patterns.

#### Approach 2: OpenAPI + Code Generation Templates
**Description**: Combine machine-readable OpenAPI spec with code generation templates that agents can use to generate API client code. Agents parse the OpenAPI YAML directly.

**Pros**:
- Machine-readable (NFR-5 requirement)
- OpenAPI is a well-established standard
- Enables agent to generate type-safe code from schemas
- Can be validated with standard tools (swagger-cli, redocly)

**Cons**:
- OpenAPI alone is insufficient — agents need context about patterns, conventions, constraints
- YAML parsing may consume more tokens than markdown
- No built-in guidance for naming conventions or SQLC patterns

**Use Cases**: Support for FR-6.1 (agent API reference). The OpenAPI spec serves as the machine-readable layer, complemented by AGENTS.md-style guidance.

#### Approach 3: MCP Tool Manifest + agents.md
**Description**: Create a machine-readable MCP (Model Context Protocol) tool manifest (`mcp.json`) alongside human-readable agents.md documentation. Agents read agents.md for strategy, mcp.json for tool invocation schemas.

**Pros**:
- Separates concerns: human-readable guidance vs machine-readable schemas
- MCP is gaining adoption as agent protocol
- Structured JSON schemas for precise tool invocation
- Complementary to AGENTS.md (not competing)

**Cons**:
- MCP is not yet universally adopted
- Adds complexity (two files to maintain)
- May be premature for documentation-only unit

**Use Cases**: Future consideration if agents need to invoke documentation tools dynamically. Not required for the current documentation-only scope.

### Comparison Matrix

| Criteria | AGENTS.md Markdown | OpenAPI + Code Gen | MCP + agents.md |
|----------|-------------------|-------------------|-----------------|
| Industry Standard | Yes (Linux Foundation) | Yes (OpenAPI Initiative) | Emerging |
| Agent Compatibility | Excellent (all major agents) | Good (code gen focus) | Limited (MCP clients) |
| Token Efficiency | Excellent (4x vs HTML) | Moderate (YAML overhead) | Good |
| Machine Readability | Moderate | Excellent | Excellent |
| Human Readability | Excellent | Good | Good |
| Maintenance Effort | Low | Moderate | Moderate-High |
| Version Control | Excellent | Excellent | Good |

### Recommendation for Agent Integration

**Primary: AGENTS.md-Style Structured Markdown** — Create all agent-facing documentation (FR-6.1 through FR-6.5) following the AGENTS.md pattern. Structure with:
- Clear role definition and project knowledge
- Executable commands and validation steps
- Code style examples (naming conventions, SQLC patterns)
- Explicit boundaries (never hand-edit SQLC files, always include `down` functions)
- Prompt templates for common tasks

**Supplementary: OpenAPI Spec Reference** — The agent documentation references the machine-readable OpenAPI spec (generated by Annot8) as the source of truth for API schemas. Agents use the markdown for guidance and the YAML for type generation.

---

### 4.1 Agent Prompt Templates for Schema-Aware Code Generation (FR-6.2)

#### Industry Standards

AI agent prompt templates for database code generation (2025-2026):
- **Structured prompts** with clear requirements, constraints, and examples outperform free-form prompts
- **Template patterns** should include: context, task description, constraints, expected output format, validation steps
- **Cursor rules / CLAUDE.md files** provide persistent context that agents read at session start
- **In-context learning examples** (question/SQL pairs) improve query generation accuracy (Oracle Generative AI Agents documentation)

#### Prompt Template Patterns Evaluated

##### Pattern 1: Structured Task Template
**Description**: Template with explicit sections: Context → Task → Constraints → Output Format → Validation.

**Example — Migration Generation**:
```
Context: Table `users` exists with columns (id UUID, name VARCHAR(255), created_at TIMESTAMPTZ).
Task: Create a Goose migration to add an `email` column (VARCHAR(255), NOT NULL, UNIQUE).
Constraints:
- Use Go migration functions (not SQL files)
- Include both up and down functions
- Follow naming convention: YYYYMMDDHHMMSS_add_email_to_users.go
- Use init() registration pattern
- Add index: idx_users_email
Validation:
- Verify migration file follows naming convention
- Verify down function properly removes column and index
- Run goose status to confirm migration is detected
```

**Pros**: Clear structure, explicit constraints, built-in validation steps. Proven effective in GitHub's AGENTS.md study.

**Cons**: Longer prompts consume more context window. May be over-engineered for simple tasks.

##### Pattern 2: Code Generation with Schema Context
**Description**: Provide schema context inline with the generation task. Agent sees the full schema before generating code.

**Example — SQLC Query Generation**:
```
Given this schema:
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'idle',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

Generate SQLC queries for CRUD operations on the agents table.
Requirements:
- One query file: db/queries/agents.sql
- Use SQLC annotations: -- name: FunctionName :one/:many/:exec
- Include: GetById, ListByUserId, Create, Update, Delete
- Use named parameters ($1, $2, etc.)
- Add soft delete support (WHERE deleted_at IS NULL for reads)
```

**Pros**: Agent has full context. Generates accurate SQL matching actual schema. Reduces hallucination.

**Cons**: Schema context consumes tokens. May need to truncate for large schemas.

##### Pattern 3: Constraint-First Template
**Description**: Lead with constraints and prohibitions, then describe the task. Emphasizes what NOT to do.

**Example — Repository Code Generation**:
```
CONSTRAINTS (CRITICAL):
- NEVER hand-edit files in db/sqlc/ — these are generated
- ALWAYS use the generated types from db/sqlc/models.go
- NEVER use raw SQL strings — use SQLC generated functions
- ALWAYS include context.Context as first parameter
- ALWAYS handle errors (never use _ for error returns)

Task: Create a repository method for listing active agents by user_id.
Use the SQLC-generated ListAgentsByUserId function.
Return ([]*db.Agent, error).
```

**Pros**: Constraints are unmissable. Prevents common agent mistakes. Mirrors AGENTS.md boundary patterns.

**Cons**: May be overly restrictive for experienced agents.

#### Comparison Matrix

| Criteria | Structured Task | Schema Context | Constraint-First |
|----------|-----------------|----------------|------------------|
| Accuracy | High | Very High | High |
| Token Usage | High | Very High | Medium |
| Flexibility | Low | Medium | Low |
| Best For | Migrations | SQLC queries | Repository code |
| Agent Compatibility | All | All | All |

#### Recommendation for Agent Prompt Templates

**Primary: Structured Task Template** — Document as the default pattern for agent-facing tasks. Include template examples for the three most common operations:
1. "Create a migration for table X with columns Y"
2. "Generate SQLC queries for CRUD on table X"
3. "Create repository methods for entity X"

**Supplementary: Constraint-First for Critical Operations** — Use constraint-first pattern for operations that commonly produce agent errors (migrations, SQLC file management).

---

### 4.2 Agent Constraints for Database Code Generation (FR-6.3)

#### Industry Standards

Effective agent constraints for database code (2025-2026):
- **Rules files** (`.cursor/rules/`, `CLAUDE.md`, `AGENTS.md`) provide persistent constraints read at session start
- **Negative constraints** ("NEVER do X") are more effective than positive instructions alone
- **Specific tool constraints** outperform generic style guidelines (GitHub study: "Never commit secrets" was most helpful constraint)
- **Human-in-the-loop for critical actions** — database migrations, destructive operations should require approval

#### Agent Constraint Categories

##### Category 1: Tooling Constraints (Critical)
These prevent irreversible mistakes:
- **NEVER** hand-edit files in `db/sqlc/` — modify `.sql` query files and run `sqlc generate`
- **NEVER** run `goose down` in production without explicit approval
- **NEVER** use `DROP TABLE` without a backup or migration rollback plan
- **ALWAYS** run `sqlc generate` after modifying `.sql` query files
- **ALWAYS** verify `goose status` before running `goose up`

##### Category 2: Schema Constraints (Must Follow)
These ensure schema consistency:
- **ALWAYS** use `gen_random_uuid()` for new table primary keys
- **ALWAYS** include `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` and `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` columns
- **ALWAYS** include `down` function in migrations (even if it's a no-op for data migrations)
- **ALWAYS** add `CREATE TRIGGER set_{table}_updated_at` for new tables
- **NEVER** use `timestamp without time zone` — always use `TIMESTAMPTZ`
- **NEVER** use `VARCHAR` without length constraint unless truly unlimited

##### Category 3: Naming Constraints (Should Follow)
These maintain convention compliance:
- Tables: `snake_case`, plural nouns
- Columns: `snake_case`, singular
- Indexes: `idx_{table}_{columns}`
- Constraints: `{type}_{table}_{columns}` (e.g., `fk_agents_user_id`)
- SQLC queries: `-- name: FunctionName :one/:many/:exec`

##### Category 4: Safety Constraints (Recommended)
These reduce risk:
- **ALWAYS** test migrations on a local database before committing
- **ALWAYS** include `IF NOT EXISTS` / `IF EXISTS` in migrations for idempotency
- **CONSIDER** backward compatibility — use expand-contract pattern for column renames
- **VERIFY** foreign key references exist before adding constraints

#### Decision Trees for Agent Tasks

**Decision: Creating a new table?**
```
Start → Include id (UUID, gen_random_uuid()) → Include created_at, updated_at (TIMESTAMPTZ) →
Add trigger set_{table}_updated_at → Add indexes for foreign keys →
Create migration with up + down → Create SQLC queries →
Run sqlc generate → Test migration locally → Commit
```

**Decision: Modifying an existing column?**
```
Start → Is it a rename? → YES: Use expand-contract (add new, backfill, migrate code, drop old)
                        → NO: Is it a type change? → YES: Expand-contract with validation
                                                   → NO: Direct ALTER with default for NOT NULL
```

#### Recommendation for Agent Constraints

Document as `documentation/agents/patterns.md` with four constraint categories (tooling, schema, naming, safety). Include decision trees for common agent tasks. Constraints should be in the AGENTS.md format with clear "NEVER" and "ALWAYS" markers.

---

## 5. Pattern Documentation (FA-3)

### 5.1 Naming Conventions (FR-3.1)

#### Industry Standards

PostgreSQL naming conventions have converged on clear standards (2025-2026):
- **snake_case** is the universal standard for PostgreSQL identifiers (tables, columns, indexes, constraints)
- PostgreSQL folds unquoted identifiers to lowercase, making PascalCase/camelCase require double quotes everywhere
- The Bytebase SQL Review Guide (used by large-scale PostgreSQL deployments) mandates: "Object names must use only lowercase letters, underscores, and numbers, and follow snake_case"
- Consensus across GeeksforGeeks, 42 Coffee Cups, Business Compass LLC, and PostgreSQL community: snake_case is non-negotiable

#### Alternative Approaches Evaluated

##### Approach 1: snake_case Convention (PostgreSQL-Native)
**Description**: Use snake_case for all database objects. Tables as plural nouns (`agents`, `memory_nodes`), columns as singular (`created_at`, `user_id`), indexes as `idx_{table}_{columns}`, constraints as `{type}_{table}_{columns}`.

**Pros**:
- PostgreSQL-native (no quoting required)
- Industry standard (unanimous consensus)
- Clean queries without double quotes
- Compatible with all ORMs and tools
- Bytebase-enforced in enterprise PostgreSQL deployments

**Cons**:
- Different from Go naming conventions (PascalCase/camelCase)
- Requires mental context-switching between SQL and Go code

**Use Cases**: Primary naming convention for all database objects. Non-negotiable for PostgreSQL.

##### Approach 2: Hybrid Convention (snake_case SQL, PascalCase Go)
**Description**: Use snake_case in PostgreSQL but document the mapping to Go types. SQLC handles the conversion automatically (snake_case columns → PascalCase Go fields).

**Pros**:
- Best of both worlds: PostgreSQL-native SQL, Go-native code
- SQLC automates the conversion
- No manual mapping required
- Clear documentation of the boundary

**Cons**:
- Two naming systems to understand
- Requires SQLC configuration for custom mappings

**Use Cases**: Document the mapping as part of naming conventions. This is the de facto standard when using SQLC.

##### Approach 3: Linter-Enforced Convention
**Description**: Use a SQL linter (e.g., sqlfluff, Bytebase) to enforce naming conventions automatically in CI/CD.

**Pros**:
- Automated enforcement
- Catches violations before merge
- Consistent across all contributors

**Cons**:
- Additional CI dependency
- May need custom rules for project-specific patterns

**Use Cases**: Supplementary enforcement mechanism. Document as part of validation tooling.

#### Comparison Matrix

| Criteria | snake_case (native) | Hybrid (snake_case + SQLC) | Linter-enforced |
|----------|---------------------|---------------------------|-----------------|
| PostgreSQL Compatibility | Excellent | Excellent | Excellent |
| Go Integration | Manual | Automatic (SQLC) | N/A |
| Enforcement | Manual | Automatic | Automated |
| Industry Adoption | Universal | Universal | Growing |

#### Recommendation for Naming Conventions

**Primary: snake_case with Hybrid Mapping** — Document snake_case as the universal standard for PostgreSQL. Document the SQLC mapping (snake_case → PascalCase) as part of the conventions. This is the established standard with unanimous industry consensus.

**Supplementary: Linter Integration** — Document the option to add a SQL linter to CI/CD for automated enforcement.

---

### 5.2 Data Type Conventions (FR-3.2)

#### Industry Standards

PostgreSQL data type conventions (2025-2026):
- **Primary keys**: `UUID DEFAULT gen_random_uuid()` for distributed systems (unanimous consensus)
- **Timestamps**: `TIMESTAMPTZ NOT NULL DEFAULT NOW()` — always with timezone (Timescale, 42 Coffee Cups, Reintech)
- **Money**: `NUMERIC(10,2)` — never use floating point for money
- **JSON data**: `JSONB` over `JSON` — faster queries, indexable with GIN
- **Boolean flags**: `BOOLEAN NOT NULL DEFAULT false`
- **Text**: `TEXT` for flexible strings, `VARCHAR(n)` only when length constraint is meaningful

#### Decision Matrix: JSONB vs Normalized Tables

| Use JSONB When | Normalize When |
|----------------|----------------|
| Schema varies by record | Schema is stable and well-defined |
| Metadata/tags/attributes | Aggregate queries (SUM, AVG, GROUP BY) are core |
| User preferences/settings | Referential integrity needed (foreign keys) |
| Third-party API responses | Frequent filtering on specific fields |
| Product catalog with type-specific attributes | Data integrity constraints required |

The hybrid approach (normalized columns + JSONB for flexible parts) is the recommended pattern.

#### UUID Strategy: gen_random_uuid() vs UUIDv7

| Criteria | gen_random_uuid() (v4) | UUIDv7 |
|----------|------------------------|--------|
| Ordering | Random | Time-ordered |
| Index Performance | Fragmented inserts | Sequential inserts |
| Privacy | Excellent (unpredictable) | Leaks creation time |
| PostgreSQL Support | pgcrypto extension | PostgreSQL 18+ |
| Use Case | Public-facing IDs, tokens | Write-heavy tables, internal IDs |

**Recommendation**: Use `gen_random_uuid()` (v4) as the default for primary keys. Consider UUIDv7 for high-write tables where index performance is critical (PostgreSQL 18+).

#### Recommendation for Data Type Conventions

Document the standard type mappings with decision guidance. Provide examples for each type and document when to use JSONB vs normalized tables.

---

### 5.3 Connection Pooling (FR-3.3)

#### Industry Standards

Go connection pooling for PostgreSQL (2025-2026):
- **pgxpool** is the recommended pool for PostgreSQL-heavy applications (better performance, PostgreSQL-specific features)
- **database/sql** pool is the standard library option (works with lib/pq or pgx/stdlib driver)
- Key configuration: `MaxConns`, `MinConns`, `MaxConnLifetime`, `MaxConnIdleTime`
- Rule of thumb: `MaxConns = (CPU cores * 2) + 1` as baseline
- Health check: `db.Ping()` in `/health/ready` endpoint

#### Alternative Approaches Evaluated

##### Approach 1: pgxpool (pgx/v5 Native)
**Description**: Use pgxpool from the pgx/v5 library for PostgreSQL-specific connection pooling. Bypasses database/sql abstraction layer.

**Pros**:
- Better performance (no abstraction overhead)
- PostgreSQL-specific features (COPY, LISTEN/NOTIFY)
- Better type handling with pgx
- Connection health monitoring
- Prepared statement caching

**Cons**:
- PostgreSQL-specific (not portable to other databases)
- Separate API from database/sql

**Use Cases**: Primary approach for PostgreSQL-heavy applications. Recommended for ACE Framework.

##### Approach 2: database/sql Pool
**Description**: Use Go's standard library database/sql connection pool with pgx/stdlib driver.

**Pros**:
- Standard library (no additional dependencies)
- Portable across databases
- Well-understood API
- Built-in connection recycling

**Cons**:
- Abstraction overhead
- Limited PostgreSQL-specific features
- Less fine-grained control

**Use Cases**: Alternative when database portability is a requirement.

##### Approach 3: PgBouncer External Pooler
**Description**: Use PgBouncer as an external connection pooler in front of PostgreSQL.

**Pros**:
- Reduces PostgreSQL connection overhead
- Works across multiple application instances
- Transaction-level pooling mode
- Battle-tested in production

**Cons**:
- Additional infrastructure component
- Configuration complexity
- May interfere with prepared statements

**Use Cases**: Supplementary approach for production deployments with multiple application instances.

#### Comparison Matrix

| Criteria | pgxpool | database/sql Pool | PgBouncer |
|----------|---------|-------------------|-----------|
| Performance | Excellent | Good | Excellent |
| PostgreSQL Features | Full | Limited | N/A |
| Portability | PostgreSQL only | Cross-DB | Cross-DB |
| Complexity | Low | Low | Medium |
| External Dependencies | None | None | PgBouncer |

#### Recommendation for Connection Pooling

**Primary: pgxpool** — Document pgxpool configuration as the recommended approach.

##### Parameter Tuning with Environment-Specific Values (FR-3.3 Requirement)

| Parameter | Description | Development | Production (Single Instance) | Production (Multi-Instance) |
|-----------|-------------|-------------|------------------------------|----------------------------|
| `MaxConns` | Maximum open connections | 10 | 25-50 | 10-20 per instance |
| `MinConns` | Minimum idle connections kept warm | 2 | 5-10 | 2-5 per instance |
| `MaxConnLifetime` | Max time a connection is reused | 30 min | 1 hour | 30 min (with load balancer) |
| `MaxConnIdleTime` | Max time idle before closing | 10 min | 30 min | 5-10 min |
| `HealthCheckPeriod` | Connection health check interval | 1 min | 1 min | 30 sec |

**Tuning Guidelines**:
- **MaxConns baseline**: `(CPU cores * 2) + 1` — but adjust based on PostgreSQL's `max_connections` (default 100)
- **PostgreSQL max_connections budget**: Reserve 10-20 connections for admin/monitoring. Divide remainder across application instances
- **Example**: PostgreSQL `max_connections=100`, 3 API instances → each instance gets ~25 max connections
- **Connection lifetime**: Shorter lifetimes when using PgBouncer or load-balanced replicas (avoids stale connections)
- **Idle connections**: Set `MinConns = MaxConns` in production to avoid connection creation latency under load

**Health Check Pattern**:
```go
// Include in /health/ready endpoint
err := pool.Ping(ctx)
if err != nil {
    return fmt.Errorf("database not ready: %w", err)
}
```

**Monitoring**: Track pool stats via `pool.Stat()`:
- `AcquireCount()` — total connections acquired
- `AcquireDuration()` — total time waiting for connections
- `IdleConns()` — current idle connections
- `TotalConns()` — total connections in pool
- Alert on `AcquireDuration` increases (pool exhaustion indicator)

**Supplementary: PgBouncer** — Document PgBouncer as an option for production deployments with multiple API instances.

---

### 5.4 Transaction Patterns (FR-3.4)

#### Industry Standards

Go/PostgreSQL transaction patterns (2025-2026):
- **READ COMMITTED** is the PostgreSQL default and recommended for most operations
- **REPEATABLE READ** for consistent reads within a transaction
- **SERIALIZABLE** for critical sections (financial, inventory) — requires retry logic
- Deferred rollback pattern: `defer tx.Rollback()` after `db.BeginTx()`
- Deadlock prevention: consistent lock ordering, short transactions, proper indexing
- Transaction helper pattern: wrapper function that handles Begin/Commit/Rollback lifecycle

#### Alternative Approaches Evaluated

##### Approach 1: Standard Pattern (BeginTx + defer Rollback)
**Description**: Use the standard Go pattern: `db.BeginTx(ctx, opts)` → `defer tx.Rollback()` → operations → `tx.Commit()`.

**Pros**:
- Idiomatic Go
- Automatic rollback on error or panic
- Clear transaction boundaries
- Context propagation for cancellation

**Cons**:
- Boilerplate in every repository method
- Must remember defer pattern

**Use Cases**: Primary transaction pattern. Document with examples for all common scenarios.

##### Approach 2: Transaction Helper Function
**Description**: Create a helper function that wraps the transaction lifecycle, accepting a function that receives the transaction.

**Pros**:
- Reduces boilerplate
- Consistent error handling
- Panic recovery built-in
- Isolation level configuration centralized

**Cons**:
- Additional abstraction layer
- May obscure transaction boundaries

**Use Cases**: Supplementary pattern for complex operations requiring multiple repository calls.

##### Approach 3: Retry with Exponential Backoff
**Description**: Implement retry logic for serialization errors and deadlocks when using SERIALIZABLE isolation.

**Pros**:
- Handles transient failures
- Essential for SERIALIZABLE isolation
- Configurable retry limits

**Cons**:
- Adds complexity
- Must ensure idempotency of operations
- Only needed for SERIALIZABLE isolation

**Use Cases**: Document as a pattern for critical operations requiring SERIALIZABLE isolation.

#### Comparison Matrix

| Criteria | Standard Pattern | Transaction Helper | Retry with Backoff |
|----------|-----------------|-------------------|-------------------|
| Simplicity | High | Medium | Low |
| Boilerplate | High | Low | Medium |
| Error Handling | Manual | Automatic | Automatic |
| Panic Recovery | Via defer | Built-in | Built-in |
| Use Case | General | Complex operations | SERIALIZABLE |

#### Recommendation for Transaction Patterns

**Primary: Standard Pattern** — Document the standard BeginTx + defer Rollback pattern with clear examples. Include isolation level guidance and deadlock prevention tips.

**Supplementary: Transaction Helper** — Document the optional helper function pattern for complex operations.

---

### 5.5 Query Pattern Library (FR-1.4, FR-3.5)

#### Industry Standards

Query pattern documentation (2025-2026):
- **Cursor-based pagination** is preferred for large datasets (no offset degradation)
- **CTEs** (Common Table Expressions) are the standard for multi-step queries in PostgreSQL
- **JSONB operators** (`@>`, `->`, `->>`) for querying semi-structured data
- **GIN indexes** for JSONB and full-text search
- Query patterns documented with: SQL example, SQLC annotation, Go repository usage, performance notes

#### Cursor vs Offset Pagination (FR-1.4 Acceptance Criteria)

The FSD requires documenting performance trade-offs, use case guidance, and index requirements for pagination. Research (Sentry, Gusto Embedded, SupaExplorer, caduh.com) establishes clear guidance:

##### Performance Comparison

| Metric | Offset Pagination | Cursor Pagination |
|--------|-------------------|-------------------|
| Time Complexity | O(N) — degrades linearly | O(1) — constant regardless of position |
| Page 1 Latency | ~50ms | ~50ms |
| Page 100 Latency | ~500ms | ~50ms |
| Page 1000 Latency | 5+ seconds (100x slower) | ~50ms |
| Write Stability | Unstable — inserts/deletes cause gaps/duplication | Stable — tracks records, not positions |
| Total Count | Easy (COUNT query) | Hard/expensive |
| Random Access | Native (jump to page N) | Not native (sequential only) |

**Offset Degradation Mechanism**: PostgreSQL must scan and discard OFFSET rows before returning results. At `OFFSET 99000`, the database processes 99,100 rows to return 100. This grows linearly and becomes unusable beyond page 100 on tables with 1M+ rows.

**Cursor Efficiency Mechanism**: Cursor uses indexed WHERE clause (e.g., `WHERE id > 1234 ORDER BY id LIMIT 100`) — the database uses the index to jump directly to the cursor position. No row skipping, constant performance.

##### When to Use Each Approach

| Requirement | Use Offset | Use Cursor |
|-------------|------------|------------|
| "Jump to page N" UI | ✅ | ❌ |
| Infinite scroll / "Load more" | ❌ | ✅ |
| Small datasets (<10K rows) | ✅ | Either |
| Large datasets (>100K rows) | ❌ | ✅ |
| Admin dashboards with page numbers | ✅ | ❌ |
| Real-time feeds (social, chat) | ❌ | ✅ |
| Data exports with pagination | ✅ | ✅ |
| High write-rate tables | ❌ | ✅ |
| SEO category pages with numbered links | ✅ | ❌ |

##### Index Requirements for Efficient Pagination

**Offset Pagination**:
- Index on ORDER BY column(s) — B-tree on `created_at DESC`
- Covering index for common filter combinations
- Example: `CREATE INDEX idx_posts_created ON posts(created_at DESC);`

**Cursor Pagination (Keyset)**:
- **Required**: Composite index on `(sort_column, tie_breaker)` — e.g., `(created_at DESC, id DESC)`
- **Critical**: Always include a unique tie-breaker column (typically `id`) in ORDER BY to ensure deterministic ordering when sort column has duplicates
- **Filter indexes**: Extend index for common filters: `(author_id, created_at DESC, id DESC)`
- Example: `CREATE INDEX idx_posts_cursor ON posts(created_at DESC, id DESC);`

**Cursor Encoding**: Encode cursor as base64url of JSON containing last row's sort values. Include filter snapshots for validation.

#### Documentation Approach

Document query patterns as markdown files organized by category:
- **CRUD operations**: Create, Read (single/list), Update, Delete (hard/soft)
- **Filtering**: Static filters, dynamic filters, JSONB containment
- **Pagination**: Cursor-based (recommended), offset-based (with performance caveats)
- **Joins**: INNER, LEFT, subqueries, CTEs
- **Aggregations**: GROUP BY, HAVING, window functions
- **PostgreSQL-specific**: JSONB operations, array operations, CTEs, partial indexes

Each pattern includes:
1. SQL example
2. SQLC annotation (`-- name: FunctionName :one/:many/:exec`)
3. Go repository usage
4. Performance considerations

#### Recommendation for Query Pattern Library

Document patterns as markdown files in `documentation/database-design/query-patterns/`. Cross-reference with SQLC query files for living examples.

---

## 6. Index Strategy Documentation (FR-1.3)

### Industry Standards

PostgreSQL index documentation (2025-2026):
- **B-tree** is the default index type (equality, range, sorting)
- **GIN** for JSONB, arrays, full-text search
- **GiST** for geometric data, exclusion constraints
- **Partial indexes** for filtered queries (e.g., `WHERE deleted_at IS NULL`)
- **Composite indexes** for multi-column queries
- Index naming convention: `idx_{table}_{columns}`

### Documentation Approach

Document indexes as:
1. Section within each table's schema documentation
2. Summary document: `documentation/database-design/indexes.md`

For each index, document:
- Table and column(s)
- Index type (B-tree, GIN, GiST, partial)
- Purpose (which query pattern it supports)
- Partial conditions (if applicable)
- Performance rationale

### Recommendation for Index Documentation

Use the custom Go script (Section 1) to extract index metadata from `pg_catalog`. Document as sections within table docs plus a summary file. Include missing index recommendations based on query pattern analysis.

---

## 7. SQLC Workflow Documentation (FR-2.3)

### Industry Standards

SQLC workflow patterns for Go/PostgreSQL (2025-2026):
- **One query file per domain**: `users.sql`, `agents.sql`, `memory.sql`
- **Meaningful query names**: `GetActiveUsers`, `CreateOrderWithItems` (not `GetData`)
- **SQLC annotations**: `-- name: FunctionName :one/:many/:exec` controls return types
- **Repository pattern**: Encapsulate SQLC queries in repository methods
- **CI/CD verification**: `sqlc diff` in pipeline to detect generated code drift

### Alternative Approaches Evaluated

#### Approach 1: Domain-Organized Query Files
**Description**: Organize SQLC query files by domain (one file per entity). Each file contains all queries for that entity.

**Pros**:
- Easy to navigate as codebase grows
- Clear ownership of queries
- Matches repository pattern structure
- Industry standard pattern

**Cons**:
- Cross-domain queries need special handling
- May lead to large files for complex entities

**Use Cases**: Primary organization approach for SQLC query files.

#### Approach 2: Operation-Organized Query Files
**Description**: Organize query files by operation type (crud.sql, reports.sql, admin.sql).

**Pros**:
- Groups similar operations together
- May simplify certain analyses

**Cons**:
- Queries for same entity scattered across files
- Harder to maintain entity-centric view

**Use Cases**: Not recommended for this unit. Domain organization is superior.

### Recommendation for SQLC Workflow

**Primary: Domain-Organized** — Document the domain-based query file organization. Include:
- `sqlc.yaml` configuration and options
- Query annotation syntax with examples
- Workflow: edit `.sql` → run `sqlc generate` → use generated Go code
- Repository pattern integration
- CI/CD verification with `sqlc diff`

---

## 8. Endpoint-to-Database Mapping (FR-2.2)

### Industry Standards

Endpoint-to-database mapping documentation (2025-2026):
- Trace the call chain: Handler → Service → Repository → SQLC query
- Document transaction boundaries
- Include SQLC query file locations and generated Go function names
- Map data transformations between layers

### Documentation Approach

Create `documentation/api/endpoint-map.md` with:
1. For each endpoint:
   - HTTP method and path
   - Handler function name
   - Service layer method
   - Repository methods invoked
   - SQLC queries called (with file references)
   - Transaction boundaries
   - Data transformations
   - Authorization checks

### Recommendation for Endpoint-to-Database Mapping

Document as a markdown file organized by endpoint. Use tables to show the call chain. Include SQLC file references and Go function names.

---

## 9. Error Code Documentation (FR-2.4)

### Industry Standards

API error code standardization (2025-2026):
- **RFC 9457 (Problem Details)** is the current standard for HTTP API error responses (supersedes RFC 7807)
- **Standard envelope**: `{ "success": false, "error": { "code": "...", "message": "...", "details": [...] } }`
- **Error taxonomy**: Client errors (4xx), server errors (5xx), transient errors (retryable), permanent errors
- **Database-to-API error mapping**: Unique violation → 409, foreign key violation → 404, not null → 400

### Alternative Approaches Evaluated

#### Approach 1: RFC 9457 Problem Details
**Description**: Follow RFC 9457 for structured error responses with type, title, status, detail, and instance fields.

**Pros**:
- Industry standard (IETF standard)
- Machine-readable error types (URI-based)
- Extensible with custom fields
- Supported by modern API frameworks
- Cloudflare reports 98% token reduction for AI agents vs HTML errors

**Cons**:
- More complex than simple error objects
- URI-based type field may be overkill for internal APIs

**Use Cases**: Recommended for external-facing APIs. Consider for internal APIs if agent consumption is important.

#### Approach 2: Custom Envelope with Error Codes
**Description**: Use a custom error envelope with string error codes, messages, and details array. Map database errors to API error codes.

**Pros**:
- Simple and flexible
- Easy to implement
- Clear error code taxonomy
- Matches existing ACE response pattern

**Cons**:
- Not a standard format
- Clients must learn custom format

**Use Cases**: Primary approach if RFC 9457 is too complex. Document the error code taxonomy clearly.

#### Approach 3: Stripe-Style Error Codes
**Description**: Use snake_case error codes organized by category (e.g., `card_declined`, `resource_missing`, `parameter_missing`).

**Pros**:
- Proven pattern (Stripe's API is widely admired)
- Human-readable error codes
- Easy to search and document
- Category-based organization

**Cons**:
- Proprietary pattern (not a standard)
- Requires careful taxonomy design

**Use Cases**: Inspiration for error code naming conventions.

### Comparison Matrix

| Criteria | RFC 9457 | Custom Envelope | Stripe-Style |
|----------|----------|-----------------|--------------|
| Standard Compliance | Full IETF standard | Custom | Inspired by Stripe |
| Machine Readability | Excellent | Good | Good |
| Human Readability | Good | Good | Excellent |
| Implementation Complexity | Medium | Low | Low |
| Agent Consumption | Excellent (98% token reduction) | Good | Good |

### Recommendation for Error Code Documentation

**Primary: Custom Envelope with Structured Error Codes** — Document the existing ACE response envelope pattern with a clear error code taxonomy. Map database errors to API error codes:
- Unique constraint violation → 409 Conflict
- Foreign key violation → 404 Not Found
- Not null violation → 400 Bad Request
- Check constraint → 400 Bad Request

**Supplementary: RFC 9457 Reference** — Document RFC 9457 as an option for future adoption, particularly for agent consumption benefits.

---

## 10. Standardization & Legacy Migration (FA-5)

### 10.1 Legacy Pattern Identification (FR-5.1)

#### Industry Standards

Legacy pattern identification methods (2025-2026):
- **Automated scanning**: Use linters and schema analysis tools to detect non-standard patterns
- **Manual audit**: Review existing schema against documented conventions
- **Migration complexity scoring**: Categorize patterns by migration difficulty (trivial, moderate, complex)

#### Alternative Approaches Evaluated

##### Approach 1: Custom Go Script + pg_catalog Analysis
**Description**: Write a Go script that queries PostgreSQL's system catalogs to detect non-standard patterns: camelCase columns, missing NOT NULL constraints, inconsistent data types, missing indexes on foreign keys.

**Pros**:
- Full control over detection rules
- Integrates with existing Go/PostgreSQL stack
- Can generate markdown report with findings
- Can be run in CI/CD for ongoing compliance checking
- Matches existing documentation generation approach (Section 1)

**Cons**:
- Must maintain detection rules as conventions evolve
- Initial development effort

**Use Cases**: Primary approach for ongoing legacy pattern detection. Run periodically and in CI/CD.

##### Approach 2: SchemaCrawler Lint (Open-Source)
**Description**: Java-based schema analysis tool with built-in lint checks. Detects: columns with same name but different types across tables, foreign keys without indexes, tables without primary keys, redundant indexes, tables with all nullable columns, badly named columns.

**Pros**:
- 20+ built-in lint checks for common schema anti-patterns
- Configurable rules via YAML
- Supports PostgreSQL
- Free and open-source
- Mature tool (widely used in enterprise)

**Cons**:
- Requires Java runtime
- Output format is not markdown (requires conversion)
- No direct integration with Go tooling

**Use Cases**: Supplementary tool for comprehensive schema analysis. Run against production database for thorough audit.

##### Approach 3: pgSchema Declarative Diff
**Description**: Use pgschema (Go-based) to dump current schema, compare against desired state, and identify deviations from standards.

**Pros**:
- Written in Go (stack-aligned)
- Terraform-style declarative workflow
- Detects all schema objects including PostgreSQL-specific features
- Can identify objects that don't match desired conventions

**Cons**:
- Requires defining "desired state" schema
- Primarily designed for migration, not linting
- Newer tool (2025)

**Use Cases**: Alternative for teams wanting declarative schema management. Can be combined with custom lint rules.

##### Approach 4: Manual Audit with Schema Diff
**Description**: Manual review of schema against documented conventions, using tools like pgAdmin Schema Diff or pgCompare to identify differences between environments.

**Pros**:
- Catches nuances automated tools miss
- Human judgment for complex decisions
- No tooling dependencies

**Cons**:
- Time-consuming
- Not repeatable or scalable
- Subject to human error
- Cannot be automated in CI/CD

**Use Cases**: Complementary to automated approaches. Use for initial assessment and complex pattern evaluation.

#### Comparison Matrix

| Criteria | Custom Go Script | SchemaCrawler Lint | pgSchema Diff | Manual Audit |
|----------|-----------------|-------------------|---------------|--------------|
| Stack Alignment | Excellent | Poor (Java) | Excellent (Go) | N/A |
| Automation | Full (CI/CD) | Partial | Full | None |
| Detection Depth | Custom | 20+ built-in checks | Schema-level | Variable |
| Maintenance | Custom rules | Community | Community | Manual |
| Output Format | Markdown | Custom | SQL | Ad-hoc |
| Cost | Free | Free | Free | Time-intensive |

#### Recommendation for Legacy Pattern Identification

**Primary: Custom Go Script** — Extend the documentation generation script (Section 1) to include legacy pattern detection. Check for:
- Non-standard naming (camelCase columns, inconsistent prefixes)
- Missing constraints (NOT NULL, CHECK, FOREIGN KEY)
- Inconsistent data types (VARCHAR for timestamps, INTEGER for booleans)
- Missing indexes on foreign keys
- Hardcoded status values (integers instead of ENUM or VARCHAR)

**Supplementary: SchemaCrawler Lint** — Run periodically for comprehensive audit. Configure to detect project-specific patterns.

#### Documentation Approach

Create `documentation/database-design/legacy-patterns.md` with:
1. Catalog of legacy patterns:
   - Non-standard naming (e.g., camelCase columns)
   - Missing constraints (e.g., missing NOT NULL)
   - Inconsistent data types (e.g., VARCHAR for timestamps)
   - Hardcoded values (e.g., status as integers instead of strings)
2. For each pattern:
   - Current implementation
   - Modern equivalent
   - Migration complexity
   - Backward compatibility considerations

### Recommendation for Legacy Pattern Documentation

Document as a catalog sorted by migration complexity. Use the custom Go script to identify patterns programmatically.

---

### 10.2 Phased Migration Strategy (FR-5.2)

#### Industry Standards

Phased migration strategies for database standardization (2025-2026):
- **Phase 1: Non-breaking changes** — Renaming indexes, adding comments, adding documentation
- **Phase 2: Low-risk changes** — Adding constraints, standardizing types (with defaults)
- **Phase 3: Moderate changes** — Column renames with backward-compatible views
- **Phase 4: Breaking changes** — Table restructures, data migrations

#### Alternative Approaches Evaluated

##### Approach 1: Strangler Fig Pattern
**Description**: Gradually migrate from legacy to modern patterns, keeping both running simultaneously during transition.

**Pros**:
- Zero downtime
- Incremental validation
- Rollback capability at each step

**Cons**:
- Complexity of maintaining two systems
- Longer migration timeline
- Dual-write requirements

**Use Cases**: Recommended for breaking changes (Phase 4).

##### Approach 2: Blue-Green Schema Migration
**Description**: Create new schema alongside old, migrate data, switch over.

**Pros**:
- Clean break from legacy
- Full rollback capability
- Clear before/after state

**Cons**:
- Higher risk (big-bang cutover)
- Data migration complexity
- Requires downtime or dual-write

**Use Cases**: Alternative for major schema restructures.

##### Approach 3: View-Based Backward Compatibility
**Description**: Create database views that present legacy column names while underlying table uses modern names.

**Pros**:
- Allows incremental code migration
- No application downtime
- Transparent to consumers

**Cons**:
- View maintenance overhead
- Performance implications for complex views
- Temporary solution (must be removed)

**Use Cases**: Recommended for column renames (Phase 3).

### Comparison Matrix

| Criteria | Strangler Fig | Blue-Green | View-Based |
|----------|---------------|------------|------------|
| Downtime | Zero | Potential | Zero |
| Complexity | High | Medium | Low |
| Rollback | Incremental | Full | Easy |
| Timeline | Longer | Shorter | Medium |
| Use Case | Breaking changes | Major restructures | Column renames |

### Recommendation for Phased Migration Strategy

**Primary: Four-Phase Plan with Hybrid Approaches** — Document the phased migration strategy using:
- Phase 1-2: Direct migration (non-breaking changes)
- Phase 3: View-based backward compatibility (column renames)
- Phase 4: Strangler fig pattern (breaking changes)

---

### 10.3 Backward Compatibility Framework (FR-5.4)

#### Industry Standards

Backward compatibility patterns for database migrations (2025-2026):
- **Expand-contract pattern**: Add new column → migrate data → update code → remove old column
- **Database views for renamed columns**: Create view with old column names pointing to new columns
- **API versioning**: Maintain API response stability during schema migration
- **Deprecation notices**: Communicate breaking changes with advance notice period

#### Alternative Approaches Evaluated

##### Approach 1: Expand-Contract Pattern
**Description**: Four-phase migration that maintains backward compatibility at every step:
1. **Expand**: Add new column/table alongside old (old code continues working)
2. **Migrate**: Dual-write to both old and new; backfill data
3. **Shift**: Update application code to read from new schema only
4. **Contract**: Drop old column/table after all consumers migrated

**Pros**:
- Safe rollback at phases 1-3 (switch back to old schema without touching DB)
- Zero downtime
- Proven in production at scale (Stripe, Gusto, major SaaS platforms)
- Clear separation of schema changes from application changes

**Cons**:
- Four deployments required (higher operational overhead)
- Temporary dual-write complexity
- Longer migration timeline
- Must coordinate across services

**Use Cases**: Primary approach for any change that breaks existing consumers (column renames, type changes, table restructures).

##### Approach 2: Database View Compatibility
**Description**: Create database views that present old column names while underlying table uses new names. Application gradually migrates from view to base table.

**Example**:
```sql
-- After renaming 'name' to 'full_name'
CREATE VIEW users_compat AS
SELECT id, full_name AS name, email, created_at FROM users;
```

**Pros**:
- Allows incremental code migration without dual-write
- No application downtime
- Transparent to consumers using the view
- Simple to implement for column renames

**Cons**:
- View maintenance overhead (must update when base table changes)
- Performance implications for complex views (especially with joins)
- Temporary solution (must be removed eventually)
- Doesn't work well for type changes

**Use Cases**: Best for simple column renames where the consumer set is well-defined. Complementary to expand-contract for Phase 3 compatibility.

##### Approach 3: API Versioning with Schema Abstraction
**Description**: Version the API (e.g., `/v1/`, `/v2/`) and map each version to the appropriate schema representation. Old API version continues working with adapter layer.

**Pros**:
- Clean API contract stability
- Multiple versions can coexist indefinitely
- Frontend teams migrate at their own pace
- Clear deprecation timeline per version

**Cons**:
- Significant code duplication (adapter layer per version)
- Higher maintenance burden
- Doesn't address internal service-to-service calls
- Overkill for simple column renames

**Use Cases**: Best for major API contract changes where multiple frontend versions must be supported simultaneously.

#### Comparison Matrix

| Criteria | Expand-Contract | Database View | API Versioning |
|----------|-----------------|---------------|----------------|
| Downtime | Zero | Zero | Zero |
| Complexity | High (4 phases) | Low | Very High |
| Rollback Safety | Excellent (phases 1-3) | Good | Excellent |
| Timeline | Longest | Short | Long |
| Code Duplication | Temporary | None | Permanent |
| Best For | Any breaking change | Column renames | Major API changes |
| Data Migration | Built-in | Manual | Adapter layer |

#### Recommendation for Backward Compatibility

**Primary: Expand-Contract Pattern** — Document as the default approach for any breaking schema change. The four-phase pattern provides safe rollback at every step and is the industry standard for production migrations.

**Supplementary: Database Views** — Document view-based compatibility as a lightweight option for simple column renames. Use as Phase 3 of expand-contract or as standalone for minor changes.

**Documentation**: Create `documentation/database-design/compatibility.md` with examples of each pattern, decision criteria for when to use each, and deprecation timeline templates.

---

### 10.4 Refactoring Guidelines (FR-5.3)

#### Industry Standards

Database refactoring testing strategies (2025-2026):
- **Before/after data validation**: Verify data integrity before and after migration
- **Performance regression testing**: Compare query performance before and after
- **Application integration testing**: Test all API endpoints against migrated schema
- **Rollback testing**: Verify rollback functions restore previous state

#### Documentation Approach

Document refactoring patterns with testing requirements:
1. How to rename columns with backward compatibility (views, aliases)
2. How to change data types safely (expand-contract)
3. How to add constraints to existing data (validate existing data first)
4. How to split/merge tables (data migration + application changes)

Each pattern includes:
- Step-by-step instructions
- Testing requirements
- Rollback procedure
- Review process (PR requirements, required reviewers)

### Recommendation for Refactoring Guidelines

Document as section within `documentation/database-design/migration-plan.md`. Include testing requirements and review process for each refactoring pattern.

---

## 11. Migration Documentation

### Industry Standards

Database migration documentation for Go/PostgreSQL (2025-2026):
- **Goose** is the dominant Go migration tool, supporting both SQL and Go migration functions
- Timestamp-based file naming: `YYYYMMDDHHMMSS_description.go`
- Forward-only strategy is recommended for production (simpler, safer)
- CI/CD integration for migration validation and testing
- Schema drift detection via declarative schema tools (Atlas) or automated comparison

### Alternative Approaches Evaluated

#### Approach 1: Goose Native Documentation Pattern
**Description**: Document the existing Goose workflow (already in use) including `init()` registration, `goose.AddMigration(up, down)`, timestamp naming, and Go-based migrations. Include `goose status`, `goose up`, `goose down` command references.

**Pros**:
- Already in use (no tooling change required)
- Go-based migrations allow complex logic (data transformations, conditionals)
- `init()` registration pattern ensures migrations are discovered automatically
- Well-documented tool with active community (GitHub, r/golang consensus: "Goose all day")
- Forward-only strategy supported

**Cons**:
- Limited built-in drift detection
- Manual testing required (no automated schema comparison)
- `down` functions must be maintained manually

**Use Cases**: Primary migration documentation approach. Document existing Goose workflow, rollback patterns, testing patterns, and schema versioning.

#### Approach 2: Atlas Declarative + Goose Hybrid
**Description**: Use Atlas for schema visualization and drift detection while keeping Goose for execution. Atlas compares live database against desired state; Goose handles the actual migration execution.

**Pros**:
- Drift detection (compare live DB vs desired state)
- Schema visualization (ERD from schema definitions)
- CI/CD integration for automated schema checks
- Keeps existing Goose workflow

**Cons**:
- Additional tool dependency
- Atlas extensions behind paywall (e.g., PostgreSQL extensions)
- Complexity of maintaining two tools
- Risk of divergence between Atlas schema and Goose migrations

**Use Cases**: Supplementary approach for drift detection and schema validation in CI/CD. Document as an optional enhancement.

#### Approach 3: Schema Dump + Migration Validation Pattern
**Description**: After running migrations, dump the resulting schema to a `schema.sql` file. CI compares the dump against expected schema. Detects drift when someone makes manual changes outside of migrations.

**Pros**:
- Simple, no additional tool dependencies
- Works with existing Goose workflow
- Git-trackable schema state
- Drift detection via diff comparison

**Cons**:
- Schema dump must be regenerated on every migration change
- Requires careful CI setup
- Manual process to keep dump in sync

**Use Cases**: Recommended approach for schema validation. Document as part of migration testing patterns (FR-4.3).

### Comparison Matrix

| Criteria | Goose Native | Atlas + Goose Hybrid | Schema Dump + Validation |
|----------|-------------|---------------------|------------------------|
| Existing Stack | Yes | Requires Atlas | Yes |
| Drift Detection | No | Yes | Yes (via diff) |
| CI/CD Integration | Manual | Built-in | Custom |
| Complexity | Low | Medium | Low |
| Cost | Free | Freemium | Free |
| Schema Viz | No | Yes | No |

### Recommendation for Migration Documentation

**Primary: Goose Native Documentation** — Document the existing Goose workflow comprehensively. Include:
- Migration strategy (forward-first, FR-4.1)
- Rollback patterns with decision tree (FR-4.2)
- Testing patterns (FR-4.3): unit tests for up/down, integration tests with fresh DB, rollback tests
- Schema versioning via `goose_db_version` table (FR-4.4)

**Supplementary: Schema Dump Validation** — Document the pattern of dumping schema after migration and comparing in CI. This provides drift detection without additional tool dependencies.

---

### 11.1 Rollback Patterns & Decision Tree (FR-4.2)

#### Industry Standards

Rollback strategies for database migrations (2025-2026):
- **Forward fix is preferred over rollback** — once new data exists, undoing schema changes can be more dangerous than adapting behavior (DevX, ArgoCD guidance)
- **Expand-contract pattern** provides safe rollback at every phase without touching the database (OneUptime, DevX)
- **Compensating migrations** (write a new migration that undoes the effect) are safer than `down` migrations in production
- **Pre-migration backups** enable point-in-time recovery for critical failures

#### Rollback Scenarios (FR-4.2 Requirement)

##### Scenario 1: Pre-Deployment Rollback (Safe)
**When**: Migration has been written but not yet deployed to production.
**Action**: Simply don't deploy. Revert the migration commit in Git.
**Risk**: Low — no production data affected.
**Safety checks**: None required beyond normal code review.

##### Scenario 2: Post-Deployment Rollback (Risky)
**When**: Migration has been applied to production; application code may already depend on new schema.
**Action**: Use compensating migration or expand-contract rollback.
**Risk**: High — new data may exist in new schema; old application code may not handle new data.
**Safety checks**:
1. Verify no dependent deployments reference new schema
2. Check if new data has been written to new columns/tables
3. Backup affected data before rollback
4. Test rollback procedure in staging with production-like data
5. Coordinate with frontend/API teams on response format changes

##### Scenario 3: Data Migration Rollback (Very Risky)
**When**: Migration included data transformation (backfill, column rename with data copy).
**Action**: Point-in-time recovery from pre-migration backup, or write compensating data migration.
**Risk**: Very high — data loss possible.
**Safety checks**:
1. Pre-migration backup is mandatory
2. Verify backup integrity before starting rollback
3. Test data restoration in staging
4. Plan for data written between migration and rollback (may be lost)

#### Rollback Decision Tree

```
Migration failed or causing issues?
│
├─ NOT YET DEPLOYED TO PRODUCTION?
│  └─ YES → Revert Git commit. Do not deploy. (SAFE)
│
├─ DEPLOYED TO PRODUCTION?
│  │
│  ├─ Is it an ADDITIVE change? (new column, new table, new index)
│  │  ├─ NO DATA written to new schema yet?
│  │  │  └─ YES → Run compensating migration (DROP column/table/index) (MODERATE RISK)
│  │  │
│  │  └─ DATA EXISTS in new schema?
│  │     └─ YES → Forward fix preferred. Keep new schema, fix application code. (PREFER FORWARD FIX)
│  │
│  ├─ Is it a DESTRUCTIVE change? (DROP column, DROP table, type change)
│  │  ├─ Pre-migration backup EXISTS?
│  │  │  └─ YES → Point-in-time recovery from backup (HIGH RISK — data loss possible)
│  │  │
│  │  └─ NO backup?
│  │     └─ Forward fix mandatory. Cannot safely rollback. (FORWARD FIX ONLY)
│  │
│  └─ Is it a RENAME or RESTRUCTURE?
│     ├─ Expand-contract phase 1-3 (old schema still exists)?
│     │  └─ YES → Switch reads/writes back to old schema via feature flag (LOW RISK)
│     │
│     └─ Expand-contract phase 4 (old schema dropped)?
│        └─ Restore from backup or forward fix (HIGH RISK)
```

#### `down` Function Requirements

Document the following requirements for Goose `down` functions:
1. **Must restore schema to previous state** — reverse all changes made in `up`
2. **Must handle data loss gracefully** — document that `down` may destroy data
3. **Must be tested before deployment** — run `up` then `down` in test DB, verify schema state
4. **Must be idempotent** — use `IF EXISTS` / `IF NOT EXISTS` for safety
5. **Should NOT be used in production for data-destructive operations** — prefer compensating migrations

#### Recommendation for Rollback Documentation

Document the three rollback scenarios with the decision tree above. Include `down` function requirements and safety checklists. Emphasize forward-fix as the preferred approach for post-deployment issues.

---

## 12. Documentation Validation & Freshness

### Industry Standards

Documentation validation has become a first-class CI/CD concern (2025-2026):
- Schema drift detection via automated comparison (Atlas, Liquibase/Flyway diff modules)
- Pre-commit hooks validate documentation when schema changes are detected
- CI/CD pipeline fails when documentation doesn't match live schema
- Automated diff comparison between schema and documentation on every PR

### Alternative Approaches Evaluated

#### Approach 1: Custom Go Validation Script
**Description**: Write a Go script that extracts schema from migrations/`pg_catalog`, compares against documentation markdown files, and fails the build if discrepancies are found. Run in CI/CD pipeline.

**Pros**:
- Full control over validation logic
- Integrates with existing Go tooling
- Can validate specific aspects (column types, constraints, indexes)
- Fast execution (Go compiled binary)
- Matches FR-1.1 acceptance criteria exactly

**Cons**:
- Must maintain as schema evolves
- No community tooling to fall back on

**Use Cases**: Primary validation approach for FR-1.1 and NFR-1 (documentation freshness).

#### Approach 2: Atlas Schema Diff in CI/CD
**Description**: Use Atlas's `schema diff` command in CI/CD to compare live database against expected schema. Fails build on drift.

**Pros**:
- Purpose-built for drift detection
- Integrates with CI/CD workflows
- Can detect destructive changes (column drops, etc.)

**Cons**:
- Requires Atlas adoption
- Freemium model (some features behind paywall)
- Doesn't validate documentation markdown files specifically

**Use Cases**: Supplementary validation approach for schema drift detection.

#### Approach 3: Mermaid CLI Validation
**Description**: Use `@mermaid-js/mermaid-cli` (`mmdc`) to validate Mermaid diagram syntax. Run in CI/CD to ensure diagrams are valid.

**Pros**:
- Catches syntax errors in Mermaid diagrams
- Fast validation (Node.js CLI)
- Can generate SVG/PNG from validated diagrams

**Cons**:
- Only validates syntax, not content accuracy
- Requires Node.js in CI environment

**Use Cases**: Validation step for all Mermaid ERD files.

### Comparison Matrix

| Criteria | Custom Go Script | Atlas Schema Diff | Mermaid CLI |
|----------|-----------------|-------------------|-------------|
| Validates Markdown | Yes | No | No |
| Validates Schema | Yes | Yes | N/A |
| Validates Diagrams | No | No | Yes |
| CI/CD Integration | Excellent | Good | Good |
| Stack Alignment | Excellent | External | External |
| Maintenance | Custom | Community | Community |

### Recommendation for Documentation Validation

**Primary: Custom Go Validation Script** — Build a Go script that:
1. Extracts schema from `pg_catalog` (or runs migrations on test DB)
2. Reads documentation markdown files
3. Compares schema state against documentation
4. Reports discrepancies
5. Fails the build if discrepancies found

**Supplementary Tools**:
- Mermaid CLI (`mmdc`) for diagram syntax validation
- OpenAPI validator (swagger-cli or redocly) for OpenAPI spec validation
- Markdown linter for documentation formatting

---

## Overall Recommendations Summary

| Functional Area | Primary Approach | Supplementary |
|-----------------|-----------------|---------------|
| FA-1: Schema Documentation | Custom Go script + pg_catalog | tbls for exploration |
| FA-1: ERD Generation | SQL → Mermaid generator | Hand-crafted master ERD |
| FA-1: Index Strategy | Custom Go script extraction | Summary markdown file |
| FA-1: Query Patterns | Domain-organized markdown | Cross-referenced with SQLC |
| FA-2: OpenAPI Spec | Annot8 (Chi-native) | swaggo/swag fallback |
| FA-2: Endpoint Mapping | Markdown call chain docs | SQLC file references |
| FA-2: SQLC Workflow | Domain-organized queries | CI/CD verification |
| FA-2: Error Codes | Custom envelope + taxonomy | RFC 9457 reference |
| FA-3: Naming Conventions | snake_case + SQLC mapping | Linter enforcement |
| FA-3: Data Types | Standard type mapping | JSONB vs normalized guidance |
| FA-3: Connection Pooling | pgxpool | PgBouncer for multi-instance |
| FA-3: Transactions | Standard BeginTx pattern | Transaction helper function |
| FA-3: Query Helpers | Documented in markdown | Cross-referenced with code |
| FA-4: Migration Docs | Goose native documentation | Schema dump validation |
| FA-5: Legacy Patterns | Catalog with complexity scoring | Automated scanning |
| FA-5: Migration Plan | Four-phase hybrid approach | View-based compatibility |
| FA-5: Refactoring Guidelines | Pattern-based with testing | Expand-contract pattern |
| FA-5: Backward Compatibility | Expand-contract + views | Deprecation timeline |
| FA-6: Agent Integration | AGENTS.md-style markdown | OpenAPI spec reference |
| Validation | Custom Go script + mmdc | OpenAPI validator |

## Rationale

1. **Stack alignment**: The primary approaches favor tools and patterns native to the Go/PostgreSQL/SQLC/Chi stack. Custom Go scripts for validation and documentation generation avoid introducing external dependencies.

2. **Version control compatibility**: All recommendations produce text-based output (markdown, Mermaid, YAML) that can be version-controlled and diffed in PRs. No binary files or GUI-only tools.

3. **CI/CD integration**: Every approach can be automated in the existing CI/CD pipeline. Documentation generation and validation run as build steps.

4. **Agent consumption**: The AGENTS.md pattern for agent-facing documentation is the industry standard (Linux Foundation, backed by all major AI labs) and provides optimal token efficiency (4x vs HTML per Cloudflare data).

5. **Minimal external dependencies**: The recommendations minimize new tool dependencies. Most approaches use tools already in the stack (Go, PostgreSQL, Mermaid CLI) or well-established tools (Annot8, Goose).

6. **Future-proofing**: Mermaid is natively supported by GitHub, GitLab, VS Code, and all major documentation platforms. OpenAPI 3.1 (via Annot8) is the latest standard. AGENTS.md is converging as the universal agent documentation format.

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Annot8 project health (small community) | swaggo/swag as fallback; Annot8 is Chi+SQLC-specific which reduces need for large community |
| Custom scripts require maintenance | Scripts are simple (pg_catalog queries, markdown generation); well-documented with clear patterns |
| Mermaid diagrams may be hard to read for complex schemas | Use entity grouping, limit columns per diagram, supplement with text descriptions |
| AGENTS.md standard still evolving | Follow current standard (Linux Foundation stewardship); minimal migration risk as format is plain markdown |
| Schema drift between migrations and documentation | CI validation script detects and fails build; pre-commit hook catches early |
| Agent context window limits | Structured markdown with shallow hierarchy; explicit token budgets per document |

## References

### Schema Documentation & ERD
- [How to Document a PostgreSQL Schema in 2025 — DbSchema](https://dbschema.com/blog/postgresql/postgresql-database-documentation/)
- [7 Best Database Documentation Tools for 2025 — Comparitech](https://www.comparitech.com/net-admin/best-database-documentation-tools/)
- [ER Diagrams with SQL and Mermaid — Cybertec PostgreSQL](https://www.cybertec-postgresql.com/en/er-diagrams-with-sql-and-mermaid/)
- [Mermaid Diagram: Complete Guide 2026 — ObsiBrain](https://www.obsibrain.com/blog/mermaid-diagram-a-complete-guide-to-diagrams-as-code-in-2026)

### OpenAPI Generation
- [Annot8 — Chi-Native OpenAPI Generator](https://pkg.go.dev/github.com/AxelTahmid/openapi-gen) (v0.3.0, 2025)
- [go-specgen — OpenAPI from Go Annotations](https://pkg.go.dev/github.com/wontaeyang/go-specgen) (v1.2.1, March 2026)
- [go-respec — Static Analysis OpenAPI](https://github.com/Zachacious/go-respec)
- [chioas — Chi Router OpenAPI Addon](https://pkg.go.dev/github.com/go-andiamo/chioas) (v1.19.1, 2026)
- [oaswrap/spec — Framework-Agnostic OpenAPI Builder](https://pkg.go.dev/github.com/oaswrap/spec) (v0.3.6, 2025)

### Agent Documentation
- [Introducing Markdown for Agents — Cloudflare Blog](https://blog.cloudflare.com/markdown-for-agents/) (February 2026)
- [How to Write a Good Spec for AI Agents — Addy Osmani](https://addyo.substack.com/p/how-to-write-a-good-spec-for-ai-agents) (January 2026)
- [AGENTS.md Standard — Ry Walker Research](https://rywalker.com/research/agents-md-standard)
- [How to Write a Great agents.md — GitHub Blog](https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/)
- [AGENTS.md — EmergentMind Topic Analysis](https://www.emergentmind.com/topics/agents-md-files)
- [In Agentic AI, It's All About the Markdown — Visual Studio Magazine](https://visualstudiomagazine.com/articles/2026/02/24/in-agentic-ai-its-all-about-the-markdown.aspx)

### Migration & Schema Management
- [Goose Migrations — Volito Digital](https://volito.digital/goose-migrations-for-automating-database-changes-version-control-and-rollbacks-with-minimal-downtime/)
- [Database Migrations in Go — OneUptime](https://oneuptime.com/blog/post/2026-02-01-go-database-migrations/view)
- [Schema Drift Detection in CI/CD — ResearchGate](https://www.researchgate.net/publication/398567149)
- [Database CI/CD & Schema Change Tools 2026 — DbVisualizer](https://medium.com/the-table-sql-and-devtalk/top-database-ci-cd-schema-change-tools-to-know-in-2026-4fdb798f39fd)

### Naming Conventions & Data Types
- [Why Snake Case Is the Best Naming Convention for PostgreSQL — Piyush Doorwar](https://medium.com/mr-plan-publication/why-snake-case-is-the-best-naming-convention-for-postgresql-776063a57ff3) (July 2025)
- [PostgreSQL Naming Conventions — GeeksforGeeks](https://www.geeksforgeeks.org/postgresql/postgresql-naming-conventions/) (July 2025)
- [PostgreSQL SQL Review and Style Guide — Bytebase](https://www.bytebase.com/blog/postgres-sql-review-guide/) (February 2025)
- [JSON and JSONB in PostgreSQL: When and How — Grizzly Peak Software](https://www.grizzlypeaksoftware.com/library/json-and-jsonb-in-postgresql-when-and-how-eimf94cb) (February 2026)
- [gen_random_uuid() vs UUIDv7 in PostgreSQL — Pawan Sharma](https://medium.com/@pawanpg0963/gen-random-uuid-vs-uuidv7-in-postgresql-a-deep-practical-comparison-334d3f564f05) (October 2025)
- [PostgreSQL Data Types: A Guide — Timescale](https://medium.com/timescale/choosing-the-right-postgresql-data-types-for-performance-and-functionality-d3c7cc0a55eb) (April 2025)

### Connection Pooling
- [How to Implement Connection Pooling in Go for PostgreSQL — OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-postgresql-connection-pooling/view) (January 2026)
- [Mastering Database Connection Pooling in Go — dev.to](https://dev.to/nithinbharathwaj/mastering-database-connection-pooling-in-go-performance-best-practices-4mic) (April 2025)
- [Go database/sql Deep Dive: Connection Pool Tuning — syarif](https://elsyarifx.medium.com/go-database-sql-deep-dive-connection-pool-tuning-robust-error-handling-f3f8256d87d4) (May 2025)

### Transaction Patterns
- [PostgreSQL Transaction Isolation Levels Explained — Mydbops](https://www.mydbops.com/blog/postgresql-transaction-isolation-levels-guide) (March 2026)
- [PostgreSQL Concurrency Control — Nemanja Tanaskovic](https://nemanjatanaskovic.com/postgresql-concurrency-control-isolation-levels-locks-and-real-world-race-conditions/) (February 2026)
- [Advanced Go & PostgreSQL: Production-Ready Best Practices](https://henrywithu.com/advanced-go-postgresql-production-ready-best-practices/) (August 2025)
- [Database Transactions: A Comprehensive Guide with Go — dev.to](https://dev.to/jad_core/database-transactions-a-comprehensive-guide-with-go-2e52) (July 2025)
- [Database Deadlock Prevention Guide — ai2sql.io](https://ai2sql.io/learn/database-deadlock-prevention-guide)

### SQLC Workflow
- [How to Use sqlc for Type-Safe Database Access in Go — OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-sqlc-type-safe-database/view) (January 2026)
- [Mastering Data Access in Go with Repositories & sqlc — dev.to](https://dev.to/greyisheepai/clean-performant-and-testable-mastering-data-access-in-go-with-repositories-sqlc-2m9m) (May 2025)
- [Go with PostgreSQL: Complete CRUD Tutorial Using sqlc — Reintech](https://reintech.io/blog/go-postgresql-crud-tutorial-sqlc) (February 2026)

### Error Code Standards
- [API Error Handling That Won't Make Users Rage-Quit — Zuplo](https://zuplo.com/learning-center/optimizing-api-error-handling-response-codes/) (April 2025)
- [Best Practices for Consistent API Error Handling — Zuplo](https://zuplo.com/blog/2025/02/11/best-practices-for-api-error-handling) (February 2025)
- [Problem Details (RFC 9457) — Swagger.io](https://swagger.io/blog/problem-details-rfc9457-doing-api-errors-well) (May 2024)
- [Errors Best Practices in REST API Design — Speakeasy](https://www.speakeasy.com/api-design/errors) (January 2026)
- [Error Handling Patterns in Distributed APIs — FinlyInsights](https://finlyinsights.com/error-handling-patterns-in-distributed-apis/) (March 2026)
