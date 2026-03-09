# AGENTS.md

## Getting Started

### Design Documentation (Always Read First!)
- **CRITICAL**: Always read `design/README.md` before starting any work on the codebase
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

## Project Structure

The ace_prototype repository is organized as follows:

- `design/` - All design documentation for the system
  - `design/README.md` - Overall system architecture and design
  - `design/units/` - Individual unit specifications (features, components, refactors)
  - `design/units/template/` - Template documents for creating new units
- `documentation/` - Project documentation and changelogs
  - `documentation/changelogs/` - Daily changelog files
- `backend/` - Go backend source code (when implemented)
- `frontend/` - SvelteKit/TypeScript frontend source code (when implemented)

## Working on the Code

### Design Documentation
- Reference design/README.md for the overall system design
- Reference design/units/README.md for each individual piece of the system and leg of work
- When creating a unit of work, fully complete the documentation before beginning with code
- After fully completing a code change for a unit (merged), create a PR to update the documentation/ folder with relevant changes pertaining to the unit (add files as needed, edit existing files as needed, no changes if nothing relevant)
- Also, add-by-copying-template/update the relevant changelog file for the day in documentation/changelogs/ with the relevant changes

### Units

**What are Units?**
Units are discrete pieces of work in the ACE Framework. Each unit represents a feature, component, or refactoring that can be independently designed, implemented, and documented. Examples include:
- Adding a new API endpoint
- Creating a new UI component
- Implementing a database migration
- Refactoring existing code

**Unit Template Structure**
Each unit should have a complete set of documentation. The template includes 13 documents:
1. `BSD.md` - Business Specification Document (what we're building and why)
2. `FSD.md` - Functional Specification Document (how we'll build it)
3. `user_stories.md` - User stories and acceptance criteria
4. `architecture.md` - Technical architecture decisions
5. `implementation.md` - Implementation plan and details
6. `security.md` - Security considerations
7. `design.md` - Visual/UX design specifications
8. `mockups.md` - Wireframes and mockups
9. `migration_and_rollback.md` - Database migration and rollback plans
10. `testing.md` - Testing strategy and test cases
11. `api.md` - API specifications
12. `monitoring.md` - Observability requirements
13. `dependencies.md` - External dependencies

**Unit Workflow**
1. Create a new unit by copying the template directory: `cp -r design/units/template design/units/<unit-name>`
2. Complete ALL planning documents (BSD through dependencies) BEFORE writing any code
3. Create a PR for the BSD first, then the FSD
4. Only begin implementation after all design documents are approved
5. One document type per PR (e.g., one PR for BSD, one for FSD)

### Unit Documents
- **BSD (Business Specification Document)**: Defines the "what" - business case, scope, success criteria. Not the "how" (that's FSD).
- **FSD (Functional Specification Document)**: Defines the "how" - technical implementation details.
- BSD comes first, then FSD. Each in separate PRs.

### Coding Best Practices

#### Go Backend
- **Types**: Use explicit types, avoid `any` wherever possible
- **Error Handling**: Always handle errors, never ignore with `_`
- **Naming**: 
  - Variables: camelCase
  - Types/Exports: PascalCase
  - Constants: PascalCase or SCREAMING_SNAKE_CASE
- **Database**: Use SQLC for type-safe database access (no raw SQL queries)
- **Context**: Use context.Context for request-scoped values and cancellation
- **Testing**: Write unit tests alongside code using Go's testing package

#### TypeScript/SvelteKit Frontend
- **Types**: Strict typing - never use `any`
- **Prefer**: Use interfaces over types where possible
- **Naming**:
  - Components: PascalCase (e.g., `UserProfile.svelte`)
  - Functions: camelCase
  - Variables: camelCase
- **Svelte 5**: If using Svelte 5, use runes syntax (`$state`, `$derived`, `$effect`)
- **Components**: Keep components small and focused on single responsibilities

#### General Guidelines
- **Naming**: Use meaningful, descriptive names for variables, functions, and types
- **Comments**: Explain "why", not "what" - code should be self-documenting
- **Functions**: Keep functions small and focused on a single responsibility
- **DRY**: Don't Repeat Yourself - extract common patterns into reusable functions

## Testing Requirements

All code changes must include appropriate tests:

- **Unit Tests**: Required for new code - aim for 80% coverage
- **Integration Tests**: Required for API and database operations
- **Frontend Tests**: Use Vitest for unit tests
- **E2E Tests**: Required for critical user flows
- **Running Tests**:
  - Backend: `go test ./...`
  - Frontend: `npm test` or `vitest run`

## Documentation Updates

After any code change is merged:
1. Update the relevant design documents in `design/units/<unit-name>/` to reflect the final implementation
2. Add entries to the daily changelog in `documentation/changelogs/<YYYY-MM-DD>.md`
3. Ensure BSD/FSD documents match the actual implementation
4. Update API documentation if endpoints changed

## GitHub Workflow

### Branch Naming
- `feature/<description>` - New features
- `fix/<description>` - Bug fixes
- `docs/<description>` - Documentation changes
- `refactor/<description>` - Code refactoring
- `test/<description>` - Adding or updating tests

### Commit Messages
- Use clear, descriptive commit messages
- Conventional commits are optional but recommended: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`

### Pull Requests
- Always link PRs to the user once you have created them
- Always update the user on any changes made to the PR
- When you have created a PR, constantly check for comments and address them until the PR is merged
- Always aim for minimal changes when addressing PR comments, reduce your changes
- Once the PR is merged, checkout to main, pull, delete the old branch and git fetch --prune
- Always create new PRs for each piece of work
- Aim for micro-PRs, split PRs into divisible chunks if needed
  - Max 500 adds per planning document
  - Max 500 lines for new backend adds + Max 500 lines for unit tests
  - Max 150 lines per backend edit + Max 150 lines per backend unit tests edit
  - Max 1000 lines for new frontend adds + Max 500 lines for unit tests
  - Max 350 lines per frontend edit + Max 150 lines per frontend unit tests edit
- Attach test results to PRs (both backend and frontend)
- If a user is asking a question in a comment, answer the question before making changes
- Always respond to comments you've addressed, explaining the fix or reasoning
- **PR Descriptions**: Include clear description of changes, test results, and any relevant context
