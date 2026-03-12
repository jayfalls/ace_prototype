# Research Document

## Topic
Containerized development environment setup for Go backend and SvelteKit frontend with hot reloading

## Industry Standards
- Docker Compose is the industry standard for local development orchestration
- Most modern Go projects use air or similar for hot reload
- SvelteKit/Vite has built-in HMR (Hot Module Replacement)
- Official PostgreSQL and NATS Docker images are widely used

## Alternative Approaches Evaluated

### Container Runtime: Docker vs Podman

**Docker**
- Pros: Industry standard, widest community support, extensive documentation
- Cons: Requires daemon running, potential license concerns (though current version is still open)
- Use Cases: Most teams, largest ecosystem

**Podman**
- Pros: Daemonless, rootless by default, Docker-compatible CLI
- Cons: Less community support, some edge cases with Docker Compose
- Use Cases: Security-conscious teams, rootless environments

**Recommendation**: Use Docker Compose with `docker` as the primary runtime. Support Podman by documenting that `docker compose` works with Podman in most cases.

### Go Hot Reload Solutions

**air**
- Pros: Mature, widely used, simple config, supports most Go apps
- Cons: Requires config file, occasional edge cases
- Use Cases: General Go applications

**gin**
- Pros: Fast, simple, no config needed in many cases
- Cons: Less configurable than air
- Use Cases: Simple APIs

**reflex**
- Pros: Can watch multiple directories, flexible
- Cons: Less Go-specific optimization
- Use Cases: Multi-service projects

**Recommendation**: Use `air` for Go backend - it's the most mature and widely adopted solution for Go hot reloading.

### Development vs Production Dockerfiles

**Approach 1: Single Dockerfile with multi-stage build**
- Pros: Single file to maintain
- Cons: Larger image size for dev, may include unnecessary tools in production

**Approach 2: Separate Dockerfiles (Dockerfile.dev, Dockerfile.prod)**
- Pros: Optimized images for each environment
- Cons: More files to maintain

**Approach 3: Single Dockerfile with build argument**
- Pros: Single file, conditional dev tools
- Cons: More complex build process

**Recommendation**: Use separate Dockerfiles (Dockerfile and Dockerfile.dev) to keep production images minimal and development images with necessary tools.

### Volume Mounting Strategy

**bind mount**
- Pros: Simple, files sync immediately
- Cons: Performance can be slower on some OSs, file system events
- Use Cases: Development

**named volume**
- Pros: Better performance, persists data
- Cons: Changes don't reflect immediately
- Use Cases: Database data only

**Recommendation**: Use bind mounts for source code (backend/frontend) to enable hot reloading. Use named volumes for PostgreSQL data.

## Comparison Matrix

| Criteria | Docker | Podman | air | gin | reflex |
|----------|--------|--------|-----|-----|--------|
| Community Support | ★★★★★ | ★★★☆☆ | ★★★★★ | ★★★★☆ | ★★★☆☆ |
| Maturity | ★★★★★ | ★★★★☆ | ★★★★★ | ★★★★☆ | ★★★☆☆ |
| Ease of Setup | ★★★★★ | ★★★★☆ | ★★★★☆ | ★★★★★ | ★★★☆☆ |
| Hot Reload Speed | N/A | N/A | ★★★★☆ | ★★★★★ | ★★★★☆ |
| Configurability | ★★★★★ | ★★★★★ | ★★★★★ | ★★★☆☆ | ★★★★★ |

## Recommended Approach
- **Container Runtime**: Docker (with Podman compatibility noted)
- **Go Hot Reload**: air
- **Frontend Hot Reload**: Vite built-in HMR (no additional setup needed)
- **Database**: PostgreSQL official Docker image
- **Messaging**: NATS official Docker image
- **Orchestration**: Docker Compose

## Rationale
1. Docker is the industry standard with the largest ecosystem and community support
2. air is the most mature and widely-used solution for Go hot reloading
3. SvelteKit with Vite has excellent built-in HMR that requires no additional configuration
4. Official images for PostgreSQL and NATS are well-maintained and widely used
5. Docker Compose provides a clean way to orchestrate all services with environment variables

## Risks and Mitigation
| Risk | Mitigation |
|------|------------|
| File system performance on macOS/Windows | Document that Docker Desktop with filesystem cache may be needed |
| Port conflicts | Use environment variables for all port configurations |
| Volume permission issues | Document proper volume ownership with USER directive |

## References
- https://github.com/cosmtrek/air - Air Go hot reload
- https://docs.docker.com/compose/ - Docker Compose docs
- https://hub.docker.com/_/postgres - PostgreSQL official image
- https://hub.docker.com/_/nats - NATS official image
- https://vitejs.dev/guide/features.html#hot-module-replacement - Vite HMR
