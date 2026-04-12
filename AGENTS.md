# AGENTS.md

## 1. Core Directives (Unbreakable)
- **Minimalism:** Make the smallest change possible to solve the problem. No "drive-by" refactoring.
- **Vertical Slicing:** Work MUST be planned and executed in vertical slices (e.g., One API route + Service + Repo + Svelte UI component). Never build entire layers (e.g., "All APIs") at once.
- **One Artifact Per PR:** Every PR/Session must produce exactly ONE primary deliverable (one doc or one vertical code slice).
- **Unit Accountability:** Every PR, commit, and terminal command must be tagged with the unit: `[unit: unit-name]`.

## 2. Communication & State
- **No Fluff:** Use terse, constraint-based language. Avoid "I can help with that" or "Sure thing."
- **Git-as-State:** The repository is the source of truth. Read `git status`, `git log`, and PR comments to determine current state.
- **Reporting:** Every response MUST end with a "Files Affected" list using absolute repository paths.

## 3. Code Constraints (Universal)
- **Logic:** No any. No else (use early returns). No _ (wrap/handle all errors).
- **Structure:** Small, single-responsibility functions. DRY via shared modules.
- **Context:** Descriptive names. Comments explain "Why" (intent), not "What" (syntax).

## 4. Go (Backend) & SQL
- **Architecture:** Handler (HTTP/JSON) → Service (Logic) → Repo (SQLC).
- **Persistence:** SQLC strictly. No raw SQL. Goose Go-migrations only.

## 5. SvelteKit (Frontend)
- **Svelte 5:** Runes strictly ($state, $derived, $effect, $props).
- **Composition:** Small components. Complex logic in .svelte.ts modules.

## 6. QA & Execution Loop
- **TDD-Lite:** Task = "Done" only if make test PASSES.
- **Auto-Fix:** If make test fails, loop-fix immediately. Do not request permission.
- **Evidence:** Terminal output log is mandatory for every PR/Response.

## 7. Project Directory Structure
- `/backend`: Go source code.
- `/frontend`: SvelteKit source code.
- `/design/units/{unit-name}`: All planning and architecture docs.
- `/design/units/README.md`: The global Unit Index (Status: Discovery, Research, Design, Implementation, QA).
