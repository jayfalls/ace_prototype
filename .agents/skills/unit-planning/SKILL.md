---
name: unit-planning
description: Provides templates for planning agents to create unit design documents
---

# Unit Planning Templates

This skill provides templates for creating unit design documents.

## Document Sequence

**Complete planning documents in this order:**

### Phase 1: Discovery
1. **problem_space.md** - Problem exploration through questions (REQUIRED first)

### Phase 2: Requirements
2. **bsd.md** - Business Specification
3. **user_stories.md** - User stories and acceptance criteria
4. **fsd.md** - Functional Specification

### Phase 3: Research & Design
5. **research.md** - Technology research and evaluation
6. **architecture.md** - Technical architecture
7. **api.md** - API specifications
8. **security.md** - Security considerations
9. **monitoring.md** - Observability requirements

### Phase 4: UX
10. **design.md** - Visual design
11. **mockups.md** - Wireframes

### Phase 5: Planning
12. **testing.md** - Testing strategy
13. **implementation.md** - Implementation plan (how to build)
14. **migration_and_rollback.md** - Database migrations

### Phase 6: Dependencies
15. **dependencies.md** - External dependencies

## Templates

Located in `.agents/skills/unit-planning/unit-templates/`:
- `problem_space.md` - Problem exploration
- `bsd.md` - Business Specification
- `user_stories.md` - User stories
- `fsd.md` - Functional Specification
- `research.md` - Technology research
- `architecture.md` - Technical architecture
- `api.md` - API specifications
- `security.md` - Security considerations
- `monitoring.md` - Observability
- `design.md` - Visual design
- `mockups.md` - Wireframes
- `testing.md` - Testing strategy
- `implementation.md` - Implementation plan
- `migration_and_rollback.md` - Database migrations
- `dependencies.md` - External dependencies

## Usage

When creating a design document:
1. Read the relevant template from `.agents/skills/unit-planning/unit-templates/{template_name}.md`
2. Follow the document sequence above
3. Fill in the template based on the unit context
4. Save to `design/units/{UNIT_NAME}/{template_name}.md`
