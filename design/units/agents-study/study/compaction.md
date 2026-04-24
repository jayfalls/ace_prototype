# Slice 3: Context Compaction & Token Budgets

**Unit:** agents-study  
**Output:** `design/units/agents-study/study/compaction.md`  
**Research Repos Investigated:** openclaw/openclaw, anomalyco/opencode, nousresearch/hermes-agent, chauncygu/collection-claude-code-source-code, EverMind-AI/MSA  
**Research Papers:** TurboQuant (KV cache compression)

---

## 1. Introduction

Context compaction is the critical mechanism that allows long-running agentic systems to remain within token budgets while preserving essential task state. This study examines compaction strategies across five agent systems and one research paper, analyzing the graduated compression spectrum from lightweight pruning to heavyweight summarization, alongside the radical alternative of sparse attention that eliminates compaction entirely.

**Key finding:** All production agents employ multi-layered compaction strategies, typically 3-4 layers, rather than a single mechanism. The most sophisticated systems (Claude Code, Hermes Agent) separate concerns between safety-net rough compression and precise agent-controlled summarization.

---

## 2. Claude Code — 4-Layer Graduated Compression

### Source: `src/services/compact/` (compact.ts, autoCompact.ts, microCompact.ts, sessionMemoryCompact.ts)

Claude Code implements the most sophisticated graduated compression system observed, with four distinct layers operating at different granularities.

### Layer 1: Snip (Message-Level Removal)

**Trigger:** Individual messages exceeding token thresholds  
**Mechanism:** Selective removal of messages from the conversation chain without LLM involvement  
**Key insight:** Snip removes messages but the surviving assistant's usage metrics still reflect pre-snip context, so tokenCountWithEstimation cannot see the savings. A `snipTokensFreed` delta is computed and passed to `shouldAutoCompact`.

```typescript
// autoCompact.ts:166
// Subtract the rough-delta that snip already computed
const tokenCount = tokenCountWithEstimation(messages) - snipTokensFreed
```

### Layer 2: Microcompact (Tool Result Clearing)

**Trigger:** Time-based (cache TTL expiration) or count-based thresholds  
**Mechanism:** Replaces old tool results with `[Old tool result content cleared]` markers, or uses cache_edits API to remove tool results without invalidating the cached prefix

**Two paths:**
1. **Time-based:** When gap since last assistant message exceeds threshold, clears all but most recent N compactable tool results
2. **Cached microcompact:** Uses Anthropic's cache_edits API to surgically remove tool results while preserving cache prefix (avoids cache invalidation cost)

```typescript
// microCompact.ts:36
export const TIME_BASED_MC_CLEARED_MESSAGE = '[Old tool result content cleared]'
```

**Targets:** File read, shell, grep, glob, web search, web fetch, file edit, file write

### Layer 3: Collapse (Context-Collapse)

**Trigger:** 90% of effective context window  
**Mechanism:** Full context management system with commit/partial/commit pattern; separate from autocompact to avoid racing

```typescript
// autoCompact.ts:202-223
// Context-collapse mode: same suppression. Collapse IS the context
// management system when it's on — the 90% commit / 95% blocking-spawn
// flow owns the headroom problem.
if (feature('CONTEXT_COLLAPSE')) {
  const { isContextCollapseEnabled } = require('../contextCollapse/index.js')
  if (isContextCollapseEnabled()) {
    return false  // Suppress autocompact when collapse is active
  }
}
```

### Layer 4: Autocompact (Full Summarization)

**Trigger:** Token count >= effective context window - 13,000 buffer  
**Mechanism:** LLM-driven summarization of middle conversation turns

**Thresholds:**
```typescript
// autoCompact.ts:62-65
export const AUTOCOMPACT_BUFFER_TOKENS = 13_000
export const WARNING_THRESHOLD_BUFFER_TOKENS = 20_000
export const ERROR_THRESHOLD_BUFFER_TOKENS = 20_000
export const MANUAL_COMPACT_BUFFER_TOKENS = 3_000
```

**Post-compact budgets:**
```typescript
// compact.ts:122-130
export const POST_COMPACT_MAX_FILES_TO_RESTORE = 5
export const POST_COMPACT_TOKEN_BUDGET = 50_000
export const POST_COMPACT_MAX_TOKENS_PER_FILE = 5_000
export const POST_COMPACT_MAX_TOKENS_PER_SKILL = 5_000
export const POST_COMPACT_SKILLS_TOKEN_BUDGET = 25_000
```

### Slot Reservation & Escalation

Claude Code implements an 8K → 64K escalation pattern. The autocompact threshold reserves 13,000 tokens for output during compaction, based on p99.99 of compact summary output being 17,387 tokens.

```typescript
// autoCompact.ts:28-30
const MAX_OUTPUT_TOKENS_FOR_SUMMARY = 20_000
const effectiveContextWindow = getContextWindowSize(model)
return contextWindow - reservedTokensForSummary
```

### Compact Boundary Events

The SDK emits `compact_boundary` events that carry pre-compact discovered tool state:

```typescript
// compact.ts:606-610
const preCompactDiscovered = extractDiscoveredToolNames(messages)
if (preCompactDiscovered.size > 0) {
  boundaryMarker.compactMetadata.preCompactDiscoveredTools = [...preCompactDiscovered].sort()
}
```

---

## 3. Hermes Agent — Dual Compression with Preflight Checks

### Source: `agent/context_compressor.py`, `gateway/run.py`, `agent/prompt_caching.py`

Hermes Agent employs a two-tier compression architecture with distinct responsibilities.

### Tier 1: Gateway Session Hygiene (85% threshold)

**Location:** `gateway/run.py` — runs before agent processes message  
**Purpose:** Safety net for sessions that escape the agent's compressor (e.g., overnight accumulation in Telegram/Discord)

```python
# gateway/run.py — Session hygiene: auto-compress
# Fires at 85% of model context length
# Only when len(history) >= 4 and compression is enabled
```

**Key characteristic:** Uses rough character-based token estimation, not accurate API-reported tokens. Intentional lag prevents premature compression on every turn.

### Tier 2: Agent ContextCompressor (50% threshold, configurable)

**Location:** `agent/context_compressor.py`  
**Purpose:** Primary compression system with accurate API-reported token counts

```yaml
# config.yaml
compression:
  enabled: true
  threshold: 0.50        # Fraction of context window (default: 50%)
  target_ratio: 0.20    # Tail protection token budget
  protect_last_n: 20    # Minimum protected tail messages
```

**Computed values for 200K context:**
```
context_length       = 200,000
threshold_tokens     = 200,000 × 0.50 = 100,000
tail_token_budget    = 100,000 × 0.20 = 20,000
max_summary_tokens  = min(200,000 × 0.05, 12,000) = 10,000
```

### 4-Phase Compression Algorithm

**Phase 1: Prune Old Tool Results (cheap, no LLM)**
Replaces tool outputs >200 chars outside protected tail with `[Old tool output cleared to save context space]`

**Phase 2: Determine Boundaries**
```
[0..2]  ← protect_first_n (system + first exchange)
[3..N]  ← middle turns → SUMMARIZED
[N..end] ← tail (by token budget OR protect_last_n)
```

**Phase 3: Structured Summary Generation**
Uses iterative re-compression — previous summary passed to LLM with instruction to UPDATE rather than summarize from scratch.

**Summary template:**
```
## Goal
## Constraints & Preferences
## Progress
### Done
### In Progress
### Blocked
## Key Decisions
## Relevant Files
## Next Steps
## Critical Context
```

**Phase 4: Assemble Compressed Messages**
Head (with compaction note appended to system prompt on first compression) → Summary → Tail

### Ephemeral Prompt Layers

Hermes injects ephemeral prompts at API-call time that are never persisted:

```python
# Pre-fill messages injected at start of every API call
# Ephemeral — never saved to sessions or trajectories
# Re-loaded from JSON file automatically on restart
```

Applied via `HERMES_PREFILL_MESSAGES_FILE` environment variable.

### Anthropic Prompt Caching Integration

**Strategy:** `system_and_3` — 4 cache_control breakpoints maximum

```
Breakpoint 1: System prompt (stable across all turns)
Breakpoint 2: 3rd-to-last non-system message
Breakpoint 3: 2nd-to-last non-system message
Breakpoint 4: Last non-system message
```

```python
# Cache marker format
marker = {"type": "ephemeral"}
# Or for 1-hour TTL:
marker = {"type": "ephemeral", "ttl": "1h"}
```

**Cache-aware principles:**
1. Stable system prompt = breakpoint 1 cached across turns
2. Cache hits require prefix matching
3. Compression invalidates compressed region cache but system prompt survives
4. Rolling 3-message window re-establishes caching within 1-2 turns

---

## 4. OpenCode — Plugin-Driven Compaction with Smart Pruning

### Source: `packages/opencode/src/session/compaction.ts`, config.mdx

OpenCode implements plugin-driven compaction with configurable hooks and aggressive tool output pruning.

### Tool Output Pruning (Pre-Compression)

```typescript
// compaction.ts:33-36
export const PRUNE_MINIMUM = 20_000
export const PRUNE_PROTECT = 40_000
const TOOL_OUTPUT_MAX_CHARS = 2_000
const PRUNE_PROTECTED_TOOLS = ["skill"]
```

**Prune logic:** Walks backward through messages until finding PRUNE_PROTECT tokens worth of tool calls, then erases output of older tool calls to free context space. Skips `skill` tool to preserve agent capabilities.

```typescript
// compaction.ts:295-341
// Goes backwards through parts until there are PRUNE_PROTECT tokens
// worth of tool calls, then erases output of older tool calls
loop: for (let msgIndex = msgs.length - 1; msgIndex >= 0; msgIndex--) {
  // ... accumulate tool output tokens ...
  if (total <= PRUNE_PROTECT) continue
  pruned += estimate
  toPrune.push(part)
}
```

### Plugin Hook System

```typescript
// experimental.session.compacting — before LLM generates summary
// Allows injecting domain-specific context
const compacting = yield* plugin.trigger(
  "experimental.session.compacting",
  { sessionID: input.sessionID },
  { context: [], prompt: undefined },
)
const nextPrompt = compacting.prompt ?? buildPrompt({ previousSummary, context: compacting.context })
```

### Tail Protection Budget

```typescript
// compaction.ts:134-139
function preserveRecentBudget(input: { cfg: Config.Info; model: Provider.Model }) {
  return (
    input.cfg.compaction?.preserve_recent_tokens ??
    Math.min(MAX_PRESERVE_RECENT_TOKENS, Math.max(MIN_PRESERVE_RECENT_TOKENS, Math.floor(usable(input) * 0.25)))
  )
}
const MIN_PRESERVE_RECENT_TOKENS = 2_000
const MAX_PRESERVE_RECENT_TOKENS = 8_000
```

### Auto-Continue After Compaction

When auto-compaction completes, OpenCode can automatically inject a continue message:

```typescript
// compaction.ts:521-554
// experimental.compaction.autocontinue hook
// Internal marker for auto-compaction followups
metadata: { compaction_continue: true }
synthetic: true
text: "Continue if you have next steps, or stop and ask for clarification..."
```

---

## 5. OpenClaw — Checkpoint-Based Compaction with Retry Guards

### Source: `src/agents/pi-embedded-runner/compact.ts`

OpenClaw's compaction is tightly integrated with its checkpoint system, enabling branching and restoration from pre-compaction states.

### Compaction Checkpoint System

```typescript
// session-compaction-checkpoints.ts
captureCompactionCheckpointSnapshot()
persistSessionCompactionCheckpoint()
cleanupCompactionCheckpointSnapshot()
resolveSessionCompactionCheckpointReason()
```

### Overflow Detection & Retry

```typescript
// pi-embedded-runner/compact.ts:1053
`context overflow detected (attempt ${overflowCompactionAttempts}/${MAX_OVERFLOW_COMPACTION_ATTEMPTS}); attempting auto-compaction`
```

**Retry circuit breaker:**
```typescript
// pi-embedded-runner/compact.ts:1151
`auto-compaction succeeded for ${provider}/${modelId}; retrying prompt`
```

### Pre-Compaction Memory Flush

```typescript
// extensions/memory-core/src/flush-plan.ts
"The session is near auto-compaction; capture durable memories to disk."
```

Threshold: `distance_to_compaction_trigger` config option for tuning when memory flush fires relative to compaction.

### Context Window Guard Integration

```typescript
// context-window-guard.ts
resolveContextWindowInfo()
```

Prevents API failures by detecting context overflow before sending requests.

---

## 6. MSA — Sparse Attention as Alternative to Compaction

### Source: `src/msa/memory_sparse_attention.py`, README.md

MSA (Memory Sparse Attention) represents a fundamentally different approach: instead of compressing context, it uses sparse attention mechanisms to handle 100M tokens natively.

### Architecture Overview

MSA integrates retrieval and generation into a single differentiable loop:

```
1. Global Memory Encoding (offline): forward over corpus to cache chunk-pooled (K̄, V̄, K̄ᵣ)
2. Online Routing & Context Assembly: project query to Qᵣ, match with K̄ᵣ to pick Top-k, load only selected K̄/V̄
3. Sparse Generation: autoregress over sparse context
```

### Key Mechanisms

**Chunk-Mean Pooling:** Compresses K/V states via mean pooling within chunks:
```python
# memory_sparse_attention.py:467-486
def sequence_pooling_kv(self, key_states, value_states, doc_indices, global_chunk_ids):
    pooled_k_chunks = compute_pooled_states_via_cumsum(k_docs, chunk_counts_view, chunk_lengths)
    pooled_v_chunks = compute_pooled_states_via_cumsum(v_docs, chunk_counts_view, chunk_lengths)
    return pooled_k_chunks, pooled_v_chunks
```

**Router Projector:** Selects Top-k documents via cosine similarity:
```python
# Config: top_k_docs, router_layer_idx, decouple_router
# Scoring: mean-pooled over heads, then token-wise max
```

**Document-Wise RoPE:** Prevents position drift between train-short and infer-long:
```
Parallel RoPE: Each document resets positions from 0
Global RoPE: Query's starting index offset by k (Top-k retrieved blocks)
```

### Performance

- **Scaling:** <9% degradation from 16K to 100M tokens
- **Hardware:** 100M token throughput on 2×A800 GPUs
- **Memory Parallel:** Shards routing keys across GPUs, content K̄/V̄ in host DRAM

### Comparison to Compaction

| Aspect | Compaction-Based | MSA Sparse Attention |
|--------|-----------------|---------------------|
| Context limit | Model window (128K-1M) | 100M tokens |
| Information loss | Summarization is lossy | Near-lossless (chunk pooling) |
| Latency | Summarization LLM call | Routing + sparse attention |
| Complexity | Multiple layers, careful tuning | Single differentiable mechanism |
| Hardware | Standard GPU | Requires Memory Parallel infrastructure |

---

## 7. TurboQuant — KV Cache Compression Research

### Source: `research/papers/turboquant.html`

TurboQuant is a research paper from Google demonstrating extreme KV cache compression for transformer models.

### Core Innovation: Two-Stage Compression

**Stage 1: PolarQuant (High-Quality Compression)**
- Randomly rotates data vectors to simplify geometry
- Converts to polar coordinates (radius + angle)
- Uses most compression bits for primary concept
- Eliminates memory overhead of traditional quantization

**Stage 2: QJL (1-bit Error Correction)**
- Uses Johnson-Lindenstrauss Transform to shrink data
- Reduces each vector number to single sign bit (+1 or -1)
- Uses special estimator balancing high-precision query with low-precision data
- Acts as mathematical error-checker eliminating bias

### Results

| Metric | Value |
|--------|-------|
| Compression ratio | 6x (32-bit → 3-bit) |
| Speedup (H100 GPU) | 8x |
| Accuracy loss | Near-zero (demonstrated on LongBench, Needle-in-Haystack, RULER) |
| Approach | Data-oblivious (no training required) |

### Implications for Agents

TurboQuant operates at the KV cache level, not the conversation level. It could complement agentic compaction by:
1. Enabling larger effective context windows before compaction triggers
2. Reducing memory pressure during generation
3. Serving as infrastructure for graduated compaction rather than replacement

---

## 8. Cross-Cutting Analysis

### 8.1 Compression Strategy Spectrum

| System | Strategy | Layers | Summarization | Pruning | Caching |
|--------|----------|--------|---------------|---------|---------|
| Claude Code | Graduated 4-layer | 4 | Yes (autocompact) | Yes (snip) | Yes (microcompact cache_edits) |
| Hermes Agent | Dual-tier | 2 | Yes | Yes (tool results) | Yes (Anthropic breakpoints) |
| OpenCode | Plugin + prune | 2 | Yes | Yes (tool output) | No |
| OpenClaw | Checkpoint + retry | 2 | Yes | Via memory flush | No |
| MSA | Sparse attention | 0 | No (chunk pooling) | No | Different mechanism |

### 8.2 Token Budget Management

| System | Trigger Threshold | Buffer Reserved | Tail Protection |
|--------|------------------|----------------|------------------|
| Claude Code | window - 13,000 | 13,000 (output) | 50,000 post-compact |
| Hermes Agent | 50% context | 20% of threshold | 20 messages OR budget |
| OpenCode | overflow detected | adaptive (25% of usable) | 2,000-8,000 tokens |
| OpenClaw | 85% (gateway), configurable | Not specified | Memory flush before compact |

### 8.3 Pressure Monitoring Thresholds

| System | Warning | Auto-Compact | Blocking |
|--------|---------|--------------|----------|
| Claude Code | window - 20,000 | window - 13,000 | window - 3,000 |
| Hermes Agent | 85% of threshold (42.5%) | 50% context | N/A |
| OpenCode | N/A | overflow detection | N/A |
| OpenClaw | N/A | configurable | configurable |

### 8.4 Slot Reservation Patterns

**Claude Code (8K → 64K escalation):**
- Reserved for summary output during compaction: 20,000 tokens (based on p99.99)
- Post-compact file restoration budget: 50,000 tokens
- Per-file budget: 5,000 tokens
- Per-skill budget: 5,000 tokens

**OpenCode (2K → 8K adaptive):**
```typescript
MIN_PRESERVE_RECENT_TOKENS = 2_000
MAX_PRESERVE_RECENT_TOKENS = 8_000
// Adaptive: 25% of usable context
```

**Hermes Agent (20% of threshold):**
```
tail_token_budget = threshold_tokens × 0.20
```

### 8.5 Context Loss Mitigation

| System | Mechanism |
|--------|-----------|
| Claude Code | Iterative re-compression; preservedSegment metadata; compact_boundary with preCompactDiscoveredTools |
| Hermes Agent | Iterative update summarization; structured summary template preserving Done/In-Progress/Blocked |
| OpenCode | Previous summary passed to LLM for updating; plugin hooks for domain context injection |
| OpenClaw | Compaction checkpoints enable branch/restore; memory flush before compact |
| MSA | Chunk-mean pooling is lossy but predictable; Top-k selection is content-aware |

### 8.6 Sparse Attention as Alternative

**When to use compaction:**
- Need to reduce API costs (summarized context = fewer tokens)
- Model has limited native context window
- Task benefits from explicit memory consolidation (summary becomes agent memory)

**When to use sparse attention (MSA approach):**
- Need maximum context fidelity (no information loss)
- Have infrastructure for Memory Parallel (GPU clusters)
- Working with very long documents (>1M tokens)
- Task is retrieval-intensive rather than reasoning-intensive

---

## 9. ACE Recommendation Table

| Pattern | System | Recommendation | Rationale |
|---------|--------|----------------|-----------|
| Graduated multi-layer compression | Claude Code | **ADOPT** | Most robust — separates concerns, prevents premature heavy compression |
| 4+ compression layers | Claude Code | **ADOPT** | snip → microcompact → collapse → autocompact provides graduated response |
| Dual-tier (safety net + primary) | Hermes Agent | **ADOPT** | Gateway catches what agent misses; 85%/50% separation is well-calibrated |
| Iterative re-compression | Claude Code, Hermes | **ADOPT** | Preserves context across compaction cycles; not just dump-and-summarize |
| Structured summary template | Hermes, OpenCode | **ADOPT** | Consistent format enables reliable parsing; Done/In-Progress/Blocked is actionable |
| Cache-edits for tool results | Claude Code | **ADOPT** | Preserves cache prefix, avoids 75% cache invalidation cost |
| Tool output pruning | OpenCode | **ADOPT** | Cheap pre-pass recovers tokens without LLM call; 40K protect threshold is reasonable |
| Plugin hooks for compaction | OpenCode | **ADOPT** | Extensibility for domain-specific context injection |
| Ephemeral prompt layers | Hermes Agent | **ADOPT** | Injection without persistence avoids context pollution |
| Anthropic prompt caching | Hermes Agent | **ADOPT** | 75% input cost reduction; rolling 3-message window maintains cache hits |
| Compact boundary events | Claude Code | **ADOPT** | Enables SDK consumers to track state; preserves discovered tools |
| Checkpoint-based compaction | OpenClaw | **ADOPT** | Enables branch/restore from pre-compact state |
| Circuit breaker on retries | Claude Code | **ADOPT** | MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES=3 prevents infinite retry loops |
| Time-based microcompact | Claude Code | **ADOPT** | Cache TTL expiration = predictable token recovery |
| Sparse attention (MSA) | EverMind-AI | **ADAPT** | Requires specialized hardware/infrastructure; not suitable for all deployments |
| KV cache quantization (TurboQuant) | Google Research | **ADAPT** | Infrastructure-level optimization; complements rather than replaces agent compaction |
| 90%/95% collapse thresholds | Claude Code | **CAUTION** | Tight thresholds may race with autocompact; ensure mutual exclusion |
| Single-threshold compaction | All | **AVOID** | Without graduated response, either compresses too early or too late |
| Lossy-only compression | Most | **AVOID** | Without iterative update, previous summaries degrade over multiple compactions |
| Blocking compaction UI | OpenClaw | **AVOID** | User-facing blocking indicators add complexity without proportional benefit |

---

## 10. Technical Specifications Reference

### Claude Code Compaction Constants

```typescript
// autoCompact.ts
const MAX_OUTPUT_TOKENS_FOR_SUMMARY = 20_000
const AUTOCOMPACT_BUFFER_TOKENS = 13_000
const WARNING_THRESHOLD_BUFFER_TOKENS = 20_000
const ERROR_THRESHOLD_BUFFER_TOKENS = 20_000
const MANUAL_COMPACT_BUFFER_TOKENS = 3_000
const MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES = 3

// compact.ts
const POST_COMPACT_MAX_FILES_TO_RESTORE = 5
const POST_COMPACT_TOKEN_BUDGET = 50_000
const POST_COMPACT_MAX_TOKENS_PER_FILE = 5_000
const POST_COMPACT_MAX_TOKENS_PER_SKILL = 5_000
const POST_COMPACT_SKILLS_TOKEN_BUDGET = 25_000
```

### Hermes Agent Compression Config

```yaml
compression:
  enabled: true
  threshold: 0.50      # 50% of context
  target_ratio: 0.20   # 20% of threshold = tail budget
  protect_last_n: 20
protect_first_n: 3    # Hardcoded
```

### OpenCode Compaction Constants

```typescript
const PRUNE_MINIMUM = 20_000
const PRUNE_PROTECT = 40_000
const TOOL_OUTPUT_MAX_CHARS = 2_000
const DEFAULT_TAIL_TURNS = 2
const MIN_PRESERVE_RECENT_TOKENS = 2_000
const MAX_PRESERVE_RECENT_TOKENS = 8_000
```

### MSA Configuration

```python
# Router config
top_k_docs: int           # Top-k documents to select
router_layer_idx: str      # "all" or comma-separated layer indices
decouple_router: bool     # Separate router from main attention

# Pooling config
pooling_kernel_size: int  # Chunk size for mean pooling
head_reduce_method: str   # "max" or "mean"
query_reduce_method: str  # "max", "mean", or "last"
chunk_reduce_method: str   # "max" or "mean"
```

---

## 11. Synthesis

### What Works

1. **Graduated compression** across multiple layers — no single mechanism handles all scenarios
2. **Iterative re-compression** — updating existing summaries preserves context across cycles
3. **Cache-aware design** — prompt caching integration requires careful marker placement and cache invalidation handling
4. **Safety nets** — dual-tier compression (e.g., Hermes 85%/50%) catches what primary layer misses
5. **Tool output pruning** — cheap pre-pass before LLM summarization recovers significant tokens
6. **Structured templates** — consistent summary format enables downstream processing

### What Doesn't Work

1. **Single-threshold systems** — either over-compress (every turn) or under-compress (misses overflow)
2. **Blocking user-facing compaction** — adds complexity without proportional benefit
3. **Lossy-only approaches** — without iterative update, multiple compactions degrade context
4. **Ignoring cache invalidation** — compression that breaks prompt cache causes 75%+ cost increase

### Open Questions

1. **Optimal layer count** — Is 4 layers (Claude Code) better than 2 (Hermes)? Tradeoff between complexity and coverage.
2. **Sparse attention readiness** — Can MSA-style architecture be integrated into production agent frameworks without specialized hardware?
3. **TurboQuant timeline** — When will KV cache quantization be available in standard inference stacks?

---

**Files Affected:**
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/compaction.md`
