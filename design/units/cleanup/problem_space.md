# Problem Space — Cleanup Unit

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: The ACE Framework has grown with multiple completed units (architecture, core-api, core-infra, database-design, messaging-paradigm, observability). Before building new features on top of this foundation, we need a comprehensive quality assurance review to ensure everything is clean, maintainable, scalable, and aligned with the original design intent. This is a preventative quality gate — catching issues now is exponentially cheaper than fixing them after more code is built on top.

**Q: Who are the users?**
A: The development team (including AI agents and human developers) who will build future features on top of the existing codebase. The cleanup unit ensures they work from a solid, well-understood foundation.

**Q: What are the success criteria?**
A: A detailed report documenting all issues found across three review phases, followed by fixes for every issue. The goal is zero known issues remaining after the cleanup is complete.

**Q: What constraints exist (budget, timeline, tech stack)?**
A: No specific budget or timeline constraints. The constraint is thoroughness — we must review everything, not just a subset. The tech stack includes Go backend, SvelteKit frontend, PostgreSQL, NATS, and the observability pipeline (OTel, Prometheus, Loki, Tempo).

## Iterative Exploration

### Follow-up Questions and Answers

#### Question 1
**Q: What is the scope of the review?**
A: Review ALL units in the design/units/ folder:
- architecture
- core-api
- core-infra
- database-design
- messaging-paradigm
- observability

#### Question 2
**Q: What is the three-phase approach?**
A: The cleanup follows a structured three-phase approach:
1. **Phase 1:** Review all unit design documents (problem_space.md, bsd.md, user_stories.md, fsd.md, README.md)
2. **Phase 2:** Review the entire codebase (backend Go code, frontend SvelteKit, shared packages, infrastructure files)
3. **Phase 3:** Review all documentation alignment with actual implementation

#### Question 3
**Q: What are the focus areas for the review?**
A: The review covers five key focus areas:
1. **Code quality** — clean code, maintainability, adherence to patterns
2. **Documentation alignment** — do docs match actual implementation?
3. **Architecture consistency** — are shared patterns followed across all units?
4. **Completeness gaps** — planned features not implemented?
5. **Scalability concerns** — will the current design scale?

#### Question 4
**Q: What severity levels should be flagged?**
A: ALL severity levels must be flagged:
- **Critical** — Issues that could cause system failures, data loss, or security vulnerabilities
- **Medium** — Issues that affect maintainability, scalability, or developer experience
- **Low** — Minor improvements, style inconsistencies, or documentation gaps

#### Question 5
**Q: What is the deliverable format?**
A: A detailed report with actionable items for every issue found. After the report is complete, the cleanup unit will fix everything. The report should be comprehensive enough that any developer (human or AI) can understand and address each issue.

#### Question 6
**Q: How does this relate to the existing coding standards in AGENTS.md?**
A: The cleanup must verify compliance with all standards defined in AGENTS.md, including:
- One document per PR principle
- Go backend standards (explicit types, no else chains, Handler → Service → Repository pattern)
- TypeScript/SvelteKit frontend standards (Svelte 5 runes, small focused components)
- Testing requirements (80% coverage target, integration tests for APIs)
- GitHub workflow compliance (branch naming, commit messages, PR descriptions)

## Key Insights

- **Preventative investment**: This is a quality gate that prevents technical debt from compounding. Issues caught now are exponentially cheaper to fix than after more features are built.
- **Foundation review**: Six completed units form the foundation of the entire ACE Framework. Any issues here propagate to all future work.
- **Three-phase structure**: The systematic approach (docs → code → alignment) ensures comprehensive coverage without missing areas.
- **All severity levels**: By flagging everything (critical, medium, low), we create a complete picture of the codebase health and can prioritize fixes appropriately.
- **Actionable deliverable**: The report must include specific action items, not just observations. This ensures the cleanup leads to actual improvements.

## Open Questions (Unanswered)

- Should the cleanup unit also review the shared packages (shared/messaging, shared/telemetry) as part of the codebase review, or are they considered separate?
- Are there any specific test coverage targets for the cleanup itself (e.g., should we add tests for previously untested code)?
- Should the cleanup include performance benchmarking, or is that a separate concern?

## Dependencies Identified

- **All completed units**: The cleanup depends on having access to all unit design documents and their corresponding code implementations
- **AGENTS.md standards**: The cleanup uses these as the baseline for code quality assessment
- **Makefile and pre-commit hooks**: These tools are used to validate compliance during the cleanup

## Assumptions Made

- All six units listed (architecture, core-api, core-infra, database-design, messaging-paradigm, observability) have complete design documentation available
- The codebase is in a functional state and can be tested locally
- The cleanup will address issues in priority order (critical first, then medium, then low)
- The cleanup unit itself will follow the same quality standards it enforces on other units

## Next Steps

- Proceed to BSD once all clarifying questions are answered
- Begin Phase 1 (unit design document review) during BSD creation
- Revisit this document if new questions arise during BSD creation
