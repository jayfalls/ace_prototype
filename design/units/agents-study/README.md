# Existing Agents Study

**Status:** Complete

## Overview

Comprehensive cross-cutting analysis of 13+ agent frameworks and 5 research papers, producing 14 comparison documents in `study/`.

## Deliverables

All research is in `study/`:

| Document | Description |
|----------|-------------|
| `architecture.md` | System topology, deployment models, surface separation patterns |
| `memory.md` | Memory systems: storage backends, retrieval strategies, tier structures |
| `compaction.md` | Context compaction: 4-layer compression, token budgets, KV cache quantization |
| `loops.md` | Agent loop architectures: ReAct, AsyncGenerator, delegation-driven patterns |
| `delegation.md` | Multi-agent delegation: hierarchy models, context isolation, parallel execution |
| `tools-skills.md` | Tool/skill systems: MCP integration, auto-generation, security models |
| `browser-automation.md` | Browser automation: Playwright, screenshot-based, DOM interaction |
| `computer-use.md` | Desktop automation: vision-based, OS-level scripting, VM sandboxing |
| `communication.md` | Internal communication: message passing, NATS validation |
| `self-improvement.md` | Learning loops: trajectory capture, evolutionary search, RL fine-tuning |
| `ux-dx.md` | Developer experience: setup friction, configuration models, debugging |
| `strengths-weaknesses.md` | Synthesis: 85+ adopt, 30+ adapt, 25+ avoid patterns |
| `user-feedback.md` | Community feedback: Reddit, X, YouTube, GitHub issues |
| `research-synthesis.md` | Research papers: TurboQuant, RLM/RISE, Meta-Harness, AlphaEvolve, RotorQuant |

## Systems Studied

**Agent Frameworks:** OpenClaw, Claude Code, Open Code, Oh My OpenAgent, Devin, Goose, Hermes Agent

**Specialized Systems:** pi-mono, andrej-karpathy-skills, playwright-skill, honcho, OpenViking, MSA, karpathy/autoresearch

**Research Papers:** TurboQuant, RLM/RISE, Meta-Harness, AlphaEvolve, RotorQuant

## Research Data

Cloneable repos and papers are in `research/` (gitignored).

## Feeding Into

- Providers unit (RotorQuant KV cache findings)
- Memory unit (RLM context decomposition, tier structures)
- Cognitive Engine unit (loop patterns, delegation)
- Skills unit (SKILL.md standard, MCP patterns)
- Learning Loop unit (trajectory capture, population search)
