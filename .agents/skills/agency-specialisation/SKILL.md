---
triggers:
  - merged
  - next
  - commit
  - push
  - address
  - comment
  - start
  - begin
  - work
  - unit
  - design
  - plan
  - agent
  - flow
---

# ACE Framework Agent Specialisation

This skill provides guidance for dynamically loading agent context based on the workflow.

## Agency Specialist Activation

- **CRITICAL**: Always activate the appropriate specialist agent for each workflow stage. Agents should NOT have to infer which specialist applies - it must be stated directly. To activate a specialist agent, include their full path in your prompt. For example:
```
Use the Backend Architect agent from agency-agents/engineering/engineering-backend-architect.md 
to design the API architecture.
```

- The `agency-agents/` directory contains specialized AI agents that map to different stages of the ACE Framework unit workflow.

## Agency Mappings

| Workflow Stage | Agency Specialist | Activation Instruction |
|---------------|-------------------|------------------------|
| **Problem Space Discovery** | Product Sprint Prioritizer | "Activate the **Sprint Prioritizer** (from `agency-agents/product/product-sprint-prioritizer.md`)" |
| **BSD (Business Spec)** | Product Sprint Prioritizer | "Activate the **Sprint Prioritizer** (from `agency-agents/product/product-sprint-prioritizer.md`)" |
| **User Stories** | Product Feedback Synthesizer | "Activate the **Feedback Synthesizer** (from `agency-agents/product/product-feedback-synthesizer.md`)" |
| **Research** | Product Trend Researcher + Testing Tool Evaluator | "Activate the **Trend Researcher** (from `agency-agents/product/product-trend-researcher.md`) for market analysis AND **Tool Evaluator** (from `agency-agents/testing/testing-tool-evaluator.md`)" |
| **Backend Implementation** | Backend Architect | "Activate the **Backend Architect** (from `agency-agents/engineering/engineering-backend-architect.md`). Also read `design/README.md` for ACE-specific patterns." |
| **Frontend Implementation** | Frontend Developer | "Activate the **Frontend Developer** (from `agency-agents/engineering/engineering-frontend-developer.md`)" |
| **DevOps/Infrastructure** | DevOps Automator | "Activate the **DevOps Automator** (from `agency-agents/engineering/engineering-devops-automator.md`)" |
| **Security Review** | Security Engineer | "Activate the **Security Engineer** (from `agency-agents/engineering/engineering-security-engineer.md`)" |
| **Testing - Evidence** | Testing Evidence Collector | "Activate the **Evidence Collector** (from `agency-agents/testing/testing-evidence-collector.md`)" |
| **Testing - Quality Gate** | Testing Reality Checker | "Activate the **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)" |
| **Testing - API** | Testing API Tester | "Activate the **API Tester** (from `agency-agents/testing/testing-api-tester.md`)" |
| **Testing - Performance** | Testing Performance Benchmarker | "Activate the **Performance Benchmarker** (from `agency-agents/testing/testing-performance-benchmarker.md`)" |
| **Code Review** | Senior Developer + Reality Checker | "Activate the **Senior Developer** (from `agency-agents/engineering/engineering-senior-developer.md`) AND **Reality Checker** (from `agency-agents/testing/testing-reality-checker.md`)" |
| **UX Design** | UI Designer + UX Researcher | "Activate the **UI Designer** (from `agency-agents/design/design-ui-designer.md`) AND **UX Researcher** (from `agency-agents/design/design-ux-researcher.md`)" |
