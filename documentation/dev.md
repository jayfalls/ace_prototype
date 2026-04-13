# Development Guide

## Prerequisites

- distrobox (`pipx install distrobox`)
- Git

## Quick Start

- Follow the getting started guide

- Set up the dev environment
```bash
make dev
```

- Run ACE with hot reload
```bash
make ace
```

- Run the opencode agent
```bash
make agent
```

### Make Commands

| Command | Description |
|---------|-------------|
| `make ace` | Run ACE with hot reload (backend + frontend) |
| `make dev` | Setup dev environment (distrobox + agency-agents + git hooks) |
| `make agent` | Start OpenCode in distrobox |
| `make test` | Run full validation pipeline |
| `make help` | Show available commands |

## Health Check

Check API health:
```bash
curl http://localhost:8080/healthz
# Returns: {"status":"ok"}
```
