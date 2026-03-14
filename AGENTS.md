# Startup
- **CRITICAL**: On initialisation before responding to any messages from the user, run the `.openhands/setup.sh` script. This will install all your tooling(go, npm, docker, etc)

# Design Documentation (Always Read First!)
- **CRITICAL**: Always read `design/README.md` before starting any work on the codebase
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

# Project Structure
The ace_prototype repository is organized as follows:
- `agency-agents` - Bespoke workflow instructions
- `design/` - All design documentation for the system
  - `design/README.md` - Overall system architecture and design
  - `design/units/` - Individual unit specifications (features, components, refactors)
  - `design/units/template/` - Template documents for creating new units
- `devops` - Deployment files
- `documentation/` - Project documentation and changelogs
  - `documentation/changelogs/` - Daily changelog files
- `backend/` - Go backend source code (when implemented)
- `frontend/` - SvelteKit/TypeScript frontend source code (when implemented)

# Documentation Updates

**IMPORTANT**: Before making any changelog or documentation updates:
1. **Check the current date** - Use `date` command to get today's date
2. **Check existing changelog files** - List `documentation/changelogs/` to see what files exist and their dates
3. **Only update/add to existing files** - Never overwrite existing changelog content, only append new entries

After every commit:
1. Update the relevant design documents in `design/units/<unit-name>/` to reflect the final implementation
2. Update the `design/README.md` if relevant
3. Add entries to the daily changelog in `documentation/changelogs/<YYYY-MM-DD>.md`
4. Ensure BSD/FSD documents match the actual implementation
5. Update API documentation if endpoints changed
6. Update the user wiki documentation/ folder with relevant changes

# Agency Specialist Activation
- **CRITICAL**: Always activate the appropriate specialist agent for each workflow stage. Agents should NOT have to infer which specialist applies - it must be stated directly. To activate a specialist agent, include their full path in your prompt. For example:
```
Use the Backend Architect agent from agency-agents/engineering/engineering-backend-architect.md 
to design the API architecture.
```
- The `agency-agents/` directory contains specialized AI agents that map to different stages of the ACE Framework unit workflow. Below is the explicit mapping:

| Workflow Stage | Agency Specialist | Activation Instruction |
|---------------|-------------------|------------------------|
| **Problem Space Discovery** | Product Sprint Prioritizer | "Activate the **Sprint Prioritizer** (from `agency-agents/product/product-sprint-prioritizer.md`)" |
| **BSD (Business Spec)** | Product Sprint Prioritizer | "Activate the **Sprint Prioritizer** (from `agency-agents/product/product-sprint-prioritizer.md`)" |
| **User Stories** | Product Feedback Synthesizer | "Activate the **Feedback Synthesizer** (from `agency-agents/product/product-feedback-synthesizer.md`)" |
| **Research** | Product Trend Researcher + Testing Tool Evaluator | "Activate the **Trend Researcher** (from `agency-agents/product/product-trend-researcher.md`) for market analysis AND **Tool Evaluator** (from `agency-agents/testing/testing-tool-evaluator.md`)" |
| **Backend Implementation** | Backend Architect | "Activate the **Backend Architect** (from `agency-agents/engineering/engineering-backend-architect.md`). Also read `design/README.md` for ACE-specific patterns." |
| **Frontend Implementation** | Frontend Developer | "Activate the **Frontend Developer** (from `agency-agents/engineering/engineering-frontend-developer.md`)" |
| **DevOps/Infrastructure** | DevOps Automator | "Activate the **DevOps Automator** (from `agency-agents/engineering/engineering-devops-automator.md`)" |
| **Security Review** | Security Engineer | "Activate the **Security Engineer** (from `agency-agents/engineering/engineering-security-engineer.md`)" |
| **Testing - Evidence** | Testing Evidence Collector | "Activate the **Evidence Collector** (from `agency-agents/testing/testing-evidence-collector.md`)" |
| **Testing - Quality Gate** | Testing Reality Checker | "Activate the **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)" |
| **Testing - API** | Testing API Tester | "Activate the **API Tester** (from `agency-agents/testing/testing-api-tester.md`)" |
| **Testing - Performance** | Testing Performance Benchmarker | "Activate the **Performance Benchmarker** (from `agency-agents/testing/testing-performance-benchmarker.md`)" |
| **Code Review** | Senior Developer + Reality Checker | "Activate the **Senior Developer** (from `agency-agents/engineering/engineering-senior-developer.md`) AND **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)" |
| **UX Design** | UI Designer + UX Researcher | "Activate the **UI Designer** (from `agency-agents/design/design-ui-designer.md`) AND **UX Researcher** (from `agency-agents/design/design-ux-researcher.md`)" |

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
