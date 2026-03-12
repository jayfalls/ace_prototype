# Unit Template

<!--
Intent: Provide a complete template structure for documenting a unit from conception to implementation.
Scope: All documents needed for a comprehensive unit specification.
Used by: AI agents to understand the full lifecycle of unit development.
-->

This template provides a complete structure for documenting a unit from conception to implementation.

## Usage

To create a new unit:
1. Copy the `template/` directory to a new folder under `units/`
2. Rename the folder to your unit name (e.g., `units/my-new-unit/`)
3. Fill in each document as separate PRs (one document per PR)

## Template Documents

- [problem_space.md](problem_space.md) - Problem space exploration through questions (MUST complete before BSD)
- [bsd.md](bsd.md) - Business Specification Document
- [user_stories.md](user_stories.md) - User stories and acceptance criteria
- [research.md](research.md) - Research and evaluate different approaches
- [fsd.md](fsd.md) - Functional Specification Document
- [architecture.md](architecture.md) - Technical architecture decisions
- [implementation.md](implementation.md) - Implementation plan and details
- [security.md](security.md) - Security considerations
- [design.md](design.md) - Visual/UX design specifications
- [mockups.md](mockups.md) - Wireframes and mockups
- [migration_and_rollback.md](migration_and_rollback.md) - Database migration and rollback plans
- [testing.md](testing.md) - Testing strategy and test cases
- [api.md](api.md) - API specifications
- [monitoring.md](monitoring.md) - Observability requirements
- [dependencies.md](dependencies.md) - External dependencies

## Recommended Workflow

1. Start with **problem_space.md** to explore the problem through questions (REQUIRED)
2. Create **bsd.md** to define the business case
3. Create **user_stories.md** to capture user requirements
4. Conduct **research.md** to evaluate different approaches and make informed design decisions
5. Write **fsd.md** for technical details
6. Design **architecture.md** for system integration
7. Plan **implementation.md** for execution
8. Document **security.md** considerations
9. Complete remaining documents as needed