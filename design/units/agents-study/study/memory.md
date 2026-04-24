# Slice 2: Memory Systems — Cross-Cutting Analysis

**Unit:** agents-study  
**Output:** `design/units/agents-study/study/memory.md`  
**Date:** 2026-04-23  
**Systems Analyzed:** OpenClaw, Claude Code, Open Code, Hermes Agent, Honcho, OpenViking, MSA, AutoResearch, karpathy/autoresearch

---

## 1. Storage Backends

### 1.1 File-Based Memory

| System | Implementation | Backing Store | Bounded |
|--------|---------------|---------------|---------|
| **Hermes Agent** | `memory_tool.py` — `MEMORY.md` / `USER.md` | Flat text files, `§`-delimited entries | Yes: 2200 chars (memory), 1375 chars (user) |
| **Claude Code** | `memdir/` — auto-memory directory, per-project | Multiple Markdown files with frontmatter | No hard limit; staleness warnings |
| **OpenClaw** | `memory-core` plugin + `memory-wiki` plugin | Markdown files in `~/.openclaw/memories/` | No hard limit |
| **Open Code** | Plugin ecosystem (`opencode-working-memory`, `open-mem`) | Typed observation files | Configurable |

**Analysis:** File-based memory is simple, auditable, and survives tool failures. Hermes Agent's character-limit bounds are unique in enforcing small, curated entries. Claude Code and OpenClaw use unbounded files with staleness detection rather than hard limits.

### 1.2 Database-Backed Memory

| System | Implementation | Database | Search |
|--------|---------------|----------|--------|
| **Hermes Agent** | `session_search_tool.py` — `hermes_state.py` | SQLite + FTS5 | Full-text search with BM25 ranking |
| **Honcho** | `crud/document.py` — vector storage | HNSW indexes | Semantic similarity via embeddings |
| **OpenViking** | `openviking/storage/` — vector DB | RocksDB/LevelDB + in-memory indexes | Hybrid sparse + dense retrieval |

**Analysis:** Hermes Agent's SQLite + FTS5 is lightweight and zero-dependency. Honcho's HNSW vector DB enables semantic search at scale. OpenViking's tiered storage (RocksDB for persistence, in-memory indexes for speed) is the most sophisticated.

### 1.3 GPU/Accelerated Memory

| System | Implementation | Hardware | Use Case |
|--------|---------------|---------|----------|
| **MSA** | `MemorySparseAttention` — KV cache in GPU | NVIDIA A800, VRAM | 100M-token context via sparse attention |
| **AutoResearch** | `train.py` — VRAM tracking | GPU VRAM | Experiment memory management |

**Analysis:** MSA is unique in making GPU memory the primary storage for long-context reasoning. The router projector and chunk-mean pooling enable selective KV cache retrieval.

---

## 2. Retrieval Strategies

### 2.1 Keyword / Full-Text Search

| System | Strategy | Details |
|--------|----------|---------|
| **Hermes Agent** | FTS5 BM25 | `db.search_messages()` — phrase, proximity, boolean FTS5 queries |
| **Claude Code** | `grep` + `read` | Tool-based access to memory files via Read/Grep/Glob tools |

### 2.2 Semantic / Embedding Search

| System | Strategy | Details |
|--------|----------|---------|
| **Honcho** | HNSW + cosine similarity | `embedding_client.embed()` + `crud.query_documents()` |
| **Open Code plugins** | Embedding-based recall | `opencode-working-memory` — semantic similarity |

### 2.3 Hybrid Retrieval

| System | Strategy | Details |
|--------|----------|---------|
| **OpenViking** | Two-stage retrieval | 1. Vector search (sparse/dense) 2. Reranking (`rerank()`) |
| **MSA** | Router projector + sparse attention | Top-k document selection via cosine similarity on pooled K/V |

### 2.4 LLM-Powered Recall

| System | Strategy | Details |
|--------|----------|---------|
| **Hermes Agent** | Gemini Flash summarization | `session_search` — summarize top-k sessions via LLM |
| **Claude Code** | Sibling query (Sonnet side-query) | Mid-message "memory reminders" via LLM classification |
| **OpenViking** | ReAct extraction loop | `memory_extractor` — tool/skill memory extraction via ReAct |

### 2.5 Recursive / Decompositional

| System | Strategy | Details |
|--------|----------|---------|
| **RLM (RISE)** | REPL environment + recursive sub-calls | Root LM peeks, greps, partitions, maps context recursively |

**Analysis:** RLM is architecturally distinct — it treats context as a variable to interact with rather than retrieve from. This eliminates the need for a retrieval step entirely.

---

## 3. Attribution Models

### 3.1 Source Citation

| System | Attribution | Format |
|--------|------------|--------|
| **OpenClaw** | Dream diary + memory wiki | `memory/YYYY-MM-DD.md:line` citations |
| **Claude Code** | Memory file path in system prompt | `~/.claude/projects/<path>/memory/` |
| **Hermes Agent** | Entry delimiter + lock file | `§`-delimited entries, atomic write |

### 3.2 Provenance Tracking

| System | Tracking | Details |
|--------|----------|---------|
| **OpenViking** | Monotonic versioning | `memory_extractor` checks monotonic violation; skips stale writes |
| **Claude Code** | Frontmatter metadata | `name`, `description`, `type` fields per memory file |
| **Honcho** | Observation IDs | `include_observation_ids` flag for traceable recall |

**Analysis:** OpenViking's monotonic versioning is the most rigorous — it prevents memory regression by rejecting stale writes. Claude Code's frontmatter approach is developer-friendly but requires discipline.

---

## 4. Tier Structures

### 4.1 Timescale-Based Tiers

| System | Tiers | Structure |
|--------|-------|-----------|
| **OpenViking** | L0/L1/L2 | `ABSTRACT` (L0) → `OVERVIEW` (L1) → `DETAIL` (L2) via `ContextLevel` enum |
| **Claude Code** | 4 memory types | `User` → `Project` → `Local` → `Managed` (scope: private/team) |
| **Honcho** | Observation → Representation → Peer Card | Ephemeral → Indexed → Synthesized |

### 4.2 Scope-Based Tiers

| System | Scopes | Details |
|--------|--------|---------|
| **Claude Code** | `private` / `team` | Auto-memory dir + team memory subdirectory |
| **Honcho** | `workspace` / `peer` / `session` | Multi-tenant via workspace isolation |
| **Hermes Agent** | `memory` / `user` | Two separate files: MEMORY.md and USER.md |

### 4.3 Content-Based Tiers

| System | Types | Details |
|--------|-------|---------|
| **Claude Code** | `user`, `feedback`, `project`, `reference` | Semantic taxonomy with scope guidance |
| **OpenViking** | `viking://memory/`, `viking://session/` | URI-based context type classification |

**Analysis:** OpenViking's L0/L1/L2 progressive loading is unique in enabling progressive disclosure based on query complexity. Claude Code's 4-type taxonomy is the most sophisticated content classification.

---

## 5. Consolidation Approaches

### 5.1 Periodic Consolidation

| System | Trigger | Method |
|--------|---------|--------|
| **Claude Code** | Auto-dream (background) | LLM consolidation into memory files |
| **Honcho** | Dreamer agent (scheduled) | Random walk exploration + deduplication |
| **OpenClaw** | Dream diary replay | Memory wiki compilation from raw dream diary |

### 5.2 On-Demand Consolidation

| System | Trigger | Method |
|--------|---------|--------|
| **Claude Code** | `/remember` skill | User-invoked memory review + promotion |
| **Hermes Agent** | Tool call | Agent writes to MEMORY.md manually |

### 5.3 Event-Driven Consolidation

| System | Trigger | Method |
|--------|---------|--------|
| **Honcho** | Message creation → deriver agent | Background queue → `create_observations` tool |
| **OpenViking** | Tool/skill usage → ReAct loop | `memory_extractor` extracts and writes |

### 5.4 LLM-Free Consolidation

| System | Method | Details |
|--------|--------|---------|
| **Hermes Agent** | Exact-match deduplication | `dict.fromkeys()` preserves order, removes exact dupes |
| **OpenViking** | Monotonic violation check | `updated_at` timestamp comparison |

**Analysis:** Honcho's 3-agent architecture (deriver/dialectic/dreamer) is the most sophisticated — dedicated agents for ingestion, recall, and consolidation. Claude Code's auto-dream is simpler but effective.

---

## 6. Compression Ratios

### 6.1 Bounded Memory Systems

| System | Limit | Compression |
|--------|-------|------------|
| **Hermes Agent** | 2200 chars (memory), 1375 chars (user) | Hard cap; entries are dense by design |
| **MSA** | 100M tokens (GPU KV cache) | Chunk-mean pooling: O(L) complexity |

### 6.2 Unbounded Systems with Staleness

| System | Mechanism | Details |
|--------|-----------|---------|
| **Claude Code** | Memory drift caveat | `MEMORY_DRIFT_CAVEAT` — verify memories against current state |
| **Claude Code** | Staleness warnings | `statusNoticeDefinitions.ts` — large file warnings |
| **OpenViking** | Progressive loading | L0 (abstract) loads first; L1/L2 on demand |

### 6.3 Context Compaction vs. Memory

| System | Approach | Details |
|--------|----------|---------|
| **Claude Code** | 4-layer compaction | snip → microcompact → collapse → autocompact |
| **MSA** | Sparse attention | No compaction needed; <9% degradation 16K→100M |
| **RLM** | Recursive decomposition | No compaction; context peeks/grep/partitions |

**Analysis:** MSA achieves the highest effective compression ratio through sparse attention — 100M tokens without compaction. RLM avoids compaction by recursive context interaction. Claude Code's 4-layer approach is the most battle-tested.

---

## 7. Key Design Patterns

### 7.1 Frozen Snapshot Pattern

**Hermes Agent:** System prompt snapshot captured at `load_from_disk()` time. Mid-session writes persist to disk but do NOT update the snapshot. This preserves prefix cache stability.

```python
# memory_tool.py — frozen snapshot for system prompt
self._system_prompt_snapshot = {
    "memory": self._render_block("memory", self.memory_entries),
    "user": self._render_block("user", self.user_entries),
}
```

### 7.2 Atomic Write Pattern

**Hermes Agent:** Memory writes use temp-file + `os.replace()` for atomicity. Separate `.lock` file allows concurrent reads of the main file.

```python
# memory_tool.py — atomic write via rename
fd, tmp_path = tempfile.mkstemp(dir=str(path.parent), suffix=".tmp", prefix=".mem_")
os.replace(tmp_path, str(path))  # Atomic on same filesystem
```

### 7.3 Monotonic Memory Versioning

**OpenViking:** `memory_extractor` rejects writes if `updated_at` would decrease. This prevents stale writes from overwriting fresher content.

### 7.4 Entity-Centric Memory

**Honcho:** Peer-centric model where observations are indexed by `observer`/`observed` peers. Representations synthesized from observations. Dedicated Dialectic agent for recall.

### 7.5 Virtual Filesystem URI

**OpenViking:** `viking://` URI scheme unifies memory (`viking://memory/`), resources (`viking://resources/`), and skills (`viking://agent/skills/`). All accessible via same retrieval pipeline.

---

## 8. RLM (Recursive Language Models) Analysis

**Paper:** [Recursive Language Models (RISE)](https://alexzhang13.github.io/blog/2025/rlm/) — Zhang & Khattob, MIT CSAIL

### 8.1 Core Concept

RLM treats the input context as a **variable in a Python REPL environment**. The root LM can:
1. **Peek** — read the first N characters of context
2. **Grep** — regex search for keywords
3. **Partition + Map** — chunk context, launch recursive sub-calls over chunks
4. **Summarize** — distill subsets for outer LM

### 8.2 Implications for ACE

| RLM Pattern | ACE L2/L3 Implication |
|-------------|----------------------|
| Recursive decomposition | L2 summarization should support nested/contextual summaries |
| REPL-style interaction | ACE context should be addressable, not just retrievable |
| Sub-call routing | ACE should support multi-level memory queries |

### 8.3 Comparison: RLM vs. Embedding vs. Sparse Attention

| Approach | Context Handling | Retrieval | Scalability |
|----------|-----------------|-----------|-------------|
| **RLM** | Variable in REPL | Recursive sub-calls | 10M+ tokens |
| **Embedding (Honcho, Open Code)** | Indexed vectors | Similarity search | Bounded by index size |
| **Sparse Attention (MSA)** | Full KV cache | Top-k router | 100M tokens |

---

## 9. Comparative Summary Table

| System | Storage | Retrieval | Attribution | Tiers | Consolidation | Compression |
|--------|---------|-----------|-------------|-------|---------------|-------------|
| **OpenClaw** | File (Markdown) | Semantic (plugin) + dream diary | File path citations | Plugin-based | Scheduled dreams | Unbounded |
| **Claude Code** | File (Markdown) | Tool-based + LLM recall | Frontmatter + path | 4 types (private/team) | Auto-dream + manual | Unbounded + drift caveat |
| **Hermes Agent** | File (Markdown) | FTS5 + LLM summarization | Entry delimiter | 2 stores (memory/user) | Manual tool call | Hard cap (2200 chars) |
| **Honcho** | Vector DB (HNSW) | Semantic embedding | Observation IDs | Peer/session/workspace | Deriver + Dreamer agents | Unbounded |
| **OpenViking** | RocksDB + indexes | Hybrid vector + rerank | URI + monotonic versioning | L0/L1/L2 | ReAct extraction | Progressive loading |
| **MSA** | GPU KV cache | Router projector + sparse attn | N/A (no attribution) | Single tier | None (no compaction) | 100M token via sparse |
| **AutoResearch** | VRAM | N/A | N/A | Single tier | None | 5-min fixed budget |
| **RLM** | REPL environment | Recursive sub-calls | N/A | Dynamic partitioning | Recursive decomposition | Near-infinite via recursion |

---

## 10. ACE Recommendation Table

| Pattern | System | Recommendation | Rationale |
|---------|--------|---------------|-----------|
| **Character-limited bounded memory** | Hermes Agent | **ADOPT** | Hard limits prevent unbounded growth; frozen snapshot preserves prefix cache |
| **FTS5 full-text search** | Hermes Agent | **ADOPT** | Zero-dependency, SQLite-based, phrase + boolean + proximity queries |
| **Atomic file writes** | Hermes Agent | **ADOPT** | Temp-file + rename prevents corruption from concurrent access |
| **Memory drift caveat** | Claude Code | **ADOPT** | Staleness warning prevents acting on outdated information |
| **Four-type memory taxonomy** | Claude Code | **ADOPT** | `user`/`feedback`/`project`/`reference` captures semantic distinctions |
| **Frontmatter metadata** | Claude Code | **ADOPT** | Structured, machine-parseable, supports filtering |
| **Monotonic versioning** | OpenViking | **ADOPT** | Prevents stale writes from regressing memory quality |
| **L0/L1/L2 progressive loading** | OpenViking | **ADAPT** | Progressive disclosure is valuable; URI-based routing adds complexity |
| **3-agent architecture (deriver/dialectic/dreamer)** | Honcho | **ADOPT** | Clear separation: ingestion, recall, consolidation |
| **HNSW vector storage** | Honcho | **ADAPT** | Powerful but requires embedding service; consider lighter alternatives for ACE |
| **LLM-powered session summarization** | Hermes Agent | **ADAPT** | Expensive per-query; use sparingly or cache aggressively |
| **Sparse attention (MSA)** | MSA | **MONITOR** | Extreme scalability (100M tokens) but requires custom CUDA kernels |
| **Recursive decomposition (RLM)** | RLM | **ADAPT** | Paradigm shift from retrieval to interaction; ACE L2/L3 tiers could use recursive summarization |
| **Virtual filesystem URI** | OpenViking | **AVOID** | Adds coupling between memory and resource access; separate namespaces simpler |
| **Unbounded file storage** | OpenClaw | **AVOID** | No compaction mechanism; grows indefinitely |
| **Hard VRAM limits** | AutoResearch | **AVOID** | OOM-based experimentation is fragile; prefer explicit memory management |

---

## 11. ACE L1–L4 Memory Architecture Alignment

### L1: Ephemeral (Session Context)
- **Pattern:** Frozen snapshot at session start (Hermes Agent)
- **Pattern:** Tool-based session transcript access (Claude Code)

### L2: Short-Term (Cross-Session Recall)
- **Pattern:** FTS5 search + LLM summarization (Hermes Agent)
- **Pattern:** Semantic embedding search (Honcho)
- **Pattern:** L0/L1 progressive loading (OpenViking)

### L3: Long-Term (Curated Memory)
- **Pattern:** Character-bounded entries (Hermes Agent)
- **Pattern:** Four-type taxonomy with scope (Claude Code)
- **Pattern:** Deriver → Dreamer consolidation (Honcho)

### L4: Procedural (Skills as Memory)
- **Pattern:** Skills as procedural memory (Hermes Agent `skill_manager_tool.py`)
- **Pattern:** Tool/skill memory extraction via ReAct (OpenViking)

---

## 12. Open Questions

1. **Attribution granularity:** Should ACE memories carry fine-grained source citations (file:line) or coarse (session ID)?
2. **Consolidation trigger:** Scheduled (Honcho) vs. on-demand (Claude Code) vs. event-driven (OpenViking). Which for ACE?
3. **Embedding dependency:** Honcho's semantic search requires an embedding service. Is this acceptable for ACE's zero-dependency goal?
4. **Tier boundary:** At what context length should ACE switch from L1 (ephemeral) to L2 (FTS/embedding) retrieval?

---

## 13. Source Files Investigated

| System | Key Files |
|--------|-----------|
| **Hermes Agent** | `tools/memory_tool.py`, `tools/session_search_tool.py`, `hermes_state.py` |
| **Claude Code** | `src/memdir/memoryTypes.ts`, `src/utils/memory/types.ts`, `src/services/extractMemories/`, `src/services/autoDream/` |
| **Honcho** | `src/utils/agent_tools.py`, `src/deriver/`, `src/dialectic/`, `src/dreamer/` |
| **OpenViking** | `openviking/core/context.py` (ContextLevel), `openviking/session/memory_extractor.py` |
| **MSA** | `src/msa/memory_sparse_attention.py`, `src/msa_service.py`, `README.md` |
| **RLM** | `research/papers/rlm-rise.html` |
| **AutoResearch** | `program.md` (no persistent memory — VRAM-only experimentation) |
| **OpenClaw** | `extensions/memory-core/`, `extensions/memory-wiki/`, `extensions/memory-lancedb/` |
