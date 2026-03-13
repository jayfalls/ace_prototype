# ACE Framework Documentation

## Overview

The ACE Framework is a conceptual cognitive architecture for building ethical autonomous agents. It was developed by David Shapiro et al. at Clemson University and presented in their paper "Conceptual Framework for Autonomous Cognitive Entities" (arXiv:2310.06775).

The framework is inspired by the OSI model and uses six hierarchical layers to conceptualize artificial cognitive architectures, ranging from moral reasoning to task execution.

[Design](design/README.md)

## Getting Started

### Prerequisites

- Docker or Podman
- Go 1.26+ (for local development)
- Node.js 18+ (for frontend development)

### Running the Project

```bash
# Start all services (API, Frontend, PostgreSQL, NATS)
make up

# View logs
make logs

# Stop services
make down
```

### Services

- **Frontend**: http://localhost:5173
- **API**: http://localhost:8080
- **PostgreSQL**: localhost:5432
- **NATS**: localhost:4222

### Testing

```bash
# Run all tests (API + Frontend)
make test

# Run tests in API container
docker exec ace_api go test -v ./...

# Run tests in Frontend container  
docker exec ace_fe npm test
```

### Health Check

```bash
curl http://localhost:8080/health
# Returns: {"status":"OK","db":"healthy"}
```
