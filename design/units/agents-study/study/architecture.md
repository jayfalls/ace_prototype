# Architecture.md — Agent System Topology & Design Philosophy

**Unit:** agents-study  
**Slice:** 1 of 14  
**Date:** 2026-04-23  
**Status:** Complete

---

## 1. Introduction

This document examines the system topologies, deployment models, runtime boundaries, and surface separation patterns across 8 agent systems (+ Devin as a proprietary reference). The goal is to extract cross-cutting architectural lessons that inform ACE's cognitive unit design.

---

## 2. OpenClaw

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                     OpenClaw Gateway (Daemon)                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Session    │  │   Channel    │  │   Plugin             │  │
│  │   Store      │  │   Registry   │  │   Loader             │  │
│  │   (SQLite)   │  │  (24+ chan) │  │   (Manifest-based)   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│         │                 │                      │               │
│  ┌──────┴────────────────┴──────────────────────┴───────────┐  │
│  │                    Session Router                         │  │
│  │  session_key → agent_id → workspace                      │  │
│  └──────────────────────────┬───────────────────────────────┘  │
│                             │                                   │
│  ┌──────────────────────────┴───────────────────────────────┐  │
│  │              Agent Engine (per-session)                  │  │
│  │  ReAct loop │ Tool execution │ Memory │ Compaction        │  │
│  └─────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
        ▲                ▲                ▲                ▲
   [CLI/stdin]     [ACP Bridge]     [macOS App]      [Channel Adapters]
                                                ┌────────┬────────┬────────┐
                                          [Telegram] [Discord] [Slack] ... (24+)
```

### Key Architectural Details

**Gateway as Central Control Plane:** The Gateway (`src/gateway/server.impl.ts`) is a long-running daemon process. All sessions, channels, tools, and events route through it. The Gateway owns session state, authentication, and the agent lifecycle.

**Session Key Model:** Sessions are identified by `session_key` strings (e.g., `agent:main:main`, `acp:<uuid>`). The key encodes agent identity and workspace scope. Multiple sessions can exist per agent; sessions are the unit of isolation and persistence.

**Channel Adapter Pattern:** `src/channels/` implements a registry-based adapter system. Each channel (Telegram, Discord, Slack, WhatsApp, iMessage, etc.) normalizes inbound messages into a canonical session format. Channels are plugin-owned via `extensions/<name>/`, but core channel logic lives in `src/channels/`.

**Plugin Architecture:** Three-layer plugin system:
- **Bundled plugins** (`extensions/`): First-party channel providers (Discord, Slack, Telegram, etc.)
- **Provider plugins** (`extensions/<provider>/`): LLM provider integrations
- **Third-party plugins**: Via public SDK (`src/plugin-sdk/*`)

Core must stay extension-agnostic; extensions cross into core only via the plugin SDK, manifest metadata, and injected runtime helpers. Extension code must not import core `src/**`.

**ACP Bridge:** `openclaw acp` exposes an Agent Client Protocol endpoint over stdio, bridging IDE clients (Zed, VS Code) to the Gateway over WebSocket. Session keys map ACP sessions to Gateway sessions.

**Sandboxing:** Main session runs tools on the host with full access. Non-main sessions run in Docker containers by default (SSH and OpenShell backends also available). Sandboxing is configurable per session.

**Deployment:** Local daemon via launchd (macOS) or systemd (Linux), Docker support, Fly.io cloud deployment, Podman alternative.

---

## 3. Claude Code

### Topology

```
┌──────────────────────────────────────────────────────────────────┐
│                     Claude Code (TypeScript Monorepo)              │
│                        ~163K lines of TypeScript                   │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │  src/main.tsx (4,683 lines) — CLI entry + REPL bootstrap    │ │
│  └─────────────────────────────┬────────────────────────────────┘ │
│                                │                                   │
│  ┌─────────────────────────────┴────────────────────────────────┐ │
│  │  src/query.ts (685KB) — Core AsyncGenerator agent loop       │ │
│  │  src/QueryEngine.ts — SDK/Headless query lifecycle engine    │ │
│  └─────────────────────────────┬────────────────────────────────┘ │
│                                │                                   │
│  ┌──────────────┬──────────────┬───────────────┬────────────────┐ │
│  │   services/   │    tools/    │  commands/    │   components/  │ │
│  │   (22 dirs)   │  (44 dirs)  │   (~87 cmds)  │  (React/Ink)  │ │
│  └──────────────┴──────────────┴───────────────┴────────────────┘ │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │  Multi-Agent Coordination                                  │   │
│  │  coordinator/  tasks/  plugins/  memdir/  remote/        │   │
│  └───────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Monolithic Single File:** The core agent loop lives in `query.ts` at 685KB — an unusually large single file. The AsyncGenerator pattern (`src/query.ts`) streams output while simultaneously executing tools, enabling speculative execution of read-only tools during streaming.

**SDK / Headless Separation:** `QueryEngine.ts` abstracts the query lifecycle as a reusable SDK surface. This enables headless operation separate from the CLI TUI. Claude Code can operate as a library.

**Tool System:** 40+ tools organized in `src/tools/` (44 subdirectories) with `Tool.ts` defining the `buildTool` factory. Tools emit typed events; the system tracks tool call state, arguments, and results.

**Permission Pipeline:** A 14-step permission pipeline classifies tools by safety level before the model sees them. `plan` mode restricts tool visibility.

**Multi-Agent:** `coordinator/` and `tasks/` packages suggest multi-agent coordination primitives. The AsyncGenerator loop handles concurrent tool execution partitioned by safety classification.

**Memory System:** 7-layer memory architecture (per public reporting on the leak). `memdir/` handles long-term memory. File-based memory (`MEMORY.md`) with LLM-powered recall.

**Plugin System:** `plugins/` directory for extensibility. `remote/` for remote mode operation.

---

## 4. Open Code

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                     Open Code (Bun/TypeScript)                   │
│                                                                  │
│  packages/                                                       │
│  ├── app/           — Application core                           │
│  ├── console/       — Console/TUI components                     │
│  ├── desktop/       — Desktop-specific code                       │
│  ├── desktop-electron/ — Electron wrapper                         │
│  ├── opencode/      — Core agent logic                           │
│  ├── plugin/        — Plugin system                              │
│  ├── sdk/           — SDK (js/, python/)                         │
│  │   └── js/script/build.ts — JS SDK generation                  │
│  ├── ui/            — UI components                               │
│  └── web/           — Web interface                              │
│                                                                  │
│  session.json — Session state persistence (JSON)                  │
│  sst.config.ts — SST framework configuration                    │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Multi-Package Monorepo:** Distinct surface-specific packages under `packages/`. Desktop, console, SDK, and web are separate deployment targets sharing core logic.

**Client/Server Model:** Open Code supports remote operation — the agent can run on a server while the UI connects remotely. WebSocket-based communication between client and server.

**SDK Architecture:** `packages/sdk/` provides client libraries for JavaScript and Python. The JS SDK is generated via `./packages/sdk/js/script/build.ts`.

**Session Persistence:** `session.json` at the root level stores session state. Simple JSON-based persistence.

**Plugin System:** `packages/plugin/` implements the plugin architecture, enabling extensions to the core agent.

**Desktop App:** Electron-based desktop application alongside CLI. Downloadable DMG/EXE/AppImage for end-user deployment.

**Deployment:** Local-first with remote access capability. Single-binary install via `curl | bash` or package managers.

---

## 5. Oh My OpenAgent

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│              Oh My OpenAgent (OpenCode Plugin)                    │
│           1,766 TypeScript files, 377K LOC, Bun-only            │
│                                                                  │
│  Entry: src/index.ts → 5-step init                               │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  loadConfig → createManagers → createTools →                │ │
│  │  createHooks → createPluginInterface                        │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │  THREE-LAYER ARCHITECTURE                                  │   │
│  │                                                            │   │
│  │  ┌──────────┐   Intent Gate (classifies: research /        │   │
│  │  │  Intent  │   implementation / investigation /           │   │
│  │  │   Gate   │   evaluation / fix)                          │   │
│  │  └────┬─────┘                                              │   │
│  │       │                                                     │   │
│  │  ┌────┴─────┐   Prometheus (planning, 11 built-in agents)  │   │
│  │  │Prometheus│   Sisyphus, Hephaestus, Oracle, Librarian,   │   │
│  │  └────┬─────┘   Explore, Atlas, Prometheus, Metis, Momus,   │   │
│  │       │        Multimodal-Looker, Sisyphus-Junior            │   │
│  │       │                                                     │   │
│  │  ┌────┴─────┐   Atlas (execution) + Workers                 │   │
│  │  │  Atlas   │   Background agents run in parallel           │   │
│  │  └──────────┘                                              │   │
│  └───────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐   │
│  │ 52 Lifecycle  │  │  26 Tools     │  │ 3-Tier MCP System │   │
│  │ Hooks         │  │ (ToolRegistry │  │ (built-in +       │   │
│  │ Core(43)+     │  │ factory)      │  │  .mcp.json +      │   │
│  │ Continuation(7)│  │              │  │  skill-embedded)   │   │
│  │ +Skill(2)     │  │              │  │                   │   │
│  └───────────────┘  └───────────────┘  └───────────────────┘   │
│                                                                  │
│  Config: Project → User → Defaults (JSONC, Zod v4)              │
│  11 platform binaries: darwin/linux/windows × AVX2+baseline      │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**OpenCode Plugin:** This is not a standalone agent — it extends OpenCode (and by extension Claude Code compatibility) with 11 additional agents, 52 lifecycle hooks, and a 3-tier MCP system. The plugin requires Claude Code to function.

**Intent Gate Pattern:** The Intent Gate classifies incoming user intent before routing to the appropriate agent. This pre-classification enables specialized agent selection and runtime configuration.

**Three-Layer Architecture:**
1. **Intent Gate** — Classifies request type (research, implementation, investigation, evaluation, fix)
2. **Prometheus** — Orchestration layer managing 11 specialized agents
3. **Atlas + Workers** — Execution layer distributing tasks to specialized agents

**Model Routing:** Multi-model orchestration across Claude, Kimi, GLM (orchestration), GPT (reasoning), Minimax (speed), Gemini (creativity). 4-step model resolution: override → category-default → provider-fallback → system-default.

**Hook System:** 52 lifecycle hooks across three tiers (Core, Continuation, Skill). Hooks intercept at session, tool-guard, transform, continuation, and skill phases. Enables deep integration with Claude Code's execution pipeline.

**Three-Tier MCP:**
- Tier 1: Built-in remote MCPs (websearch via Exa/Tavily, context7, grep_app)
- Tier 2: Claude Code's `.mcp.json` with env var expansion
- Tier 3: Skill-embedded MCPs managed by `SkillMcpManager` (per-session, stdio + HTTP)

**Multi-Level Config:** Project-level `.opencode/oh-my-opencode.jsonc` overrides user-level `~/.config/opencode/oh-my-opencode.jsonc` which overrides defaults. Deep merge for `agents`, `categories`, `claude_code`; Set union for `disabled_*` arrays; override for all other fields.

---

## 6. Devin

### Topology

```
┌──────────────────────────────────────────────────────────────────┐
│                     Devin (Proprietary, Cloud-Native)             │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                     COGNITION TENANT                        │  │
│  │                    (Azure-hosted)                           │  │
│  │                                                              │  │
│  │  ┌────────────────┐    ┌─────────────────────────────────┐ │  │
│  │  │  Devin's Brain │    │  Devin's Brain Container         │ │  │
│  │  │  (Intelligence)│◄───│  (isolated, per-session)        │ │  │
│  │  │                │    │  - Secrets decrypted at start    │ │  │
│  │  │  - Context     │    │  - Loaded as environment vars   │ │  │
│  │  │    processing  │    │  - Re-encrypted after session    │ │  │
│  │  │  - Action      │    └─────────────────────────────────┘ │  │
│  │  │    decisions   │                                         │  │
│  │  └────────────────┘                                         │  │
│  └────────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                    WebSocket (wss://)                            │
│                              │                                   │
│  ┌──────────────────────────┴────────────────────────────────┐  │
│  │              Devin's DevBox (VM, per-session)               │  │
│  │                                                              │  │
│  │  ┌─────────────────────────────────────────────────────┐   │  │
│  │  │  Isolated VM per session                             │   │  │
│  │  │  - Shell, editor, browser capabilities               │   │  │
│  │  │  - VSCode server                                    │   │  │
│  │  │  - VNC server for browser control                    │   │  │
│  │  │  - git, python, docker, java                         │   │  │
│  │  └─────────────────────────────────────────────────────┘   │  │
│  │                                                              │  │
│  │  Customer VPC (Enterprise SaaS) or Cognition Cloud          │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                     MultiDevin Architecture                       │
│                                                                  │
│        ┌──────────────────┐                                      │
│        │  Manager Devin    │ ← Coordinates, scopes work,         │
│        │  (coordinator)    │   monitors progress, resolves        │
│        └────────┬─────────┘   conflicts, compiles results         │
│                 │                                                   │
│    ┌────────────┼────────────┐                                    │
│    ▼            ▼            ▼        ... up to 10                │
│ ┌──────┐   ┌──────┐    ┌──────┐                                  │
│ │Worker│   │Worker│    │Worker│   Each in own VM, own shell,    │
│ │Devin │   │Devin │    │Devin │   own test runner, own session  │
│ └──┬───┘   └──┬───┘    └──┬───┘   link. Independent execution.  │
│    └──────────┴───────────┘                                      │
└──────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Brain/DevBox Separation:** Devin's architecture splits intelligence ("Brain", in Cognition's Azure tenant) from execution ("DevBox", in customer VPC or Cognition Cloud). The Brain processes context snippets to determine every action; the DevBox executes code in an isolated VM.

**Stateless Brain:** The Brain is stateless per-session — isolated containers are created per session, authorized to customer data stores. Secrets are decrypted at session start, loaded as environment variables, then re-encrypted.

**WebSocket Communication:** On DevBox startup, a WebSocket opens and connects to an isolated Brain container. All subsequent exchanges happen over this connection. Only outbound HTTPS/443 is required from the customer VPC.

**MultiDevin (Manager/Worker):** Up to 10 managed Devins per coordinator. Each managed Devin is a full Devin running in its own isolated VM with its own terminal, browser, and development environment. The manager scopes work, assigns tasks, monitors progress, resolves conflicts, and merges results.

**Session Model:** Each Devin session requires a new VM. VM snapshots enable fast session startup from pre-configured states.

**Enterprise Deployment Modes:**
- **Enterprise SaaS:** Both Brain and DevBox in Cognition's multi-tenant cloud
- **Customer Dedicated SaaS:** Brain in Cognition Cloud, DevBox in customer-dedicated single-tenant VPC via AWS Private Link or IPSec tunnel
- Stateless system guarantee: no data at rest outside customer environment

**gRPC/WebSocket Protocol:** The Devin AI agent client (desktop/CLI) communicates via gRPC/WebSockets for bidirectional file sync (<50ms latency), stream broadcasting (terminal, editor, browser), local port forwarding, and session management.

---

## 7. Goose

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                     Goose (Rust, AAIF/Linux Foundation)          │
│                                                                  │
│  crates/                                                         │
│  ├── goose              — Core agent logic                      │
│  ├── goose-acp-macros   — ACP proc macros                       │
│  ├── goose-cli          — CLI entry (goose CLI binary)          │
│  ├── goose-server      — Server binary (goosed)                 │
│  ├── goose-mcp          — MCP extensions                        │
│  ├── goose-test         — Test utilities                         │
│  └── goose-test-support — Test helpers                           │
│                                                                  │
│  ui/desktop/           — Electron desktop app                    │
│  services/              — Backend services                        │
│                                                                  │
│  Surface Separation:                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────┐  │
│  │  CLI     │   │ Desktop  │   │  API     │   │ Extensions │  │
│  │(goose-cli)│   │(Electron)│   │(goosed)  │   │(goose-mcp)│  │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘   └─────┬──────┘  │
│       │              │              │                │          │
│  All surfaces share crates/goose core                           │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Rust-Based:** Goose is implemented in Rust with a clean crate separation between core logic and surface-specific entry points.

**Multi-Surface Architecture:**
- **CLI** (`goose-cli`): Terminal-first interface
- **Desktop** (`ui/desktop/`, Electron): Native desktop app
- **API** (`goose-server`): Headless API server (`goosed` binary)
- **Extensions** (`goose-mcp`): MCP integration layer

All surfaces share `crates/goose/` — the core agent logic is surface-agnostic.

**Extension Loading:** MCP (Model Context Protocol) is the extension mechanism. `goose-mcp` provides the MCP client integration. Extensions are loaded dynamically.

**Provider Abstraction:** `providers/base.rs` defines a `Provider` trait. 15+ providers supported (Anthropic, OpenAI, Google, Ollama, OpenRouter, Azure, Bedrock, etc.).

**Recipe System:** `goose run --recipe` enables scripted agent workflows. Self-test via `goose-self-test.yaml`.

**ACP Protocol:** Agent Client Protocol support via `goose-acp-macros`. Enables integration with ACP-compatible IDEs and clients.

**Deployment:** Cross-platform binaries (macOS, Linux, Windows). Docker and cloud deployment guides. Moved to Linux Foundation's Agentic AI Foundation (AAIF).

---

## 8. Hermes Agent

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                 Hermes Agent (Python, Nous Research)              │
│                    ~497K lines, 3,000+ tests                     │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  run_agent.py — AIAgent class, core conversation loop     │  │
│  │                                                             │  │
│  │  while api_call_count < max_iterations:                     │  │
│  │      response = client.chat.completions.create(...)         │  │
│  │      for tool_call in response.tool_calls:                  │  │
│  │          result = handle_function_call(...)                 │  │
│  └─────────────────────────┬──────────────────────────────────┘  │
│                            │                                      │
│  ┌─────────────────────────┴──────────────────────────────────┐ │
│  │  model_tools.py — Tool orchestration, discover_builtin_    │ │
│  │                   tools(), handle_function_call()           │ │
│  └─────────────────────────┬──────────────────────────────────┘ │
│                            │                                      │
│  ┌─────────────────────────┴──────────────────────────────────┐ │
│  │  tools/registry.py — Central tool registry                  │ │
│  │  (auto-discovers all tools/* files at import time)         │ │
│  └─────────────────────────┬──────────────────────────────────┘ │
│                            │                                      │
│  ┌─────────────┬──────────┴──────────┬─────────────────────┐  │
│  │ tools/*.py  │  agent/             │  gateway/           │  │
│  │ (47 tools,  │  ├── prompt_builder │  ├── run.py         │  │
│  │  19 toolsets│  ├── context_compr.  │  │   (main loop)     │  │
│  │              │  ├── prompt_caching │  ├── session.py     │  │
│  │              │  ├── auxiliary_client│  │   (SQLite+FTS5)  │  │
│  │              │  └── display.py     │  └── platforms/     │  │
│  │              │                     │      (15+ adapters) │  │
│  └──────────────┴─────────────────────┴─────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  TUI Architecture (hermes --tui)                                  │
│                                                                  │
│  ┌───────────────────┐      stdio JSON-RPC      ┌─────────────┐ │
│  │  Node (Ink/React)  │◄────────────────────────►│Python TTY   │ │
│  │  ui-tui/src/      │                          │Gateway     │ │
│  │  - Renders screen │      JSON-RPC            │tui_gateway/│ │
│  │  - User input     │                          │             │ │
│  │  - Streaming      │                          │ (session,   │ │
│  └───────────────────┘                          │  tools,     │ │
│                                                   │  model)     │ │
│                                                   └─────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Python Monolith:** Single `run_agent.py` contains the `AIAgent` class with the core synchronous conversation loop. `model_tools.py` handles tool orchestration. `tools/registry.py` provides the central registry with auto-discovery.

**Tool Auto-Discovery:** Any `tools/*.py` file with a top-level `registry.register()` call is imported automatically. No manual import list. Each tool file calls `registry.register()` at import time.

**Gateway Daemon:** `gateway/run.py` is the messaging platform gateway. It routes messages to/from 15+ platform adapters (Telegram, Discord, Slack, WhatsApp, Signal, Home Assistant, QQ, etc.). The gateway is a separate process from the CLI agent.

**TUI Separation:** When run with `hermes --tui`, the terminal UI is a TypeScript Ink/React application communicating with a Python JSON-RPC backend over stdio. TypeScript owns rendering; Python owns sessions, tools, and model calls.

**ACP Adapter:** `acp_adapter/` provides Agent Client Protocol server for VS Code, Zed, and JetBrains integration.

**Terminal Backends:** Six backend options for tool execution: local, Docker, SSH, Daytona, Singularity, Modal. Daytona and Modal offer serverless persistence (hibernation/wake on demand).

**Memory Architecture:** SQLite with FTS5 for session search. Markdown-based persistent memory. Honcho integration for entity-centric user modeling.

**Delegation:** `delegate_tool.py` spawns subagents. Max depth of 2. Child agents get isolated context with restricted toolsets.

**Batch Processing:** `batch_runner.py` for parallel batch processing. `trajectory_compressor.py` for RL training data.

**Skin/Theme Engine:** Data-driven CLI visual customization. Skins are YAML files in `~/.hermes/skins/`. Built-in skins: default (gold/kawaii), ares (crimson), mono (grayscale), slate (blue).

---

## 9. pi-mono

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                    pi-mono (TypeScript Monorepo)                 │
│                                                                  │
│  packages/                                                       │
│  ├── ai/              — Multi-provider LLM API                   │
│  │   └── src/providers/  (OpenAI, Anthropic, Google, etc.)       │
│  │                                                             │  │
│  ├── agent/           — pi-agent-core (surface-agnostic runtime) │  │
│  │   ├── agent-loop.ts  (turn loop)                             │  │
│  │   ├── agent.ts      (16.5KB)                                │  │
│  │   ├── types.ts      (13.8KB)                               │  │
│  │   ├── proxy.ts      (10.4KB)                                │  │
│  │   └── index.ts                                           │  │
│  │                                                             │  │
│  ├── coding-agent/    — CLI interface (pi command)               │  │
│  │   └── src/cli.ts                                           │  │
│  │                                                             │  │
│  ├── tui/             — Terminal UI library (differential        │  │
│  │                       rendering)                             │  │
│  │                                                             │  │
│  ├── web-ui/          — Web chat components                     │  │
│  │                                                             │  │
│  ├── mom/              — Slack bot (delegates to coding-agent) │  │
│  │                                                             │  │
│  └── pods/             — CLI for vLLM deployments on GPU pods   │  │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Surface-Agnostic Core:** `pi-agent-core` (`packages/agent/`) is the runtime engine. It knows nothing about how it's invoked — CLI, TUI, web, or Slack are all separate packages that import and use the core.

**Agent Loop:** `agent-loop.ts` implements the turn loop: context shaping → LLM call → tool execution → continuation decision. `agent.ts` (16.5KB) defines types and core agent logic.

**Provider Abstraction:** `pi-ai` provides a unified multi-provider LLM API. Adding a new provider requires changes in `packages/ai/src/providers/` plus registration in `packages/ai/src/providers/register-builtins.ts`.

**Surface Packages:** Each surface is a separate npm package:
- `pi-coding-agent`: Interactive CLI (`pi` command)
- `pi-tui`: Terminal UI library with differential rendering
- `pi-web-ui`: Web components for chat interfaces
- `pi-mom`: Slack bot

**Minimal Footprint:** Designed to run on low-cost infrastructure ($5 VPS). Minimal dependencies. Simple configuration.

**Session Sharing:** The mom Slack bot delegates to the same `pi-agent-core` used by the CLI. Same agent, different interface.

---

## 10. autoresearch (karpathy)

### Topology

```
┌─────────────────────────────────────────────────────────────────┐
│           autoresearch (Karpathy, Minimal Research Loop)         │
│                                                                  │
│  3 files only:                                                  │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  prepare.py     │  │   train.py      │  │  program.md     │  │
│  │  (fixed)        │  │   (agent edits) │  │  (human edits)  │  │
│  │                 │  │                 │  │                 │  │
│  │  - Data prep    │  │  - GPT model    │  │  - Agent        │  │
│  │  - Tokenizer    │  │  - Optimizer    │  │    instructions │  │
│  │  - Dataloader   │  │  - Training     │  │  - Loop rules   │  │
│  │  - Evaluation   │  │    loop         │  │  - Output       │  │
│  │  - Constants    │  │                 │  │    format       │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
│  Experiment Loop:                                                 │
│  1. git checkout -b autoresearch/<tag>                         │
│  2. Edit train.py                                               │
│  3. git commit                                                  │
│  4. uv run train.py > run.log 2>&1  (5-minute fixed budget)    │
│  5. grep "^val_bpb:" run.log                                    │
│  6. Record in results.tsv                                       │
│  7. If improved: keep commit; else: git reset                   │
│  8. Repeat indefinitely (NEVER STOP)                            │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Details

**Minimal Scope:** Only 3 files. The agent only touches `train.py`. `program.md` provides agent instructions. `prepare.py` is read-only and contains all fixed constants.

**Fixed Time Budget:** Every training run takes exactly 5 minutes (wall clock, excluding startup). This makes experiments comparable regardless of what the agent changes (model size, batch size, architecture). ~12 experiments/hour.

**Single-File Model:** The entire model (GPT with Muon + AdamW optimizer, training loop) lives in `train.py`. ~630 lines total. Agent edits this file exclusively.

**Metric:** `val_bpb` (validation bits per byte) — lower is better, vocab-size-independent for fair architectural comparison.

**No Distributed Training:** Single GPU only. No external dependencies beyond PyTorch and a few packages.

**Git-Based Versioning:** Each experiment branch. Results in `results.tsv` (untracked by git). Git commit per experiment enables rollback.

**"NEVER STOP" Principle:** Once the loop starts, the agent runs indefinitely without asking. Human provides setup; agent runs overnight if needed.

---

## 11. Cross-Cutting Analysis

### System Topology Summary

| System | Core Location | Runtime Model | Process Boundary |
|--------|--------------|---------------|------------------|
| OpenClaw | `src/gateway/` | Long-running daemon | Gateway = single process; ACP bridge = child |
| Claude Code | `src/query.ts` | AsyncGenerator loop | Single process (TypeScript) |
| Open Code | `packages/opencode/` | Client/server | TUI ↔ Agent server (WebSocket) |
| Oh My OpenAgent | `src/index.ts` | Plugin on Claude Code | Runs within Claude Code process |
| Devin | Brain + DevBox | Cloud VM | Brain in Cognition tenant; DevBox in VPC/Cloud |
| Goose | `crates/goose/` | Multi-binary | CLI/Desktop/API share core |
| Hermes | `run_agent.py` | Synchronous loop | Gateway = separate process; TUI over stdio |
| pi-mono | `packages/agent/` | Turn loop in `agent-loop.ts` | Surfaces import core as library |
| autoresearch | `train.py` | Single experiment loop | Single Python process |

### Deployment Models

| System | Local | Cloud | Hybrid | Notes |
|--------|-------|-------|--------|-------|
| OpenClaw | ✅ | Fly.io, Docker | ✅ | Daemon runs locally; remote access optional |
| Claude Code | ✅ | ❌ | ❌ | CLI tool; cloud use only via API keys |
| Open Code | ✅ | Optional | ✅ | Remote server mode available |
| Oh My OpenAgent | N/A | N/A | N/A | Plugin requires Claude Code host |
| Devin | ❌ | ✅ | ✅ VPC | Cloud-only by default; VPC option |
| Goose | ✅ | Docker | ✅ | Local-first; Docker for server |
| Hermes | ✅ | Modal, Daytona | ✅ | Local by default; cloud backends optional |
| pi-mono | ✅ | ❌ | ❌ | Designed for $5 VPS |
| autoresearch | ✅ | ❌ | ❌ | Single GPU workstation |

### Surface Separation Patterns

**1. Single Process, Multiple Entry Points (OpenClaw, Hermes CLI)**
- One daemon process handles all surfaces
- Channel adapters normalize inbound messages
- Session router dispatches to appropriate agent
- *Pattern: Hub-and-spoke*

**2. Multi-Process with IPC (Hermes TUI, Goose)**
- UI process + agent/gateway process
- stdio JSON-RPC or WebSocket between them
- TypeScript/Ink for rendering; Python/Rust for logic
- *Pattern: Screen scraping + RPC bridge*

**3. Surface-Agnostic Core (pi-mono, Goose)**
- Core runtime in a shared library (`pi-agent-core`, `crates/goose`)
- CLI, TUI, Web, Slack are separate packages importing core
- *Pattern: Library-first design*

**4. Cloud-Native with Thin Client (Devin)**
- Agent runs entirely in cloud VM
- Desktop/CLI client is a display + input bridge
- gRPC/WebSocket for low-latency streaming
- *Pattern: Remote display + thin client*

**5. Plugin Architecture (Oh My OpenAgent, Claude Code plugins)**
- Core host (Claude Code, OpenCode) provides execution context
- Plugin extends with agents, hooks, tools, MCPs
- Plugin lifecycle tied to host process
- *Pattern: Extension API + runtime injection*

**6. Single-File Experiment (autoresearch)**
- Minimal scope: one editable file
- Fixed infrastructure: `prepare.py` never changes
- Agent operates as researcher-in-the-loop
- *Pattern: Bounded autonomy with fixed substrate*

### Design Philosophies

| System | Philosophy | Evidence |
|--------|-----------|----------|
| OpenClaw | "Extension-agnostic core; channels are plugins" | Core must not special-case bundled plugins; plugin SDK as only seam |
| Claude Code | "AsyncGenerator enables speculative execution" | Streaming + concurrent tool execution during stream |
| Open Code | "Multi-surface via package separation" | Separate packages for app, console, desktop, SDK, web |
| Oh My OpenAgent | "Intent routing before execution" | Intent Gate → Prometheus → Atlas → Workers chain |
| Devin | "Cloud isolation + parallel VMs" | Each managed Devin in own VM; manager coordinates |
| Goose | "Rust performance + MCP as extension model" | `goose-mcp` for extensions; `crates/goose` shared |
| Hermes | "Message bus + surface adapters" | Gateway routes to 15+ platforms; `tools/registry.py` auto-discovers |
| pi-mono | "Surface-agnostic runtime" | `pi-agent-core` imported by CLI, TUI, web, Slack |
| autoresearch | "Minimal interface, maximal autonomy" | 3 files; agent only touches train.py |

---

## 12. ACE Recommendation Table

| Architectural Pattern | System Evidence | ACE Recommendation | Rationale |
|-----------------------|-----------------|-------------------|-----------|
| **Hub-and-spoke gateway** | OpenClaw, Hermes | **ADOPT** | Single control plane simplifies auth, session routing, and surface routing. ACE's NATS message bus aligns with this pattern. |
| **Surface-agnostic core** | pi-mono (`pi-agent-core`), Goose (`crates/goose`) | **ADOPT** | Core runtime should know nothing about how it's invoked. ACE cognitive units should be callable from CLI, TUI, API without code changes. |
| **Auto-discovery tool registry** | Hermes (`tools/registry.py`) | **ADOPT** | Plugins/tools registered at import time without manual wiring. Reduces extension boilerplate. |
| **Intent classification before routing** | Oh My OpenAgent (Intent Gate) | **ADAPT** | Pre-classification adds latency but enables specialized execution paths. ACE should support intent-aware routing but keep it optional. |
| **Manager/worker with isolated VMs** | Devin (MultiDevin) | **ADAPT** | Clean isolation but heavy infrastructure. ACE pods should support process isolation but not full VM isolation (too expensive). |
| **AsyncGenerator streaming + concurrent execution** | Claude Code | **ADOPT** | Enables speculative tool execution during streaming. Critical for responsive UX. ACE agent loop must support concurrent tool dispatch. |
| **TUI as separate rendering process** | Hermes (`hermes --tui`), Goose | **ADOPT** | Separation of concerns: TypeScript/Ink for rendering, Python/Rust for logic. ACE should support headless operation. |
| **Plugin host + extension API** | Oh My OpenAgent, Claude Code plugins | **ADOPT** | Extensibility via well-defined API. ACE should expose a typed plugin SDK with versioned contracts. |
| **Cloud-native with thin client** | Devin | **AVOID** (for local-first) | Cloud dependency unsuitable for privacy-sensitive workloads. ACE should prioritize local execution; cloud is opt-in. |
| **Minimal scope experiment loop** | autoresearch | **ADAPT** | Fixed-time budget + single-file modification is powerful for focused autonomy. ACE should support bounded experiment modes for self-improvement. |
| **Multi-level config (project → user → defaults)** | Oh My OpenAgent | **ADOPT** | Supports both project-local and user-wide configuration. ACE should merge config hierarchically with project taking precedence. |
| **Fixed substrate + bounded agent edits** | autoresearch | **ADOPT** for self-improvement | Agent modifies `train.py`; `prepare.py` is sacred. ACE should support "fixed substrate" modes where certain files are protected from modification. |
| **Channel adapter normalization** | OpenClaw (24+ channels) | **ADOPT** | Unified session model across heterogeneous messaging platforms. ACE NATS subjects should normalize surface differences. |
| **MCP as extension mechanism** | Goose, Hermes, Oh My OpenAgent | **ADOPT** | MCP is the emerging standard. ACE should implement MCP server + client. |
| **Stateless brain, stateful devbox** | Devin | **ADOPT** for cloud | Brain statelessness enables horizontal scaling and session isolation. ACE should decouple inference (stateless) from execution state (sessionful). |
| **Lazy-loaded plugin runtime** | OpenClaw | **ADOPT** | Avoid materializing plugin runtime for static descriptor queries. ACE should lazy-load heavy extensions. |

---

## 13. Key Findings

### Convergence Points

1. **Surface/Core Separation:** Multiple systems converge on the pattern of a surface-agnostic core with multiple entry points (CLI, TUI, API, messaging). pi-mono and Goose are the cleanest examples.

2. **Tool Auto-Discovery:** Hermes's `registry.register()` at import time and OpenClaw's manifest-first plugin system both solve the same problem: reduce wiring boilerplate for extensions.

3. **Async/Streaming UX:** Claude Code's AsyncGenerator pattern enables tools to execute during streaming output. This pattern is essential for responsive agent UX.

4. **MCP as Standard:** Goose, Hermes, and Oh My OpenAgent all implement MCP. This convergence suggests MCP is becoming the de facto extension standard.

5. **Intent-Aware Routing:** Oh My OpenAgent's Intent Gate and Devin's manager/worker topology both route based on task classification. Specialized routing improves execution quality.

### Divergence Points

1. **Local vs. Cloud:** Devin is purely cloud; pi-mono and autoresearch are purely local. OpenClaw and Hermes support both. ACE should prioritize local-first with optional cloud.

2. **Monolith vs. Modular:** Claude Code's 685KB `query.ts` vs. Hermes's modular `tools/*.py` files. Both work; modular is easier to maintain.

3. **Process Isolation Level:** OpenClaw uses Docker containers for sandboxing; Devin uses full VMs; pi-mono relies on process boundaries. ACE should use process isolation as the baseline, containers as the option.

4. **Plugin Model:** OpenClaw uses a manifest-first control plane; Oh My OpenAgent uses hook composition. Both are valid extension patterns for different host architectures.

---

*Document Complete. Next: `study/memory.md` — Memory Systems*
