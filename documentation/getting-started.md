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

### Verify Hot Reloading

After running `make up`, verify hot reloading is working for both services:

#### API Service (Go)
```bash
# Check API logs - you should see air rebuilding on file changes
make logs-api

# Or watch for the air banner:
# ███╗   ███╗ █████╗ ███╗   ██╗██╔███╗██
# ████╗ ████║██╔══██╗████╗  ██║██║╚██╗██║
# ██╔████╔██║██║  ██║██╔██╗ ██║██║ ╚██║
# ██║╚██╔╝██║██║  ██║██║╚██╗██║██║ ╚██║
# ██║ ╚═╝ ██║╚██████╔╝██║ ╚████║██║ ╚██║
# ╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝

# Test the API is running
curl http://localhost:8080
# Expected: {"message":"ACE API Server"}
```

#### Frontend Service (SvelteKit)
```bash
# Check frontend logs
make logs-fe

# Or verify Vite HMR is running - look for:
# VITE v6.x.x  ready in XXX ms
#   ➜  Local:   http://localhost:5173/
#   ➜  Network: http://0.0.0.0:5173/

# Test the frontend is running
curl http://localhost:5173
# Expected: HTML page with "Welcome to ACE Framework"
```

#### Testing Hot Reload

1. **API**: Edit any file in `api/main.go` and save - air will rebuild automatically
2. **Frontend**: Edit any file in `frontend/src/routes/+page.svelte` and save - Vite will hot reload

You should see the changes reflected immediately without restarting containers.

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
