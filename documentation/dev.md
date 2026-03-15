# Getting Started

## Prerequisites

- distrobox (`pipx install distrobox`)
- Git

## Quick Start

- Follow the getting started guide

- Set up the dev environment
```bash
make dev
```

- Run the opencode agent
```bash
make agent
```

### Make Commands

| Command | Description |
|---------|-------------|
| `make dev` | Setup dev environment (distrobox + agency-agents + git hooks) |
| `make agent` | Start OpenCode in distrobox |
| `make test` | Run all tests (API + Frontend) |
| `make help` | Show available commands |

## Health Check

Check API and database health:
```bash
curl http://localhost:8080/health/ready
# Returns: {"checks":{<individual-component-checks>},"status":"ok"}
```
