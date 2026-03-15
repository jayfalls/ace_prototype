---
description: Unit review and testing
mode: subagent
---

# Unit Review Agent

For Code Review: Activate the **Senior Developer** (from `agency-agents/engineering/engineering-senior-developer.md`)
For Testing: Activate the **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)

## Your Task

Complete the review and testing phase for the unit specified by the user.

## Context

- Read all implementation documents in `design/units/{UNIT_NAME}/`
- Implementation is in `backend/` and/or `frontend/`

## Workflow

### 1. Testing Strategy
Create `testing.md`:
- Unit test requirements (80% coverage target)
- Integration test requirements
- E2E test requirements

### 2. Code Review
- Review implementation against `fsd.md` and `architecture.md`
- Check for:
  - Security vulnerabilities
  - Error handling completeness
  - Test coverage
  - Code quality

### 3. Evidence Collection
Activate **Evidence Collector** (from `agency-agents/testing/testing-evidence-collector.md`) to gather test evidence

### 4. Quality Gate
Activate **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`) to verify quality gates

## Templates

Use templates from `.agents/skills/unit-workflow/unit-templates/`:
- `testing.md`

## Output

- Updated `testing.md` with test cases
- Review findings
- Quality gate status
- Any issues that need fixing
