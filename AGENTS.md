# Coding Standards

## General Principles

**Always Do Minimal Changes Where Possible**
- Prefer small, focused changes over large rewrites
- When fixing issues, only change what's necessary
- Avoid refactoring unrelated code
- Make the smallest change that solves the problem
- This applies to documents, code, and any deliverables

**Always Report Files Affected**
- Every change MUST report which files were changed/created in the response
- This allows QA to check relevant git diffs
- Include file paths in the final output

**One Document Per PR**
Every piece of work should create ONLY ONE document per session/PR. If multiple documents need creation, they should be created one at a time.

**Always Read Context First**
- Always read `design/README.md` before starting any work
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

## Unit Reference
Every PR, commit, and issue MUST include the unit name so progress can be tracked across sessions.

**Format:**
- PR title: `[unit: opencode-integration] Add memory system`
- Commit: `feat: add memory system [unit: opencode-integration]`
- Issue: `[unit: observability] How should we handle logs?`

# Working on the Code

## Coding Best Practices
- **Types**: Use explicit types, avoid `any` wherever possible
- **No else chains**: Avoid else statements - use early returns instead
- **Naming**: Use meaningful, descriptive names for variables, functions, and types
- **Comments**: Explain "why", not "what" - code should be self-documenting
- **Functions**: Keep functions small and focused on a single responsibility
- **DRY**: Don't Repeat Yourself - extract common patterns into reusable functions

### Go Backend
- **Error Handling**: Always handle errors, never ignore with `_`
- **Naming**: 
  - Variables: camelCase
  - Types/Exports: PascalCase
  - Constants: PascalCase or SCREAMING_SNAKE_CASE
- **Database**: Use SQLC for type-safe database access (no raw SQL queries)
- **Context**: Use context.Context for request-scoped values and cancellation
- **Migrations**: Write all migrations in GO directly using Goose.
- **Layered Architecture**: Always use Handler → Service → Repository pattern:
  - Handler: HTTP request/response only, no business logic
  - Service: Business logic, orchestrates repositories
  - Repository: Database queries only (via SQLC)

### TypeScript/SvelteKit Frontend
- **Prefer**: Use interfaces over types where possible
- **Svelte 5**: Use runes syntax (`$state`, `$derived`, `$effect`)
- **Components**: Keep components small and focused on single responsibilities

## Testing Requirements

All code changes must include appropriate tests:
- **Unit Tests**: Required for new code - aim for 80% coverage
- **Integration Tests**: Required for API and database operations
- **Frontend Tests**: Use Vitest for unit tests
- **E2E Tests**: Required for critical user flows
