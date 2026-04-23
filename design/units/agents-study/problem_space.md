# Problem Space: Existing Agents Study

## Core Conflict

We are about to build the ACE Framework's cognitive engine, memory system, tools/skills, multi-agent orchestration, and self-improvement loops. These are complex subsystems with many possible design choices. Building without studying how existing agent frameworks solved (or failed to solve) these problems risks repeating known mistakes, missing proven patterns, and wasting weeks on approaches that the community has already validated or discarded.

## Context

ACE is a hierarchical agent swarm framework with six cognitive layers, tiered memory, continuous processing loops, and a single-binary deployment model. The design decisions already made (documented in `design/README.md`) establish constraints but leave significant freedom in implementation details for:

- Memory architecture (L1-L4 tiers, consolidation, importance scoring)
- Agent delegation and swarm orchestration
- Tool/skill systems and execution models
- Self-improvement and learning loops
- Context management and prompt engineering
- UX patterns for developer and end-user interaction

## Systems Under Study

### Agent Frameworks
- **OpenClaw** - Open-source agent framework
- **Claude Code** - Anthropic's coding agent (source code collection available)
- **Open Code** - Open-source coding agent
- **Oh My OpenAgent** - Agent orchestration framework
- **Devin** - Cognition's autonomous coding agent
- **Goose** - Open-source agent framework
- **Hermes Agent** - Nous Research's agent system

### Specialized Systems
- **pi-mono** - Browser automation agent
- **andrej-karpathy-skills** - Skills system design
- **playwright-skill** - Browser automation skill implementation
- **honcho** - Memory/identity system for agents
- **OpenViking** - VolcEngine's agent framework
- **MSA** - Multi-System Agent architecture

### Research Papers
- **TurboQuant** - AI efficiency through extreme compression (Google Research)
- **RLM** - Reinforcement Learning for Memory (alexzhang13)
- **arxiv 2603.28052v1** - (pending content analysis)
- **arxiv 2506.13131** - (pending content analysis)
- **rotorquant** - Quantization research

## Constraints

- **Depth required:** Each system must be analyzed at architectural and pattern level, not just surface features
- **Cross-cutting output:** Each output file compares ALL systems on one dimension (e.g., memory.md covers every system's memory approach)
- **Patterns over code:** Focus on design decisions, tradeoffs, and architectural patterns, not implementation details
- **Actionable:** Findings must directly inform ACE's design decisions for upcoming units

## Success Metrics

1. Every system listed is analyzed across all output dimensions
2. Each output file contains specific, comparable details (not vague summaries)
3. Strengths, weaknesses, and user feedback are documented with evidence
4. Findings identify concrete patterns ACE should adopt, avoid, or adapt
5. Self-improvement mechanisms are thoroughly documented across all systems that implement them

## Output Structure

The `study/` subdirectory will contain cross-cutting comparison files:

- `architecture.md` - Core architecture patterns, system topology, design philosophies
- `memory.md` - Memory systems, context management, retrieval strategies, consolidation
- `delegation.md` - Agent delegation, orchestration, multi-agent coordination
- `tools-skills.md` - Tool execution, skill systems, capability management
- `communication.md` - Internal communication, context passing, message patterns
- `self-improvement.md` - Learning loops, self-modification, fine-tuning, prompt evolution
- `ux-dx.md` - Developer experience, user interfaces, configuration, debugging
- `strengths-weaknesses.md` - Comparative analysis of what works and what doesn't
- `user-feedback.md` - Community complaints, compliments, common issues
- `research-synthesis.md` - Key findings from research papers and their implications for ACE

**Note:** Additional output files may be added during research as new dimensions emerge.

## Dependencies

- None - this is a pure research unit with no code dependencies
- Feeds into: Cognitive Engine, Memory, Skills, Multi-Agent, Auto Research units
