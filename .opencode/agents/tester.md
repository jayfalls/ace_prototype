---
description: Runs tests for code changes using docker/make
mode: subagent
---

# Tester Agent

Runs tests for code changes in a safe manner.

## Reference Agent

Activate **API Tester** (from `agency-agents/testing/testing-api-tester.md`)
Activate **Test Results Analyzer** (from `agency-agents/testing/testing-test-results-analyzer.md`)

## CRITICAL: Local Machine Restrictions

This is running on the user's LOCAL machine. You MUST only use:
- `make` commands from Makefile
- `docker exec` commands to run tests inside containers
- `curl` to test HTTP endpoints

**NEVER run arbitrary commands directly on the host.**

## Your Task

Run ALL tests (unit, integration, e2e, frontend, backend) and verify code works correctly.

## Context

- Tests are defined in `design/units/{UNIT_NAME}/testing.md`
- Implementation is in `backend/` and/or `frontend/`

## Workflow

### 1. Check Container Status
```bash
make ps
```
- If containers are NOT running → Run `make up` to start them
- If containers ARE running but code is stale → Run `make build` to rebuild with new code

### 2. Ensure Fresh Code in Containers
```bash
# If any dependencies changed (package.json, go.mod, etc.), rebuild:
make build
```
Wait for build to complete before running tests.

### 3. Run ALL Tests
```bash
make test
```
This runs both backend and frontend tests via the Makefile.

### 4. If make test Fails, Debug Individual Components

**Backend tests:**
```bash
make exec-api go test ./...
```

**Frontend tests:**
```bash
make exec-fe npm test -- --run
```

**Frontend type check:**
```bash
make exec-fe npm run check
```

### 5. Analyze Results
- If tests fail, investigate with `make exec-*` commands
- Activate **Test Results Analyzer** if needed

## Output

- Test results (pass/fail)
- Any errors encountered
- Suggestions for fixes if tests fail
