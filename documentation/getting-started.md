# Getting Started

## Quick Start

```bash
# Clone the repository
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype

# Start all services
make up
```

## Configuration

Copy `.env.example` to `.env` if you want to customize any settings.

## Make Commands

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
| `make test` | Run all tests (API + Frontend) |
| `make help` | Show available commands |

## Services

| Service | URL |
|---------|-----|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| PostgreSQL | localhost:5432 |
| NATS | localhost:4222 |

## Testing

Run all tests in containers:
```bash
make test
```

Run tests individually:
```bash
# API tests
docker exec ace_api go test -v ./...

# Frontend tests
docker exec ace_fe npm test
```

## Health Check

Check API and database health:
```bash
curl http://localhost:8080/health
# Returns: {"status":"OK","db":"healthy"}
```
