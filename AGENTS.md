# AGENTS.md

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## Core Principles

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**
Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**
- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- No `any`. No `else` (use early returns). No `_` (wrap/handle all errors).
- Descriptive names. Comments explain "why" (intent), not "what" (syntax).
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**
When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**
Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

## Repo Workflow

### 1. Repo Specifics

**Vertical slices. Small writes. One artifact per PR.**
- Work in vertical slices (e.g., one API route + Service + Repo + Svelte UI component). Never build entire layers at once.
- When using the write/edit tool, write small blocks — there is a bug that causes timeouts on large writes.
- Every PR/session produces exactly one primary deliverable (one doc or one vertical code slice).
- Every PR, commit, and terminal command must be tagged: `[unit: unit-name]`.

### 2. Communication & State

**Terse. No fluff. Git is the source of truth.**
- Use constraint-based language. No "I can help with that" or "Sure thing."
- Read `git status`, `git log`, and PR comments to determine current state.
- Every response must end with a "Files Affected" list using absolute repository paths.

### 3. QA & Execution Loop

**Tests pass or you're not done. Auto-fix failures.**
- Task = "Done" only if `make test` passes.
- If `make test` fails, loop-fix immediately. Do not request permission.

### 4. Project Directory Structure

**Know where things live. Don't guess.**
- `/backend`: Go source code.
- `/frontend`: SvelteKit source code.
- `/design/README.md`: The global repo context.
- `/design/units/{unit-name}`: All planning and architecture docs.
- `/design/units/README.md`: The global Unit Index (Status: Discovery, Research, Design, Implementation, QA).

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
