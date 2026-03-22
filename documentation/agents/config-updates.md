# Agent Config Updates

**FSD Requirement**: FR-6.5

---

## Overview

This document covers the process for updating OpenCode agent configurations to reference and use the database design documentation. It includes YAML templates, documentation path integration, and integration test specifications.

---

## Agent Configuration Template

Add the following to agent tool configurations to enable documentation-aware code generation:

```yaml
# Agent tool context directories
# Add these paths so agents can reference documentation during code generation
context_directories:
  - documentation/database-design/schema/
  - documentation/database-design/conventions.md
  - documentation/database-design/indexes.md
  - documentation/database-design/query-patterns/
  - documentation/database-design/sqlc.md
  - documentation/database-design/migrations.md
  - documentation/api/openapi.yaml
  - documentation/api/endpoint-map.md
  - documentation/api/errors.md
  - documentation/agents/patterns.md
  - documentation/agents/schema-generation.md
  - documentation/agents/api-reference.md

# Prompt templates — reference specific training docs for common tasks
prompt_templates:
  migration_generation:
    reference: documentation/agents/training/adding-a-table.md
    context:
      - documentation/database-design/conventions.md
      - documentation/database-design/migrations.md

  sqlc_query_generation:
    reference: documentation/agents/schema-generation.md
    context:
      - documentation/database-design/sqlc.md
      - documentation/database-design/conventions.md

  api_endpoint_creation:
    reference: documentation/agents/api-reference.md
    context:
      - documentation/api/openapi.yaml
      - documentation/api/errors.md
      - documentation/agents/patterns.md

  schema_documentation:
    reference: documentation/database-design/schema/
    context:
      - documentation/database-design/conventions.md
      - documentation/database-design/erd/
```

---

## Documentation Path Integration

### Required Paths

These paths **MUST** be in the agent context:

| Path | Purpose |
|------|---------|
| `documentation/database-design/conventions.md` | Naming conventions and data types |
| `documentation/database-design/migrations.md` | Migration patterns and templates |
| `documentation/agents/patterns.md` | Constraint-first quick reference |
| `documentation/agents/schema-generation.md` | SQLC/Goose generation workflows |

### Recommended Paths

These paths improve agent output quality:

| Path | Purpose |
|------|---------|
| `documentation/api/openapi.yaml` | Machine-readable API spec |
| `documentation/api/errors.md` | Error code reference |
| `documentation/database-design/sqlc.md` | SQLC workflow |
| `documentation/agents/training/` | Scenario-based guides |

---

## Integration Test Specifications

### TestLoadAPIDocs

**Purpose**: Verify the OpenAPI spec exists and is parseable YAML.

```go
func TestLoadAPIDocs(t *testing.T) {
    data, err := os.ReadFile("documentation/api/openapi.yaml")
    if err != nil {
        t.Fatalf("openapi.yaml not found: %v", err)
    }

    var spec map[string]interface{}
    if err := yaml.Unmarshal(data, &spec); err != nil {
        t.Fatalf("openapi.yaml is not valid YAML: %v", err)
    }

    if spec["openapi"] != "3.1.0" {
        t.Errorf("expected OpenAPI 3.1.0, got %v", spec["openapi"])
    }
}
```

### TestSchemaDocsExist

**Purpose**: Verify schema documentation directories contain markdown files.

```go
func TestSchemaDocsExist(t *testing.T) {
    dirs := []string{
        "documentation/database-design/schema/usage",
    }

    for _, dir := range dirs {
        entries, err := os.ReadDir(dir)
        if err != nil {
            t.Errorf("directory %s not found: %v", dir, err)
            continue
        }

        hasMarkdown := false
        for _, e := range entries {
            if filepath.Ext(e.Name()) == ".md" {
                hasMarkdown = true
                break
            }
        }
        if !hasMarkdown {
            t.Errorf("directory %s contains no markdown files", dir)
        }
    }
}
```

### TestNamingConventions

**Purpose**: Verify naming conventions document exists and contains key rules.

```go
func TestNamingConventions(t *testing.T) {
    data, err := os.ReadFile("documentation/database-design/conventions.md")
    if err != nil {
        t.Fatalf("conventions.md not found: %v", err)
    }

    content := string(data)
    requiredPatterns := []string{
        "snake_case",
        "idx_{table}_{columns}",
        "TIMESTAMPTZ",
        "gen_random_uuid()",
    }

    for _, pattern := range requiredPatterns {
        if !strings.Contains(content, pattern) {
            t.Errorf("conventions.md missing pattern: %s", pattern)
        }
    }
}
```

### TestAgentPatternCompliance

**Purpose**: Verify agent-generated code follows documented patterns.

```go
func TestAgentPatternCompliance(t *testing.T) {
    data, err := os.ReadFile("documentation/agents/patterns.md")
    if err != nil {
        t.Fatalf("agents/patterns.md not found: %v", err)
    }

    content := string(data)
    requiredRules := []string{
        "ALWAYS",
        "NEVER",
        "agent_id",
        "gen_random_uuid()",
    }

    for _, rule := range requiredRules {
        if !strings.Contains(content, rule) {
            t.Errorf("agents/patterns.md missing rule: %s", rule)
        }
    }
}
```

---

## Rollback Procedure

If agent configuration changes cause issues:

1. Remove the added `context_directories` entries
2. Remove the added `prompt_templates` entries
3. Restart agent session
4. Agent falls back to inferred patterns (less reliable but functional)

---

## Notes

- Agent configs are typically per-project, not per-unit
- The documentation paths are relative to the project root
- As new documentation files are created, add them to `context_directories`
- Integration tests run in CI to verify agents can access documentation
