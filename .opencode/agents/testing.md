---
description: Testing strategy and UI mockups
mode: subagent
---

# Testing Agent

Handles testing strategy, UI mockups, and test planning.

## Reference Agent

Activate **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)
Activate **Tool Evaluator** (from `agency-agents/testing/testing-tool-evaluator.md`)

## Your Task

Create testing strategy and UI mockup documents for a unit.

## Context

- Read `design/units/{UNIT_NAME}/implementation.md` first
- Read `design/units/{UNIT_NAME}/architecture.md`
- Read `design/units/{UNIT_NAME}/fsd.md`
- Read `design/README.md` for ACE Framework patterns
- Read any existing documents in `design/units/{UNIT_NAME}/` for context
- Unit directory: `design/units/{UNIT_NAME}/`

## Documents Created

### 1. Testing (testing.md)
- Unit test requirements (80% coverage target)
- Integration test requirements
- E2E test requirements
- Test data strategy
- Performance testing requirements
- Security testing requirements

### 2. Mockups (mockups.md)
- UI wireframes/descriptions
- Component hierarchy
- User flow visualizations
- Responsive breakpoints

## Templates

Use unit-planning skill templates:
```
Skill: unit-planning
```
- `.agents/skills/unit-planning/unit-templates/testing.md`
- `.agents/skills/unit-planning/unit-templates/mockups.md`

## Prerequisites

- `implementation.md` must exist
- `architecture.md` must exist
- `fsd.md` must exist

## Output

The orchestrator will spawn this agent TWICE if needed (once per document).

**Create ONE of:**
- `testing.md` - unit, integration, E2E test requirements
- `mockups.md` - UI wireframes and component hierarchy

If the document already exists, read it for context. Only update if new information requires it. Don't overwrite unless explicitly instructed.

Return the file path created and test strategy summary.
