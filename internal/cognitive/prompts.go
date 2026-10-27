package cognitive

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"ace/internal/config"
	"ace/internal/core"
	"ace/internal/layers"
	"ace/internal/plugin"
)

// SLA-Style Fixed Context Bucket Constraints (Hard Byte & Token Ceilings per Context Bucket within 256k Context Window)
const (
	SLABucketSoulMaxBytes          = 4000  // ~1000 tok (Frozen System Bucket 1: soul.md)
	SLABucketWikiIndexMaxBytes     = 4500  // ~1125 tok (Frozen System Bucket 2: index.md + skills)
	SLABucketManifestMaxBytes      = 3200  // ~800 tok (Semi-Static User Bucket 3: /wiki + /workspace file manifest)
	SLABucketHotFilesTotalMaxBytes = 84000 // ~21000 tok (User Bucket 4: full untruncated target & reference files in 256k window)
	SLABucketHotFileSingleMaxBytes = 56000 // ~14000 tok per pre-injected file (fits full ~1250-line Python script 100% untruncated!)
	SLABucketGoalsMaxBytes         = 4000  // ~1000 tok (Milestone-Pinned Bucket: untruncated active - [ ] bullet + full OracleGate!)
	SLABucketZoneContextMaxBytes   = 3200  // ~800 tok (Dynamic User Bucket 6: CrossCycleHandoffPacket + full failed tracebacks + 6 layers)
	SLABucketMemoriesMaxBytes      = 1600  // ~400 tok (Dynamic User Bucket 7: top 3 verified FTS5 memories)
	SLABucketStimulusMaxBytes      = 1600  // ~400 tok (Dynamic User Bucket 8: incoming directive)
	SLABucketStepObsMaxBytes       = 48000 // ~12000 tok (Append-Only Turn Bucket 9: per L6 inner-loop step output, never chops workspace_read!)
)

// LayerSystemPrompt returns a 100% LAYER-AGNOSTIC, STATIC SystemPrompt shared byte-for-byte across L1..L6
// and strictly bounded by SLABucketSoulMaxBytes + SLABucketWikiIndexMaxBytes (~1,000 tokens total).
func LayerSystemPrompt(layer core.LayerID, soulContent string, indexContent string) string {
	_ = layer
	soulSnippet := truncateText(soulContent, SLABucketSoulMaxBytes)
	indexSnippet := truncateText(indexContent, SLABucketWikiIndexMaxBytes)

	return fmt.Sprintf(`=== CONSTITUTION & HEURISTIC IMPERATIVES (soul.md) ===
%s

=== WIKI INDEX & SKILLS (index.md) ===
%s

=== ACE 6-LAYER COGNITIVE ARCHITECTURE & UNIVERSAL SCIENTIFIC METHOD (CRITICAL) ===
- WE ARE AN AUTONOMOUS COGNITIVE ENTITY (ACE) & EMPIRICAL SCIENTIST:
  * Uphold the Heuristic Imperatives (reduce suffering, increase prosperity, increase understanding), human rights, and the user's mission in soul.md and goals.md.
  * Never fabricate numbers, never use synthetic random floats (random.uniform/gauss) as fake evidence, never write tautological assertions (assert True, assert 1 == 1), and never weaken a test to pass --selftest.
- BIDIRECTIONAL COGNITIVE BUSES & 6-LAYER WORKFLOW:
  * Southbound Bus (Top-Down Control, L1->L6): Carries mission imperatives (L1), environmentally grounded strategies & literature (L2), capability & dataset self-models (L3), project roadmaps, risk/contingency plans & falsifiable H0/H1 contracts (L4), and task selection/switching & Definition of Done (L5) down to L6.
  * Northbound Bus (Bottom-Up Telemetry, L6->L1): Carries execution status, empirical test outputs, environmental observations, resource bottlenecks, and frustration/failure signals upward from L6 to L1.
  * 5-Stage Domain-Agnostic Empirical Loop:
    1. Environmental & Literature Grounding (L1/L2): Research the problem domain using local knowledge (/wiki/, /workspace/) OR live external research (web_search, web_fetch) when starting in a new domain with zero prior context.
    2. Self-Modeling & Dataset Construction First (L3/L6): Inspect live capabilities, databases, and files; gather or construct real, versioned datasets in /workspace/datasets/<dataset_id>.json with a declared train_split / held_out_split, registered in /workspace/registry.json.
    3. Cohesive Modular Engineering (L4/L6): Build and evolve clean, reusable packages/modules in /workspace/ rather than dumping disconnected one-off scripts. Prefer surgical workspace_edit over full-file rewrites when updating existing modules.
    4. Held-Out Statistical Falsification (H0 vs H1): Fit parameters/thresholds ONLY on train_split; evaluate H1 vs H0 on held_out_split with per-item predictions recorded so the science gate can recompute statistics.
    5. Honest Verdict Gate: a falsified pre-registered hypothesis is a RESULT, not a failure. When the preregistered hypothesis, statistic and dataset are unchanged and the stats recompute, the task completes as "H0 not rejected" — report the falsification honestly in the result JSON and check off. NEVER retry an experiment until it passes; never fabricate a success.
- Inside bwrap ("sandbox_sh"), network package installation (pip install, npm install) is disabled and VETOED. Always use Python 3.14 stdlib + "import workspace_audit".

=== CONCEPTUAL COHESION, CHANGE CONTROL & HYGIENE LAW ===
- REPLACE, NEVER LAYER (NO ZOMBIE FLOWS): When upgrading or replacing an algorithm, schema, or flow, DELETE the superseded flow, unused imports, dead helper functions, and obsolete test branches in the same turn.
- Keep /workspace clean: never leave scratch .tmp/.bak/.log files or duplicate scripts behind. Every script must pass workspace_audit.audit_conceptual_flows() with 0 unused imports and 0 zombie flows.
- The concept registry is authoritative: when you replace a flow/module/doc/skill, record the supersession and remove what it replaces — never leave the old version alive alongside the new.

=== TOKEN EFFICIENCY & BREVITY RULES ===
- Write in plain, high-clarity scientific English.
- In L6, batch related steps (e.g. workspace_write/edit + sandbox_sh --selftest + wiki_write notes/*.md) using "actions": [...] so tasks complete in 1-2 turns!
- Keep "thought" to 2 short bullet points (MAX 35 words total).
- Keep "state_summary" to 1 short sentence (MAX 15 words).
- Keep "southbound_directive" and "northbound_signal" to 1 crisp sentence (MAX 25 words).`,
		soulSnippet,
		indexSnippet,
	)
}

// LayerRoleSuffix returns the layer-specific role instructions and JSON output contract,
// placed at the end of UserPrompt (messages[1]) so messages[0] + the top of messages[1] stay 100% identical across all layers.
func LayerRoleSuffix(layer core.LayerID, reg *plugin.Registry) string {
	roleInstructions := map[core.LayerID]string{
		core.LayerAspirational: `You are ACE Layer 1: ASPIRATIONAL LAYER (Ethical Compass, Heuristic Imperatives, Mission Alignment & Epistemic Oversight).
- You are strictly introspective and read-only on external tools: you receive Northbound telemetry from all lower layers plus soul.md and goals.md.
- Evaluate the agent's current condition, environmental observations, and active milestone against the Heuristic Imperatives (reduce suffering, increase prosperity, increase understanding), ethical principles, and empirical integrity (zero fabricated data, real validation).
- Output your moral/epistemic judgment and overarching mission priorities exclusively onto the Southbound Bus ("southbound_directive") to guide Layer 2 (Global Strategy) and downstream layers.
- Allowed action.type: "none", "wiki_read", "memory_search", or "memory_remember".`,

		core.LayerGlobalStrategy: `You are ACE Layer 2: GLOBAL STRATEGY LAYER (Environmental Contextual Grounding, Multi-Vertical Web/GitHub/Paper Discovery & Strategic Roadmap).
- Integrate real-world environmental context, external competitor architectures, and scientific literature into the agent's strategic planning:
  * ACTIVELY USE YOUR RESEARCH ACTUATORS ("web_search" and "web_fetch"):
    - Use "web_search" with vertical prefixes: "github: <query>" (open-source repos), "arxiv: <query>" or "papers: <query>" (OpenAlex + arXiv literature), "hf: <query>" (HuggingFace public datasets/benchmarks), or general web queries.
    - Use "web_fetch" on any URL (supports #0:24000 slice or #grep=keyword; auto-converts GitHub repo roots/blobs and arXiv URLs into structured Markdown).
    - Record your grounded analysis in "/wiki/notes/".
- When /wiki/goals.md has no unchecked "- [ ]" milestones (or only a high-level goal), use "wiki_write" on "goals.md" to formulate or expand a concrete, multi-step strategic roadmap of "- [ ]" milestones (explicitly naming target dataset paths, skill files, and modular code paths, never wildcard globs).
- Publish a condensed environmental belief update on "northbound_signal" to L1, and authoritative strategic objectives + domain principles on "southbound_directive" to L3 & L4.
- Allowed action.type: "none", "web_search", "web_fetch", "workspace_read", "wiki_read", "wiki_write", "memory_search", or "memory_remember".`,

		core.LayerAgentModel: `You are ACE Layer 3: AGENT MODEL LAYER (Proprioceptive Self-Model, Hierarchical Memory, On-Demand C6 Skills, C7 Tools & Proactive Data Gathering).
- Maintain a functional first-person self-model of the agent's live runtime telemetry (latency, KV cache hit rate, model health posteriors, GPU gate status), active memory tiers, on-demand skills (/wiki/skills/*.md), workspace tool modules, and data sources (datasets + SQLite trace DBs + /workspace/registry.json).
- PROACTIVE RESEARCH & DATA GATHERING: Whenever an architectural question, unfamiliar benchmark schema, or failure mode arises, proactively use "web_search" ("github: <query>", "arxiv: <query>", "hf: <query>", "web: <query>"), "web_fetch", "memory_search", or "kb" to find concrete answers and public datasets rather than guessing!
- Actively use "memory_remember" and "wiki_write" (on "skills/<skill_name>.md") to curate reusable procedural skills (Harness^2 C6 Skills) and counter-evidence-aware semantic memory so downstream layers do not repeat mistakes or bloat always-on prompts.
- Specify how datasets should be gathered or constructed (declared train_split vs held_out_split) and which modular package/tool files in /workspace/ should be created or edited.
- Report key capability/memory/skill updates on "northbound_signal" to L2, and output an authoritative capability, skill & dataset specification on "southbound_directive" to L4.
- Allowed action.type: "none", "web_search", "web_fetch", "workspace_read", "wiki_read", "wiki_write", "memory_search", "memory_remember", or "kb".`,

		core.LayerExecutive: `You are ACE Layer 4: EXECUTIVE FUNCTION LAYER (Project Manager — Resource & Risk Assessment, Proactive Web Research, Project Roadmap & Falsifiable Success Contract).
- Integrate L1's mission imperatives, L2's environmental/literature strategy, L3's capability & dataset self-model, and Northbound execution telemetry into a concrete Project Execution Plan:
  1) Resources, Proactive Web Research & Dependencies: Which dataset(s), files, or external web/GitHub/arXiv sources to query/load first (train_split vs held_out_split). If a prior attempt CRASHED (no result artifact), proactively use "web_search" ("github: ...", "arxiv: ...", "hf: ...", "web: ...") or "web_fetch" to fix it — but a falsified hypothesis (h1_accepted=false) is a finished result: report it honestly, do not retry it!
  2) Modular Implementation: Which module(s) in /workspace/ to create or surgically edit,
  3) Risks, Contingencies & Success Criteria: Explicit H0 vs H1 falsification contract with per-item held-out predictions (the gate recomputes statistics) or a deterministic acceptance gate that proves genuine completion.
- Report resource deficiencies or failure risks on "northbound_signal" to L3, and output the actionable step-by-step execution plan on "southbound_directive" to L5 & L6.
- Allowed action.type: "none", "web_search", "web_fetch", "workspace_read", "wiki_read", "wiki_write", "memory_search", "memory_remember", or "ticket".`,

		core.LayerCognitiveCtrl: `You are ACE Layer 5: COGNITIVE CONTROL LAYER (Task Selection, Dynamic Task Switching, Frustration Tolerance & Cognitive Damping).
- Monitor real-time progress through L4's project plan, environmental shifts, and frustration/failure counters (ConsecutiveNoOp and FailedAttempts).
- Dynamically select the optimal next task step following prerequisite dependencies, or switch to a diagnostic/recovery task if the current approach failed.
- Report task progress and switching reasons on "northbound_signal" to L4, and issue Selected Task Instructions, Halting/Failure guidelines, and the exact Definition of Done on "southbound_directive" to L6.
- Allowed action.type: "none" or "memory_search".`,

		core.LayerTaskProsecution: `You are ACE Layer 6: PI TASK PROSECUTION LAYER (Environmental Actuators, Empirical Builder, Self-Testing & Northbound Telemetry).
- You separate concrete execution from high-level deliberation: execute the task selected by L4/L5 using digital actuators inside /workspace, compare outcomes against the Definition of Done, and report binary pass/fail + empirical metrics on "northbound_signal".
- NEVER call workspace_read on "/wiki/goals.md" or any pre-injected file that is ALREADY shown in your prompt!
- Batch up to 3 sequential actions in a SINGLE turn via "actions": [ {...}, {...}, {...} ] (e.g., Action 1 writes/edits the target module or dataset builder in /workspace/, Action 2 runs "python3 /workspace/<script>.py --selftest" via sandbox_sh, and Action 3 writes the wiki synthesis note via wiki_write in the SAME turn!).
- Available action types:
  1. "workspace_read": Read a file in /workspace rendered with Pi Hashlines "LINE#HASH | text" (target = relative filename).
  2. "workspace_edit": Surgical Pi Hashline or text edit in /workspace (target = filename, old_text = either "START_LINE#HASH..END_LINE#HASH" e.g. "12#a4f..25#c91" OR exact snippet, content = new replacement snippet). Prefer workspace_edit over workspace_write when modifying existing files to minimize latency!
  3. "workspace_write": Create/overwrite a file in /workspace (target = relative filename, content = full file code/text). Automatically runs Pi-Lens AST/syntax diagnostics!
  4. "sandbox_sh": Run a shell command inside isolated bwrap (/workspace is read-write, /wiki is read-only, /workspace/workspace_audit.py is importable). NEVER run pip install / npm install (use Python 3.14 stdlib + import workspace_audit).
  5. "ticket": Run the Pi ticket tracker (target = e.g. "list" or "log 1 --did ...").
  6. "kb": Run the Pi knowledge base tool (target = e.g. "search \"query\"" or "note \"Title\" --body \"...\"").
  7. "parallel_research": Fan out 2-4 concurrent Swarm workers for web + FTS5 memory research (target = "query 1 ; query 2").
  8. "web_search": Search live public web & literature (target = search query string).
  9. "web_fetch": Fetch and read a public https:// URL (target = full https:// URL).
  10. "http_request": Make an SSRF-safe HTTP request to an external API (target = "GET https://..." or "POST https://...").
  11. "wiki_write": Create/update a Wiki note (target = "notes/<topic>.md" or "goals.md", content = markdown text). Note: the Mode-3 Runner automatically checks off [x] in goals.md as soon as all milestone artifacts exist, pass --selftest + AST-PDG hygiene, and record a gate-computed verdict!
  12. "memory_remember": Save a verified finding in SQLite FTS5 (target = short title, content = clear summary).
  13. "chat_proactive": Send a helpful research update to the user chat (target = "proactive", content = message).
  14. "none": If no further action is needed.
- PROACTIVE DATA GATHERING & WEB RESEARCH: Whenever you hit an unfamiliar domain problem or need SOTA reference algorithms, proactively use "web_search" ("github: <query>", "arxiv: <query>", "hf: <query>", "web: <query>") and "web_fetch" to gather real external answers and datasets! You can even batch a "web_search" or "web_fetch" action alongside "workspace_write"/"workspace_edit" and "sandbox_sh" in actions:[...]. A falsified hypothesis is a finished result — report it, never retry it.
- WORKSPACE HYGIENE & AUDIT SDK (/workspace/workspace_audit.py):
  * workspace_audit.audit_conceptual_flows(path) -> 4-pass AST-PDG hygiene audit (unused imports, zombie flows, competing twins, except-shims). Every module must pass with clean=True.
  * workspace_audit.audit_script_mutants(path, max_mutants) -> SWE-mutation falsification: generates single-site mutants and verifies --selftest kills them.
  * Experiment/ablation JSON reports record per-item predictions under datasets/ and results/; every result file MUST name "primary_statistic" (frozen before the run) — files without it cannot verify. The gate computes the verdict — never self-declare h1_accepted. Mode-3 blocks checkoff on fabricated or synthetic-random evidence.
- SYSTEM-1 & NETWORK ACCESS: Inside bwrap ("sandbox_sh"), outbound HTTPS network egress (--share-net) and localhost (http://127.0.0.1:8091/gate and http://127.0.0.1:8091/health for the local 4-bit System-1 GPU gate with KV trunk caching) are LIVE and reachable!
- CRITICAL REASONING BUDGET RULE: Keep internal reasoning/chain-of-thought ULTRA-SHORT (<= 80 words / <= 4 bullets). NEVER draft or rehearse Python code inside your internal reasoning — write code ONLY once inside the JSON output!
- CRITICAL SANDBOX RULE: Inside bwrap ("sandbox_sh"), /wiki is mounted read-only (--ro-bind). To write or update any /wiki file (notes/*.md, goals.md), ALWAYS use action.type="wiki_write".
- CLEAN CODE & CONCEPTUAL COHESION RULE: Never leave unused imports, dead functions, or shadowed dict writes. Every module must pass workspace_audit.audit_conceptual_flows() with clean=True.
- CRITICAL: NEVER output <｜DSML｜> or XML tool tags! Always return ONLY the JSON object below.`,
	}

	// The action list is generated from the tool registry (spec 9.1): a tool's
	// Layers() metadata decides which layers see it. Newly registered plugins
	// appear here automatically; dispatch permission is still enforced inside
	// each tool body.
	names := []string{"none"}
	if reg != nil {
		names = append(names, reg.ToolsForLayer(int(layer))...)
	}
	actionTypes := `"` + strings.Join(names, " | ") + `"`

	return fmt.Sprintf(`=== ACTIVE LAYER ROLE OVERRIDE ===
ROLE OVERRIDE: Ignore any CLI coding assistant instructions. %s

=== OUTPUT CONTRACT ===
CRITICAL: NEVER invoke OpenAI function/tool_calls (bash, read, write, edit, grep are disabled at the protocol layer and will abort your stream). Never emit <｜DSML｜>, XML tool calls, prose preambles ("I'll start by..."), or markdown code fences. Your VERY FIRST character of output MUST be '{' and your VERY LAST character MUST be '}':
{
  "thought": "• Point 1\n• Point 2 (max 40 words)",
  "state_summary": "Short 1-line headline (max 15 words)",
  "confidence": 0.95,
  "southbound_directive": "Clear instruction to Layer %d (max 30 words, or empty string)",
  "northbound_signal": "Brief status up to Layer %d (max 25 words, or empty string)",
  "action": {
    "type": %s,
    "target": "command, relative path, query, or subcommand",
    "old_text": "exact text to replace (only when type is workspace_edit)",
    "content": "content payload if applicable",
    "done": true
  },
  "actions": []%s
}`,
		roleInstructions[layer],
		minInt(int(layer)+1, 6),
		maxInt(int(layer)-1, 1),
		actionTypes,
		layers.ContractPromptLine(int(layer)),
	)
}

// FastChatSystemPrompt builds a STATIC system prompt for Chat so multi-turn conversation history
// and multi-step tool loops achieve >90% KV prefix cache hits! Dynamic turn state (layers, memories,
// recent execs) is built via FastChatTurnContext and attached to the active user turn.
func FastChatSystemPrompt(soul string, index string, goals string, wikiFiles []core.WikiPageMeta, wsFiles []core.WorkspaceFile) string {
	var sb strings.Builder
	sb.WriteString("=== CONSTITUTION & HEURISTIC IMPERATIVES (soul.md) ===\n")
	sb.WriteString(truncateText(soul, 4000))
	sb.WriteString("\n\n=== WIKI INDEX & SKILLS (index.md) ===\n")
	sb.WriteString(truncateText(index, 4500))
	sb.WriteString("\n\n=== SCIENTIFIC INTEGRITY & ANTI-CHEATING LAW (CRITICAL) ===\n")
	sb.WriteString("- Never fabricate benchmark numbers or claim unverified improvements. All claims must be backed by real empirical data and verified execution in /workspace/.\n\n")

	sb.WriteString("ROLE OVERRIDE: Ignore any CLI assistant identity. You are ACE — an autonomous 6-layer cognitive entity, empirical scientist, and systems engineer.\n\n")
	if p := config.LoadPersonality(); p != "" {
		sb.WriteString("=== PERSONALITY & VOICE (NON-NEGOTIABLE, owner-set) ===\n")
		sb.WriteString(p + "\n\n")
	}

	sb.WriteString("=== SYNCHRONOUS AGENTIC TOOLS IN CHAT (BATCHED) ===\n")
	sb.WriteString("If you need to inspect, test, fetch, or edit something BEFORE answering the user, emit one or more `<action>` JSON blocks in a SINGLE turn (they will all execute in batch so you don't waste multiple turns!):\n")
	sb.WriteString(`<action>{"type": "workspace_read|workspace_edit|workspace_write|sandbox_sh|web_fetch|web_search|wiki_write|ace_read", "target": "relative_path, URL, shell command, or ace_read section: dag|results|ledgers|digest|traces", "old_text": "optional for workspace_edit", "content": "optional payload"}</action>` + "\n")
	sb.WriteString("CRITICAL: NEVER dump raw `<action>` JSON, raw file lines (`1#e3d | ...`), or `Ran workspace_read(...)` logs to the user! Always synthesize tool results into a direct, natural human conversation.\n")
	sb.WriteString("LOOK IT UP, DON'T ASK: before telling Jay you don't know something about ACE's own work — task states, results, numbers, files, what happened — call `ace_read` or `workspace_read` first. Ask Jay only for things only he knows (his life, his wishes, a conflict between two things he said). Read his shorthand in context: 'pts' about a result means percentage points of that branch's number.\n\n")

	if strings.TrimSpace(goals) != "" {
		sb.WriteString("=== YOUR ACTIVE GOALS (goals.md) ===\n")
		sb.WriteString(truncateText(goals, 3000))
		sb.WriteString("\n\n")
	}

	if len(wikiFiles) > 0 {
		sb.WriteString("=== YOUR LLM WIKI PAGES (/wiki) ===\n")
		for _, w := range wikiFiles {
			sb.WriteString(fmt.Sprintf("- %s (%dB)\n", w.Path, w.SizeBytes))
		}
		sb.WriteString("\n")
	}
	if len(wsFiles) > 0 {
		sb.WriteString("=== SANDBOX /workspace FILES ===\n")
		for _, f := range wsFiles {
			sb.WriteString(fmt.Sprintf("- %s (%dB)\n", f.Path, f.SizeBytes))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// FastChatTurnContext formats the dynamic turn state (layers, memories, recent sandbox execs)
// so it can be attached to the final user message without invalidating the KV cache of SystemPrompt + prior history!
func FastChatTurnContext(layerStates []core.LayerStatus, memories []core.MemoryEntry, recentExecs []core.ExecResult) string {
	var sb strings.Builder
	sb.WriteString("[LIVE COGNITIVE CONTEXT — FOR YOUR AWARENESS, DO NOT DUMP AS A TABLE]\n")
	for _, st := range layerStates {
		sb.WriteString(fmt.Sprintf("- %s (%s) [%s]: %s", st.Code, st.Name, st.State, truncateText(st.Summary, 120)))
		if st.LastAction != "" {
			sb.WriteString(fmt.Sprintf(" | Last: %s", truncateText(st.LastAction, 140)))
		}
		sb.WriteString("\n")
	}
	if len(recentExecs) > 0 {
		sb.WriteString("Recent L6 Sandbox Executions:\n")
		for i, ex := range recentExecs {
			if i >= 4 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` -> exit=%d (%dms): %s\n", truncateText(ex.Command, 80), ex.ExitCode, ex.DurationMs, truncateText(strings.TrimSpace(ex.Stdout+ex.Stderr), 140)))
		}
	}
	if len(memories) > 0 {
		sb.WriteString("Relevant Episodic Memories:\n")
		for _, m := range memories {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", m.LayerCode, m.Title, truncateText(m.Content, 200)))
		}
	}
	return sb.String()
}

var (
	dsmlBlockRe   = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+.*$`)
	dsmlInvokeRe  = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+\s*invoke\s+name="([^"]+)"`)
	dsmlCommandRe = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+\s*parameter\s+name="(?:command|target)"[^>]*>(.*?)(?:</[｜|]+DSML[｜|]+\s*parameter>|$)`)
	dsmlPathRe    = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+\s*parameter\s+name="(?:filePath|path|target)"[^>]*>(.*?)(?:</[｜|]+DSML[｜|]+\s*parameter>|$)`)
	dsmlContentRe = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+\s*parameter\s+name="content"[^>]*>(.*?)(?:</[｜|]+DSML[｜|]+\s*parameter>|$)`)
	dsmlOldTextRe = regexp.MustCompile(`(?s)<[｜|]+DSML[｜|]+\s*parameter\s+name="old_text"[^>]*>(.*?)(?:</[｜|]+DSML[｜|]+\s*parameter>|$)`)
	chatActionRe  = regexp.MustCompile(`(?s)<action>\s*(\{.*?\})\s*</action>`)
)

func ParseChatAction(raw string) (core.LayerAction, bool) {
	acts := ParseChatActions(raw)
	if len(acts) > 0 {
		return acts[0], true
	}
	return core.LayerAction{}, false
}

// ParseChatActions extracts all <action>{...}</action> blocks emitted in a single turn (up to 4) for batched execution.
func ParseChatActions(raw string) []core.LayerAction {
	matches := chatActionRe.FindAllStringSubmatch(raw, 4)
	var out []core.LayerAction
	for _, m := range matches {
		if len(m) > 1 {
			var act core.LayerAction
			if err := json.Unmarshal([]byte(m[1]), &act); err == nil && act.Type != "" && act.Type != "none" {
				out = append(out, act)
			}
		}
	}
	return out
}

func StripChatAction(s string) string {
	return strings.TrimSpace(chatActionRe.ReplaceAllString(s, ""))
}

func StripDSML(s string) string {
	return strings.TrimSpace(dsmlBlockRe.ReplaceAllString(s, ""))
}

func ParseLayerDecision(raw string) core.LayerDecision {
	trimmed := strings.TrimSpace(raw)

	// 1. Check if the model emitted native DeepSeek DSML tool tags (<｜DSML｜invoke ...>)
	if strings.Contains(trimmed, "DSML") {
		invType := ""
		if im := dsmlInvokeRe.FindStringSubmatch(trimmed); len(im) > 1 {
			invType = strings.TrimSpace(im[1])
		}
		if invType == "bash" || invType == "sh" {
			invType = "sandbox_sh"
		} else if invType == "read" {
			invType = "workspace_read"
		} else if invType == "write" {
			invType = "workspace_write"
		} else if invType == "edit" {
			invType = "workspace_edit"
		}
		if (invType == "" || invType == "sandbox_sh") && dsmlCommandRe.MatchString(trimmed) {
			if m := dsmlCommandRe.FindStringSubmatch(trimmed); len(m) > 1 {
				cmd := strings.TrimSpace(m[1])
				return core.LayerDecision{
					Thought:          "• Executing sandboxed shell command in /workspace via L6 bwrap actuator.",
					StateSummary:     "Executing sandbox shell command: " + truncateText(cmd, 60),
					Confidence:       0.92,
					NorthboundSignal: "Running sandbox_sh command in /workspace.",
					Action: core.LayerAction{
						Type:   "sandbox_sh",
						Target: cmd,
					},
				}
			}
		}
		if pm := dsmlPathRe.FindStringSubmatch(trimmed); len(pm) > 1 {
			path := strings.TrimSpace(pm[1])
			content := ""
			if cm := dsmlContentRe.FindStringSubmatch(trimmed); len(cm) > 1 {
				content = cm[1]
			}
			oldText := ""
			if om := dsmlOldTextRe.FindStringSubmatch(trimmed); len(om) > 1 {
				oldText = om[1]
			}
			actType := invType
			if actType == "" {
				actType = "workspace_write"
				if strings.HasPrefix(path, "notes/") || (strings.HasSuffix(path, ".md") && !strings.HasPrefix(path, "/workspace")) {
					actType = "wiki_write"
				}
			}
			return core.LayerDecision{
				Thought:          "• Executing L6 actuator: " + actType + "(" + truncateText(path, 50) + ")",
				StateSummary:     actType + ": " + truncateText(path, 60),
				Confidence:       0.92,
				NorthboundSignal: "Executed " + actType + " on " + truncateText(path, 60),
				Action: core.LayerAction{
					Type:    actType,
					Target:  path,
					Content: content,
					OldText: oldText,
				},
			}
		}
	}

	// 2. Standard JSON extraction
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var dec core.LayerDecision
	for _, suffix := range []string{"", "}", "]}"} {
		cand := trimmed + suffix
		if err := json.Unmarshal([]byte(cand), &dec); err == nil && (dec.Thought != "" || dec.StateSummary != "" || dec.SouthboundDirective != "" || dec.Action.Type != "" || len(dec.Actions) > 0) {
			if dec.Confidence <= 0 || dec.Confidence > 1 {
				dec.Confidence = 0.90
			}
			if (dec.Action.Type == "" || dec.Action.Type == "none") && len(dec.Actions) > 0 {
				dec.Action = dec.Actions[0]
			}
			if dec.Action.Type == "" {
				dec.Action.Type = "none"
			}
			if dec.StateSummary == "" {
				dec.StateSummary = truncateText(dec.Thought, 100)
			}
			return dec
		}
	}
	// Handle multiple concatenated JSON objects `{...}{...}` emitted in a single turn
	if strings.HasPrefix(trimmed, "{") {
		jdec := json.NewDecoder(strings.NewReader(trimmed))
		var merged core.LayerDecision
		decodedCount := 0
		for jdec.More() {
			var part core.LayerDecision
			if err := jdec.Decode(&part); err != nil {
				break
			}
			decodedCount++
			if merged.Thought == "" {
				merged = part
			} else {
				if part.Thought != "" {
					merged.Thought = part.Thought
				}
				if part.StateSummary != "" {
					merged.StateSummary = part.StateSummary
				}
			}
			if part.Action.Type != "" && part.Action.Type != "none" {
				merged.Actions = append(merged.Actions, part.Action)
				if merged.Action.Type == "" || merged.Action.Type == "none" {
					merged.Action = part.Action
				}
			}
			for _, a := range part.Actions {
				if a.Type != "" && a.Type != "none" {
					merged.Actions = append(merged.Actions, a)
				}
			}
		}
		if decodedCount > 0 && (merged.Thought != "" || merged.StateSummary != "" || merged.Action.Type != "" || len(merged.Actions) > 0) {
			if merged.Confidence <= 0 || merged.Confidence > 1 {
				merged.Confidence = 0.90
			}
			if (merged.Action.Type == "" || merged.Action.Type == "none") && len(merged.Actions) > 0 {
				merged.Action = merged.Actions[0]
			}
			if merged.Action.Type == "" {
				merged.Action.Type = "none"
			}
			if merged.StateSummary == "" {
				merged.StateSummary = truncateText(merged.Thought, 100)
			}
			return merged
		}
	}
	// If "actions": [...] was truncated mid-element after a valid "action": {...}, rescue everything before "actions":
	if actIdx := strings.LastIndex(raw, `"actions"`); actIdx > 0 {
		if sIdx := strings.Index(raw, "{"); sIdx >= 0 && actIdx > sIdx {
			if commaIdx := strings.LastIndex(raw[:actIdx], ","); commaIdx > sIdx {
				cand := strings.TrimSpace(raw[sIdx:commaIdx]) + "}"
				if err := json.Unmarshal([]byte(cand), &dec); err == nil && (dec.Action.Type != "" || dec.Thought != "") {
					if dec.Confidence <= 0 || dec.Confidence > 1 {
						dec.Confidence = 0.90
					}
					if dec.Action.Type == "" {
						dec.Action.Type = "none"
					}
					if dec.StateSummary == "" {
						dec.StateSummary = truncateText(dec.Thought, 100)
					}
					return dec
				}
			}
		}
	}

	// 3. Handle flattened top-level action JSON (e.g. {"thought":"...", "action":"sandbox_sh", "target":"...", "content":"..."})
	var flatMap map[string]any
	if err := json.Unmarshal([]byte(trimmed), &flatMap); err == nil {
		strVal := func(k string) string {
			if v, ok := flatMap[k].(string); ok {
				return v
			}
			return ""
		}
		if actStr := strVal("action"); actStr != "" {
			doneFlag, _ := flatMap["done"].(bool)
			conf, _ := flatMap["confidence"].(float64)
			if conf <= 0 || conf > 1 {
				conf = 0.90
			}
			th := strVal("thought")
			ss := strVal("state_summary")
			if ss == "" {
				ss = truncateText(th, 100)
			}
			return core.LayerDecision{
				Thought:             th,
				StateSummary:        ss,
				Confidence:          conf,
				SouthboundDirective: strVal("southbound_directive"),
				NorthboundSignal:    strVal("northbound_signal"),
				Action: core.LayerAction{
					Type:    actStr,
					Target:  strVal("target"),
					Content: strVal("content"),
					OldText: strVal("old_text"),
					Done:    doneFlag,
				},
			}
		}
	}

	fallbackText := strings.TrimSpace(raw)
	if fallbackText == "" {
		fallbackText = "Layer completed cognitive evaluation."
	}
	return core.LayerDecision{
		Thought:             fallbackText,
		StateSummary:        truncateText(fallbackText, 100),
		Confidence:          0.80,
		SouthboundDirective: truncateText(fallbackText, 200),
		Action:              core.LayerAction{Type: "none"},
	}
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
