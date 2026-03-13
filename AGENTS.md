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
- **SQLC**: Always run `sqlc generate` after modifying SQL query files before committing
- **Layered Architecture**: Always use Handler → Service → Repository pattern:
  - Handler: HTTP request/response only, no business logic
  - Service: Business logic, orchestrates repositories
  - Repository: Database queries only (via SQLC)
- **No else chains**: Avoid else statements - use early returns instead

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

### Verifying Code Before Pushing

**CRITICAL**: Always test and verify code changes before pushing to remote.

1. **Install Dependencies**: The agent has sudo access and can install missing dependencies:
   ```bash
   # For Go projects
   sudo apt-get update && sudo apt-get install -y golang-go
   
   # For Node.js projects
   sudo apt-get update && sudo apt-get install -y nodejs npm
   ```

2. **Build Verification**: Always verify the code compiles before pushing:
   ```bash
   # For Go
   go build ./...
   
   # For frontend
   npm run build
   ```

3. **Run Tests**: Execute tests to verify functionality:
   ```bash
   # For Go
   go test -v ./...
   
   # For Node.js
   npm test
   ```

4. **Verify in Container**: For Docker-based projects, verify in the container:
   ```bash
   # Build and run containers
   make up
   
   # Run tests in container
   make test
   
   # Check health endpoint
   curl http://localhost:8080/health
   ```

5. **Check for Common Issues**:
   - Unused imports
   - Variable scope issues
   - Type mismatches
   - API compatibility (check library documentation for correct usage)

6. **If Tests Fail**: Fix the issues locally before pushing. Do not push broken code.

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

### Technology Recommendations
When suggesting technologies, libraries, or frameworks:
1. **Always perform web searches** to find current options
2. **Provide multiple alternatives** - never recommend just one
3. **Verify active maintenance** - check GitHub activity, last release date, issue response time
4. **Recommend latest stable versions** - check for the most recent releases
5. **Consider community adoption** - look at stars, downloads, and real-world usage

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
- **Issue Linking**: Always include "Closes #XX" or "Fixes #XX" in the PR description to automatically close the referenced issue when the PR is merged
- **Post-Merge Changelog**: After a PR is merged, immediately update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the relevant changes
- **Documentation Requirement**: After creating a PR AND after addressing any PR comments, always update the documentation:
  - Add or update relevant documentation files in `documentation/` if needed
  - Update the changelog in `documentation/changelogs/<YYYY-MM-DD>.md` with the changes made
  - This applies to both initial PR creation and follow-up commits addressing comments

### Post-Merge Workflow
After any PR is merged (whether documentation or code):
1. Immediately update the relevant changelog file in `documentation/changelogs/<YYYY-MM-DD>.md`
2. Include the PR title, number, and a brief summary of changes
3. If it's a design document approval, mark that document as approved in the unit's README

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
