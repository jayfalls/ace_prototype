# Getting Started

## Quick Start

### Clone and Run

```bash
# Clone the repository
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype

# Start all services
make up
```

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
| `make rebuild` | Rebuild all images (no cache) |
| `make help` | Show available commands |
