# Browser Automation & Frontend Testing

**Slice:** 7  
**Unit:** agents-study  
**Output:** `design/units/agents-study/study/browser-automation.md`

---

## 1. Introduction

Browser automation in AI agent systems spans a wide spectrum: from model-invoked dynamic Playwright code generation to MCP-based web tool interoperability to generative UI synthesis. This document cross-cuts seven systems plus Google Stitch, analyzing browser automation along five axes: automation approaches, screenshot/DOM interaction, progressive disclosure, tool generation for web tasks, and comparison of architectural paradigms.

**Key finding:** Most systems do NOT have native browser automation. True browser control (DOM access, tab management, screenshot with interaction) exists in only three systems: playwright-skill, OpenClaw, and Oh My OpenAgent. The rest use information retrieval tools (`webfetch`/`websearch`) or are browser-embedded UIs that lack programmatic control.

---

## 2. System-by-System Analysis

### 2.1 playwright-skill (lackeyjb/playwright-skill)

**Approach:** Model-invoked dynamic code generation  
**Architecture:** Universal executor (`run.js`) + progressive disclosure documentation

The skill works by having the LLM write custom Playwright JavaScript code per request, then execute it via a universal Node.js runner. The model generates Playwright scripts at request time rather than using pre-built commands.

**Key components:**
- `run.js` — Universal executor accepting file path, inline code, or stdin
- `lib/helpers.js` — Utility layer: `detectDevServers()`, `safeClick()`, `safeType()`, `takeScreenshot()`, `handleCookieBanner()`, `createContext()`, `retryWithBackoff()`
- `SKILL.md` — Entry-level documentation (450 lines)
- `API_REFERENCE.md` — Comprehensive Playwright API reference (653 lines)

**Execution flow:**
1. Agent calls `detectDevServers()` to find localhost dev servers
2. Agent writes Playwright script to `/tmp/playwright-test-*.js` (parameterized URL)
3. Agent executes via `cd $SKILL_DIR && node run.js /tmp/playwright-test-*.js`
4. Temp files auto-cleaned on next run

**Progressive disclosure:**
- Level 1 (SKILL.md): Common patterns, execution workflow, available helpers
- Level 2 (API_REFERENCE.md): Full Playwright API, selectors, network interception, CI/CD
- The model only loads API_REFERENCE.md when advanced features needed

**DOM interaction:** Full Playwright API — `page.goto()`, `page.fill()`, `page.click()`, `page.waitForSelector()`, `page.screenshot()`, `page.evaluate()` (arbitrary JS injection)

**Screenshot approach:** Uses Playwright's native `page.screenshot()` with `fullPage: true` option. Screenshots saved to `/tmp` with timestamped names via `helpers.takeScreenshot()`.

**Security model:** Scripts run in skill directory context, `process.chdir(__dirname)` ensures proper module resolution. Custom headers via `PW_HEADER_NAME`/`PW_HEADER_VALUE` env vars allow backend identification. Scripts are ephemeral (written to `/tmp`).

---

### 2.2 OpenClaw (openclaw/openclaw)

**Approach:** Built-in browser extension + QA Lab plugin for full automation  
**Architecture:** Plugin-based, browser control via Chrome DevTools Protocol (CDP)

OpenClaw has a bundled `browser` extension that provides `browser.open`, `browser.snapshot`, and `browser.act` tools. Additionally, the QA Lab extension provides deeper browser control for testing scenarios.

**Browser extension components (`extensions/browser/`):**
- `browser-control-auth.ts` — Authentication for browser control
- `browser-cdp.ts` — CDP connection management
- `browser-bridge.ts` — Bridge between gateway and browser
- `browser-runtime-api.ts` — Runtime API surface
- `cli/browser-cli.ts` — CLI for manual browser control

**QA Lab browser runtime (`extensions/qa-lab/src/browser-runtime.ts`):**
```
callQaBrowserRequest()   — HTTP request through gateway
qaBrowserOpenTab()        — Open new tab (POST /tabs/open)
qaBrowserSnapshot()       — Get DOM snapshot (GET /snapshot, format: "ai" | "aria")
qaBrowserAct()            — Perform action (POST /act)
waitForQaBrowserReady()   — Poll until CDP ready
```

**Snapshot format options:**
- `"ai"` — AI-optimized snapshot format
- `"aria"` — ARIA accessibility tree

**Act request types:** click, doubleClick, hover, fill, select, drag, key press, scroll, screenshot, waitForSelector

**DOM interaction:** Via QA Lab's `browserAct` with detailed action requests. Supports element targeting by selector, text content, or ARIA roles.

**Screenshot approach:** `qaBrowserAct` with `kind: "screenshot"` action. Also supports full-page screenshots. Screenshots returned as binary through gateway.

**Security model:** 
- SSRF protection via `ssrf-policy-helpers.ts`
- Secret comparison via `safeEqualSecret()`
- Path guards for filesystem safety
- WebSocket secure random for auth tokens
- Browser process execution isolated in `node-host/`

---

### 2.3 pi-mono (badlogic/pi-mono)

**Approach:** Browser-embedded agent, not browser controller  
**Architecture:** Web surface (`packages/web-ui`) embeds agent in user's browser

pi-mono's web surface (`packages/web-ui/src/tools/extract-document.ts`) operates differently from other systems. The agent is embedded in the user's browser and can read/modify the current active tab via JavaScript injection. This is NOT browser automation in the traditional sense — it's in-browser tool execution.

**Key tool:** `extract_document` — Downloads and extracts text from PDF/DOCX/XLSX/PPTX via URL with CORS proxy fallback. Max 50MB.

**DOM interaction approach:** The agent uses `browser_javascript` tool (referenced in session transcripts) to inject JavaScript into the current tab. However, this is a browser-embedded AI assistant, NOT a separate browser automation system.

**Screenshot approach:** None. pi-mono does not provide screenshot tools. The web UI is for chat-based interaction with documents and code.

**Limitation:** No programmatic browser control beyond document extraction. Cannot open tabs, navigate, fill forms, or take screenshots of arbitrary pages.

---

### 2.4 OpenCode (anomalyco/opencode)

**Approach:** Information retrieval only — no browser automation  
**Architecture:** `webfetch` and `websearch` tools for content retrieval

OpenCode's "browser" capabilities are limited to fetching web content and searching. There are no tools for opening browsers, taking screenshots, or interacting with DOM elements.

**Available tools:**
- `webfetch` — Fetch a specific URL (content retrieval)
- `websearch` — Web search query
- `codesearch` — Code search (ecosystem plugin)

**DOM interaction:** None. OpenCode does not interact with live DOMs.

**Screenshot approach:** None.

**Security:** Permission filtering in `plan` mode restricts `webfetch` to allowed URLs.

---

### 2.5 Goose (aaif-goose/goose)

**Approach:** MCP-based browser extensions via Model Context Protocol  
**Architecture:** Rust-based core with MCP extension system

Goose does not have native browser automation. Browser capabilities are delivered through MCP extensions that provide web tools.

**MCP integration patterns:**
- Extensions configured via `ExtensionConfig::streamable_http()`
- MCP servers loaded via `uvx` or `npx`
- Example: `mcp-server-fetch` for web fetching, GitHub MCP for repository operations

**Browser-related MCP usage:**
- Web content via MCP fetch server
- Docs viewing via `docs-viewer.ts`
- No direct browser control tools

**DOM interaction:** None directly. MCP provides tool-based access to remote services.

**Screenshot approach:** None in core. Goose's desktop app provides OS-level integration but not browser control.

---

### 2.6 Oh My OpenAgent (code-yeongyu/oh-my-openagent)

**Approach:** MCP-based Playwright integration with skill routing  
**Architecture:** Skill system with `BrowserAutomationProvider` enum and provider-gated skills

Oh My OpenAgent integrates `@anthropic-ai/mcp-playwright` as a built-in MCP. The skill system has a `browserProvider` configuration that gates which browser automation skill is available.

**Browser automation configuration:**
```typescript
// src/plugin/skill-context.ts
const PROVIDER_GATED_SKILL_NAMES = new Set(["agent-browser", "playwright"])
const browserProvider: BrowserAutomationProvider = 
  pluginConfig.browser_automation_engine?.provider ?? "playwright"
```

**Available browser providers:**
- `"playwright"` — Uses `@anthropic-ai/mcp-playwright`
- `"agent-browser"` — Alternative browser provider skill

**MCP Playwright tools (from test analysis):**
- `browser_type` — Select browser type
- `browser_navigate` — Navigate to URL
- `browser_click` — Click element
- Additional tools via MCP protocol

**DOM interaction:** Via MCP protocol to `@anthropic-ai/mcp-playwright` server. The model invokes MCP tools rather than generating code directly.

**Screenshot approach:** Presumably via Playwright MCP (screenshots are standard Playwright capability).

**Security:** Skills are filtered by `browserProvider` — only the configured provider's skills are available.

---

### 2.7 Claude Code (chauncygu/collection-claude-code-source-code)

**Approach:** Web search/fetch + clipboard screenshot — no browser control  
**Architecture:** SDK with web search/request tracking

Claude Code has NO browser automation. Its "browser" references are about:
- Web search requests tracking (`webSearchRequests` in cost tracking)
- Web fetch requests tracking
- Screenshot clipboard integration (`screenshotClipboard.ts`)

**Browser-related capabilities:**
- `web_search_tool_result` / `web_fetch_tool_result` — Tool result types
- Image handling for screenshots pasted from clipboard
- No ability to open browsers, navigate pages, or interact with DOM

**DOM interaction:** None.

**Screenshot approach:** Claude Code can paste screenshots from clipboard, but this is for user-to-agent image sharing, NOT browser page capture. No programmatic screenshot of browser pages.

---

### 2.8 Google Stitch (google-labs-code/stitch-sdk)

**Approach:** Generative UI — UI generation from prompts, NOT browser automation  
**Architecture:** MCP server + SDK + AI-native design canvas

Google Stitch is fundamentally different from all other systems. Rather than automating existing browsers, it GENERATES new UI designs from natural language prompts. It's a generative AI design tool, not a browser automation system.

**Key capabilities:**
- Text-to-UI: Generate complete interfaces from natural language
- Image-to-UI: Transform sketches/wireframes to digital designs
- Voice canvas: "Vibe Design" with voice commands
- Design agent: Tracks project evolution, manages multiple agents
- Export: HTML/CSS, Figma, AI Studio, Antigravity

**Stitch tools (`stitchTools()`):**
- `create_project` — Create new project
- `generate_screen_from_text` — Generate UI from prompt
- `get_screen` — Retrieve screen by ID
- `Screen.edit()` — Edit screen with text prompt
- `Screen.variants()` — Generate design variants
- `Screen.getHtml()` — Get HTML download URL
- `Screen.getImage()` — Get screenshot download URL

**Architecture comparison:** Stitch generates UIs; others automate browsers. Stitch produces artifacts (HTML/CSS); others interact with live pages.

**Screenshot approach:** Stitch CAPTURES its own generated designs via `Screen.getImage()`. This is screenshot-of-generated-design, not screenshot-of-live-site.

---

## 3. Cross-Cutting Comparison

### 3.1 Automation Approaches

| System | Approach | Execution Model | Browser Instance |
|--------|----------|-----------------|------------------|
| playwright-skill | Dynamic code generation | Model writes JS → `run.js` executor | New browser per task |
| OpenClaw | CDP-based control | Gateway → browser extension → CDP | Persistent browser(s) |
| pi-mono | In-browser injection | Agent embedded in user's browser | User's current tab |
| OpenCode | HTTP fetch only | `webfetch` tool → HTTP GET | None |
| Goose | MCP tool protocol | MCP servers → remote tools | None |
| Oh My OpenAgent | MCP Playwright | `@anthropic-ai/mcp-playwright` | MCP server's browser |
| Claude Code | Search/fetch only | `web_search/fetch` tools | None |
| Google Stitch | Generative UI | Prompt → Gemini 2.5 → HTML/CSS | None (generates) |

**Architecture spectrum:**
1. **Dynamic code generation** (playwright-skill) — Most flexible, requires sandboxing
2. **CDP-based control** (OpenClaw) — Deepest browser access, highest complexity
3. **MCP-based** (Oh My OpenAgent, Goose) — Standardized tool protocol, less flexibility
4. **In-browser agent** (pi-mono) — User's browser context, no automation
5. **Information retrieval only** (OpenCode, Claude Code) — No browser control

### 3.2 Screenshot & DOM Interaction

| System | Screenshots | DOM Access | Action Scope |
|--------|-------------|------------|--------------|
| playwright-skill | Yes (`page.screenshot()`) | Full Playwright API | Complete browser control |
| OpenClaw | Yes (via `browserAct`) | Snapshot + actions | Complete via CDP |
| pi-mono | No | Via `browser_javascript` injection | Current tab only |
| OpenCode | No | No DOM | Information retrieval only |
| Goose | No | No DOM | MCP tool access |
| Oh My OpenAgent | Yes (via Playwright MCP) | Via Playwright MCP | MCP tool scope |
| Claude Code | Clipboard paste only | No DOM | None |
| Google Stitch | Of generated designs only | N/A | Generates, doesn't automate |

**DOM interaction depth ranking:**
1. **OpenClaw (QA Lab)** — Full CDP access, `browserAct` with 20+ action kinds
2. **playwright-skill** — Full Playwright API, arbitrary `page.evaluate()` JS injection
3. **Oh My OpenAgent** — Playwright MCP tools, model-invoked actions
4. **pi-mono** — JS injection into current tab only
5. **Others** — No DOM interaction

### 3.3 Progressive Disclosure Patterns

**playwright-skill (strongest progressive disclosure):**
- Level 1: `SKILL.md` (450 lines) — Usage patterns, common tasks, setup
- Level 2: `API_REFERENCE.md` (653 lines) — Full API reference, loaded on demand
- Model loads basic docs first; advanced reference only when needed
- Code templates provided at each level

**OpenClaw:**
- `docs/tools/browser.md` — Browser tool documentation
- CLI help system (`browser-cli.ts`)
- Extension architecture means browser capabilities discovered via plugin system

**pi-mono:**
- No progressive disclosure for browser — minimal browser tool
- Web UI is document-centric, not automation-centric

### 3.4 Tool Generation for Web Tasks

| System | Generates Tools? | Generates Code? | Dynamic Adaptation |
|--------|-----------------|------------------|-------------------|
| playwright-skill | No (uses existing Playwright) | Yes — full JS scripts | Per-request custom code |
| OpenClaw | Via QA Lab scenarios | Scenario definitions | Test scenario execution |
| pi-mono | No | No | No |
| OpenCode | No | No | No |
| Goose | Via MCP extensions | No | MCP tool selection |
| Oh My OpenAgent | Via skill loading | No | Skill-based routing |
| Claude Code | No | No | No |
| Google Stitch | N/A — generates UI | N/A — generates designs | Prompt-based iteration |

**Dynamic code generation advantage:** playwright-skill generates custom scripts per request, enabling tasks that weren't anticipated by pre-built tools. This is the most flexible approach but requires the executor to safely handle untrusted code.

### 3.5 Comparison of Architectural Paradigms

**Paradigm A: Dynamic Code Generation (playwright-skill)**
```
User request → LLM writes Playwright JS → run.js executes → Output
```
- **Pros:** Maximum flexibility, any Playwright task possible, no tool schema needed
- **Cons:** Security risk of running generated code, requires safe execution environment
- **Best for:** Complex, one-off browser tasks, responsive design testing, form automation

**Paradigm B: MCP-Based Browser Tools (Oh My OpenAgent, partially Goose)**
```
Agent → MCP protocol → @anthropic-ai/mcp-playwright → Browser
```
- **Pros:** Standardized tool interface, tool schemas available to model
- **Cons:** Less flexible than code generation, tool schema limits actions
- **Best for:** Skill-based ecosystems, standardized tool catalogs

**Paradigm C: CDP Deep Control (OpenClaw)**
```
Gateway → Browser extension → Chrome DevTools Protocol → Full browser control
```
- **Pros:** Deepest browser control, persistent sessions, can control any browser feature
- **Cons:** Complex architecture, browser-specific (Chrome-based), high implementation cost
- **Best for:** QA testing, comprehensive browser automation, enterprise scenarios

**Paradigm D: In-Browser Agent (pi-mono)**
```
User browser → Agent embedded → JS injection into current tab
```
- **Pros:** Zero browser installation, agent operates in user's context
- **Cons:** No persistent automation, limited to current tab, no screenshots
- **Best for:** Light assistance within browsing session, document extraction

**Paradigm E: Generative UI (Google Stitch)**
```
Prompt → Gemini 2.5 → UI design → HTML/CSS export
```
- **Pros:** Creates new UIs from scratch, no browser needed for generation
- **Cons:** Doesn't automate existing sites, design-focused not automation-focused
- **Best for:** UI prototyping, design iteration, design-to-code workflow

---

## 4. Key Tradeoffs

### 4.1 Flexibility vs. Security

**Dynamic code generation (playwright-skill)** offers maximum flexibility but requires sandboxing. The model generates arbitrary JS that runs via Node.js. Security mitigations:
- Scripts written to `/tmp` (ephemeral)
- Run from skill directory (controlled module context)
- Custom headers for traffic identification
- No persistent state between runs

**CDP control (OpenClaw)** offers deep control with complex security:
- SSRF protection, secret comparison, path guards
- WebSocket auth tokens
- Browser process isolation

### 4.2 Tool Schema vs. Ad-hoc Execution

**MCP tools (Oh My OpenAgent)** provide explicit schemas the model can reason about, but limit actions to defined tools. The model knows what's available.

**Dynamic code (playwright-skill)** has no predefined schema — the model generates any Playwright code. More powerful but less constrained.

### 4.3 Single-Task vs. Persistent Browser

**Ephemeral (playwright-skill):** Each task launches a new browser, no session state. Clean slate each time.

**Persistent (OpenClaw):** Browser sessions persist, can maintain state across actions, supports tabs and profiles.

### 4.4 Browser Control vs. Information Retrieval

Most systems (OpenCode, Claude Code, Goose) don't control browsers at all — they retrieve information from the web. True browser automation (playwright-skill, OpenClaw, Oh My OpenAgent) is rare.

---

## 5. ACE Recommendations

| Approach | System(s) | Recommendation | Rationale |
|----------|-----------|----------------|-----------|
| **Dynamic code generation** | playwright-skill | **ADOPT** | Most flexible model-invoked approach. Custom scripts per request. `run.js` universal executor is solid pattern. Security model (tmp, module context) is appropriate. Progressive disclosure (SKILL.md → API_REFERENCE.md) is exemplary. |
| **CDP-based control** | OpenClaw | **ADAPT** | Deep browser control via gateway/browser extension is powerful but complex. ACE's single-binary model makes bundling a browser extension challenging. Consider lightweight CDP wrapper if desktop use cases emerge. |
| **MCP Playwright integration** | Oh My OpenAgent | **ADOPT** | Skill-gated browser provider pattern is sound. MCP protocol standardization is valuable. Integrate with ACE's skill system when browser tools are needed. |
| **In-browser agent** | pi-mono | **AVOID** | Useful for document extraction but not true automation. Doesn't fit ACE's model where agent operates independently of user's browser. Extract document patterns may be useful separately. |
| **webfetch/websearch only** | OpenCode, Claude Code | **ADOPT** (as-is) | Information retrieval via webfetch/websearch is appropriate for coding agents. Don't add browser control if not needed. Claude Code's model (web search tracking, clipboard screenshots) is adequate for its use case. |
| **MCP browser extensions** | Goose | **ADOPT** | MCP extension pattern for web tools is standard and interoperable. ACE should expose web tools via MCP when requested. |
| **Generative UI synthesis** | Google Stitch | **AVOID** (for browser automation) | Stitch generates UIs, doesn't automate browsers. Not relevant to browser automation concerns. May be relevant for design/prototyping units separately. |
| **Progressive disclosure** | playwright-skill | **ADOPT** | SKILL.md → API_REFERENCE.md pattern is excellent. Two-tier documentation with model-aware loading. ACE should apply to all skill documentation. |

### 5.1 Specific ACE Guidance

**For ACE cognitive units focused on browser automation:**
1. **Prefer MCP-based browser tools** over dynamic code generation for security-hardened environments
2. **Consider playwright-skill's `run.js` pattern** for flexible, model-invoked browser tasks in development tools
3. **Avoid embedding agent in user's browser** (pi-mono pattern) — ACE operates independently
4. **Use webfetch/websearch** as the baseline for web information retrieval; don't add unnecessary browser control
5. **Apply progressive disclosure** to all skill documentation (two tiers: usage + API reference)

---

## 6. Summary

Browser automation in AI agents falls into three clear tiers:

1. **Full browser automation** (playwright-skill, OpenClaw, Oh My OpenAgent) — Can navigate, interact, screenshot, evaluate JS
2. **Information retrieval** (OpenCode, Claude Code, Goose) — Can fetch pages and search, but not interact
3. **Generative UI** (Google Stitch) — Creates UIs, doesn't automate browsers

For ACE, the recommended approach is:
- **ADOPT** playwright-skill's dynamic code generation model for flexibility
- **ADOPT** MCP-based browser tools for standardized, secure browser control
- **ADOPT** webfetch/websearch for information retrieval use cases
- **ADAPT** OpenClaw's CDP insights for potential future desktop integration
- **AVOID** in-browser agent patterns and Stitch-style generative UI for automation purposes

The progressive disclosure pattern in playwright-skill is particularly valuable and should be replicated across all ACE skill documentation.

---

**Files Reviewed:**
- `research/playwright-skill/skills/playwright-skill/{run.js,SKILL.md,API_REFERENCE.md,lib/helpers.js}`
- `research/openclaw/extensions/browser/{browser-cdp.ts,browser-runtime-api.ts,browser-bridge.ts}`
- `research/openclaw/extensions/qa-lab/src/browser-runtime.ts`
- `research/pi-mono/packages/web-ui/src/tools/extract-document.ts`
- `research/openclaw/ui/src/ui/chat/tool-cards.ts`
- `research/oh-my-openagent/src/plugin/skill-context.ts`
- `research/oh-my-openagent/src/tools/skill/types.ts`
- `research/collection-claude-code-source-code/original-source-code/src/utils/screenshotClipboard.ts`
- Web search: Google Stitch documentation and SDK
