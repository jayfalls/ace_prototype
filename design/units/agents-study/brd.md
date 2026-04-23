# Business Requirements: Existing Agents Study

## Objective

Deliver a comparative architectural study of existing agent frameworks, specialized systems, and research papers to de-risk design decisions for ACE's cognitive engine, memory, skills, multi-agent orchestration, and self-improvement subsystems.

## Scope

### In Scope
- Analysis of 7 agent frameworks and 6 specialized systems listed in `problem_space.md`.
- Synthesis of 5 research papers.
- Cross-cutting comparison across initial dimensions: architecture, memory, delegation, tools/skills, communication, self-improvement, UX/DX, strengths/weaknesses, user feedback, research synthesis. Additional dimensions will be added as research uncovers new comparison axes.

### Out of Scope
- Implementation of any ACE subsystem.
- Benchmarking or performance testing of external systems.
- Contribution back to upstream projects.

## Success Criteria

1. Every listed system is analyzed across all output dimensions (initial list below, expanded as research uncovers new axes).
2. Each output file contains specific, comparable design decisions (not surface-level summaries).
3. Findings explicitly map to ACE design choices: **adopt**, **avoid**, or **adapt**.
4. User feedback (community complaints/praise) is documented with evidence.
5. Self-improvement mechanisms are thoroughly compared where present.

## Stakeholders

- **Cognitive Engine unit**: Needs patterns for layer loops and continuous processing.
- **Memory unit**: Needs proven memory tiering, retrieval, and consolidation strategies.
- **Skills unit**: Needs tool/skill execution models and capability management patterns.
- **Multi-Agent unit**: Needs delegation, orchestration, and swarm coordination patterns.
- **Auto Research unit**: Needs self-improvement and learning loop patterns.

## Constraints

- **Depth over breadth:** Architectural and pattern-level analysis, not feature lists.
- **Cross-cutting structure:** Each output dimension compares all systems together.
- **Actionability:** Vague recommendations are unacceptable; every finding must tie to an ACE decision.
- **No code:** This is a pure research unit with zero implementation dependencies.

## Value Proposition

Without this study, ACE risks spending weeks on approaches the community has already validated or discarded. The study converts external experimentation into internal design confidence, reducing rework and accelerating the core cognitive units.

## Deliverables

| File | Dimension |
|------|-----------|
| `study/architecture.md` | Core architecture patterns, system topology, design philosophies |
| `study/memory.md` | Memory systems, context management, retrieval strategies, consolidation |
| `study/compaction.md` | Context compaction, summarization strategies, token budget management |
| `study/delegation.md` | Agent delegation, orchestration, multi-agent coordination |
| `study/tools-skills.md` | Tool execution, skill systems, capability management |
| `study/communication.md` | Internal communication, context passing, message patterns |
| `study/loops.md` | Agent loops, execution cycles, concurrency models, iteration patterns |
| `study/self-improvement.md` | Learning loops, self-modification, fine-tuning, prompt evolution |
| `study/ux-dx.md` | Developer experience, user interfaces, configuration, debugging |
| `study/strengths-weaknesses.md` | Comparative analysis of what works and what doesn't |
| `study/user-feedback.md` | Community complaints, compliments, common issues |
| `study/research-synthesis.md` | Key findings from research papers and their implications for ACE |

**Note:** The deliverables list is the initial set. Additional files will be added as research uncovers dimensions not captured above (e.g., self-improvement to the scaffold itself, safety/guardrails patterns, evaluation frameworks).
