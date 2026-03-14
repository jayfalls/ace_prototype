---
triggers:
  - unit
  - units
  - BSD
  - user_stories
  - research
  - FSD
  - implementation
  - design document
  - design documents
---

# ACE Framework Unit Workflow

This skill provides guidance for working with units in the ACE Framework. Units are discrete pieces of work that can be independently designed, implemented, and documented.

## What are Units?

Units are discrete pieces of work in the ACE Framework. Each unit represents a feature, component, or refactoring that can be independently designed, implemented, and documented.

**Examples:**
- Adding a new API endpoint
- Creating a new UI component
- Implementing a database migration
- Refactoring existing code

## Unit Template Structure

Each unit should have a complete set of documentation in `design/units/<unit-name>/`:

1. `README.md` - Unit overview and status
2. `bsd.md` - Business Specification Document (what we're building and why)
3. `user_stories.md` - User stories and acceptance criteria
4. `research.md` - Research and evaluate different approaches before design decisions
5. `fsd.md` - Functional Specification Document (how we'll build it)
6. `architecture.md` - Technical architecture decisions
7. `implementation.md` - Implementation plan and details
8. `security.md` - Security considerations
9. `design.md` - Visual/UX design specifications
10. `mockups.md` - Wireframes and mockups
11. `migration_and_rollback.md` - Database migration and rollback plans
12. `testing.md` - Testing strategy and test cases
13. `api.md` - API specifications
14. `monitoring.md` - Observability requirements
15. `dependencies.md` - External dependencies

## Unit Workflow

Follow these steps when working on a unit:

### 1. Create the Unit

```bash
cp -r design/units/template design/units/<unit-name>
```

### 2. Complete Planning Documents

Complete ALL planning documents BEFORE writing any code:

1. **BSD** (Business Specification Document) - Defines the "what", business case, scope, success criteria
2. **User Stories** - Captures user requirements and acceptance criteria
3. **Research** - Research and evaluate different approaches. Always perform web searches to determine current industry standards.

Then continue with:
4. **FSD** (Functional Specification Document) - Defines the "how", technical implementation details
5. **Architecture** - Technical architecture decisions
6. **Implementation** - Implementation plan and details
7. **Security** - Security considerations
8. **Design** - Visual/UX design specifications
9. **Mockups** - Wireframes and mockups
10. **Migration** - Database migration and rollback plans
11. **Testing** - Testing strategy and test cases
12. **API** - API specifications
13. **Monitoring** - Observability requirements
14. **Dependencies** - External dependencies

### 3. Create PRs for Each Document

**One document type per PR** (e.g., one PR for research, one for BSD):
1. First PR: BSD
2. Second PR: User Stories  
3. Third PR: Research
4. Then: FSD, Architecture, Implementation, etc.

### 4. Problem Space Discovery

**IMPORTANT**: Before starting any document in a unit, explore the topic with the user through questions.

1. **Question Loop Process**: For each document, ask clarifying questions
2. **Initial Discovery**: Understand the problem, users, success criteria, constraints
3. **Iterative Exploration**: Follow-up questions until fully understood
4. **Document Findings**: The answers form the relevant document
5. **Do NOT proceed to writing** until you have a clear understanding

### 5. Implementation

Only begin implementation after all design documents are approved.

### 6. Unit Completion

When all design documents are approved and merged:
1. file_editor `implementation.md` to understand the work breakdown
2. Create GitHub issues that break implementation into micro-PRs
3. Each issue should reference the unit name and acceptance criteria
4. Create one GitHub issue per micro-PR
5. Update the changelog with a summary of the issues created

## Template Files Reference

The template files are located at `design/units/template/`:

- `design/units/template/bsd.md` - BSD template
- `design/units/template/user_stories.md` - User stories template
- `design/units/template/research.md` - Research template
- `design/units/template/fsd.md` - FSD template
- `design/units/template/architecture.md` - Architecture template
- `design/units/template/implementation.md` - Implementation template
- `design/units/template/security.md` - Security template
- `design/units/template/design.md` - Design template
- `design/units/template/mockups.md` - Mockups template
- `design/units/template/migration_and_rollback.md` - Migration template
- `design/units/template/testing.md` - Testing template
- `design/units/template/api.md` - API template
- `design/units/template/monitoring.md` - Monitoring template
- `design/units/template/dependencies.md` - Dependencies template

## Key Guidelines

- Always read `design/README.md` before starting any work
- Reference `design/units/README.md` for individual unit documentation
- One document type per PR
- Ask questions until you understand the problem space
- Research must include web searches for current industry standards
