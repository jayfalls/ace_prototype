# Computer Use & Desktop Automation

**Slice:** 8  
**Unit:** agents-study  
**Output:** `design/units/agents-study/study/computer-use.md`

---

## 1. Introduction

Computer use — the ability for an AI agent to perceive and interact with a graphical desktop environment — is the defining capability frontier for agentic AI in 2025–2026. Unlike browser automation, which operates within a single web application context, desktop automation spans all GUI applications, OS-level interactions, and cross-application workflows. This document cross-cuts six systems plus three Anthropic products (Claude Computer Use, Cowork, Design) and Devin, analyzing desktop automation along six axes: GUI automation approaches, screen coordinate vs DOM-based interaction, OS-level integration depth, desktop environment support, agent access primitives (mouse/keyboard/screenshots), and safety/trust models.

**Key finding:** There are three distinct paradigms for desktop automation: (1) **screen-coordinate vision-based** (Claude Computer Use, Devin), where the agent sees screenshots and issues absolute coordinate commands; (2) **OS-level API scripting** (Goose with Peekaboo, pi-mono via clipboard primitives), where the agent drives native OS APIs; and (3) **VM sandboxing** (Devin full desktop mode), where the agent runs in an isolated VM with full screen recording. The screen-coordinate approach is the most通用 but least precise; OS-level APIs are more precise but require platform-specific tooling; VM sandboxing provides isolation but adds infrastructure complexity. ACE's single-binary model strongly favours the OS-level API scripting approach, specifically Goose's cross-platform `computer_control` + `automation_script` pattern.

---

## 2. System-by-System Analysis

### 2.1 Claude Computer Use (Anthropic)

**Type:** Proprietary API (beta)  
**Models:** Claude Sonnet 4.6, Claude Opus 4.6 (`computer_20251124` tool version)  
**Documentation:** [docs.anthropic.com](https://docs.anthropic.com/en/docs/agents-and-tools/computer-use)

#### 2.1.1 Architecture

Claude Computer Use is a beta feature in the Claude API that provides a `computer_20251124` tool type. The agent loop is **implemented by the calling application**, not by Anthropic. The developer is responsible for screenshot capture, mouse/keyboard execution, and loop termination. Anthropic provides a reference Docker image (`anthropic-quickstarts:computer-use-demo-latest`) with Xvfb + Mutter + Tint2 for Linux headless operation.

#### 2.1.2 Primitives

| Action | Description |
|--------|-------------|
| `screenshot` | Capture current display (requires `output` param for path) |
| `left_click [x,y]` | Click at absolute coordinates |
| `right_click [x,y]` | Right-click |
| `double_click [x,y]` | Double-click |
| `triple_click [x,y]` | Triple-click |
| `mouse_move [x,y]` | Move cursor |
| `mouse_location` | Query current cursor position |
| `left_click_drag [x1,y1,x2,y2]` | Drag from one coordinate to another |
| `left_mouse_down` / `left_mouse_up` | Fine-grained press/release |
| `type "text"` | Type a text string |
| `key "ctrl+s"` | Press key or key combination |
| `hold_key "shift" duration_s` | Hold a key for duration |
| `scroll direction amount` | Scroll (direction: up/down/left/right, amount: integer) |
| `wait seconds` | Pause between actions |

#### 2.1.3 Coordinate System

Claude counts pixels from the **top-left corner** of the display. Training required teaching the model to count pixels accurately — a non-trivial capability for LLMs. The API requires `display_width_px` and `display_height_px` in the tool definition so the model knows the coordinate space. Screenshots are downscaled before being sent to the model (e.g., a 3456×2234 Retina display captures at ~1372×887).

#### 2.1.4 Safety

Computer use is gated behind a beta flag (`betas=["computer-use-2024-10-22"]` or `computer-use-2025-11-24`). The model can be blocked from certain apps via a denied-apps list. Claude Code CLI (macOS) requires user enablement in Settings → General. The denied-apps list is configurable in Desktop app Settings. Auto-unhide toggle is optional in Desktop, always on in CLI.

#### 2.1.5 Key Limitation

Claude's view is a **"flipbook" of screenshots** — not a continuous video stream. It can miss short-lived actions or notifications that appear and disappear between frames. The agent must actively request screenshots at decision points.

---

### 2.2 Claude Cowork (Anthropic)

**Type:** Proprietary desktop product (research preview, January 2026)  
**Documentation:** [claude.com/product/cowork](https://www.anthropic.com/product/claude-cowork)

#### 2.2.1 Relationship to Computer Use

Cowork uses the **same agentic architecture that powers Claude Code**, now accessible within the Claude Desktop app. Computer use in Cowork is a last-resort capability: Claude "reaches for your connectors and integrations first, falls back to your browser when needed, and only uses your screen as a last resort." This prioritization reflects the inherent imprecision and cost of screen-coordinate automation compared to direct API integration.

#### 2.2.2 Architecture

- **Runs on desktop** — Claude operates on the local machine, reading/writing files natively without manual upload/download.
- **Sub-agent coordination** — Complex work is divided into smaller tasks with parallel workstreams. Multiple Cowork sub-agents can work simultaneously.
- **Projects** — Persistent, self-contained workspaces with their own files, links, instructions, and memory for recurring or long-running tasks.
- **Mobile access** — Pro/Max users can message Claude from their phone while the desktop stays active; results are delivered back to the same conversation.
- **Claude in Chrome** — Browser extension that pairs with Cowork to automate web tasks. Claude can click, fill, navigate, and screenshot within Chrome.

#### 2.2.3 Extensibility

Cowork supports MCP connectors (linking to external services and data sources), custom instructions (teaching reusable workflows), and bundleable packages (skills + connectors + sub-agents packaged as shareable units).

#### 2.2.4 Safety Model

Built with human oversight: "Claude completes tasks, but consequential decisions remain with the user." Anthropic's agent safety research (trust, access, control) applies. No seat licensing — consumed through existing AWS/Anthropic billing.

---

### 2.3 Claude Design (Anthropic Labs)

**Type:** Proprietary visual design product (research preview, April 2026)  
**Model:** Claude Opus 4.7  
**Documentation:** [claude.ai/design](https://www.anthropic.com/news/claude-design-anthropic-labs)

#### 2.3.1 Architecture

Claude Design is a canvas-based AI design tool built on Claude Opus 4.7. It is **not a computer use system** in the traditional sense — it does not take screenshots of an existing desktop or drive mouse/keyboard at coordinates. Instead, it generates designs on a canvas from text prompts, then iteratively refines through conversation and inline comments.

#### 2.3.2 Design System Extraction

During onboarding, Claude reads the user's codebase, Figma files, font folders, logo assets, and GitHub repositories to **automatically build a design system** — extracting brand colors, typography, spacing tokens, and components. Every subsequent project inherits this system automatically.

#### 2.3.3 Claude Code Handoff

The key integration point is the **Claude Code handoff bundle**: when a design is ready to build, Claude Design packages it as a machine-readable spec (component structure, design tokens, layout hierarchy, asset references) that Claude Code reads natively. This is not a PNG or Figma URL — it is a structured spec that the coding agent can act on directly, enabling a prototype-to-production workflow without a translation step.

#### 2.3.4 Relevance to Desktop Automation

Claude Design is relevant to this slice only insofar as it demonstrates a **design-to-code pipeline** using structured specs rather than screenshots. Its automated design system extraction (reading actual code to extract tokens) is a model for how agents can understand existing UI without pixel-counting.

---

### 2.4 Devin (Cognition)

**Type:** Proprietary cloud product  
**Documentation:** [docs.devin.ai](https://docs.devin.ai/)

#### 2.4.1 Full Desktop Testing Architecture

Devin's full desktop testing mode (announced 2026) uses **computer use to test any desktop app that can run on Linux**. Devin runs the app, uses its desktop (via VNC) to click around, and sends the user an edited video recording of the test session.

#### 2.4.2 VM-Based Isolation

Devin operates in an **isolated cloud VM** with its own filesystem, browser, terminal, and now desktop environment. The VM boundary provides security isolation (the agent cannot directly access the user's host machine) and reproducibility (the environment is defined and controlled).

#### 2.4.3 Video Recording Features

- **Annotation** — Text labels appear at key moments marking what Devin is testing
- **Auto-zoom** — Video zooms into where Devin clicks and interacts, panning to follow the cursor
- **Processing** — Raw recordings are processed to highlight important actions and compress idle time
- **Sent as attachments** — Videos attach to messages in the Devin webapp or Slack

#### 2.4.4 Interactive Browser (VNC)

Devin's browser use operates over VNC with <50ms latency streaming. A "reconnecting screen" shows when the VNC stream recovers from brief drops. This is a continuous video stream (not flipbook screenshots) — more responsive than Claude Computer Use but requiring cloud infrastructure.

#### 2.4.5 Delegation Model

Devin can orchestrate a **team of managed Devins** in parallel, each with its own isolated VM. The main session acts as coordinator: scoping work, monitoring progress, resolving conflicts, compiling results. Up to 10 workers per manager.

---

### 2.5 anomalyco/opencode

**Type:** Open source (MIT)  
**Desktop approach:** Electron app + Tauri desktop binary  
**Relevant primitives:** File system access, clipboard, system notifications

#### 2.5.1 Desktop Integration

OpenCode's desktop app (Electron/Tauri) is primarily a **GUI wrapper around the TUI/server architecture**. It does not expose screen-coordinate automation primitives to the agent. The desktop app communicates with the opencode server via WebSocket, and the agent runs server-side.

#### 2.5.2 Computer Use / Screenshot Capabilities

OpenCode has **no built-in computer use or desktop automation tools**. The matches found in the repository are overwhelmingly documentation references to "desktop app" as a deployment surface, not as an automation target. The agent cannot control the mouse, take screenshots of the host display, or interact with native GUI applications.

#### 2.5.3 OS-Level Access

OpenCode's OS-level access is limited to:
- **File system** — Full read/write access to project files
- **Clipboard** — Via Electron's clipboard module (desktop app only)
- **Shell** — Via the `bash` tool (server-side, not host-desktop)
- **Notifications** — Desktop system notifications when responses are ready

#### 2.5.4 Verdict

OpenCode's desktop app is a **deployment surface**, not an automation surface. It is not suitable for desktop automation tasks requiring GUI interaction. Browser automation is available only via external MCP integrations (Playwright, agent-browser).

---

### 2.6 aaif-goose/goose

**Type:** Open source (MIT)  
**Desktop approach:** Native Rust + Electron desktop client  
**Computer control:** `goose-mcp` crate with `computercontroller` module

#### 2.6.1 Architecture

Goose's desktop automation lives in `crates/goose-mcp/src/computercontroller/`. This is a **MCP server** (using the `rmcp` SDK) that provides two primary tools: `automation_script` and `computer_control`.

#### 2.6.2 Platform Abstraction Layer

```
platform/mod.rs
├── create_system_automation() -> Box<dyn SystemAutomation>
├── SystemAutomation trait:
│   ├── execute_system_script(script: &str) -> Result<String>
│   ├── get_shell_command() -> (&'static str, &'static str)
│   ├── get_temp_path() -> PathBuf
│   └── has_display() -> bool
```

Three platform implementations:
- **macOS** — Peekaboo CLI (Homebrew auto-install), Accessibility + Screen Recording TCC permissions
- **Linux (X11)** — `xdotool` for click/type/key, `wmctrl` for window management, `xclip` for clipboard, Python intermediary for complex sequences
- **Linux (Wayland)** — `wtype` for text/key, `wl-copy`/`wl-paste` for clipboard

#### 2.6.3 Tools

**`automation_script`** — Create and run small scripts (Shell/batch/Ruby/PowerShell) for automation tasks. Supports multi-line scripts that execute sequentially via Python intermediary on Linux.

**`computer_control`** — Platform-specific UI automation:
- **macOS** — Full Peekaboo CLI passthrough: `see` (annotated UI maps with element IDs), `click` (by element ID or coordinates), `type`, `press`, `hotkey`, `paste`, `scroll`, `drag`, `swipe`, `move`, `app`/`window`/`dock`/`menu` management, `clipboard`, `permissions status`
- **Linux** — Shell-based: `click`, `type`, `key`, `activate`, clipboard get/set; limited compared to Peekaboo

#### 2.6.4 Screenshot in Goose

No dedicated screenshot tool in `computercontroller`. Screenshots are implicitly supported via the `capture_screenshot: true` flag on `computer_control` actions (macOS only). On Linux, the agent must use `automation_script` with a tool like `scrot` or `gnome-screenshot`.

#### 2.6.5 Security

Goose's `computercontroller` is explicitly listed in the **adversary inspector's denylist** (`DEFAULT_TOOLS` blocklist includes `computercontroller__automation_script`). This indicates Goose's developers are aware that script execution + OS-level automation is a high-severity capability requiring containment.

#### 2.6.6 Desktop Surface

Goose also ships an **Electron desktop app** (`ui/desktop/`) that wraps the Rust backend. The desktop app can launch `goosed` (the server process) with full environment access, and the Electron layer handles UI rendering.

---

### 2.7 badlogic/pi-mono

**Type:** Open source (MIT)  
**Desktop approach:** Multi-surface (CLI, TUI, Web, Slack)  
**Relevant primitives:** Clipboard, path utilities, keyboard shortcuts

#### 2.7.1 Architecture

pi-mono's `pi-agent-core` is surface-agnostic — the same core powers CLI, TUI, Web, and Slack surfaces. Desktop interaction is primarily via the **TUI surface** and **clipboard operations**. The `packages/coding-agent/src/utils/` directory contains platform-specific clipboard handling.

#### 2.7.2 OS-Level Access

pi-mono's OS-level access is limited to:
- **Clipboard** — `copyToClipboard()` with platform detection: macOS (`pbcopy`/`pbpaste`), Linux (X11: `xclip`/`xsel`; Wayland: `wl-copy`/`wl-paste`; Termux: `termux-clipboard-set`), Windows (PowerShell `Get-Clipboard`/`Set-Clipboard`), and a native `@mariozechner/clipboard` Node module
- **Keyboard shortcuts** — TUI-level Kitty keyboard protocol support (key release detection, compose key, non-Latin layouts)
- **macOS screenshot path handling** — Path utilities handle macOS screenshot Unicode filenames (NFD é, curly apostrophes, narrow no-break spaces)

#### 2.7.3 Verdict

pi-mono has **no native desktop automation** — no screen capture, no mouse control, no window management. The TUI surface handles terminal input (keyboard, mouse SGR sequences for terminal apps), not GUI desktop apps. Its "desktop surface" is the web-based UI (`packages/web-ui/`), not a native OS GUI.

---

### 2.8 code-yeongyu/oh-my-openagent

**Type:** Open source  
**Desktop approach:** Built on OpenCode desktop (Tauri)  
**Relevant primitives:** Playwright CLI, agent-browser, dev-browser skills

#### 2.8.1 Browser Automation (Not Desktop Automation)

oh-my-openagent's automation is primarily **browser-based** via Playwright MCP and the `dev-browser`/`agent-browser` skills. These provide screenshot capabilities (`agent-browser screenshot`, `agent-browser screenshot --annotate`), but these are browser-app screenshots, not OS desktop screenshots.

#### 2.8.2 Screenshot Capabilities

The `agent-browser` skill supports:
- `screenshot` / `screenshot path.png` / `screenshot --full` / `screenshot --annotate`
- Annotated screenshots overlay numbered labels `[N]` on interactive elements
- Visual diffing: `diff screenshot --baseline before.png`
- URL diffing with screenshot: `diff url https://v1.com https://v2.com --screenshot`

#### 2.8.3 Verdict

oh-my-openagent wraps OpenCode's desktop app but does not extend it with GUI automation primitives. Browser automation via Playwright is well-developed, but OS-level desktop control (mouse/keyboard/screenshots of the host desktop) is absent.

---

### 2.9 chauncygu/collection-claude-code-source-code

**Type:** Claude Code open source reconstruction  
**Computer use:** Full Claude Computer Use integration

#### 2.9.1 Architecture

This is a **mirrored reconstruction of Claude Code's computer use system**, not the original source. The `computerUse/` directory in `src/utils/` contains the full implementation:

- `executor.ts` — CLI `ComputerExecutor` wrapping two native modules:
  - `@ant/computer-use-input` (Rust/enigo) — mouse, keyboard, frontmost app
  - `@ant/computer-use-swift` — SCContentFilter screenshots, NSWorkspace apps, TCC
- `swiftLoader.ts` — Lazy-loads `@ant/computer-use-swift` (macOS only)
- `inputLoader.ts` — Lazy-loads `@ant/computer-use-input`
- `hostAdapter.ts` — Host detection (terminal sentinel, surrogate host for macOS)
- `setup.ts` — `buildComputerUseTools()` + `setupClaudeInChrome()`
- `mcpServer.ts` — MCP server for `computer-use` tools
- `gates.ts` — Feature flags (`mouseAnimation`, `coordinateMode`)

#### 2.9.2 Claude Code CLI vs Desktop

Claude Code has **two distinct computer use surfaces**:
- **CLI (macOS only)** — Uses `@ant/computer-use-input` (enigo) and `@ant/computer-use-swift` (TCC/Swift screenshots)
- **Desktop** — Uses Electron-based overlay with `BrowserWindow.setIgnoreMouseEvents(true)` for click-through; different executor in `packages/desktop/computer-use-mcp/src/executor.ts`

#### 2.9.3 Coordinate Handling

The executor has sophisticated coordinate handling:
- **Logical → physical → API target dims** — `computeTargetDims()` applies scale factors
- **Terminal sentinel** — The CLI executor detects the terminal emulator bundle ID and exempts it from screenshots
- **Surrogate host** — macOS uses a sentinel host bundle ID so the terminal being frontmost doesn't interfere with target app clicks
- **Downscaling** — Every screenshot is downscaled before sending to the model (no need to lower display resolution)

#### 2.9.4 Mouse Animation

A notable feature is `mouseAnimationEnabled` — enigo's `move_mouse` on macOS reads `NSEvent.pressedMouseButtons` to distinguish `.leftMouseDragged` from `.mouseMoved`. A 50ms sleep after press ensures correct event emission.

---

## 3. Cross-Cutting Comparison

### 3.1 GUI Automation Approaches

| System | Approach | Paradigm |
|--------|----------|----------|
| Claude Computer Use | Vision-based loop (screenshot → decide → act → repeat) | Screen-coordinate |
| Devin (full desktop) | VM sandbox + VNC stream + video annotation | VM + continuous stream |
| Claude Cowork | Outcome-oriented delegation + connectors + browser extension | Hybrid (API-first, screen last) |
| Claude Design | Canvas-based generation + structured spec handoff | Generative, not coordinate-based |
| Goose | OS API scripting (Peekaboo CLI / xdotool/wtype) | OS-level command |
| pi-mono | Terminal I/O + clipboard (no GUI automation) | Terminal-focused |
| OpenCode | File system + shell + clipboard (no GUI automation) | File system + shell |
| Oh My OpenAgent | Playwright browser automation + desktop app wrapper | Browser-first |
| Claude Code (reconstruction) | Full Claude Computer Use stack (Rust/enigo + Swift/TCC) | Screen-coordinate |

### 3.2 Screen Coordinate vs DOM-Based Interaction

| System | Interaction Model | Element Targeting |
|--------|-------------------|-------------------|
| Claude Computer Use | Absolute pixel coordinates | None (vision-based) |
| Devin | VNC continuous stream | None (visual) |
| Claude Cowork (browser) | DOM via browser extension | Clickable elements |
| Goose (macOS Peekaboo) | Element IDs from `see` annotation | Named element IDs (`B1`, `T2`) |
| Goose (Linux X11) | xdotool coordinates | None (coordinate-based) |
| Oh My OpenAgent (agent-browser) | DOM via Playwright | Named selectors, annotated overlays |
| playwright-skill | DOM via Playwright | Full Playwright selectors |
| Claude Design | Canvas generation | Not applicable |

**Key insight:** The most precise automation systems use **element ID targeting** (Peekaboo's `see --annotate` → `click --on B3`) rather than raw coordinates. This requires a UI inspection step (`see` or `screenshot --annotate`) before interaction, but dramatically improves reliability over coordinate-based clicking.

### 3.3 OS-Level Integration

| System | Depth | Platform-specific |
|--------|-------|-----------------|
| Claude Computer Use | Low — calls your app's implementation | You implement it |
| Devin | High — isolated VM with full OS | Cloud-hosted Linux |
| Claude Cowork | Medium — filesystem + connectors | Desktop app + mobile |
| Claude Design | N/A — generative, not OS-level | Electron + Rust canvas |
| Goose | High — Peekaboo CLI / xdotool / wtype | macOS, X11, Wayland |
| pi-mono | Low — clipboard + keyboard shortcuts | Cross-platform terminal |
| OpenCode | Low — file system + shell | Cross-platform (Electron/Tauri) |
| Oh My OpenAgent | Low — browser automation via Playwright | Cross-platform browser |
| Claude Code (reconstruction) | High — enigo (Rust) + TCC/Swift (macOS) | macOS primary |

### 3.4 Desktop Environment Support

| System | macOS | Linux X11 | Linux Wayland | Windows |
|--------|-------|-----------|---------------|---------|
| Claude Computer Use | ✅ (your impl) | ✅ (your impl) | ✅ (your impl) | ✅ (your impl) |
| Devin | ❌ | ✅ (cloud VM) | ❌ | ❌ |
| Claude Cowork | ✅ | ❌ | ❌ | ❌ |
| Claude Design | ✅ (Electron) | ✅ (Electron) | ✅ (Electron) | ✅ (Electron) |
| Goose (Peekaboo/xdotool) | ✅ | ✅ | ✅ (partial) | ✅ (PowerShell) |
| pi-mono | ✅ | ✅ | ✅ | ✅ |
| OpenCode | ✅ (Electron) | ✅ (Tauri) | ✅ (Tauri) | ✅ (Tauri) |
| Oh My OpenAgent | ✅ (Tauri) | ✅ (Tauri) | ✅ (Tauri) | ✅ (Tauri) |
| Claude Code CLI | ✅ | ❌ | ❌ | ❌ |

**Note:** Goose's Linux Wayland support is partial — `wtype` supports text/key input but not click/mouse movements (no Wayland equivalent of `xdotool click`). Window management tools (`wmctrl`) require X11.

### 3.5 Agent Access to Mouse/Keyboard/Screenshots

| System | Mouse | Keyboard | Screenshot | Notes |
|--------|-------|----------|------------|-------|
| Claude Computer Use | ✅ `mouse_move`, `left_click`, etc. | ✅ `type`, `key`, `hold_key` | ✅ (you implement capture) | Developer implements the execution loop |
| Devin | ✅ via VNC in cloud VM | ✅ via VNC | ✅ continuous stream | VM sandboxed in cloud |
| Claude Cowork | ✅ (Computer Use) | ✅ (Computer Use) | ✅ (Computer Use) | Last resort; connectors preferred |
| Claude Design | ❌ | ❌ | ❌ | Canvas-based; no screen interaction |
| Goose (macOS) | ✅ via Peekaboo | ✅ via Peekaboo | ✅ via Peekaboo (`image`, `capture`) | Full Peekaboo CLI via `computer_control` |
| Goose (Linux X11) | ✅ via xdotool | ✅ via xdotool | ❌ (manual via script) | No native screenshot tool |
| Goose (Linux Wayland) | ❌ | ✅ via wtype | ❌ (manual via script) | Wayland blocks mouse automation |
| pi-mono | ✅ (terminal mouse SGR) | ✅ (Kitty keyboard protocol) | ❌ | Terminal I/O only |
| OpenCode | ❌ | ❌ | ❌ | No GUI automation |
| Oh My OpenAgent | ❌ | ❌ | ✅ (browser screenshots) | Browser automation only |
| Claude Code CLI (macOS) | ✅ via enigo | ✅ via enigo | ✅ via Swift TCC | Rust/enigo + Swift/TCC stack |

### 3.6 Safety & Trust Models

| System | Safety Model | Isolation |
|--------|-------------|-----------|
| Claude Computer Use | Beta flag, denied-apps list, user enablement | Application-level (you implement the loop) |
| Devin | VM sandbox, user approval for QA | Cloud VM isolation |
| Claude Cowork | User oversight, Anthropic agent safety research | Desktop app process |
| Claude Design | User-driven handoff, consequential decisions with user | Not applicable |
| Goose | Adversary inspector blocks `computercontroller__automation_script` by default | MCP tool-level |
| pi-mono | No GUI automation; TUI-only | Not applicable |
| OpenCode | No GUI automation | Not applicable |
| Oh My OpenAgent | No GUI automation | Not applicable |

---

## 4. ACE Recommendation

### 4.1 Core Recommendation: Adopt Goose's `computercontroller` Pattern

Goose's approach is the most **architecturally对齐** with ACE's single-binary model. It is:
- **Self-contained** — The MCP server ships as part of the goose binary
- **Cross-platform** — Peekaboo (macOS), xdotool (X11), wtype (Wayland), PowerShell (Windows)
- **Tiered** — `automation_script` (arbitrary scripting) is high-power; `computer_control` (structured commands) is safer
- **MCP-native** — Fits ACE's MCP integration architecture

### 4.2 Detailed Recommendations

| System | Recommendation | Rationale |
|--------|---------------|-----------|
| **Goose `computercontroller`** | **ADOPT** | Cross-platform OS automation via MCP. Peekaboo on macOS provides element-ID-based precision. Fits ACE's MCP-native architecture. The tiered `automation_script` + `computer_control` model provides both power and safety. |
| **Claude Computer Use API pattern** | **ADOPT** (design) | The screen-coordinate + screenshot loop is the right abstraction for ACE's tool interface. ACE should define a `computer_20251124`-compatible tool type. The developer Implements the execution side. This is the right split. |
| **Peekaboo CLI (macOS)** | **ADOPT** (macOS target) | Element-ID-based automation with annotated UI maps is significantly more reliable than coordinate-based clicking. Auto-installed via Homebrew. The best macOS GUI automation available. |
| **xdotool/wtype (Linux)** | **ADAPT** | Functional but crude. Wayland has no equivalent of `xdotool click`. ACE should wrap these with a screenshot-first workflow (agent sees screen before acting) to mitigate coordinate errors. |
| **PowerShell automation (Windows)** | **ADAPT** | Viable but heavy (requires PowerShell). ACE should consider `uio` or similar lightweight Windows automation libraries. Coordinate-based with no element inspection. |
| **Devin VM sandboxing** | **AVOID** (for ACE core) | Cloud VM model is antithetical to ACE's single-binary, edge-deployed design. The video annotation feature is valuable for UX but requires cloud infrastructure. Not recommended for ACE's architecture. |
| **Claude Cowork sub-agents** | **ADOPT** (design pattern) | Outcome-oriented task decomposition with parallel sub-agents is the right mental model for ACE's delegation. "Connectors first, screen last" priority is correct. |
| **Claude Design handoff bundle** | **ADAPT** | Structured spec-based handoff (not PNG/Figma URL) is the right approach for design-to-code pipelines in ACE. The design system auto-extraction is worth studying for potential UI generation features. |
| **pi-mono clipboard system** | **ADOPT** (clipboard only) | Platform-detecting clipboard with fallback chains (pbcopy → xclip → wl-paste → clipboard native module) is well-engineered. ACE should replicate this cross-platform clipboard pattern. |
| **playwright-skill browser automation** | **ADOPT** (browser layer) | For ACE's browser automation needs, playwright-skill's model-invoked dynamic code generation is more flexible than static Playwright MCP commands. Combined with agent-browser's annotation, this covers web automation well. |
| **OpenCode desktop app** | **AVOID** | The desktop app is a deployment surface, not an automation surface. No GUI control primitives. ACE should not model itself on OpenCode's desktop app for automation purposes. |
| **Claude Code enigo + TCC stack** | **ADOPT** (research) | The Rust/enigo + Swift/TCC stack in the Claude Code reconstruction is the most complete open reference for cross-platform input simulation. Worth studying for ACE's own input automation implementation. |

### 4.3 Priority Implementation Path

1. **Phase 1 (MVP):** ACE MCP server with `automation_script` tool — Shell on Unix, PowerShell on Windows. Ships with the single binary. Lowest common denominator.
2. **Phase 2 (macOS):** Peekaboo CLI integration via `computer_control` tool. Element-ID-based precision once `see --annotate` identifies targets.
3. **Phase 3 (Linux):** Screenshot-first workflow: agent always calls `screenshot` before `computer_control` to visually verify state. Wraps `xdotool`/`wtype` with coordinate verification.
4. **Phase 4 (UX):** Video recording + annotation for task demonstration (inspired by Devin). This is a display feature, not a core automation requirement.
5. **Phase 5 (Design):** Design system extraction + Claude Code handoff bundle format for any potential UI generation features. Long-horizon.

---

## 5. Architectural Implications for ACE

### 5.1 Tool Interface Design

ACE should define a **standard computer automation tool interface** (compatible with `computer_20251124` shape):
```typescript
interface ComputerTool {
  // Screenshot — always available
  screenshot(options?: { path?: string; fullPage?: boolean }): Promise<string>
  // Mouse
  mouse_move(x: number, y: number): Promise<void>
  left_click(x: number, y: number): Promise<void>
  right_click(x: number, y: number): Promise<void>
  double_click(x: number, y: number): Promise<void>
  mouse_move(x1: number, y1: number, x2: number, y2: number): Promise<void> // drag
  // Keyboard
  type(text: string): Promise<void>
  key(combo: string): Promise<void> // e.g. "ctrl+s"
  hold_key(key: string, duration_s: number): Promise<void>
  // Utility
  get_clipboard(): Promise<string>
  set_clipboard(text: string): Promise<void>
}
```

### 5.2 Platform Abstraction

ACE's platform abstraction should mirror Goose's `SystemAutomation` trait — a `dyn SystemAutomation` trait with platform implementations. ACE should implement a **screenshot-first workflow** where the agent always sees the current screen state before taking action, mitigating coordinate errors especially on Linux.

### 5.3 Security Model

The `computercontroller` being on Goose's denylist by default is a strong signal: **GUI automation tools should be opt-in, not default**. ACE should require explicit user enablement for computer automation tools, with a clear permission model: what can the agent do, what requires user confirmation, what is blocked.

### 5.4 Cowork's "Connectors First" Priority

Claude Cowork's hierarchy (connectors → browser → screen) reflects a correct priority order:
1. **Direct API integration** (fastest, most precise)
2. **Browser automation** (web apps, authenticated flows)
3. **Screen automation** (last resort, GUI-only tools)

ACE should follow this same priority hierarchy in its tool design: file system and shell tools are always available; browser automation via Playwright MCP is the next layer; OS-level screen automation is the final layer for GUI-only tools.

---

## 6. Research Gaps & Future Work

1. **Wayland mouse automation** — No viable solution exists for mouse control on Wayland (no equivalent of `xdotool`). ACE should monitor `wtype` development and potentially contribute to Wayland protocol proposals.
2. **Element inspection on Linux** — macOS Peekaboo's `see --annotate` is the gold standard. Linux lacks an equivalent. ACE could explore `ydotool` + a screenshot annotation pipeline.
3. **Video stream vs flipbook** — Devin's continuous VNC stream is more responsive but requires cloud infrastructure. For edge deployment, the flipbook approach (Claude style) is more practical but misses short-lived events.
4. **Cross-platform input simulation** — The Rust `enigo` crate is the most portable input simulation library (macOS/X11/Wayland/Windows). ACE should evaluate it as the basis for cross-platform mouse/keyboard automation.
