# One Document Per PR

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

Every piece of work should create ONLY ONE document per session/PR. If multiple documents need creation, they should be created one at a time.

This ensures:
- Minimal, focused PRs
- Easier review
- Clear commit history
- Iterative validation through QA
- Always read `design/README.md` before starting any work or responding to any questions
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

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

## GitHub Workflow

### Unit Reference (CRITICAL)
Every PR, commit, and issue MUST include the unit name so progress can be tracked across sessions.

**Format:**
- PR title: `[unit: opencode-integration] Add memory system`
- Commit: `feat: add memory system [unit: opencode-integration]`
- Issue: `[unit: observability] How should we handle logs?`

### Branch Workflow (CRITICAL)
**ALWAYS create a new branch for every feature, fix, or piece of work.** Never work directly on main or any existing branch.

**NEVER commit directly to main.** All work must be done on feature branches. If you accidentally commit to main, immediately revert the commit and create a proper branch.

Steps:
1. Before starting any work: `git checkout main && git pull && git checkout -b feature/<description>`
2. One branch per feature/PR - never bundle unrelated work
3. **ALWAYS create a PR after committing changes** - no work is complete without a PR
4. After PR is merged: delete the branch immediately (`git branch -d <branch-name> && git push origin --delete <branch-name>`)
5. After deleting branch: `git checkout main && git pull && git fetch --prune`

### Branch Naming
- `feature/<description>` - New features
- `fix/<description>` - Bug fixes
- `docs/<description>` - Documentation changes
- `refactor/<description>` - Code refactoring
- `test/<description>` - Adding or updating tests

### Commit Messages
- Use clear, descriptive commit messages
- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`

### Pull Requests
**ALWAYS create a PR after committing changes** - work is not complete until a PR is created.

- Always link PRs to the user once you have created them
- Always update the user on any changes made to the PR
- Always aim for minimal changes when addressing PR comments, reduce your changes
- Once the PR is merged, checkout to main, pull, delete the old branch and git fetch --prune
- Always create new PRs for each piece of work
- Aim for micro-PRs
- Attach test results to PRs (both backend and frontend)
- **PR Descriptions**: Include clear description of changes, test results, and any relevant context
- **Issue Linking**: Always include "Closes #XX" or "Fixes #XX" in the PR description to automatically close the referenced issue when the PR is merged
- **Changelog (MANDATORY)**: After creating a PR, IMMEDIATELY update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the relevant changes. This is NOT optional - every PR must have a corresponding changelog entry.
- **Documentation Requirement**: After creating a PR AND after addressing any PR comments, always update the documentation:
  - Add or update relevant documentation files in `documentation/` if needed
  - Update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the changes made
  - This applies to both initial PR creation and follow-up commits addressing comments

### CRITICAL: Wait for Merge
**NEVER proceed to the next document or task until the current PR is merged by the user.**

- After creating a PR and updating the changelog, STOP and wait for user to merge
- Do NOT create the next document or start the next task
- Do NOT assume the user wants to continue
- Wait for explicit confirmation that the PR is merged before proceeding
- This ensures the user has full control over the workflow and can review before continuing
