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

Refer to the `unit-templates/` folder to see them all.

## Unit Workflow

Follow these steps when working on a unit:

### 1. Skeleton

1. Create the unit folder
```bash
mkdir -p design/units/<unit-name>
```

2. Create the readme in the unit folder based off the `unit-templates/README.md`

3. Link the readme in the units readme


### 2. Problem Space Discovery (Before Each Document)

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

### 3. Complete Planning Documents

**One document type per PR** (e.g., one PR for research, one for BSD).

Complete ALL planning documents BEFORE writing any code:
1. Start with **problem_space.md** to explore the problem through questions (REQUIRED)
2. Create **bsd.md** to define the business case
3. Create **user_stories.md** to capture user requirements
4. Conduct **research.md** to evaluate different approaches and make informed design decisions
5. Write **fsd.md** for technical details
6. Design **architecture.md** for system integration
7. Plan **implementation.md** for execution
8. Document **security.md** considerations
9. Complete remaining documents as needed

## Unit Completion Workflow

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


### Technology Recommendations

When suggesting technologies, libraries, or frameworks:
1. **Always perform web searches** to find current options
2. **Provide multiple alternatives** - never recommend just one
3. **Verify active maintenance** - check GitHub activity, last release date, issue response time
4. **Recommend latest stable versions** - check for the most recent releases
5. **Consider community adoption** - look at stars, downloads, and real-world usage
