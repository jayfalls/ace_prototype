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

### 1. Start Services and Verify
```bash
make up
```
- Verify all containers are running
- Verify services are healthy (curl health endpoints)

### 2. Run ALL Backend Tests
```bash
docker exec ace_api go test ./...
```

### 3. Run ALL Frontend Tests
```bash
docker exec ace_frontend npm test -- --run
```

### 4. Run ALL Make Tests
```bash
make test
```

### 5. Test HTTP Endpoints
```bash
curl -X GET http://localhost:8080/health
```

### 6. Analyze Results
- If tests fail, investigate with `docker exec` commands
- Use `curl` to test specific endpoints
- Activate **Test Results Analyzer** if needed

## Output

- Test results (pass/fail)
- Any errors encountered
- Suggestions for fixes if tests fail
