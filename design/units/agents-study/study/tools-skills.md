# Tools & Skills Systems — Cross-Cutting Comparison

**Unit:** agents-study  
**Slice:** 6  
**Systems Studied:** 8  
**Date:** 2026-04-23

---

## 1. Tool Discovery Mechanisms

### 1.1 Static Registration (Import-Time)

The most common pattern. Tools declare themselves at module import time via a central registry.

| System | Mechanism | Key Files |
|--------|-----------|-----------|
| **Hermes Agent** | `tools/registry.py` — all `tools/*.py` call `registry.register()` at import time. Auto-discovery: no manual import list needed. | `tools/registry.py`, `tools/skills_tool.py` |
| **Open Code** | `define()` + `init()` Effect-based. Tool wrapped with execution telemetry, truncation, and span attribution. | `packages/opencode/src/tool/tool.ts` |
| **Oh My OpenAgent** | 26 tools via factory pattern in `createToolRegistry()`. Factories for each tool type. | `src/plugin/tool-registry.ts` |
| **pi-mono** | `ToolDefinition` registry with custom rendering functions. Built-in tool lifecycle. | `packages/coding-agent/src/core/skills.ts` |

**Key Pattern (Hermes):**
```python
# tools/registry.py — self-registration at import time
def register(name, toolset, schema, handler, check_fn=None, requires_env=None, emoji=None):
    ...

# tools/terminal_tool.py — auto-discovery via top-level call
registry.register(
    name="terminal",
    toolset="core",
    schema={...},
    handler=lambda args, **kw: terminal_tool(...),
)
```

### 1.2 Dynamic Discovery (Filesystem-Based)

Skills discovered by scanning directories for `SKILL.md` files.

| System | Discovery Strategy | Location Priority |
|--------|-------------------|-----------------|
| **Goose** | Walks `~/.goose/skills`, `~/.claude/skills`, `~/.agents/skills`, `.claude/skills`, `.agents/skills`, project-local `.goose/skills`. Built-in skills embedded in binary. | Local → Global |
| **OpenClaw** | 6 locations: workspace `skills/`, `.agents/skills/`, `~/.agents/skills`, `~/.openclaw/skills`, bundled, `extraDirs`. Precedence: workspace > project > personal > managed > bundled > extra. | Workspace wins |
| **Hermes Agent** | Scans `~/.hermes/skills/` and external dirs. Plugin-provided skills via namespaced names (`plugin:skill`). | Local first |
| **pi-mono** | `loadSkillsFromDir()` — SKILL.md directory detection, symlink following, ignore-file support (.gitignore, .ignore, .fdignore). | Configurable via `agentDir` + `cwd` |
| **Oh My OpenAgent** | 4-scope discovery: project > opencode config > user > global (built-in). `discoverSkills()` with priority deduplication. | Project wins |

**Key Pattern (pi-mono):**
```typescript
// packages/coding-agent/src/core/skills.ts
export function loadSkillsFromDir(options: LoadSkillsFromDirOptions): LoadSkillsResult {
  // Discovery rules:
  // - if directory contains SKILL.md, treat as skill root (don't recurse)
  // - otherwise, load direct .md children in root
  // - recurse subdirectories to find SKILL.md
}
```

### 1.3 Network-Based Discovery

| System | Mechanism | Sources |
|--------|-----------|---------|
| **Hermes Agent** | Skills Hub — GitHub Contents API, skills.sh marketplace, well-known endpoints (`/.well-known/skills/index.json`). 5 default GitHub taps. | `tools/skills_hub.py` |
| **OpenClaw** | ClawHub (`clawhub.ai`) — public skills registry with install/sync commands. | `docs/tools/skills.md` |

---

## 2. Execution Models

### 2.1 Direct Execution (Synchronous Handler)

Tool logic executes immediately when called. Most common model.

| System | Pattern | Sandbox |
|--------|---------|---------|
| **Hermes Agent** | `handle_function_call()` dispatches to handler, returns JSON string. Concurrent via `ThreadPoolExecutor`. | No native sandbox |
| **Open Code** | Effect-based: `execute(args, ctx) → Effect<ExecuteResult>`. Wrapped with truncation, span attribution. | No native sandbox |
| **pi-mono** | `ToolDefinition` with `render()` function. Custom rendering per tool. | No native sandbox |

### 2.2 Dynamic Code Generation

Agent generates custom code per request, executed by a universal executor.

| System | Pattern | Example |
|--------|---------|---------|
| **playwright-skill** | Agent writes Playwright JS scripts to `/tmp/playwright-test-*.js`, executed via `node run.js`. Supports inline code and file paths. | `skills/playwright-skill/run.js` |
| **Goose** | MCP extension system — agents extend capabilities via MCP servers. | `crates/goose-mcp/src/mcp_server_runner.rs` |

**Key Pattern (playwright-skill):**
```javascript
// Universal executor — accepts file path, inline code, or stdin
// 1. Auto-installs Playwright if missing
// 2. Wraps code in async IIFE if needed
// 3. Executes from skill directory for proper module resolution
// 4. Writes to /tmp for automatic cleanup
```

### 2.3 Subagent Delegation

Agent spawns child agents with isolated context and restricted toolsets.

| System | Mechanism | Depth Limit |
|--------|-----------|-------------|
| **Hermes Agent** | `delegate_tool.py` — spawns child AIAgent with isolated context. `ThreadPoolExecutor` for concurrent tool execution. | Max depth: 2 |
| **Oh My OpenAgent** | 8 built-in categories (visual-engineering, ultrabrain, deep, artistry, quick, etc.) with model routing. | No explicit limit |
| **OpenClaw** | Multi-agent workspace — each agent has own workspace. Per-agent skill allowlists. | N/A |

---

## 3. Sandboxing Approaches

### 3.1 No Native Sandboxing

Most systems rely on the host environment's security model.

| System | Approach | Limitations |
|--------|---------|-------------|
| **Hermes Agent** | Terminal tool runs commands in host shell. Security via dangerous-command detection in `approval.py`. | No process isolation |
| **Open Code** | No sandbox. Relies on host environment. | No sandbox |
| **pi-mono** | No sandbox. | No sandbox |
| **Oh My Openagent** | No sandbox. | No sandbox |

### 3.2 Docker/Container Sandboxing

| System | Mechanism | Filesystem Isolation |
|--------|-----------|---------------------|
| **OpenClaw** | `agents.defaults.sandbox.docker.setupCommand` for skill binary installation inside container. Skills checked at load time on host AND inside container. | Yes |
| **Hermes Agent** | Remote execution environments: docker, ssh, modal, daytona, singularity. Skill env vars registered for passthrough to sandboxes. | Yes |

### 3.3 Process-Level Isolation

| System | Mechanism |
|--------|-----------|
| **OpenClaw** | Gateway runs agents in isolated processes. Skill env injection scoped to agent run, not global shell. |

---

## 4. Interoperability Standards (MCP)

### 4.1 MCP as Extension Protocol

| System | MCP Support | Implementation |
|--------|-------------|----------------|
| **Goose** | First-class. `crates/goose-mcp` for building extensions. MCP server runner for extensions. Recipe system for multi-step automation. | `crates/goose-mcp/src/mcp_server_runner.rs` |
| **Hermes Agent** | MCP client (`tools/mcp_tool.py`) — ~1050 lines. Connects to external MCP servers. 47 tools across 19 toolsets. | `tools/mcp_tool.py` |
| **OpenClaw** | Plugin SDK exposes MCP integration. Skills can declare MCP dependencies. | `packages/plugin-sdk/src/provider-tools.ts` |
| **Oh My OpenAgent** | 3-tier MCP system: (1) Built-in remote MCPs, (2) .mcp.json integrations, (3) Skill-embedded MCPs (Tier 3, per-session). `SkillMcpManager` handles stdio + HTTP + OAuth. | `src/features/skill-mcp-manager/`, `src/mcp/` |

### 4.2 Three-Tier MCP Architecture (Oh My OpenAgent)

| Tier | Source | Mechanism |
|------|--------|-----------|
| Built-in | `src/mcp/` | 3 remote HTTP MCPs: websearch, context7, grep_app |
| Claude Code | `.mcp.json` | `${VAR}` env expansion via `claude-code-mcp-loader` |
| Skill-embedded | SKILL.md YAML | `SkillMcpManager` — stdio + HTTP per session |

### 4.3 MCP Tool Invocation (Oh My OpenAgent)

```typescript
// Skill-embedded MCP in SKILL.md YAML frontmatter:
---
name: my-skill
mcp:
  - name: my-mcp
    type: stdio
    command: npx
    args: [-y, my-mcp-server]
---
```

---

## 5. Auto-Generation Patterns

### 5.1 Trajectory-to-Skill Capture

| System | Mechanism | Trigger |
|--------|-----------|---------|
| **Hermes Agent** | `skill_manager_tool.py` — agent creates/updates skills after successful complex tasks. `skill_manage()` with create/edit/patch/delete actions. | 5+ tool calls, errors overcome, user-corrected approach |
| **OpenClaw** | Skill Workshop — experimental plugin that creates workspace skills from observed procedures. Quarantines unsafe proposals. | Agent workflow observation |
| **Karpathy Skills** | SKILL.md packaging — single-file skill format. 4 principles encoded: Think Before Coding, Simplicity First, Surgical Changes, Goal-Driven Execution. | Manual authoring |

### 5.2 Self-Registration

| System | Pattern |
|--------|---------|
| **Hermes Agent** | Tools call `registry.register()` at import time. Skill manager tool (`skill_manage`) allows agent to edit skills after execution. |
| **OpenClaw** | Plugins ship skills via `openclaw.plugin.json` `skills` field. Skills auto-discovered at plugin enable time. |

---

## 6. Security Models

### 6.1 Trust Levels

| System | Levels | Policy |
|--------|--------|--------|
| **Hermes Agent** | builtin, trusted, community, agent-created | Builtin: always allow. Trusted (openai/skills, anthropics/skills): caution allowed. Community: dangerous = block. |
| **OpenClaw** | bundled, managed, workspace, project, extraDirs | Workspace wins. Critical findings block by default. Skill allowlists per agent. |

### 6.2 Security Scanning

**Hermes Agent (`tools/skills_guard.py`)** — 488-line security scanner:

| Category | Patterns | Key Detections |
|----------|----------|----------------|
| Exfiltration | curl/wget/env exfil, credential store access, DNS exfil | `curl ... $API_KEY`, `$HOME/.ssh`, `~/.hermes/.env` |
| Prompt Injection | role hijack, ignore instructions, hidden HTML/CSS, invisible unicode | `ignore previous instructions`, `<div style="display:none">` |
| Destructive | `rm -rf /`, home deletion, filesystem format | `rm -rf /`, `shutil.rmtree` on absolute path |
| Persistence | crontab, shell RC files, SSH authorized_keys, systemd | `.bashrc`, `authorized_keys`, `systemctl enable` |
| Network | Reverse shells, tunneling services, hardcoded IPs | `nc -lp`, `ngrok`, `/dev/tcp/` |
| Obfuscation | base64 decode pipe, eval, hex encoding | `base64 -d \| sh`, `eval("...")` |
| Privilege Escalation | sudo, setuid, NOPASSWD sudoers | `sudo`, `chmod +s` |
| Credential Exposure | Hardcoded API keys, private keys, GitHub tokens | `sk-...`, `ghp_...`, `-----BEGIN PRIVATE KEY-----` |

**OpenClaw (`plugins/install-security-scan.js`)** — installer security scanner:

| Pattern | Protection |
|---------|------------|
| Safe brew formula | `/^[a-z0-9][a-z0-9+._@-]*(\/[a-z0-9][a-z0-9+._@-]*){0,2}$/` |
| Safe node package | `/^(@[a-z0-9._-]+\/)?[a-z0-9._-]+(@[a-z0-9^~>=<.*|-]+)?$/` |
| Safe go module | `/^[a-zA-Z0-9][a-zA-Z0-9._-]*@[a-z0-9v._-]+$/` |

### 6.3 OpenClaw Marketplace Security Warning

> **Critical:** Cisco security assessment found significant malware rate in community skills marketplace. Third-party skills must be treated as untrusted code. OpenClaw docs explicitly warn: "Treat third-party skills as **untrusted code**. Read them before enabling."

**Evidence:** `docs/tools/skills.md` — "Security notes" section recommends sandboxed runs and dangerous-code scanning before execution.

### 6.4 Security-Gated Installation

| System | Mechanism |
|--------|-----------|
| **Hermes Agent** | Quarantine directory for hub installs. `skills_guard.py` blocks dangerous community skills. Agent-created skills only scanned if `skills.guard_agent_created` enabled (off by default). |
| **OpenClaw** | `scanSkillInstallSource()` before execution. `critical` findings block by default. Non-bundled install sources generate warnings. |

---

## 7. Skill Packaging Standards

### 7.1 SKILL.md Format (agentskills.io Standard)

All systems converge on YAML frontmatter + Markdown body:

```yaml
---
name: skill-name
description: Brief description (max 1024 chars)
version: 1.0.0
license: MIT
platforms: [macos, linux]  # pi-mono, hermes
metadata:
  hermes:
    tags: [fine-tuning, llm]
    related_skills: [peft, lora]
prerequisites:
  env_vars: [API_KEY]
  commands: [curl, jq]
compatibility: Requires X
---
# Skill Title

Instructions and content here...
```

**Field Limits (Hermes/pi-mono standard):**
- `name`: max 64 characters
- `description`: max 1024 characters

### 7.2 Directory Structure

```
skill-name/
├── SKILL.md              # Main instructions (required)
├── references/           # Supporting documentation
├── templates/            # Output templates
├── scripts/             # Executable scripts
└── assets/              # Supplementary files (agentskills.io)
```

### 7.3 Platform Compatibility

| System | Field | Values |
|--------|-------|--------|
| **pi-mono** | `platforms` | `macos`, `linux`, `windows` |
| **Hermes Agent** | `platforms` | `macos`, `linux`, `windows` (per SKILL.md) |
| **OpenClaw** | `metadata.openclaw.os` | `darwin`, `linux`, `win32` |

### 7.4 Progressive Disclosure

| System | Tier 1 (List) | Tier 2 (View) | Tier 3 (File) |
|--------|---------------|---------------|----------------|
| **Hermes Agent** | `skills_list()` — name + description | `skill_view(name)` — full SKILL.md + linked files | `skill_view(name, file_path)` — specific reference |
| **pi-mono** | System prompt lists all skills | Agent reads skill file on demand | Agent loads supporting files |
| **OpenClaw** | XML skill list in system prompt (~97 chars/skill) | Skill content loaded at runtime | Via slash command |

---

## 8. Cross-System Comparison Matrix

| Dimension | Goose | Hermes | OpenClaw | pi-mono | Karpathy | playwright-skill | opencode | oh-my-openagent |
|-----------|-------|--------|----------|---------|----------|------------------|----------|------------------|
| **Discovery** | Dir scan | Dir scan | 6-location | Dir scan | SKILL.md | SKILL.md | Static | 4-scope |
| **Execution** | MCP | Sync handler | Effect | Render fn | Read-only | Dynamic JS | Effect | Factory |
| **Sandbox** | None | Remote envs | Docker | None | N/A | None | None | None |
| **MCP** | First-class | Client | Plugin SDK | None | N/A | N/A | None | 3-tier |
| **Auto-gen** | Recipes | Trajectory | Workshop | None | Manual | Dynamic code | N/A | None |
| **Security** | MCP ext | Guard (488 patterns) | Install scan | None | N/A | N/A | N/A | N/A |
| **Packaging** | SKILL.md | SKILL.md | SKILL.md | SKILL.md | SKILL.md | SKILL.md + run.js | N/A | SKILL.md |
| **Tool Count** | MCP ext | 47 tools | Many bundled | Built-in | 1 skill | 1 skill | ~20 built-in | 26 tools |

---

## 9. ACE Recommendation

| Pattern | System | Recommendation | Rationale |
|---------|--------|----------------|-----------|
| **SKILL.md standard** | All except opencode | **ADOPT** | Universal format with YAML frontmatter + progressive disclosure. Converged independently across 5+ systems. |
| **Self-registration** | Hermes Agent | **ADOPT** | `registry.register()` at import time eliminates manual wiring. Every tool file is self-contained. |
| **Progressive disclosure** | Hermes, pi-mono | **ADOPT** | Tiered loading (list → view → file) minimizes token waste. ~97 chars/skill overhead. |
| **MCP extension protocol** | Goose, Hermes | **ADOPT** | Standardized tool/server interface. Three-tier architecture (Oh My OpenAgent) provides clear separation of concerns. |
| **Skills Hub marketplace** | Hermes | **ADAPT** | Powerful but requires security scanning. Trust tiers (builtin/trusted/community) + quarantine + scan-on-install. |
| **Security scanner (488 patterns)** | Hermes | **ADOPT** | Comprehensive regex-based static analysis. 12 finding categories. Essential for marketplace trust. |
| **OpenClaw marketplace** | OpenClaw | **AVOID** | Cisco security assessment found significant malware rate. Community skills are untrusted code. |
| **Skills Workshop** | OpenClaw | **ADAPT** | Trajectory-to-skill capture is valuable but experimental. Requires security quarantine for proposals. |
| **Dynamic code generation** | playwright-skill | **ADAPT** | Model generates custom Playwright scripts per request. Universal executor pattern (`run.js`) is elegant. Use for browser automation only. |
| **Factory pattern tools** | Oh My OpenAgent | **ADOPT** | `createToolRegistry()` + `createXXXTool()` factories. Clear separation of concerns. 26 tools via consistent pattern. |
| **Two-phase permission filtering** | opencode | **ADOPT** | Tools filtered before model sees them. `plan` mode hides dangerous tools. Essential security pattern. |
| **Recipe system** | Goose | **ADAPT** | Multi-step automation via MCP extension. Good for complex workflows but adds complexity. |
| **pi-mono ToolDefinition registry** | pi-mono | **ADOPT** | Custom rendering functions per tool. Clean separation of schema and presentation. |
| **No sandbox (host reliance)** | Most | **AVOID** | Relying entirely on host security is risky. Docker/container isolation needed for untrusted skills. |
| **Skill allowlists per agent** | OpenClaw | **ADOPT** | Per-agent skill visibility controls. Critical for multi-agent security. |
| **Skill-embedded MCP** | Oh My OpenAgent | **ADOPT** | Skills declare MCP dependencies in YAML frontmatter. Tier 3 MCP system enables per-session MCP servers. |

### Specific ACE Decisions

1. **ADOPT SKILL.md standard** — YAML frontmatter with `name`, `description`, `platforms`, `metadata` fields. Max 64-char names, 1024-char descriptions. Directory structure with `references/`, `templates/`, `scripts/`, `assets/`.

2. **ADOPT self-registration pattern** — `registry.register(name, schema, handler)` at import time. No manual import lists. Auto-discovery by directory scanning.

3. **ADOPT progressive disclosure** — Tier 1: compact list in system prompt (~97 chars/skill). Tier 2: full SKILL.md loaded on demand. Tier 3: supporting files loaded individually.

4. **ADOPT MCP as extension protocol** — Implement MCP client for external tools. Three-tier MCP: built-in remote → config-based → skill-embedded per-session.

5. **ADOPT security scanning** — Hermes-style regex scanner with 12 categories: exfiltration, injection, destructive, persistence, network, obfuscation, credential exposure, privilege escalation, supply chain, etc. Trust tiers: builtin → trusted → community with graduated policies.

6. **AVOID community marketplace without scanning** — OpenClaw ClawHub has documented malware. Any marketplace requires mandatory security scanning before install.

7. **ADAPT dynamic code generation** — playwright-skill pattern for browser automation. Universal executor (`run.js`) that accepts file path, inline code, or stdin. Write to `/tmp` for cleanup.

8. **ADOPT factory pattern for tools** — `createToolRegistry()` assembles all tools. Individual `createXXXTool()` factories. Consistent parameter normalization.

9. **ADOPT two-phase tool filtering** — Phase 1: filter tools before model sees them (plan mode restrictions). Phase 2: permission check at execution time. Never expose dangerous tools to model.

---

## 10. Key Source Files

| System | File | Purpose |
|---------|------|---------|
| Hermes | `tools/skills_guard.py` | 488-line security scanner with 488 threat patterns |
| Hermes | `tools/skills_hub.py` | Skills Hub with GitHub API, skills.sh, well-known endpoints |
| Hermes | `tools/skill_manager_tool.py` | Agent-managed skill CRUD operations |
| Hermes | `tools/skills_tool.py` | Skills list/view with progressive disclosure |
| OpenClaw | `src/agents/skills-install.ts` | Skill installation with safe regex validation |
| OpenClaw | `docs/tools/skills.md` | Security warning about third-party skills |
| pi-mono | `packages/coding-agent/src/core/skills.ts` | SKILL.md loader with ignore-file support |
| Oh My OpenAgent | `src/plugin/tool-registry.ts` | 26-tool factory registry |
| Oh My OpenAgent | `src/features/opencode-skill-loader/` | 4-scope skill discovery with priority |
| Oh My OpenAgent | `src/features/skill-mcp-manager/` | Tier-3 skill-embedded MCP lifecycle |
| playwright-skill | `skills/playwright-skill/run.js` | Universal Playwright executor |
| Goose | `crates/goose/src/agents/platform_extensions/skills.rs` | Skills MCP client with frontmatter parsing |
| opencode | `packages/opencode/src/tool/tool.ts` | Effect-based tool definition with truncation |

---

**Slice Completed:** 6  
**Files Affected:** `design/units/agents-study/study/tools-skills.md`  
**Changes Made:** Produced cross-cutting comparison of 8 agent systems' tool/skill architectures across 7 dimensions with ACE recommendations (7 adopt, 2 avoid, 4 adapt).
