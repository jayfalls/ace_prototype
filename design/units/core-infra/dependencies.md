# Dependencies

<!--
Intent: Define all external dependencies required by the feature.
Scope: Libraries, services, APIs, and tools that the feature depends on.
Used by: AI agents to understand what external resources are needed and how to integrate with them.
-->

## Overview

The core-infra unit depends on various Go libraries for the backend, PostgreSQL for the database, and npm packages for the frontend. This document outlines all external dependencies and their integration.

## External Services

### PostgreSQL
| Property | Value |
|----------|-------|
| Type | Relational Database |
| Provider | PostgreSQL |
| Purpose | Primary data store for all entities |
| Cost | Self-hosted / Cloud (varies) |

#### Configuration
```bash
# Environment variables
export DATABASE_HOST="localhost"
export DATABASE_PORT="5432"
export DATABASE_USER="ace"
export DATABASE_PASSWORD="secret"
export DATABASE_NAME="ace_framework"
export DATABASE_SSLMODE="disable"
```

#### Integration
```go
// Using pgx driver
import (
    "github.com/jackc/pgx/v5/pgxpool"
    "context"
)

pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
```

### OpenAI API (Optional)
| Property | Value |
|----------|-------|
| Type | LLM API |
| Provider | OpenAI |
| Purpose | LLM inference for agents |
| Cost | Pay-per-use |

#### Configuration
```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

#### Rate Limits
| Plan | Limit | Window |
|------|-------|--------|
| Free | 3 RPM | Per minute |
| Paid | 60 RPM | Per minute |

#### Alternatives Considered
- **Anthropic Claude**: Good alternative, different pricing model
- **Local LLM (Ollama)**: Self-hosted, privacy-focused

### Anthropic API (Optional)
| Property | Value |
|----------|-------|
| Type | LLM API |
| Provider | Anthropic |
| Purpose | Claude LLM inference |
| Cost | Pay-per-use |

---

## Backend Libraries (Go)

### Runtime Dependencies

| Library | Version | Purpose | License |
|---------|---------|---------|---------|
| gin | v1.9.1 | HTTP web framework | MIT |
| pgx/v5 | v5.5.1 | PostgreSQL driver | MIT |
| sqlc | v1.24.0 | Type-safe SQL | MIT |
| golang-migrate | v4.16.2 | Database migrations | MIT |
| golang-jwt/jwt/v5 | v5.2.0 | JWT authentication | MIT |
| bcrypt | v0.1.0 | Password hashing | BSD-3-Clause |
| google/uuid | v1.5.0 | UUID generation | BSD-3-Clause |
| prometheus/client_golang | v1.17.0 | Prometheus metrics | Apache 2.0 |
| rs/zerolog | v1.31.0 | Structured logging | MIT |
| gorilla/websocket | v1.5.1 | WebSocket support | BSD-2-Clause |
| go-playground/validator/v10 | v10.16.0 | Input validation | MIT |
| golang.org/x/crypto | v0.17.0 | Cryptography | BSD-3-Clause |
| github.com/joho/godotenv | v1.5.1 | .env file loading | MIT |

### Development Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| golang.org/x/tools | v0.16.0 | Code generation, linting |
| github.com/pressly/goose | v3.21.1 | Additional migration tools |
| github.com/stretchr/testify | v1.8.4 | Testing assertions |
| github.com/evanphx/json-patch | v5.6.0 | JSON patch testing |

### Dependency Management

```bash
# Initialize Go module
go mod init github.com/ace/framework/backend

# Add dependency
go get github.com/gin-gonic/gin@v1.9.1

# Update all dependencies
go mod tidy

# View dependencies
go list -m all
```

---

## Frontend Libraries (TypeScript/SvelteKit)

### Runtime Dependencies

| Library | Version | Purpose | License |
|---------|---------|---------|---------|
| svelte | ^4.2.0 | UI framework | MIT |
| @sveltejs/kit | ^2.0.0 | SvelteKit framework | MIT |
| typescript | ^5.0.0 | Type safety | Apache 2.0 |
| @sveltejs/adapter-auto | ^3.0.0 | Build adapter | MIT |
| svelte-i18n | ^4.0.0 | Internationalization | MIT |

### Development Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| vitest | ^1.0.0 | Unit testing |
| @testing-library/svelte | ^4.0.0 | Svelte testing |
| playwright | ^1.40.0 | E2E testing |
| eslint | ^8.0.0 | Linting |
| prettier | ^3.0.0 | Code formatting |

### Dependency Management

```bash
# Initialize npm project
npm init svelte@latest frontend

# Add dependency
npm install svelte-i18n

# Add dev dependency
npm install -D vitest
```

---

## API Integrations

### LLM Provider APIs

#### OpenAI
| Property | Value |
|----------|-------|
| Base URL | https://api.openai.com/v1 |
| Auth Type | API Key (Bearer) |
| Version | v1 |
| Documentation | https://platform.openai.com/docs |

##### Endpoints Used
| Endpoint | Purpose | Frequency |
|----------|---------|-----------|
| /chat/completions | Generate chat responses | Per user message |
| /models | List available models | On provider setup |

##### Error Handling
| Error | Handling Strategy |
|-------|------------------|
| 401 Invalid API Key | Prompt user to update key |
| 429 Rate Limited | Exponential backoff, retry |
| 500 Server Error | Retry after delay |

##### Retry Strategy
- **Max Retries**: 3
- **Backoff**: Exponential
- **Max Backoff**: 30 seconds

#### Anthropic
| Property | Value |
|----------|-------|
| Base URL | https://api.anthropic.com |
| Auth Type | API Key (x-api-key header) |
| Version | v1 |
| Documentation | https://docs.anthropic.com |

---

## Infrastructure Dependencies

### Required Infrastructure

| Resource | Type | Specification | Purpose |
|----------|------|---------------|---------|
| PostgreSQL | Database | 15+, 4GB RAM minimum | Primary data store |
| Redis | Cache | 6+ (optional) | Session storage, rate limiting |

### Optional Infrastructure

| Resource | Type | Specification | Purpose |
|----------|------|---------------|---------|
| Jaeger | Tracing | 0.50+ | Distributed tracing |
| Prometheus | Metrics | 2.40+ | Metrics collection |
| Grafana | Visualization | 9.0+ | Dashboards |

---

## Dependency Security

### Secret Management

| Secret | How Stored | Rotation |
|--------|-----------|----------|
| Database password | Environment variable | 90 days |
| JWT secret | Environment variable | 30 days |
| LLM API keys | Encrypted in DB | Per provider |

### Vulnerability Scanning

- [x] Scan dependencies on build (go vet, npm audit)
- [ ] Automated CVE alerts (Dependabot)
- [x] Update policy: Security patches immediately, others monthly

### Go Security
```bash
# Run security checks
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck ./...
```

---

## Dependency Update Strategy

- **Review updates**: Monthly
- **Security patches**: Immediate (within 24 hours)
- **Major versions**: Evaluate per case with testing

### Update Commands

```bash
# Go
go get -u ./...
go mod tidy

# npm
npm update
```
