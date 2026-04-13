# Getting Started

## Prerequisites

- Git
- Go 1.21+
- Node.js 18+ (for frontend development)

## Installation

### Install Latest Release

```bash
curl -fsSL https://ace.dev/install.sh | sh
```

### Clone for Development

```bash
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype
```

## Quick Start

Run ACE in development mode with hot reload:

```bash
make ace
```

### Make Commands

| Command | Description |
|---------|-------------|
| `make ace` | Run ACE with hot reload (backend + frontend) |
| `make test` | Run full validation pipeline |
| `make dev` | Setup dev environment (distrobox + OpenCode) |
| `make agent` | Start OpenCode agent |
| `make help` | Show available commands |

## Services

After running `make ace`:

| Service | URL |
|---------|-----|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |
