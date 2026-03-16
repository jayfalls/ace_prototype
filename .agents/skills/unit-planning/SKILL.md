---
name: unit-planning
description: Provides templates for planning agents to create unit design documents
---

# Unit Planning Templates

This skill provides templates for creating unit design documents.

## Document Sequence

**Complete planning documents in this order:**
1. **problem_space.md** - Problem exploration through questions (REQUIRED first)
2. **bsd.md** - Business Specification
3. **user_stories.md** - User stories and acceptance criteria
4. **fsd.md** - Functional Specification
5. **research.md** - Technology research and evaluation
6. **architecture.md** - Technical architecture
7. **api.md** - API specifications
8. **monitoring.md** - Observability requirements
9. **implementation.md** - Implementation plan
10. **security.md** - Security considerations
11. **migration_and_rollback.md** - Database migrations
12. **testing.md** - Testing strategy
13. **design.md** - Visual design
14. **mockups.md** - Wireframes
15. **dependencies.md** - External dependencies

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
2. Follow the document sequence above
3. Fill in the template based on the unit context
4. Save to `design/units/{UNIT_NAME}/{template_name}.md`
