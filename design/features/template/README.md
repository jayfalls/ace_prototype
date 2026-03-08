# Feature Template

<!--
Intent: Provide a complete template structure for documenting a feature from conception to implementation.
Scope: All documents needed for a comprehensive feature specification.
Used by: AI agents to understand the full lifecycle of feature development.
-->

This template provides a complete structure for documenting a feature from conception to implementation.

## Usage

To create a new feature:
1. Copy the `template/` directory to a new folder under `features/`
2. Rename the folder to your feature name (e.g., `features/my-new-feature/`)
3. Fill in each document as separate PRs (one document per PR)

## Template Documents

| Document | Purpose |
|----------|---------|
| [bsd.md](bsd.md) | Business requirements, scope, value proposition, success criteria |
| [fsd.md](fsd.md) | Technical specifications, data models, algorithms |
| [user_stories.md](user_stories.md) | User stories in Gherkin format (Given/When/Then) |
| [architecture.md](architecture.md) | Component diagrams, data flow, integration points |
| [implementation.md](implementation.md) | Step-by-step implementation guide |
| [security.md](security.md) | Threat modeling, security controls, auth requirements |
| [design.md](design.md) | UI/UX decisions, API design, data schema choices |
| [mockups.md](mockups.md) | Links to Figma/mockups, screenshots, wireframes |
| [migration_and_rollback.md](migration_and_rollback.md) | DB schema changes, migration scripts, rollback steps |
| [testing.md](testing.md) | Unit tests, integration tests, E2E test cases |
| [api.md](api.md) | REST/GraphQL endpoints, request/response schemas |
| [monitoring.md](monitoring.md) | Log events, metrics, alerting rules |
| [dependencies.md](dependencies.md) | External services, libraries, API integrations |

## Recommended Workflow

1. Start with **bsd.md** to define the business case
2. Create **user_stories.md** to capture user requirements
3. Write **fsd.md** for technical details
4. Design **architecture.md** for system integration
5. Plan **implementation.md** for execution
6. Document **security.md** considerations
7. Complete remaining documents as needed