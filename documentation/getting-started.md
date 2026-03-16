# Getting Started

## Prerequisites

- Docker or Podman

## Quick Start

- Clone the repo
```bash
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype
```

- Run the ACE
```bash
make up
```

### Make Commands

| Command | Description |
|---------|-------------|
| `make up` | Start all services |
| `make down` | Stop all services |
| `make restart` | Restart all services |
| `make logs` | View logs |
| `make logs-api` | View API logs |
| `make logs-fe` | View frontend logs |
| `make logs-db` | View database logs |
| `make logs-broker` | View broker logs |
| `make clean` | Remove all containers and volumes |
| `make build` | Build all images |
| `make ps` | Show running containers |
| `make help` | Show available commands |

## Services

| Service | URL |
|---------|-----|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| PostgreSQL | localhost:5432 |
| NATS | localhost:4222 |
