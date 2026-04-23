# Slice 11: Developer Experience & User Interfaces (UX/DX)

**Unit:** agents-study  
**Type:** Cross-cutting comparison (Slice 11/14)  
**Output:** `design/units/agents-study/study/ux-dx.md`

---

## 1. Setup Friction & Onboarding

### OpenClaw

**Setup Process:** `openclaw onboard` interactive wizard. Step-by-step guide through gateway, workspace, channels, and skills. Recommended CLI setup path.

```bash
npm install -g openclaw@latest
openclaw onboard --install-daemon
```

- **Node 22+ required** (or Node 22.16+)
- Supports npm, pnpm, or bun
- Desktop apps are optional (macOS menu bar, iOS/Android companion apps)
- First-run setup writes local config/workspace via `pnpm openclaw setup`

**Configuration Model:** JSON5 (`~/.openclaw/openclaw.json`) with minimal defaults:

```json5
{
  agent: { model: "<provider>/<model-id>" }
}
```

Full reference docs at `https://docs.openclaw.ai/gateway/configuration`.

**Workspace Files:** `AGENTS.md`, `SOUL.md`, `TOOLS.md` injected as prompt files. Skills at `~/.openclaw/workspace/skills/<skill>/SKILL.md`.

**Doctor Command:** `openclaw doctor` surfaces risky/misconfigured DM policies and config issues.

**From Source:** `pnpm install` → `pnpm openclaw setup` → `pnpm gateway:watch` for dev loop.

---

### Goose

**Setup Process:** Download desktop app OR install CLI via install script:

```bash
curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh | bash
```

**Configuration:** `~/.config/goose/` directory with profiles support.

**Multi-Provider:** 15+ providers — Anthropic, OpenAI, Google, Ollama, OpenRouter, Azure, Bedrock, and more. Connect via API keys or existing subscriptions through ACP.

**Extension Model:** MCP (Model Context Protocol) for 70+ extensions.

**Surfaces:** Desktop app (Electron), full CLI, API via goose-server (`goosed` binary).

**Development:** Rust project. Build via `cargo build`, run dev via `source bin/activate-hermit && cargo build`. UI dev via `just run-ui` (desktop) or `ui/text` for terminal UI.

---

### Hermes Agent

**Setup Process:** Interactive wizard (~3,100 lines in `hermes_cli/setup.py`):

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
source ~/.bashrc  # or ~/.zshrc
hermes setup      # Full interactive setup wizard
```

**Platforms:** Linux, macOS, WSL2, Android via Termux. Windows requires WSL2.

**Post-Setup Commands:**

```bash
hermes              # Interactive CLI
hermes model        # Choose LLM provider and model
hermes tools        # Configure enabled tools
hermes config set   # Set individual config values
hermes gateway      # Start messaging gateway (Telegram, Discord, etc.)
hermes doctor       # Diagnose any issues
```

**Migration from OpenClaw:** `hermes claw migrate` auto-detects `~/.openclaw` and offers migration of SOUL.md, memories, skills, API keys, and messaging settings.

**Config:** `~/.hermes/config.yaml` (YAML settings) + `~/.hermes/.env` (API keys).

---

### Open Code (anomalyco/opencode)

**Setup Process:** Single command YOLO install:

```bash
curl -fsSL https://opencode.ai/install | bash
```

Or package managers: `npm i -g opencode-ai`, `brew install anomalyco/tap/opencode`, `scoop install opencode`, etc.

**Configuration:** `~/.config/opencode/opencode.json` (or JSONC with comments). Desktop app available for macOS/Windows/Linux.

**Project-Local Config:** `.opencode/` directory in project root with `CLAUDE.md` and `AGENTS.md` for project-specific context.

**Client/Server Architecture:** TUI connects to agent server. Can run agent on remote machine, drive from mobile app. Multiple clients can connect.

**LSP Support:** Out-of-the-box LSP support for IDE-precision refactoring, rename, diagnostics.

**Development:** Bun runtime. TypeScript source. `bun typecheck` from package directories.

---

### Oh My OpenAgent (code-yeongyu/oh-my-openagent)

**Installation:** Designed for agent-driven installation. Paste prompt to LLM agent (Claude Code, Cursor, etc.) pointing to installation guide URL.

**Philosophy:** "Install OmO. Type `ultrawork`. Done." — opinionated defaults, no config required for basic use.

**Configuration:** JSONC format (`oh-my-openagent.jsonc`). Multi-level: project → user → defaults.

**Plugin Model:** OpenCode plugin. 11 built-in agents, 52 lifecycle hooks, 26 tools, 3-tier MCP system.

**Doctor Command:** `bunx oh-my-opencode doctor` verifies plugin registration, config, models, and environment.

**Hooks System:** 46+ configurable hooks across session lifecycle. Disabled via `disabled_hooks` arrays.

**Skill-Embedded MCPs:** Skills carry their own MCP servers, spin up on-demand, scope to task, gone when done.

---

### pi-mono (badlogic/pi-mono)

**Setup:** `npm install -g @mariozechner/pi-coding-agent`. Authenticate via API key or `/login` OAuth.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
pi
# or
pi /login  # Select provider interactively
```

**Config Locations:**

| Scope | Location |
|-------|----------|
| Global | `~/.pi/agent/settings.json` |
| Project | `.pi/settings.json` (overrides global) |

**Context Files:** `AGENTS.md` or `CLAUDE.md` loaded from `~/.pi/agent/`, parent directories, and current directory.

**Sessions:** JSONL files in `~/.pi/agent/sessions/`, organized by working directory. Tree structure with branching via `/tree`.

**Four Modes:** Interactive (default), print (`-p`), JSON (`--mode json`), RPC (`--mode rpc` for process integration).

**Philosophy:** Aggressively extensible. "Adapt pi to your workflows, not the other way around." No MCP, no sub-agents built-in, no permission popups, no plan mode — build with extensions or install packages.

---

### Claude Code (chauncygu/collection-claude-code-source-code)

**Setup:** `npm install -g @anthropic-ai/claude-code`. First-run setup flow in `src/setup.ts`.

**Headless/SDK Mode:** `QueryEngine.ts` provides headless query lifecycle. SDK mode via `entrypoints/sdk/`.

**Session Model:** JSONL sessions in `~/.claude/projects/<hash>/sessions/`. Session resumption via `--continue` or `--resume <id>`.

**CLI Entry Points:**

- `cli.tsx` — CLI main (version, help, daemon)
- `entrypoints/sdk/` — Agent SDK (types, sessions)

**TUI:** React/Ink terminal UI. Components in `src/components/`. Permission dialogs, settings panels, model selector.

**Slash Commands:** ~80 slash commands defined in `src/commands/`.

**Feature Gating:** 108 modules dead-code-eliminated via `feature()` compile-time intrinsic. Internal users get better prompts, verification agents, effort anchors.

---

### karpathy/autoresearch

**Minimal UX:** Three files: `program.md` (agent instructions), `train.py` (editable training code), `prepare.py` (fixed setup).

```bash
uv sync
uv run prepare.py  # One-time data prep
uv run train.py    # Fixed 5-minute training budget
```

**No Config:** Agent points at `program.md` and goes. Human edits `program.md` to change agent behavior. No JSON/YAML/TOML config files.

**Interface:** Plain text Markdown files. Agent sees `program.md`, edits `train.py`, commits, runs experiment, logs results to `results.tsv`.

**Zero Ceremony:** No CLI wizard, no interactive setup, no TUI. Just files and a 630-line experiment loop.

---

## 2. Configuration Models

### Comparison Matrix

| System | Config Format | Location | Project-Local | Profiles |
|--------|---------------|----------|---------------|----------|
| OpenClaw | JSON5 | `~/.openclaw/openclaw.json` | No (uses workspace files) | No |
| Goose | ? | `~/.config/goose/` | No | Yes (via profiles) |
| Hermes | YAML + .env | `~/.hermes/config.yaml` | Via context files | Yes (HERMES_HOME) |
| Open Code | JSON/JSONC | `~/.config/opencode/` | `.opencode/` dir | No |
| Oh My OpenAgent | JSONC | `~/.config/opencode/` | `.opencode/` dir | No |
| pi-mono | JSON | `~/.pi/agent/settings.json` | `.pi/settings.json` | No |
| Claude Code | ? | `~/.claude/` | CLAUDE.md per-dir | No |
| autoresearch | None | None | N/A | N/A |

### Project-Local vs Global Configuration

**Project-Local (Open Code, Oh My OpenAgent, pi-mono, Claude Code):**

- Open Code: `.opencode/CLAUDE.md`, `.opencode/AGENTS.md`
- Oh My OpenAgent: `.opencode/` with plugin config
- pi-mono: `.pi/settings.json` overrides global `~/.pi/agent/settings.json`; `AGENTS.md`/CLAUDE.md from cwd up
- Claude Code: `CLAUDE.md` loaded per-directory

**Global-Only (OpenClaw, Hermes, Goose):**

- OpenClaw: `~/.openclaw/` workspace root, workspace files injected
- Hermes: `~/.hermes/` with profile isolation via HERMES_HOME
- Goose: `~/.config/goose/` with profile concept

**No Config (autoresearch):**

- Pure `program.md` file-based instruction, no structured config

---

## 3. Debugging Surfaces

### OpenClaw

- **`openclaw doctor`** — surfaces risky/misconfigured DM policies
- **`openclaw gateway --verbose`** — verbose logging
- **Chat commands:** `/status`, `/new`, `/reset`, `/compact`, `/think <level>`, `/verbose on|off`, `/trace on|off`, `/usage`
- **Stream events:** Session tool events, compaction checkpoints
- **Control UI:** Web-based control panel at gateway port

### Hermes Agent

- **`hermes doctor`** — diagnose issues
- **Skin engine** — customizable CLI visual themes (default gold/kawaii, ares crimson/bronze, mono grayscale, slate blue)
- **KawaiiSpinner** — animated faces during API calls with activity feed for tool results
- **Slash commands:** `/compress`, `/usage`, `/insights [--days N]`
- **Gateway daemon** — separate process from CLI

### Goose

- **Rust `cargo` dev loop** — `cargo build`, `cargo test`
- **UI dev:** `just run-ui` for desktop, `ui/text` for terminal
- **MCP testing:** `just record-mcp-tests`
- **Ink TUI constraints:** Fixed character grid, no overflow clipping, no `wrap="wrap"` in fixed-height boxes

### Open Code

- **Remote driving** — TUI-to-agent connection over network
- **VSCode integration** — via desktop app
- **LSP diagnostics** — pre-build diagnostics via LSP integration

### pi-mono

- **Extension debugging** — `pi install` for third-party packages, hot-reload themes
- **tmux integration** — `tmux new-session -d -s pi-test -x 80 -y 24 && ./pi-test.sh`
- **Session tree** — `/tree` navigate to any point, continue from there

### Claude Code

- **Permission dialogs** — tool execution approval UI
- **Telemetry** — two analytics sinks (1P → Anthropic, Datadog), environment fingerprint, no UI-exposed opt-out
- **Undercover mode** — strips AI attribution in public repos (Anthropic employees auto-enabled)
- **Remote control** — hourly polling of `/api/claude_code/settings`, blocking dialogs for dangerous changes

### autoresearch

- **Git commits** — experiment state as git commits
- **`results.tsv`** — tab-separated experiment log
- **`run.log`** — training output
- **`grep "^val_bpb:" run.log`** — extract metric

---

## 4. Multi-Client Support

### OpenClaw

- **Multi-channel inbox** — WhatsApp, Telegram, Slack, Discord, Google Chat, Signal, iMessage, IRC, Teams, Matrix, etc.
- **Multi-agent routing** — route inbound channels/accounts/peers to isolated agents (workspaces + per-agent sessions)
- **Single-agent-per-session by default** — gateway limitation

### Hermes Agent

- **15+ messaging surfaces** — Telegram, Discord, Slack, WhatsApp, Signal, Email, CLI
- **Gateway daemon** — routes to appropriate surface
- **Profile isolation** — multiple HERMES_HOME instances

### Goose

- **Desktop + CLI + API** — three surfaces over same core
- **MCP extensions** — 70+ protocol extensions
- **Profile system** — configuration profiles

### Open Code

- **Client/server architecture** — TUI is one client, agent runs separately
- **Remote access** — drive agent from mobile app or remote machine
- **Multiple clients** — can connect multiple clients to same agent

### Oh My OpenAgent

- **OpenCode plugin** — extends Claude Code, not standalone
- **Multi-agent orchestration** — 11 built-in agents (Sisyphus, Hephaestus, Prometheus, Oracle, etc.)
- **Background agents** — 5+ parallel specialist agents

### Devin

- **Manager/Worker topology** — 1 manager + up to 10 workers
- **Each worker runs in isolated VM** — own terminal, browser, development environment
- **Session links** — inspect individual worker work directly
- **Message child sessions** — send instructions mid-task
- **ACU monitoring** — track compute consumption per worker
- **Put to sleep / terminate** — pause or stop workers

### Claude Code

- **Bridge layer** — Claude Desktop integration, remote sessions
- **SDK mode** — headless programmatic usage
- **Sub-agent spawning** — fork, worktree, remote modes

### pi-mono

- **Single surface per instance** — CLI, but extensible via extensions
- **SDK** — `createAgentSession()` for embedding
- **RPC mode** — stdin/stdout JSONL for process integration

---

## 5. Developer Onboarding

### Guided Setup (Interactive Wizards)

1. **OpenClaw** — `openclaw onboard` step-by-step wizard
2. **Hermes Agent** — `hermes setup` (~3,100 lines Python interactive wizard)
3. **Devin** — $500/month cloud service, web dashboard onboarding
4. **Claude Code** — First-run setup flow in `src/setup.ts`

### Zero-Config (Minimal Onboarding)

1. **autoresearch** — `uv sync && uv run prepare.py` → `uv run train.py`. Agent pointed at `program.md`.
2. **Oh My OpenAgent** — "Install. Type `ultrawork`. Done."
3. **pi-mono** — Minimal defaults: 4 tools (read, write, edit, bash), no config needed

### Package Manager Install

- **Open Code** — `curl -fsSL https://opencode.ai/install | bash` or npm/brew/scoop/pacman
- **Goose** — Desktop app download OR `curl .../download_cli.sh | bash`
- **pi-mono** — `npm install -g @mariozechner/pi-coding-agent`

### Source Build (Developer Onboarding)

| Repo | Setup | Dev Loop |
|------|-------|----------|
| OpenClaw | `pnpm install && pnpm openclaw setup` | `pnpm gateway:watch` |
| Hermes | `./setup-hermes.sh` (uv, venv, symlink) | `source venv/bin/activate && hermes` |
| Goose | `source bin/activate-hermit && cargo build` | `cargo build && cargo test` |
| pi-mono | `npm install && npm run build` | `./pi-test.sh` |
| autoresearch | `uv sync` | Edit `train.py`, run `uv run train.py` |

---

## 6. Cloud VM Interfaces (Devin-Specific)

### Web Dashboard

- **Session management** — sidebar with session list, filters (non-archived, started by you)
- **Work log** — Devin's planner with accordions showing retro at each step (🟢/🟠/🔴 grades, timestamps, duration)
- **Machine utilization** — top right corner during sessions
- **PR metrics view** — `https://app.devin.ai/metrics`

### VM Access

- **"Open VSCode"** button — browser-based VSCode opening in new tab, full file read/write, terminal access
- **"Use Devin's Machine"** — direct access to Devin's VM filesystem and terminal
- **Port forwarding** — forward cloud sandbox ports to localhost for testing

### Manager Dashboard (MultiDevin)

- **Spin up managed Devins** — break large task into pieces, delegate to separate sessions
- **Message child sessions** — send instructions, context, corrections mid-task
- **ACU consumption monitoring** — track compute per child session
- **Sleep/terminate controls** — pause or stop off-track workers
- **Session links per worker** — inspect any managed Devin's work directly

### Integrations

- **Slack** — tag Devin in conversations, route messages
- **Linear** — assign Devin tickets directly
- **GitHub** — PR review, CI feedback integration
- **Datadog** — incident triage

---

## 7. Surface UX Differentiation

### pi-mono — CLI vs TUI vs Web vs Slack

| Surface | Package | UX Character |
|---------|---------|-------------|
| **CLI** | `pi-coding-agent` | Interactive terminal, message queue, model selector, session tree |
| **TUI** | `pi-tui` | Custom component library, differential rendering, synchronized output, no flicker |
| **Web** | `pi-web-ui` | Web components (ChatPanel, AgentInterface), IndexedDB storage, artifacts |
| **Slack** | `pi-mom` | Bot that delegates to pi coding agent |

**pi-tui Differential Rendering Strategy:**

1. First render — output all lines without clearing scrollback
2. Width changed or change above viewport — clear screen, full re-render
3. Normal update — move cursor to first changed line, clear to end, render changed lines

Uses CSI 2026 for atomic screen updates (no flicker). Bracketed paste mode for large pastes.

### goose — Desktop vs CLI vs API

| Surface | Implementation | UX Character |
|---------|----------------|---------------|
| **Desktop** | Electron app | Full GUI with app window |
| **CLI** | `goose-cli` Rust binary | Terminal interaction |
| **API** | `goose-server` (goosed) | Headless API embedding |

### OpenClaw — Gateway vs Control UI vs Mobile Apps

| Surface | Purpose |
|---------|---------|
| **Gateway daemon** | Control plane, sessions, channels, tools |
| **Control UI** | Web-based control panel |
| **macOS menu bar app** | Voice Wake, push-to-talk, WebChat, debug tools, remote gateway control |
| **iOS/Android nodes** | Voice trigger forwarding, Canvas surface, device pairing |

---

## 8. Hook & Lifecycle Systems

### Oh My OpenAgent — 46+ Hooks

**Hook Tiers:**

| Tier | Count | Purpose |
|------|-------|---------|
| Session | 24 | Session lifecycle hooks |
| Tool-Guard | 14 | Pre/post tool execution |
| Transform | 5 | Message transformation |
| Continuation | 7 | Persistent continuation |
| Skill | 2 | Skill lifecycle |

**Configurable via:** `disabled_hooks` arrays in JSONC config.

### Claude Code — Hook System

**Sources:** `settings.json` hooks, CLI args, session decisions.

**Hook Types:**

- Pre-tool hooks — approve, deny, or modify input
- Permission rules — `alwaysAllowRules`, `alwaysDenyRules`, `alwaysAskRules`
- Tool-specific path sandboxing

### OpenClaw — Channel/Skill Hooks

- **Slash commands** — `/status`, `/new`, `/reset`, `/compact`, `/think`, `/verbose`, `/trace`, `/usage`, `/restart`
- **Session tools** — `sessions_list`, `sessions_history`, `sessions_send`
- **Skills registry** — ClawHub marketplace

---

## 9. Typed Events & SDK Model

### Claude Code — Typed SDK Events

```typescript
// SDK event types from query.ts
type SDKMessage =
  | { type: 'message_start'; ... }
  | { type: 'content_block_delta'; ... }
  | { type: 'message_stop'; ... }
  | { type: 'tool_use'; ... }
  | { type: 'tool_result'; ... }

// AsyncGenerator streaming from QueryEngine
async function* query(prompt: string): AsyncGenerator<SDKMessage>
```

**Headless Usage:** `QueryEngine.ts` without terminal UI. Build custom UI on top.

### pi-mono — Agent Events

```typescript
// Agent events from pi-agent-core
type AgentEvent =
  | 'agent_start'
  | 'agent_end'
  | 'turn_start'
  | 'turn_end'
  | 'message_start'
  | 'message_update'
  | 'message_end'

agent.subscribe((event) => {
  switch (event.type) {
    case 'agent_start': ...
  }
})
```

### goose — Provider Trait + MCP

- **Rust Provider trait** — `providers/base.rs`
- **MCP extensions** — `crates/goose-mcp/`

---

## 10. ACE Recommendations

| Pattern | System | Recommendation | Rationale |
|---------|--------|----------------|----------|
| **Interactive onboarding wizard** | OpenClaw, Hermes | **ADOPT** | Reduces setup friction; step-by-step guides prevent early abandonment. Hermes's 3,100-line Python wizard is thorough but heavyweight. |
| **Project-local AGENTS.md/CLAUDE.md** | Open Code, pi-mono, Claude Code | **ADOPT** | Proven pattern for project-specific context injection. Works with agent's natural file discovery. |
| **Zero-config defaults, opinionated** | Oh My OpenAgent, pi-mono | **ADOPT** | "ultrawork" and "just works with 4 tools" reduce time-to-first-result. Configurable when needed. |
| **Doctor/diagnostics command** | OpenClaw, Hermes, Oh My OpenAgent | **ADOPT** | `openclaw doctor`, `hermes doctor`, `bunx oh-my-opencode doctor` surface issues before users hit them. |
| **Profile/instance isolation** | Hermes (HERMES_HOME), Goose (profiles) | **ADOPT** | Multiple isolated instances prevent credential/state leakage between projects. |
| **Multi-surface runtime (CLI/TUI/Web/Slack)** | pi-mono, Hermes | **ADOPT** | Same core powering different interfaces. pi-mono's surface separation is cleanest. |
| **Cloud VM web dashboard** | Devin | **ADOPT with CAUTION** | Full VM access via browser is powerful but creates security attack surface. VPC deployment option essential for enterprise. |
| **Manager/worker delegation UI** | Devin MultiDevin | **ADOPT** | Breaking large tasks into scoped parallel subtasks with individual monitoring is the right mental model. |
| **Typed SDK events (AsyncGenerator)** | Claude Code, pi-mono | **ADOPT** | Claude Code's `AsyncGenerator<SDKMessage>` and pi-mono's event subscription are the right abstraction for headless embedding. |
| **Skin/theme engine** | Hermes (CLI skins), pi-mono (themes) | **ADOPT** | Customizable visuals without code changes. Hermes's YAML skin format is well-designed. |
| **Differential TUI rendering** | pi-mono (pi-tui) | **ADOPT** | CSI 2026 synchronized updates, three-strategy diffing, no flicker — production-quality terminal rendering. |
| **MCP for extension model** | Goose, Hermes | **ADOPT** | Open standard with 70+ extensions (Goose) and broad tool support (Hermes). ACE should implement MCP. |
| **JSONC config with comments** | Oh My OpenAgent | **ADOPT** | Developer-friendly config: comments, trailing commas. Zod validation on top. |
| **program.md file-based agent config** | autoresearch | **ADOPT for minimal UX** | Elegant for narrow use cases. ACE should support `program.md`-style overrides at project level. |
| **CLI-first install (curl \| bash)** | Open Code, Goose, Hermes | **ADOPT** | One command to first success. Package managers secondary. |
| **git-based experiment state** | autoresearch | **ADOPT for research tools** | Experiment state as commits enables git diff, rollback, branch-per-experiment. |
| **Heavy compile-time feature gating** | Claude Code (108 DCE'd modules) | **AVOID** | 108 dead-code-eliminated modules creates invisible complexity. OpenClaw's runtime feature flags are more debuggable. |
| **Mandatory DM pairing/allowlist** | OpenClaw | **ADAPT** | Security-default for messaging platforms is right, but pairing friction is high. Make opt-out more discoverable. |
| **Undercover mode (auto stealth)** | Claude Code | **AVOID** | Auto-stripping AI attribution in public repos raises transparency issues. Should be explicit opt-in, not automatic. |
| **Per-channel isolated agent sessions** | OpenClaw multi-agent routing | **ADAPT** | Right model but gateway is single-agent-per-session. Need session pooling for high-throughput channels. |
| **Heavyweight Python setup wizard** | Hermes (3,100 lines) | **ADAPT** | Thorough but slow. Consider splitting into "quick start" (5 questions) vs "full config" (current). |
| **Closed-source proprietary UX** | Devin | **AVOID for ACE** | Devin's cloud VM UX is excellent but opaque. ACE must be open source; can't replicate proprietary patterns verbatim. |

---

## 11. Summary: UX/DX Principles Across Systems

### What Works

1. **Zero-config defaults with progressive disclosure** — pi-mono's 4 tools, Oh My OpenAgent's `ultrawork`, autoresearch's `program.md`
2. **Doctor/diagnostics commands** — Pre-flight checks catch config errors early
3. **Project-local context files (AGENTS.md/CLAUDE.md)** —Ubiquitous pattern, proven effective
4. **Interactive wizards for first-run** — OpenClaw `onboard`, Hermes `setup`
5. **Differential TUI rendering** — pi-tui's CSI 2026 approach is production-quality
6. **Profile isolation** — Hermes HERMES_HOME, Goose profiles
7. **Multi-surface same-core** — CLI/TUI/Web/Slack from same runtime
8. **Typed SDK with event subscription** — Headless embedding without protocol guessing

### What Doesn't Work

1. **Heavyweight setup with no escape** — Hermes 3,100-line wizard is thorough but slow
2. **Compile-time DCE hiding features** — Claude Code's 108 missing modules creates invisible complexity
3. **Auto-enabled stealth modes** — Undercover mode without explicit user consent
4. **Single-binary obsession causing UX gaps** — Some systems prioritize single-binary over surface quality
5. **Credential/permissive defaults on messaging** — OpenClaw DM pairing is right security model but high friction
6. **No clear upgrade path from CLI to SDK** — Many systems treat headless as afterthought

### Cross-System Patterns (Convergent Evolution)

| Pattern | Appears In |
|---------|------------|
| `program.md` / `AGENTS.md` / `CLAUDE.md` | Open Code, pi-mono, Claude Code, autoresearch |
| Doctor command | OpenClaw, Hermes, Oh My OpenAgent |
| Skin/theme engine | Hermes, pi-mono |
| Profile/instance isolation | Hermes, Goose |
| Multi-surface (CLI/TUI/Web/Slack) | pi-mono, Hermes, OpenClaw |
| MCP for extensions | Goose, Hermes |
| Git-based experiment state | autoresearch |
| Typed event subscription | Claude Code, pi-mono |

---

**Files Affected:**

- `/home/jay/programming/ace_prototype/design/units/agents-study/study/ux-dx.md`
