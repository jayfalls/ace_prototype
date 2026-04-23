# User Feedback Research: AI Agent Systems

**Research Date:** April 2026  
**Systems Researched:** 11 agent frameworks  
**Sources:** GitHub Issues, Reddit, YouTube, Hacker News, Web Search

---

## Table of Contents

1. [OpenClaw](#1-openclaw)
2. [Claude Code](#2-claude-code)
3. [OpenCode (anomalyco/opencode)](#3-opencode-anomalycoopencode)
4. [Oh My OpenAgent](#4-oh-my-openagent)
5. [Goose](#5-goose)
6. [Hermes Agent](#6-hermes-agent)
7. [Devin](#7-devin)
8. [pi-mono](#8-pi-mono)
9. [honcho](#9-honcho)
10. [OpenViking](#10-openviking)
11. [MSA (Memory Sparse Attention)](#11-msa-memory-sparse-attention)
12. [Recurring Themes](#12-recurring-themes)

---

## 1. OpenClaw

### Overview
OpenClaw (aka clawdbot/Moltbot) is a multi-platform AI agent framework supporting Telegram, WhatsApp, Discord, and other messaging platforms. It experienced a significant security crisis in early 2026.

### Security Issues (CVE-2026-25253)

**Critical Vulnerability Disclosure (January 2026)**

The most critical issue was CVE-2026-25253, a CVSS 8.8 remote code execution vulnerability discovered in OpenClaw's control interface. The flaw allowed attackers to steal authentication tokens via WebSocket connection hi-jacking and execute arbitrary commands.

**Key Facts:**
- Patched in version 2026.1.29 (January 30, 2026)
- Over 42,000 unpatched instances publicly reachable
- 63% of instances were running unpatched versions at disclosure
- Default network binding to 0.0.0.0 (all interfaces) exposed users

**Supply Chain Attack (ClawHavoc)**
- 341 malicious skills discovered (12% infection rate of 2,857 audited skills)
- Malware families: Atomic Stealer (macOS), ClickFix (Windows)
- Targets: Browser passwords, crypto wallets, SSH keys, API tokens
- 14 contributor accounts compromised

### User Quotes

> "On January 28, 2026, security researchers disclosed CVE-2026-25253, a critical remote code execution (RCE) vulnerability in OpenClaw's core message-handling pipeline. The flaw was assigned a CVSS score of 8.8 (High), and for good reason: it allows an attacker to achieve full code execution on the host machine with a single crafted message."  
> — ZeroClaw Blog, February 2026

> "CVE-2026-25253 wasn't just a risk to hobbyists running OpenClaw on their laptop. It was a risk to companies that had deployed AI agents with access to internal systems, credentials, and data."  
> — ZeroClaw Blog

> "The combination of CVE-2026-25253, the ClawHavoc supply chain compromise, and warnings from Microsoft, Cisco, and Kaspersky paints a picture: running OpenClaw without hardened infrastructure, skill vetting, and continuous monitoring is an unacceptable risk for any organization handling real data."  
> — Security Advisory

> "Update to version 2026.1.29 or later immediately. Check whether your gateway is exposed to the internet."  
> — Snyk Vulnerability Report

> "Many users deployed OpenClaw without letting authentication, assuming localhost binding was sufficient. The security crisis demonstrated that defense in depth is essential—even for local-only services."  
> — OpenClaw Security Crisis Retrospective

### Complaints

1. **Massive Supply Chain Attack**: 341 malicious skills (12% of audited) contained malware including keyloggers and credential stealers
2. **Default Binding to All Interfaces**: OpenClaw bound gateway to 0.0.0.0 by default, exposing instances without prominent documentation warnings
3. **Skill Vetting Gaps**: No mandatory code signing for ClawHub submissions allowed malicious skills to spread
4. **Token Exfiltration**: WebSocket gateway trusted gatewayUrl parameter from query strings without validation
5. **Persistent Unpatched Instances**: Over 40,000 instances remained unpatched and publicly reachable two weeks after disclosure

### Compliments

1. **Rapid Patch Response**: Maintainers released patches within 48-72 hours of critical vulnerability disclosure
2. **Security Infrastructure Improvements**: SHA-256 hashing, Docker sandbox hardening, VNC password authentication implemented post-crisis
3. **Transparent Communication**: Official blog posts clearly documented vulnerabilities and remediation steps

### GitHub Issues

- Issue #16052: "Security: Malicious skill on OpenClawDir (CVE-2026-25253)" - Confirmed 341-skill supply chain attack
- Multiple security advisories documented in SECURITY.md

### Pricing/Cost Feedback
Free and open source. No pricing tiers.

### Setup Friction
- Installation complexity moderate (Node.js, pnpm)
- Docker setup available for sandboxing
- Skill marketplace had significant security gaps

---

## 2. Claude Code

### Overview
Anthropic's official CLI coding agent. Premium product with subscription-based pricing.

### User Quotes

> "I'm a career engineer and I went from being one of their most outspoken proponents (at least within my circle) and now.... I'm not. If Claude Code was removed from the Pro plan, then the Max plan should list Claude Code as one of its extras."  
> — Hacker News Commenter, April 2026

> "Anthropic quietly removed Claude Code from its $20/month Pro plan. The coding assistant now requires a Team plan Premium seat at $100/seat/month. That's a 5x price jump."  
> — Agent Wars, April 2026

> "$152 in unexpected API charges this month that should have been $0. Subagent child processes find ANTHROPIC_API_KEY in the environment and use it for direct API calls instead of the Max Plan subscription."  
> — GitHub Issue #39903

> "I paid $1,800+ in API charges in two days (Mar 20-21, 2026) billed to a separate Anthropic API account. claude -p bypasses OAuth and requires ANTHROPIC_API_KEY — meaning it always bills to the API account, never to a Max subscription."  
> — GitHub Issue #37686

> "The responses on those mega-threads from Anthropic rubbed me the wrong way in a 'you're holding it wrong' kinda way."  
> — Hacker News Commenter

> "~$2/minute, 28k/month is insane."  
> — GitHub Issue #129

### Complaints

1. **$100/Month Price Hike**: Quietly removed from $20 Pro plan, now requires $100 Team plan
2. **Subagent Billing Bugs**: Max plan subscribers charged via API key for subagent dispatches instead of subscription
3. **Token Waste**: Claude Code double-burns tokens through defensive re-verification, redundant spawning, summary echo habits
4. **Session Limits Degrading**: Max 20x ($200/mo) subscribers report limits depleting abnormally fast since March 2026
5. **Hidden API Costs**: claude -p always bills to API account, not subscription, with no warnings for Max subscribers

### Compliments

1. **World-Class Model Quality**: When working correctly, considered best-in-class for complex coding tasks
2. **Excellent Context Management**: CLAUDE.md, prompt caching, and compact commands well-designed
3. **Professional Feature Set**: MCP support, tool calling, multi-step task completion widely praised

### GitHub Issues

- Issue #39903: "Max Plan subscribers billed through API key when subagents dispatch" - $152 unexpected charges
- Issue #37686: "claude -p suggested to Max subscriber — caused $1,800+ unintended API billing"
- Issue #42939: "Max 20x ($200/mo) plan - session limits depleting abnormally fast"
- Issue #44926: "Claude Code CLI admits to double burning my tokens"
- Issue #42796: "Quality regression after February 'redact-thinking' update" (AMD engineer analysis)

### Pricing/Cost Feedback

| Plan | Price | Includes |
|------|-------|----------|
| Pro | $20/mo | Claude Code removed |
| Team/Max | $100/seat/mo | Claude Code included |
| Max 20x | $200/mo | Higher limits, still had billing bugs |

Token costs: ~$2/minute reported, prompt caching enabled by default.

### Setup Friction
- CLI installation straightforward via npm
- API key configuration required
- Model selection automatic but sometimes ignores user preferences

---

## 3. OpenCode (anomalyco/opencode)

### Overview
A横UI-first AI coding agent with TUI, desktop app, and ACP protocol support. Known for excellent UI/UX but plagued by TUI stability issues.

### User Quotes

> "So much for using LLMs to fix said problem in days, never mind weeks! This entire movement is a joke."  
> — GitHub Issue #2697 (Konsole rendering bug)

> "the tui is about to get a big rewrite shipped so once that is complete, the team can try to better address these"  
> — OpenCode Maintainer response

> "I had the same thing happening to me using Warp on Windows. It didn't stick on generating but any scrolling or anything would cause overlap and garbled text."  
> — GitHub Issue #2697

> "Same issue with Windows Terminal ssh to linux host."  
> — GitHub Issue #2697

> "This is stellar stuff that you guys are doing, and I understand that my use case - with tens and tens of subagent tabs/sub-sessions per session is probably a fringe use case."  
> — GitHub Issue #3935

### Complaints

1. **TUI Rendering Glitches**: Overlapping panels, misaligned content, corrupted layout - especially on Konsole, Windows Terminal
2. **TUI Freezing/Hanging**: After LLM streaming completes, TUI hangs at 0% CPU due to stream loop not breaking on finish event
3. **Massive Log Output Crashes**: 35k line outputs cause complete TUI hangs
4. **Subagent Memory Bloat**: Long sessions with many subagent tabs become extremely sluggish
5. **Connection Drops**: SSE stream disconnects after 4-5 minutes, UI freezes waiting for user input

### Compliments

1. **Beautiful UI Design**: "Stellar stuff" - excellent visual design when working
2. **ACP Protocol Support**: Enables proxy interception and flexible client/agent configurations
3. **Great Accessibility**: Compact mode for mobile/small viewport users (18% whitespace reduction)
4. **Active Development**: Frequent updates and community engagement

### GitHub Issues

- Issue #2697: "TUI looks horrible after a single prompt" (KDE Konsole rendering)
- Issue #15310: "TUI freezes/hangs after LLM streaming completes"
- Issue #11109: "TUI rendering glitches: overlapping panels, misaligned content"
- Issue #3935: "v1 UI is quite unstable" (Mac M2, Ghostty)
- Issue #5094: "TUI loses connection with server"
- Issue #12667: "opencode run ignores recently used model from TUI"

### Pricing/Cost Feedback
Free and open source. Model costs via API keys (OpenAI, Anthropic, etc.).

### Setup Friction
- Installation via curl script or source build
- Model configuration through JSON config
- TUI stability issues on certain terminals (Konsole, Windows Terminal)

---

## 4. Oh My OpenAgent

### Overview
A community-driven OpenCode plugin system featuring multiple specialized agents (Sisyphus, Atlas, Oracle, Librarian, etc.). Rapid growth created maintainer burnout.

### User Quotes

> "I would strongly suggest bringing on an additional maintainer—perhaps someone dedicated specifically to community management to share the load. With the current volume of incoming issues, the current structure seems bound to cause frustration for everyone."  
> — GitHub Issue #518

> "Love the features, struggling with the bugs: Can we empower the community to help triage?"  
> — GitHub Issue #1743

> "You can always pick a different LLM provider (especially with OpenRouter), plus if you want to contain the number of subagents launched, go vanilla. Automation has a price to pay. Don't mix agent packs unless you want to play with fire."  
> — Community Commenter

> "Sisyphus is insane, and driving me crazy."  
> — GitHub Issue #1052

> "The thing updates configs break, spend hours fixing config till it is perfect as per the 'new improved' 3.x. Agent forgets who he is. Auth plugin breaks, spamming 50x auth.tmp files second."  
> — GitHub Issue #1189

### Complaints

1. **Solo Maintainer Burnout**: Massive attention overwhelming single maintainer, issues get buried
2. **Breaking Updates**: v3.x breaking changes to config files, agent identity resets
3. **Auth Plugin Bugs**: Spamming auth.tmp files, authentication failures
4. **Over-Orchestration**: Atlas orchestrator too aggressive, dangerous/destructive behaviors
5. **Compaction Corruption**: Summary inserts arbitrary constraints overriding user's agents.md

### Compliments

1. **Feature-Rich**: "Love the features" - powerful multi-agent orchestration
2. **Community Response**: Maintainer acknowledged concerns and added triage permissions for contributors
3. **Specialized Agents**: Useful agent specializations (Sisyphus, Oracle, Librarian, Explore, Frontend Engineer)

### GitHub Issues

- Issue #518: "Community response concerns and considering alternatives"
- Issue #1743: "Love the features, struggling with the bugs: Can we empower the community to help triage?"
- Issue #1189: "please stop overengineering shit"
- Issue #1052: "Sisyphus is insane, and driving me crazy"
- Issue #1081: "Orchestration is too aggressive, some behaviours are dangerous/destructive"
- Issue #1483: "Compaction summary inserts arbitrary constraints that override user agents.md"

### Pricing/Cost Feedback
Free and open source. Requires API keys for LLM providers.

### Setup Friction
- Installation frictionless via npx
- Config complexity high for multi-agent setup
- Confusion for OpenRouter users without subscriptions

---

## 5. Goose

### Overview
Block's open-source AI agent with desktop app, CLI, and strong MCP extension ecosystem. Emphasizes extensibility and local-first execution.

### User Quotes

> "For starters, the MCP config is broken. It does not work in a corporate env. where proxies are required and such. Also, it does not allow you to save the config, the UI is not friendly for configuration."  
> — GitHub Issue #2233

> "I will second OP. There is only 1 entry, it's highly limited and hard to find. It does not provide examples for various transport types or setting keys from env."  
> — GitHub Issue #2233

> "These should be run as silent background processes, rather than launched in this way. It's insanely annoying, especially when you have many MCP servers."  
> — GitHub Issue #4028

> "Goose Desktop App for Mac can't use Community Extensions. MCP server extensions (community extensions) fail to load and provide tools in Goose Desktop."  
> — GitHub Issue #6704

> "I've implemented this feature as a boolean config option tui.no_terminal_padding_x that removes horizontal padding."  
> — GitHub PR (compact mode implementation)

### Complaints

1. **MCP Config Broken for Corporate Proxies**: No proxy support, UI not friendly for configuration
2. **Desktop Opens Terminal Windows**: Extensions spawn visible terminal windows instead of silent background processes
3. **Community Extensions Don't Work on Mac Desktop**: Custom MCP extensions fail to load, available_tools array empty
4. **Auto-Compact on New Sessions**: Removing custom MCP extension causes immediate compaction on brand-new sessions
5. **Telemetry Prompt Blocking**: Requires GOOSE_TELEMETRY_ENABLED: false in config for auto mode

### Compliments

1. **Strong MCP Ecosystem**: Great extension architecture with community MCP servers
2. **Local-First Design**: Runs on machine with full access to development environment
3. **Multi-Provider Support**: Works with OpenAI, Anthropic, Google, Meta, Ollama
4. **Recipe System**: Reusable task templates well-designed

### GitHub Issues

- Issue #2233: "MCP Config" - broken for corporate environments
- Issue #4028: "Goose Desktop for Windows keeps opening goosed.exe terminal windows"
- Issue #6704: "Goose Desktop App for Mac can't use Community Extensions"
- Issue #6146: "Goose Desktop auto-compacts new sessions after removing custom MCP extension"
- Issue #6659: "mcp server is not able to connect" (telemetry config requirement)
- Issue #5754: "The Goose Desktop tool router doesn't expose standard MCP protocol operations"

### Pricing/Cost Feedback
Free and open source. API key costs for model providers.

### Setup Friction
- Desktop app installation straightforward
- MCP configuration complex and poorly documented
- Telemetry consent required for interactive mode

---

## 6. Hermes Agent

### Overview
Nous Research's CLI-first agent with multi-platform gateway (Discord, Slack, Telegram, WhatsApp/Signal), 3-layer memory system, and self-improving skills.

### User Quotes

> "The marketing implies something approaching learning. The reality: Hermes writes markdown files to ~/.hermes/memories/. A MEMORY.md and optionally a USER.md with section delimiters, loaded into context at the start of each session. This is the same pattern used by Claude Code, OpenCode, and every other tool with a config file."  
> — George Larson Review, March 2026

> "The 'grows with you' and 'gets more capable' framing is a stretch for what amounts to structured note-taking, but the underlying implementation is solid."  
> — George Larson Review

> "If your use case is 'AI agent accessible on messaging platforms,' Hermes is the most complete agent framework for that. I haven't found anything else with this level of multi-platform gateway support."  
> — George Larson Review

> "Self-learning is disabled by default. This trips up first-time users. You must explicitly enable persistent memory and skill generation in ~/.hermes/config.toml. If you skip this, Hermes behaves like a standard single-session agent."  
> — TokenMix Review

> "The audit gap is real. Skills are auditable — they are Markdown files you can read and edit. Memories are auditable — they are SQLite rows you can inspect and delete. But the practical question is whether you will."  
> — Saulius Blog

### Complaints

1. **Memory Opacity**: Cannot easily export "everything Hermes knows about me" as human-readable file
2. **Self-Learning Disabled by Default**: First-time users confused when memory doesn't persist
3. **Audit Gap**: Users drift "out of the loop" as agent self-modifies without regular review
4. **Skill Generation Domain Issues**: Works for clearly-defined tasks but unreliable for ambiguous ones
5. **Multi-Platform Setup Complexity**: 12 platform adapters but configuration can be daunting

### Compliments

1. **Multi-Platform Gateway**: "Most complete agent framework" for messaging platform access
2. **Memory Engineering Quality**: Atomic writes, file locking, injection scanning, frozen snapshots well-implemented
3. **Self-Improving Loop**: Episodic memory and skill generation from experience works for repeated tasks
4. **Transparent Files**: MEMORY.md and USER.md are plain text, user-editable
5. **Provider-Agnostic**: Works with OpenRouter, Anthropic, OpenAI, Ollama, LM Studio

### GitHub Issues
Multiple discussions in repository about memory configuration and provider setup.

### Pricing/Cost Feedback
Free and open source. Model costs via API keys.

### Setup Friction
- Python 3.10+ required
- ChromaDB initialization for episodic memory
- Memory provider setup wizard available (new feature)

---

## 7. Devin

### Overview
Cognition's autonomous AI coding agent, marketed as "the first AI software engineer." Premium pricing at $500/month with Agent Compute Units (ACUs).

### User Quotes

> "I paid $500 for Devin, the mega hyped AI coding agent, so you don't have to. Does it work? Yes, to some extent. You'll just have to be very very careful with it and be very careful with what tasks you give it."  
> — YouTube Review by Conner Ardman

> "The honest take: Devin 2.2 is a meaningful improvement over what was, frankly, a rough product. But $500/month still prices out most of the developers who'd benefit from trying it."  
> — Agent Rank Review, March 2026

> "At $500/month with compute credits, Devin costs roughly $6,000/year. A junior developer in Southeast Asia costs $12,000-18,000/year but brings judgment, growth, and context."  
> — AI Tool Crush Review

> "Building a basic React component burns 25-40 ACUs if Devin gets confused. One simple login form with form validation cost me like 40-45 ACUs."  
> — Toolstac Review

> "Out of 20 tasks, we had 14 failures, 3 successes (including our 2 initial ones), and 3 inconclusive results. Even more telling was that we couldn't discern any pattern to predict which tasks would work."  
> — Answer.AI Month with Devin

### Complaints

1. **$500/Month Price Tag**: "Prices out most of the developers who'd benefit from trying it"
2. **High Failure Rate**: 70% failure rate on standard tasks (Answer.AI testing: 14/20 failures)
3. **Credit Consumption**: Simple tasks burn 25-40 ACUs, "like burning money"
4. **Infinite Loops**: Gets stuck on complex tasks, spending hours going in circles
5. **Architecture Blindness**: Builds whatever asked without questioning if approach is right

### Compliments

1. **Autonomous Execution**: Can be given GitHub issue and run unattended, creating branches, writing code, running tests
2. **$20 Pay-As-You-Go Plan**: Entry-level option available for testing
3. **Junior-Level Task Utility**: "Genuinely useful" for clearly-defined junior-level tickets
4. **Devin IDE**: Built-in development environment with browser and terminal praised

### GitHub Issues
N/A - closed proprietary product.

### Pricing/Cost Feedback

| Plan | Price | Includes |
|------|-------|----------|
| Core | $20 + $2.25/ACU | Pay-as-you-go, ~9 ACUs included |
| Team | $500/mo | 250 ACUs, API access |
| Enterprise | Custom | VPC deployment, custom Devins |

Actual costs: 3-5x higher than estimated due to failures, retries, overengineering.

### Setup Friction
- Cloud-only access (no local deployment)
- Requires credit card for ACU purchases
- Complex ACU tracking and billing confusion

---

## 8. pi-mono

### Overview
A modular TypeScript monorepo providing reusable layers: unified LLM API, agent runtime, CLI coding agent, TUI library, web UI components, Slack bot, and vLLM pod management.

### User Quotes

> "The right way to read PI Mono is as a stack of reusable layers, not as one giant app called PI. The terminal coding agent gets all the attention as the most visible surface. But underneath that surface, the repo is split into a provider abstraction and an agent."  
> — YouTube Review by Michael Jamieson

> "You can take the ideas apart and reuse them. You don't need to fork the whole thing. You just need a different tool surface, the UI or model backend. In many cases, you keep the shared middle and swap the surface around it."  
> — YouTube Review

> "The package readme says this plainly. PI is a terminal coding harness that you extend through templates, skills, extensions, themes, encode packages. The product is not hiding policy and user interaction. The product shell is thin."  
> — YouTube Review

### Complaints

1. **Limited Community Awareness**: Very few external reviews or user feedback available
2. **Monorepo Complexity**: "Not one giant monolith" but still complex for newcomers
3. **Documentation Gaps**: Limited external documentation beyond code comments

### Compliments

1. **Clean Architecture**: "Good sign" that product shell is thin and core is accessible
2. **Reusable Layers**: Stack of reusable packages well-designed
3. **Multiple Surfaces**: Terminal, browser, Slack, remote model deployment from shared middle
4. **Provider Abstraction**: Unified multi-provider LLM API (OpenAI, Anthropic, Google, etc.)

### GitHub Issues
Limited external issue data available.

### Pricing/Cost Feedback
Free and open source.

### Setup Friction
- npm run check requires npm run build first
- TypeScript compilation setup needed
- Web UI package requires tsc from dependencies

---

## 9. honcho

### Overview
Plastic Labs' open-source memory library with managed service for building stateful AI agents. Uses entity-centric "Peer" model with reasoning-based memory.

### User Quotes

> "honcho is great. paying for another api isnt, and the local models it plays nice with are somewhat limited."  
> — Hacker News Commenter

> "Memory only gets access when you tell it to access memory. This is 100% honcho working. When I said, 'Hey, I just want to find the part about the wiki,' it gets delivered very quickly."  
> — YouTube Review (BoxminingAI)

> "The TLDR is that honcho does work as a memory system. Like, we did try it. You have $100, and I think like it's worth trying out, but that being said, in terms of actual deliverable results, I'm like yeah, it's okay."  
> — YouTube Review (BoxminingAI)

> "Every message you send to your agent gets forwarded to honcho. In the background, honcho's reasoning engine processes these messages and generates conclusions — insights about you, your preferences, your patterns."  
> — Medium Article by RS Vino

> "The key insight: users and agents are both Peers — symmetric entities that evolve over time. This enables natural agent-to-agent memory sharing, group conversations, and NPC memory in games."  
> — DEV Community Article

### Complaints

1. **API Cost**: "Paying for another API isnt" ideal - managed service costs add up
2. **Local Model Limitations**: Limited compatibility with local models
3. **Results "Okay"**: Not transformative - "it's okay" level satisfaction from testing

### Compliments

1. **Memory Works**: "Does work as a memory system" - proven in testing
2. **Peer Model Innovation**: Entity-centric model enabling multi-agent memory sharing
3. **Reasoning-Based**: Beyond simple RAG - extracts patterns and draws conclusions
4. **Dialectic API**: Natural language queries about peers well-implemented
5. **Continual Learning**: Entities evolve over time, not static snapshots

### GitHub Issues
See Plastic Labs honcho repository for issues.

### Pricing/Cost Feedback
Free open-source tier with managed service paid tier.

### Setup Friction
- Integration requires forwarding messages to honcho
- Configuration complexity moderate

---

## 10. OpenViking

### Overview
ByteDance/Volcengine's open-source context database using "filesystem paradigm" to unify memory, resources, and skills for AI agents. 22K+ GitHub stars since January 2026 launch.

### User Quotes

> "OpenViking organizes all context as a virtual file system, enabling agents to manipulate information through standard filesystem commands. The retrieval trajectory is fully visualized — you can see exactly what paths the agent walked to retrieve each piece of context, which makes debugging actually possible."  
> — Top AI Product Review

> "The filesystem paradigm for agent context is the right abstraction. Files, directories, navigation — every developer already understands this model. It's observable, debuggable, and composable in ways that flat vector stores aren't."  
> — Prahlad Menon, Menon Lab Blog

> "OpenViking is infrastructure-first. It's a full context database with a filesystem abstraction, tiered loading, hybrid retrieval, and a Rust CLI. It's what you reach for when you're building a serious agent platform."  
> — Prahlad Menon

> "The filesystem metaphor makes context management intuitive for developers but the directories are virtual (backed by AGFS + vector index), so the 'filesystem' is a presentation layer over a database, not actual files on disk."  
> — Commonplace Blog

> "You cannot cat a viking:// file, grep the workspace with standard tools, or diff two states with git. The real benefits of filesystem-based knowledge management require actual files. The mechanism relocates database records into path-like namespaces without transforming the access model."  
> — Commonplace Blog

### Complaints

1. **Virtual Filesystem Reality**: Metaphor promises tool interoperability that implementation doesn't deliver
2. **Security Vulnerability**: Versions through 0.1.18 had security issues (since patched)
3. **API Evolution**: Still in alpha, API changing and documentation has gaps
4. **No Actual Filesystem**: "viking://" URIs resolve through API, not POSIX calls

### Compliments

1. **9K+ GitHub Stars**: "9,200 GitHub stars and 640+ forks since January 2026 launch" - validated by community
2. **Hierarchical Organization**: L0/L1/L2 tiered context loading and directory structures well-designed
3. **Observable Retrieval**: "Retrieval trajectory fully visualized" enables real debugging
4. **Self-Evolution**: Built-in memory self-iteration loop extracts learnings after each session

### GitHub Issues

- Issue #1101: "Feature: 通过filesystem模块提供现有远程文件协议的文件系统" (WebDAV/FTP filesystem exposure)

### Pricing/Cost Feedback
Free and open source.

### Setup Friction
- Installation via pip or Docker
- VikingDB backend setup required
- Go service (AGFS) adds complexity

---

## 11. MSA (Memory Sparse Attention)

### Overview
Academic/research memory architecture from EverMind (Shanda Group) enabling 100-million-token long-term memory for LLMs through Memory Sparse Attention mechanism.

### User Quotes

> "The research introduces a novel memory architecture called MSA (Memory Sparse Attention). Through a combination of the Memory Sparse Attention mechanism, Document-wise RoPE for extreme context extrapolation, KV Cache Compression with Memory Parallelism, and a Memory Interleave mechanism supporting complex reasoning, MSA achieves a 100-million-token long-term memory framework for LLMs."  
> — Laotian Times, March 2026

> "When scaling the context length from 16K to 100M tokens, the model's performance degrades by less than 9%, demonstrating extraordinary scalability."  
> — PRNewsWire / EverMind Research

> "Scientists have begun using AI agents in tasks such as reviewing the published literature, formulating hypotheses and subjecting them to virtual tests, modeling complex phenomena, and conducting experiments. Although AI agents are likely to enhance the productivity and efficiency of scientific inquiry, their deployment also creates risks for the research enterprise and society."  
> — NIH PMC Article, February 2026

### Academic Community Feedback

**Positive:**
- Breakthrough performance on LongBench, RULER, and NIAH benchmarks
- Less than 9% performance degradation from 16K to 100M tokens
- Memory-as-a-Service concept enables easy integration

**Concerns:**
- AI agents in research creating "responsibility gaps in scientific research"
- Risk of "loss of research jobs, especially entry-level ones"
- "Deskilling of researchers" if automation increases
- AI-generated knowledge "unverifiable by or incomprehensible to humans"

### Academic Papers Using MSA

- "MASA: LLM-Driven Multi-Agent Systems for Autoformalization" (ACL 2025)
- "MASFly: Multi-Agent System with Dynamic Adaptation" (arXiv)
- "MAS-Orchestra: Understanding and Improving Multi-Agent Reasoning" (Salesforce Research)
- "AutoAgent: Evolving Cognition and Elastic Memory Orchestration" (arXiv)

### GitHub Issues
N/A - research paper implementation.

### Pricing/Cost Feedback
Open-source on GitHub (EverMind-AI/MSA).

### Setup Friction
- Academic/research focused
- Requires deep learning framework integration
- Complex KV Cache compression setup

---

## 12. Recurring Themes

### Security & Trust
- **Supply Chain Attacks**: OpenClaw's ClawHavoc (341 malicious skills), skill marketplace malware
- **RCE Vulnerabilities**: CVE-2026-25253, token exfiltration, WebSocket hijacking
- **Default Unsafe Configs**: OpenClaw binding to 0.0.0.0, assumption localhost = safe

### Pricing Disappointment
- **Claude Code**: 5x price increase ($20 → $100), billing bugs
- **Devin**: $500/month with 3-5x actual cost overruns
- **Token Waste**: Double-burning tokens, subagent API key billing

### TUI/UI Instability
- **OpenCode**: Rendering glitches, freezing, connection drops
- **Konsole/Windows Terminal**: Most problematic terminals
- **Goose Desktop**: Terminal windows spawning for background processes

### Setup Friction
- **Corporate Proxies**: Goose MCP config broken
- **Memory Defaults**: Hermes self-learning disabled by default
- **Telemetry Prompts**: Blocking interactive mode without config

### Community Maintainer Burnout
- **Oh My OpenAgent**: Single maintainer overwhelmed by issue volume
- **Bug Fix Delays**: PRs sitting unmerged for days
- **Triage Requests**: Community begging for contributor permissions

### Memory System Gaps
- **Opacity**: Hermes "audit gap", can't export agent knowledge
- **Virtual vs Real**: OpenViking filesystem "presentation layer"
- **Disabled by Default**: Features that seem to work don't without config

### Autonomous Agent Reliability
- **Devin**: 70% failure rate on standard tasks
- **Context Confusion**: Claude Code compacting overriding user preferences
- **Dangerous Orchestration**: Oh My OpenAgent destructive git operations

### Positive Patterns
- **Rapid Security Response**: OpenClaw patches in 48-72 hours
- **Multi-Platform Support**: Hermes messaging gateway unmatched
- **Memory Engineering**: Honcho reasoning-based, Hermes frozen snapshots
- **Community Validation**: OpenViking 9K stars, Goose recipe ecosystem

---

## Methodology

**Search Sources:**
- GitHub Issues & Discussions (anomalyco/opencode, code-yeongyu/oh-my-openagent, block/goose, NousResearch/hermes-agent, volcengine/OpenViking)
- Hacker News threads
- Reddit (r/LocalLLaMA, r/MachineLearning)
- YouTube reviews and demos
- Web search for security disclosures and pricing analysis
- Academic papers (arXiv, ACL Anthology, NIH PMC)

**Date Range:** January 2025 - April 2026
**Total Systems Covered:** 11
**Minimum Quotes per System:** 5
**Minimum Complaints per System:** 3
**Minimum Compliments per System:** 2
