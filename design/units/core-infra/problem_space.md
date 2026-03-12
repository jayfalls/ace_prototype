# Problem Space

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: Need a consistent dev environment that's quick to set up regardless of machine or platform. Must not introduce inconsistencies or cross-platform wrestling. Should be super easy to iterate and get out of the way.

**Q: Who are the users?**
A: 
- Development team (Docker/K8s experts)
- AI agents as first-class citizens (primary users)

**Q: What are the success criteria?**
A:
- Single command setup (git clone && docker compose up)
- Hot reload for fast iteration
- Services always run together (no isolation) - unit/integration/e2e tests handle isolation
- Easy debugging with terminal logs
- Seamless dev-to-prod experience
- Everything automated (migrations, seeding)

**Q: What constraints exist?**
A: None

## Iterative Exploration

### Follow-up Questions and Answers

#### Q: What's your team's experience with Docker?
A: Everyone knows Docker and Kubernetes really well.

#### Q: What's "super easy to iterate" mean specifically?
A: 
- Hot reload is core requirement
- Easy to run services together (not in isolation)
- Must be easy to debug and see logs
- Seamless dev experience that translates to production

#### Q: For debugging, what do you prefer?
A: Logs in terminal sufficient. More advanced observability will be figured out in the observability unit.

#### Q: AI agent as first-class citizen - what does that mean?
A:
- Code should be readable/understandable by AI agents
- CI/CD and tooling should be agent-friendly (agent-friendly outputs, easy to run specific tasks)
- Agents will be the primary users of this repo

#### Q: How important is dev mirroring prod?
A:
- Same base images - YES
- Same environment variables - ideally yes, but prod env differs from dev
- Same configuration approach - YES: compose for single agent locally, K8s for multi-agent locally

#### Q: Docker compose vs kind/minikube for dev?
A: 
- Compose for single agent (hobbyist getting started)
- Kind/K8s for multi-agent scenarios (enterprise)

#### Q: Any specific Go frameworks or SvelteKit requirements?
A: Already detailed in the architecture unit.

#### Q: Database migrations - auto or manual?
A: Get everything as automated as possible (migrations and seeding if not exists, anywhere else in the system we can automate we should).

#### Q: What's ideal first-time setup?
A: Single command ideally.

#### Q: Container structure preference?
A: Separate containers per service.

#### Q: Resource expectations?
A: 4 vCPU, 8GB RAM reported minimum; 2 vCPU, 2GB RAM actual minimum.

#### Q: Security considerations for local dev?
A: Dev should not have to worry about secrets or security, though ideally gets all the benefits of the prod setup.

#### Q: Data persistence handling?
A: Keep data between restarts (named volumes).

#### Q: Multi-agent scenarios in local dev?
A: The architecture is structured so that core brain (layers, loops) are composable and horizontally scalable with pods. Another pod contains rest (FE, API, memory controller, telemetry, senses, tools). Build from ground up as multi-agent, single agent comes "for free" with multi-agent collection of 1.

#### Q: Environment variables/secrets in dev?
A: .env files perfect for dev.

#### Q: How to connect to external services?
A: The same as in prod - connect through env (secrets/env in prod).

## Key Insights

1. **AI-first architecture**: Code and tooling must be agent-friendly from the ground up
2. **Hot reload is non-negotiable**: Core requirement for fast iteration
3. **Always run together**: Services should not run in isolation locally - tests handle that
4. **Dev = Prod approach**: Same base images, same config approach (compose for single, K8s for multi)
5. **Maximum automation**: Auto migrations, auto seeding, single command setup
6. **Named volumes**: Persist data between restarts
7. **.env for dev**: Simple secrets management that mirrors prod pattern

## Dependencies Identified

- Architecture unit (for Go frameworks, SvelteKit details, component structure)
- Observability unit (for advanced debugging/monitoring - later)
- Multi-agent architecture patterns

## Assumptions Made

1. Dev environment will use docker-compose (not kind) for single-agent scenarios
2. Backend uses standard Go frameworks per architecture unit
3. Frontend uses SvelteKit per architecture unit
4. Database migrations handled via some Go migration tool (to be researched)
5. Seeding handled via application-level code or separate init container

## Open Questions (Unanswered)

1. What specific Go migration tool to use? (golang-migrate, goose, etc.)
2. How to structure multi-agent local testing with compose? (multiple backend instances?)
3. What's the exact service breakdown per the architecture? (to verify container count)

## Next Steps

- Proceed to BSD once all clarifying questions are answered
- Revisit this document if new questions arise during BSD creation
