---
name: unit-planning
description: Provides templates for planning agents to create unit design documents
---

# Unit Planning Templates

This skill provides templates for creating unit design documents. The templates define the document sequence and structure.

## Document Sequence

**CRITICAL ORDER** (must follow this sequence):
1. `problem_space.md` - Problem exploration (MUST complete before BSD)
2. `bsd.md` - Business Specification Document
3. `user_stories.md` - User stories and acceptance criteria
4. `fsd.md` - Functional Specification Document
5. `research.md` - Technology research and evaluation
6. `architecture.md` - Technical architecture decisions
7. `api.md` - API specifications
8. `monitoring.md` - Observability requirements
9. `implementation.md` - Implementation plan and details
10. `security.md` - Security considerations
11. `migration_and_rollback.md` - Database migrations
12. `testing.md` - Testing strategy and test cases
13. `design.md` - Visual/UX design specifications
14. `mockups.md` - Wireframes and mockups
15. `dependencies.md` - External dependencies

## Templates

Located in `.agents/skills/unit-planning/unit-templates/`:
- `problem_space.md` - Problem exploration (MUST complete before BSD)
- `bsd.md` - Business Specification
- `user_stories.md` - User stories
- `research.md` - Technology research
- `fsd.md` - Functional Specification
- `architecture.md` - Technical architecture
- `api.md` - API specifications
- `monitoring.md` - Observability
- `implementation.md` - Implementation plan
- `security.md` - Security considerations
- `migration_and_rollback.md` - Database migrations
- `testing.md` - Testing strategy
- `design.md` - Visual design
- `mockups.md` - Wireframes
- `dependencies.md` - External dependencies

## Usage

When creating a design document:
1. Activate this skill: `Skill: unit-planning`
2. Read the relevant template from `.agents/skills/unit-planning/unit-templates/{template_name}.md`
3. Follow the document sequence (problem_space before BSD, etc.)
4. Fill in the template based on the unit context
5. Save to `design/units/{UNIT_NAME}/{template_name}.md`
