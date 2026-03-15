---
name: unit-planning
description: Provides templates for planning agents to create unit design documents
---

# Unit Planning Templates

This skill provides templates for creating unit design documents.

## Templates

Located in `.agents/skills/unit-planning/unit-templates/`:
- `problem_space.md` - Problem exploration
- `bsd.md` - Business Specification
- `user_stories.md` - User stories
- `research.md` - Technology research
- `fsd.md` - Functional Specification
- `architecture.md` - Technical architecture
- `implementation.md` - Implementation plan
- `security.md` - Security considerations
- `testing.md` - Testing strategy
- `api.md` - API specifications
- `migration_and_rollback.md` - Database migrations
- `monitoring.md` - Observability

## Usage

When creating a design document:
1. Read the relevant template from `.agents/skills/unit-planning/unit-templates/{template_name}.md`
2. Fill in the template based on the unit context
3. Save to `design/units/{UNIT_NAME}/{template_name}.md`
