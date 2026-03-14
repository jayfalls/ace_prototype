# Design Documentation (Always Read First!)
- **CRITICAL**: Always read `design/README.md` before starting any work on the codebase
- Reference `design/units/README.md` for individual unit documentation
- Understanding the overall system design is essential before making any changes

# Project Structure
The ace_prototype repository is organized as follows:
- `design/` - All design documentation for the system
  - `design/README.md` - Overall system architecture and design
  - `design/units/` - Individual unit specifications (features, components, refactors)
  - `design/units/template/` - Template documents for creating new units
- `documentation/` - Project documentation and changelogs
  - `documentation/changelogs/` - Daily changelog files
- `backend/` - Go backend source code (when implemented)
- `frontend/` - SvelteKit/TypeScript frontend source code (when implemented)

# Working on the Code

## Design Documentation
- Reference design/README.md for the overall system design
- Reference design/units/README.md for each individual piece of the system and leg of work
- When creating a unit of work, fully complete the documentation before beginning with code

## Units

**What are Units?**
Units are discrete pieces of work in the ACE Framework. Each unit represents a feature, component, or refactoring that can be independently designed, implemented, and documented. Examples include:
- Adding a new API endpoint
- Creating a new UI component
- Implementing a database migration
- Refactoring existing code

**Unit Template Structure**
Each unit should have a complete set of documentation. The template includes 14 documents:
1. `BSD.md` - Business Specification Document (what we're building and why)
2. `user_stories.md` - User stories and acceptance criteria
3. `research.md` - Research and evaluate different approaches before design decisions
4. `FSD.md` - Functional Specification Document (how we'll build it)
5. `architecture.md` - Technical architecture decisions
6. `implementation.md` - Implementation plan and details
7. `security.md` - Security considerations
8. `design.md` - Visual/UX design specifications
9. `mockups.md` - Wireframes and mockups
10. `migration_and_rollback.md` - Database migration and rollback plans
11. `testing.md` - Testing strategy and test cases
12. `api.md` - API specifications
13. `monitoring.md` - Observability requirements
14. `dependencies.md` - External dependencies

**Unit Workflow**
1. Create a new unit by copying the template directory: `cp -r design/units/template design/units/<unit-name>`
2. Complete ALL planning documents (research through dependencies) BEFORE writing any code
3. Create a PR for the BSD first, then user_stories, then research
4. Only begin implementation after all design documents are approved
5. One document type per PR (e.g., one PR for research, one for BSD)

### Problem Space Discovery (Before Each Document)
**IMPORTANT**: Before starting any document in a unit, explore the topic with the user through questions.

1. **Question Loop Process**: For each document (BSD, user_stories, research, FSD, architecture, implementation, etc.):
   - Ask clarifying questions about what's needed for that specific document
   - Don't assume - ask until you understand
   - Document the Q&A in the relevant section

2. **Initial Discovery**: Ask clarifying questions to understand:
   - What problem are we trying to solve?
   - Who are the users?
   - What are the success criteria?
   - What constraints exist (budget, timeline, tech stack)?

3. **Iterative Exploration**: Ask follow-up questions in a loop until the problem space is fully understood:
   - Clarify ambiguous requirements
   - Explore edge cases
   - Identify dependencies and integrations
   - Understand non-functional requirements (performance, security, scalability)

4. **Document Findings**: The answers form the relevant document (problem_space.md, user_stories.md, etc.)

5. **Do NOT proceed to writing** until you have a clear understanding. It is better to ask more questions than to assume.

### Unit Documents
- **BSD (Business Specification Document)**: Defines the "what" - business case, scope, success criteria. Not the "how" (that's FSD).
- **User Stories**: Captures user requirements and acceptance criteria.
- **Research Document**: Research and evaluate different approaches before making design decisions. Includes industry standards, pros/cons analysis, and recommendations. **Always perform web searches to determine current industry standards, latest versions, and actively maintained technologies.**
- **FSD (Functional Specification Document)**: Defines the "how" - technical implementation details.
- BSD comes first, then user_stories, then research, then FSD. Each in separate PRs.

### Technology Recommendations
When suggesting technologies, libraries, or frameworks:
1. **Always perform web searches** to find current options
2. **Provide multiple alternatives** - never recommend just one
3. **Verify active maintenance** - check GitHub activity, last release date, issue response time
4. **Recommend latest stable versions** - check for the most recent releases
5. **Consider community adoption** - look at stars, downloads, and real-world usage

### Unit Completion Workflow
When all design documents for a unit have been approved and merged:
1. file_editor the unit's implementation document (implementation.md) to understand the work breakdown
2. Create detailed GitHub issues that break the implementation into micro-PRs (the smallest divisible units of work)
3. Each issue should:
   - Have a clear, focused title describing one specific task
   - Detail that the agent must read `design/README.md` and `design/units/<unit-name>/` before starting
   - Reference the relevant unit name and document
   - Include acceptance criteria from the user stories or implementation plan
   - **IMPORTANT**: Include instruction that the agent MUST respond to the issue with the PR link once created
   - Be small enough to be implemented in a single PR
4. Create one GitHub issue per micro-PR
5. After creating all issues, update the changelog with a summary of the issues created
6. Link these issues in the unit's README for tracking

## Agency Specialist Activation
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

## Documentation Updates

**IMPORTANT**: Before making any changelog or documentation updates:
1. **Check the current date** - Use `date` command to get today's date
2. **Check existing changelog files** - List `documentation/changelogs/` to see what files exist and their dates
3. **Only update/add to existing files** - Never overwrite existing changelog content, only append new entries

After any code change is commited:
1. Update the relevant design documents in `design/units/<unit-name>/` to reflect the final implementation
2. Add entries to the daily changelog in `documentation/changelogs/<YYYY-MM-DD>.md`
3. Ensure BSD/FSD documents match the actual implementation
4. Update API documentation if endpoints changed
5. Update the user wiki documentation/ folder with relevant changes

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
