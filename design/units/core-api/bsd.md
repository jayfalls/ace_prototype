# Business Specification Document

## Unit Name
Core API - API & Database Patterns

## Problem Statement
The current API codebase (`api/`) lacks established patterns, structure, and tooling. This leads to:
- No consistent approach to adding new endpoints
- No database access patterns
- No clear project organization
- Difficult to maintain, review, debug, and scale
- AI agents cannot easily navigate or extend the codebase

## Solution
Establish foundational API & DB patterns, structure, tools, and libraries that make development:
- **Easy** - clear patterns to follow
- **Maintainable** - consistent code organization
- **Scalable** - patterns that work as codebase grows
- **Reviewable** - standardized approaches
- **Debuggable** - good error handling and logging hooks

## Core Principles (from Problem Space)
1. **AI-first** - code and tooling must be agent-friendly
2. **Patterns over features** - establish structure before implementing
3. **Consistency** - same patterns across all code
4. **Minimal but complete** - only what's needed for foundation
5. **Frontend API interface** - patterns for frontend to consume API

## In Scope
- Go web framework selection and setup
- Project structure (layered or domain-driven)
- Database access patterns with SQLC
- Migration tool setup
- Request/response validation approach
- Error handling patterns
- Configuration management
- Basic middleware structure (logging, error handling)
- API route organization
- Database connection setup

## Out of Scope
- Authentication/authorization (separate auth unit)
- Full observability/monitoring (separate unit)
- Testing infrastructure (separate unit)
- Any business features (agents, memories, sessions, users, etc.)
- Frontend UI components

## Value Proposition
- Developers can easily add new API endpoints following established patterns
- Database operations are type-safe and consistent
- Code is easy to navigate for both humans and AI agents
- New team members can quickly understand the codebase
- Easy to debug issues
- Scales well as more features are added

## Success Criteria
| Criterion | Metric | Target |
|-----------|--------|--------|
| Project structure defined | Clear package organization | All code follows structure |
| Database patterns working | SQLC generates code | Queries compile type-safe |
| Migration setup | Migrations can run | DB schema can be created |
| API route pattern | New endpoints follow pattern | Consistent across all routes |
| Error handling | Standardized error responses | All errors handled uniformly |
| Configuration | Environment-based config | Works with .env |
| Frontend integration | API accessible from frontend | Frontend can call all endpoints |
