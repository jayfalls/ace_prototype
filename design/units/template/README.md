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

- [bsd.md](bsd.md)
- [user_stories.md](user_stories.md)
- [research.md](research.md)
- [fsd.md](fsd.md)
- [architecture.md](architecture.md)
- [implementation.md](implementation.md)
- [security.md](security.md)
- [design.md](design.md)
- [mockups.md](mockups.md)
- [migration_and_rollback.md](migration_and_rollback.md)
- [testing.md](testing.md)
- [api.md](api.md)
- [monitoring.md](monitoring.md)
- [dependencies.md](dependencies.md)

## Recommended Workflow

1. Start with **bsd.md** to define the business case
2. Create **user_stories.md** to capture user requirements
3. Conduct **research.md** to evaluate different approaches and make informed design decisions
4. Write **fsd.md** for technical details
5. Design **architecture.md** for system integration
6. Plan **implementation.md** for execution
7. Document **security.md** considerations
8. Complete remaining documents as needed