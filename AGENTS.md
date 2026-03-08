# AGENTS.md

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