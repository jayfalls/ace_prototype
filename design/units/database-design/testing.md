# Testing Strategy — Database Design Documentation Unit

## Overview

This document defines the testing strategy for the database-design documentation unit. Testing covers four areas: the docs-gen pipeline, OpenAPI generation, agent documentation consumption, and validation scripts.

---

## 1. Docs-Gen Pipeline Testing

**Target**: `scripts/docs-gen/`, `scripts/schema-doc-gen/`, `scripts/erd-gen/`

**Strategy**:
- Unit tests for each generator script (schema-doc-gen, erd-gen, openapi-gen, validate-docs)
- Integration test: run `make test` and verify all output files are produced
- Golden file comparison: generated markdown matches expected output

**Test Cases**:
- `TestSchemaDocGen` — connects to DB, extracts schema, produces valid markdown
- `TestERDGen` — produces valid Mermaid syntax from FK metadata
- `TestDocsGenOrchestrator` — runs all generators in sequence without error
- `TestDocsGenIdempotent` — running twice produces identical output

**Validation**:
- `make test` target runs all generators
- Output files are parseable (YAML, Mermaid, Markdown)
- No generator crashes on empty database

---

## 2. OpenAPI Generation Testing

**Target**: `scripts/openapi-gen/`, `documentation/api/openapi.yaml`

**Strategy**:
- Validate generated OpenAPI spec is valid YAML
- Validate all expected endpoints are present
- Validate response envelope schema matches `response.APIResponse` struct
- Validate request/response examples are present

**Test Cases**:
- `TestLoadAPIDocs` — verifies openapi.yaml parses and contains expected paths (existing)
- `TestOpenAPIEndpointsMatch` — verifies all Chi-registered routes appear in spec
- `TestOpenAPIComponentsValid` — verifies reusable schemas are well-formed

**Validation**:
- `swagger-cli validate documentation/api/openapi.yaml` or equivalent
- All 7 current endpoints documented
- Error response schemas cover 400, 401, 403, 404, 409, 500

---

## 3. Agent Documentation Consumption Testing

**Target**: `documentation/agents/`, `tests/agent-integration/docs_test.go`

**Strategy**:
- Verify all required agent documentation files exist
- Verify documentation follows expected structure (conventions, SQLC, training)
- Verify naming conventions are documented correctly
- Verify SQLC annotation syntax is documented

**Test Cases**:
- `TestLoadAPIDocs` — agents can load and parse OpenAPI spec (existing)
- `TestSchemaDocsExist` — entity-group directories contain markdown files (existing)
- `TestAgentDocsExist` — all agent-facing documentation files exist (existing)
- `TestNamingConventions` — conventions.md contains required patterns (existing)
- `TestSQLCAnnotations` — SQLC annotation syntax is documented correctly (added)

**Validation**:
- `go test ./tests/agent-integration/...` passes
- All agent documentation files exist at expected paths
- Content checks verify key patterns are documented

---

## 4. Validation Script Testing

**Target**: `scripts/validate-docs/`

**Strategy**:
- Test schema comparison: live DB schema vs documentation
- Test drift detection: documentation that doesn't match schema
- Test error reporting: clear messages for missing/incorrect docs

**Test Cases**:
- `TestValidateDocsMatch` — documentation matches live schema (happy path)
- `TestValidateDocsMissingTable` — detects undocumented tables
- `TestValidateDocsMissingColumn` — detects undocumented columns
- `TestValidateDocsTypeMismatch` — detects type discrepancies
- `TestValidateDocsExitCode` — exits non-zero on drift

**Validation**:
- `make test` runs validation without false positives on current docs
- Intentional drift triggers failure with clear error message
- CI/CD integration: validation runs on every PR touching schema or docs

---

## Test Execution

All tests run via `make test`:

```bash
# Full test suite
make test

# Agent integration tests only
go test ./tests/agent-integration/...

# Validation script only
go run ./scripts/validate-docs/
```

---

## Coverage Targets

| Area | Target |
|------|--------|
| Docs-gen pipeline | All generators produce output without error |
| OpenAPI generation | 100% of endpoints in spec |
| Agent docs | 100% of required files exist |
| Validation script | Drift detection for all table/column changes |
