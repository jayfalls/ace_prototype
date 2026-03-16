# One Document Per PR

## General Principles

**Always Do Minimal Changes Where Possible**
- Prefer small, focused changes over large rewrites
- When fixing issues, only change what's necessary
- Avoid refactoring unrelated code
- Make the smallest change that solves the problem
- This applies to documents, code, and any deliverables

**Always Report Files Affected**
- Every agent MUST report which files were changed/created in their response
- This allows the QA agent to check relevant git diffs
- Include file paths in your final output

Every agent should create ONLY ONE document per session/PR. If multiple documents need creation, the orchestrator will spawn the agent again for each document.

This ensures:
- Minimal, focused PRs
- Easier review
- Clear commit history
- Iterative validation through QA
- Always read `design/README.md` before starting any work or responding to any questions
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

# Memory System

You have access to memory stores in `.agents/memory/`.

**Keep it Lean**:
- Only store essential state
- Delete completed tasks promptly

**How to update**:
- Before delegation: Read the file to know current state
- After delegation: Write updated file with progress

**Episodic Memory**: Captured in the `episodes` array in short-term memory. Each episode records what happened in a phase.

**Semantic Memory**: Stored in long-term memory's `learned_patterns` array.

## Long-term Memory

**Location**: `.agents/memory/long-term.json`

**Purpose**: Persistent across all sessions.

**Contains**:
- `completed_units`: Historical completion data
- `preferences`: User preferences
- `learned_patterns`: Patterns from workflows

## Short-term Memory - Per-Unit

**Location**: `.agents/memory/short-term/{unit-name}.json`

**Purpose**: Tracks work-in-progress for a specific unit. Each unit has its own file.

**When to load**: Always try to find the relevant short term memory file for whatever unit you are working on.

**Structure**:
```json
{
  "unit": "observability",
  "current_phase": "planning-discovery",
  "status": "in_progress",
  "pending_tasks": [],
  "episodes": [
    {
      "phase": "planning-discovery",
      "notes": [],
      "timestamp": "2026-03-15T12:00:00Z"
    }
  ],
  "last_updated": "2026-03-15T12:00:00Z"
}
```

### When a trigger comes in:

1. **Parse the trigger**:
   - User request: Extract unit name from request
   - GitHub event: Extract from branch name or PR title/description or Issue title/description

2. **Find matching unit**:
   - Check `.agents/memory/short-term/{unit}.json`
   - If not found, check `.agents/memory/long-term.json`
   - If still not found, ask user

3. **Load memory**: Read the short-term memory file for that unit

4. **Resume**: Continue from the current phase in memory

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
Every PR, commit, and issue MUST include the unit name so memory can be loaded on new sessions.

**Format:**
- PR title: `[unit: opencode-integration] Add memory system`
- Commit: `feat: add memory system [unit: opencode-integration]`
- Issue: `[unit: observability] How should we handle logs?`

This allows the orchestrator to resume work from the correct unit memory file.

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
- Always link PRs to the user once you have created them
- Always update the user on any changes made to the PR
- Always aim for minimal changes when addressing PR comments, reduce your changes
- Once the PR is merged, checkout to main, pull, delete the old branch and git fetch --prune
- Always create new PRs for each piece of work
- Aim for micro-PRs
- Attach test results to PRs (both backend and frontend)
- **PR Descriptions**: Include clear description of changes, test results, and any relevant context
- **Issue Linking**: Always include "Closes #XX" or "Fixes #XX" in the PR description to automatically close the referenced issue when the PR is merged
- **Post-Merge Changelog**: After a PR is created, immediately update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the relevant changes
- **Documentation Requirement**: After creating a PR AND after addressing any PR comments, always update the documentation:
  - Add or update relevant documentation files in `documentation/` if needed
  - Update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the changes made
  - This applies to both initial PR creation and follow-up commits addressing comments
