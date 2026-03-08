# AGENTS.md

## Working on the Code

### Design Documentation
- Reference design/README.md for the overall system design
- Reference design/units/README.md for each individual piece of the system and leg of work
- When creating a unit of work, fully complete the documentation before beginning with code
- After fully completing a code change for a unit (merged), create a PR to update the documentation/ folder with relevant changes pertaining to the unit (add files as needed, edit existing files as needed, no changes if nothing relevant)
- Also, add-by-copying-template/update the relevant changelog file for the day in documentation/changelogs/ with the relevant changes

### Unit Documents
- **BSD (Business Specification Document)**: Defines the "what" - business case, scope, success criteria. Not the "how" (that's FSD).
- **FSD (Functional Specification Document)**: Defines the "how" - technical implementation details.
- BSD comes first, then FSD. Each in separate PRs.

## GitHub Workflow

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