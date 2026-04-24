# Agent Loop Architectures: Cross-System Comparison

**Study:** agents-study  
**Slice:** 4 — Agent Loops & Execution Cycles  
**Systems examined:** openclaw, opencode, oh-my-openagent, goose, hermes-agent, pi-mono, claude-code-source-code, autoresearch  
**Source:** `/home/jay/programming/ace_prototype/design/units/agents-study/research/`

---

## 1. Execution Cycle Taxonomy

### 1.1 ReAct Classic (rejected by hermes-agent)

Most agents follow the classic ReAct pattern: think → tool call → observe → think. hermes-agent explicitly rejects this in `run_agent.py`:

```python
# run_agent.py — hermes-agent
# We reject the standard ReAct pattern (rethink after every tool result).
# Instead: collect all tool results, then reason once over the full batch.
```

hermes-agent replaces ReAct with a **delegation-driven batched loop**: tool calls are collected and executed concurrently, then a single reasoning pass evaluates all results before the next iteration.

### 1.2 Delegation-Driven Loop (hermes-agent, oh-my-openagent)

Rather than agent-as-tool-executor, the agent acts as a **delegator**. It spawns subagents (workers) to perform tasks, collects results, and re-delegates or synthesizes.

| System | Delegation mechanism | Max depth | Concurrent workers |
|--------|---------------------|-----------|-------------------|
| hermes-agent | `delegate_task` in toolsets, `ThreadPoolExecutor` | 2 (isolated context, restricted toolsets) | 8 (max workers) |
| oh-my-openagent | `delegate-task` tool, category-based routing to worker agents | not hard-limited | 5 per model/provider (background agents) |
| openclaw | Embedded runner via `callGateway` (nested agent steps) | unbounded (compaction events as termination) | 1 primary + embedded |

hermes-agent's delegation uses **path-overlap detection** to prevent concurrent file-tools on overlapping paths:

```python
# run_agent.py
def _should_parallelize_tool_batch(self, tool_calls: list[ToolCall]) -> bool:
    """Check path overlaps between file tools; abort parallelization if conflict."""
```

oh-my-openagent uses a **three-layer chain**: Intent Gate → Prometheus (planner) → Atlas (executor) → workers. Atlas executes plans from Prometheus; Sisyphus (main) supervises the whole.

### 1.3 AsyncGenerator Pipeline (claude-code-source-code)

claude-code uses AsyncGenerator as the fundamental loop driver in `query.ts`:

```typescript
// query.ts — claude-code-source-code
async function* query(state: State) {
  for (;;) {
    state = await queryLoop(state);
    if (state.done) break;
    // loop continues — State carries mutable cross-iteration context
  }
}
```

`State` is a mutable bag carried across iterations containing: messages, toolUseContext, autoCompactTracking, turnCount, budgetTracker (token budget across compaction boundaries). The AsyncGenerator pattern allows the loop to be consumed incrementally by callers.

### 1.4 Dual-Loop Architecture (openclaw, pi-mono)

**openclaw** has two distinct loops at different process layers:

1. **Gateway outer loop** (`run-loop.ts`): Process lifecycle — handles SIGTERM/SIGINT/SIGUSR1, drain-on-restart, session management, compaction events.
2. **Agent inner loop** (`agent-runner.ts`): ReAct tool execution — calls `runAgentTurnWithFallback`, embedded within the gateway session.

**pi-mono** (`agent-loop.ts`) separates concerns differently:

1. **Outer loop** (`agentLoop`): Processes follow-up messages that arrive after the agent would otherwise stop. Continues when queued messages arrive.
2. **Inner loop** (`runLoop`): Processes tool calls and steering messages within a turn.

```typescript
// agent-loop.ts — pi-mono
export async function agentLoop(context: AgentContext): Promise<void> {
  for (;;) {
    const pending = context.messages.filter(isFollowUp);
    if (pending.length === 0) break; // outer loop exit
    for (const msg of pending) {
      await runLoop(context, msg); // inner loop per follow-up
    }
  }
}
```

### 1.5 Fixed-Time Experiment Loop (autoresearch)

autoresearch implements a minimal loop with a hard wall-clock budget:

```
read → propose → commit → run 5 min → measure → keep/reset → repeat
```

- Single-GPU, single-file, fixed 5-minute training budget per iteration
- Git-based checkpointing: advance on improvement, reset on regression
- **NEVER STOP** principle: autonomous until externally interrupted
- `program.md` serves as the research org code — it IS the loop protocol

This is the only system where the agent loop IS the outer experiment loop, not embedded within a host process.

### 1.6 Effect-Based Service Loop (opencode)

opencode uses the fp-ts `Effect` framework as a purely functional runtime. Agents are services (not objects) with modes:

- `build`: primary editing mode
- `plan`: read-only planning (all edit tools disabled)
- `general`: research subagent (`@general`)
- `explore`: read-only exploration
- `compaction`, `title`, `summary`: auxiliary modes

Tool permission rulesets (allow/deny) govern what each mode can do. The loop is implicit in the Effect runtime's fiber scheduling.

---

## 2. Concurrency Models

| System | Model | Tool execution | Subagent execution |
|--------|-------|---------------|-------------------|
| hermes-agent | ThreadPoolExecutor (Python) | Concurrent (path-overlap guarded) | Concurrent delegation (max 8 workers) |
| oh-my-openagent | Node.js async + worker agents | Sequential per worker | Background concurrent (5 per model/provider) |
| pi-mono | Sequential (configurable) | `executeToolCallsSequential` / `executeToolCallsParallel` | Not applicable |
| claude-code | AsyncGenerator + await | Sequential within loop iteration | Not applicable |
| openclaw | Sequential (embedded runner) | Sequential via gateway RPC | Nested via callGateway |
| goose | Rust async (Tokio?) | Via MCP Container | Not applicable |
| autoresearch | Sequential Python | Single experiment run | Not applicable |
| opencode | Effect/fiber (fp-ts) | Effect-parallel where permitted | Subagent via general mode |

**Key distinction:** hermes-agent is the only system that explicitly parallels tool execution within a single turn using ThreadPoolExecutor with path-overlap detection. pi-mono offers both sequential and parallel modes as a configuration option.

---

## 3. Speculative Execution

### claude-code — Speculative Bash Permission Checking

claude-code pre-checks bash command safety before executing:

```typescript
// bashPermissions.ts — claude-code-source-code
speculativeChecks: Map<string, SpeculativeCheck>
// Pre-compute permission states for commands before calling
```

This allows the system to surface permission prompts or substitute safe alternatives without breaking the tool call flow.

### hermes-agent — Path Overlap Detection

Before parallelizing a batch of tool calls, hermes-agent checks for file path overlaps:

```python
# run_agent.py
def _should_parallelize_tool_batch(self, tool_calls: list[ToolCall]) -> bool:
    overlap = detect_path_overlap(tool_calls)
    if overlap:
        return False  # sequentialize instead
```

This prevents race conditions when two concurrent file operations target the same directory or file.

### oh-my-openagent — Intent Gate

The Intent Gate acts as a speculative classifier: it categorizes user intent before committing to a planning or execution path. Misclassification routes to a different agent tier.

---

## 4. Error Recovery

| System | Mechanism | Details |
|--------|-----------|---------|
| hermes-agent | Stale timeout detectors | Separate timers for streaming vs non-streaming API calls; credential pool rotation with billing/rate-limit/auth recovery |
| hermes-agent | Fallback chain | Per-turn primary runtime restoration after fallback activation; tool call retries with budget |
| claude-code | Budget tracker | Token budget tracked across compaction boundaries; compaction triggered on threshold |
| goose | Compaction threshold | `DEFAULT_COMPACTION_THRESHOLD` triggers context compaction |
| openclaw | Drain-on-restart + compaction events | SIGUSR1 triggers session drain; gateway handles lifecycle |
| autoresearch | Git reset | Regression triggers reset to last checkpoint; no retry of failed experiment config |
| pi-mono | `terminate` flag on tool results | Tool can signal loop termination without error |
| oh-my-openagent | 52 lifecycle hooks | Hooks at Core/Continuation/Skill tiers allow recovery at any phase |
| opencode | Permission rule violation | Edit tools denied in plan mode → graceful fallback |

---

## 5. Iteration Pattern Details

### hermes-agent — Iteration Budget + Delegate Task

```python
# run_agent.py
class IterationBudget:
    # Thread-safe iteration counting with capacity limits
    pass

# Tool execution
def _execute_tool_calls_concurrent(self, tool_calls, iteration_budget):
    with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
        futures = {executor.submit(self._execute_single, tc): tc for tc in tool_calls}
        for future in as_completed(futures):
            result = future.result()
```

Subagent spawning uses isolated context with restricted toolsets:

```python
# delegate_tool.py (~2164 LOC)
def delegate_task(task, context, max_depth=2):
    # Isolated context, subagent toolsets restricted, max depth enforced
```

### pi-mono — Streaming + Follow-up

```typescript
// agent-loop.ts — pi-mono
streamAssistantResponse(context, turn):
    // LLM streaming with incremental message updates
    // Yields partial content as it arrives

getSteeringMessages(): Hook for external message injection without loop interruption
getFollowUpMessages(): Hook for post-turn follow-up queue
```

### claude-code — State + Budget Tracker

```typescript
// query.ts — claude-code-source-code
interface State {
    messages: Message[];
    toolUseContext: ToolUseContext;
    autoCompactTracking: AutoCompactTracking;
    turnCount: number;
    // budgetTracker carries token budget across compaction boundaries
}
```

### autoresearch — Program as Loop

```markdown
# program.md — autoresearch
LOOP:
  read previous results
  propose experiment change
  commit to git
  run for 5 minutes
  measure improvement
  if improved: advance
  else: reset to checkpoint
  goto LOOP
```

---

## 6. ACE Recommendation Table

| Pattern | System(s) | Recommendation | Rationale |
|---------|-----------|---------------|-----------|
| Delegation-driven loop (subagent spawning) | hermes-agent, oh-my-openagent | **ADOPT** | Scales agent capacity beyond single-turn context; hermes-agent's depth-2 limit prevents runaway loops |
| Batched tool execution (no ReAct per-call) | hermes-agent | **ADOPT** | Reduces round-trip latency; batch reasoning over full result set is more coherent |
| ThreadPoolExecutor for concurrent tools | hermes-agent | **ADOPT** | Path-overlap guard makes concurrent execution safe; 8-worker cap prevents resource exhaustion |
| AsyncGenerator as loop driver | claude-code | **ADOPT** | Clean separation between loop state and loop body; enables incremental consumption |
| Outer/inner loop separation | openclaw, pi-mono | **ADOPT** | Outer loop handles cross-turn concerns (lifecycle, follow-up); inner loop handles turn execution |
| Follow-up message queuing | pi-mono | **ADOPT** | Prevents premature loop exit when messages arrive during agent reasoning |
| Token budget tracking across compaction | claude-code | **ADOPT** | Maintains execution coherence across context boundaries |
| Git-based checkpointing (experiment loop) | autoresearch | **ADOPT** for research; **AVOID** for general agents | Appropriate for autonomous experiment loops; too destructive for interactive use |
| Fixed wall-clock budget loop | autoresearch | **ADOPT** for narrow research tasks; **AVAPT** for general agents | Prevents runaway experiments; may be too restrictive for open-ended tasks |
| ReAct per-call (think-act-observe repeat) | Most systems | **AVOID** (adapt from hermes) | High latency for tool-rich tasks; batch execution more efficient |
| Path-overlap detection for concurrent tools | hermes-agent | **ADOPT** | Essential safety guard for concurrent file operations |
| Speculative permission checking | claude-code | **ADOPT** | Eliminates blocking permission prompts mid-loop |
| Steer injection without loop interruption | pi-mono, hermes-agent (_pending_steer) | **ADOPT** | Enables human-in-the-loop without breaking agent flow |
| Lifecycle hooks (52 hooks) | oh-my-openagent | **ADAPT** | Excessive for most use cases; 5-10 well-chosen hooks suffice |
| Intent Gate (pre-classification) | oh-my-openagent | **ADAPT** | Useful for multi-agent routing; adds complexity for single-agent systems |
| Effect/Effect runtime | opencode | **ADAPT** | Excellent for purely functional style; overkill for imperative agents |
| Permission rule sets (allow/deny) | opencode | **ADOPT** | Clean separation of mode capabilities; scales to multi-mode agents |
| Drain-on-restart lifecycle | openclaw | **ADAPT** | Useful for long-running agents; adds complexity for short-lived invocations |

---

## 7. Key Findings

1. **ReAct is being abandoned.** hermes-agent's explicit rejection signals a broader trend: per-call observe-think cycles are being replaced by batch-then-reason patterns. The cost is complexity in the tool execution scheduler.

2. **Delegation is the dominant scaling pattern.** 5 of 8 systems either delegate to subagents (hermes, oh-my-openagent, openclaw nested steps, opencode general mode, pi-mono workers) or to background processes (autoresearch). hermes-agent's depth-2 limit with isolated context is the most disciplined implementation.

3. **Concurrency within a turn is rare but valuable.** hermes-agent's path-overlap-guarded ThreadPoolExecutor and pi-mono's optional parallel mode are the only systems that execute multiple tool calls concurrently within a single turn. Most systems serialize tool execution.

4. **Outer/inner loop separation is a winning pattern.** openclaw and pi-mono both benefit from this separation: outer loop handles session lifecycle and cross-turn state; inner loop handles turn execution. claude-code achieves similar separation via `State` carried across AsyncGenerator iterations.

5. **Speculative pre-checks improve throughput.** claude-code's bash permission pre-checking and hermes-agent's path-overlap detection are both forms of "speculative safety" — making safety decisions before committing to execution. This avoids mid-loop interruptions.

6. **Error recovery strategies cluster by domain.** Interactive agents (openclaw, goose, pi-mono, claude-code) use compaction/restart strategies. Research agents (autoresearch) use git-based reset. hermes-agent uses credential rotation and fallback chains — the most sophisticated multi-tier recovery.

---

*Generated by agents-study slice 4. Source files in `design/units/agents-study/research/`.*