# Research Synthesis: Agents-Study (Slice 14)

**Unit:** agents-study  
**Slice:** 14 (Final)  
**Date:** 2026-04-23  
**Source Papers:** TurboQuant, RLM/RISE, Meta-Harness, AlphaEvolve, RotorQuant

---

## 1. TurboQuant (Google Research, March 2026)

### Technical Summary

**Core Innovation:** Two-stage KV cache quantization combining PolarQuant + QJL (Quantized Johnson-Lindenstrauss).

**Stage 1 — PolarQuant:** Converts vectors from Cartesian to polar coordinates (radius + angles). Because angles are concentrated on a fixed circular grid, eliminates memory overhead of traditional vector quantization. Uses recursive polar transformations on coordinate pairs until distilled into one radius + descriptive angles.

**Stage 2 — QJL:** Projects the tiny quantization residual through a random Gaussian matrix, stores only 1 sign bit per dimension. The QJL estimator balances high-precision query with low-precision stored data to enable unbiased attention score estimation.

**Data-oblivious:** TurboQuant works without any dataset-specific calibration or fine-tuning. The rotation is random (QR decomposition of Gaussian matrix), making it universally applicable.

**Hardware tested:** NVIDIA H100 GPU accelerators (mentioned explicitly for 8x speedup on attention logits). Gemma and Mistral models used for benchmarks.

**Key Numbers (from blog + paper):**
- **6× memory reduction** (KV cache to 3 bits with near-zero loss)
- **8× speedup** on H100 for 4-bit TurboQuant attention logit computation
- **~3 bits per coordinate** (ternary/quatrainary quantization)
- Perfect needle-in-a-haystack results across all context lengths
- Outperforms KIVI baseline on LongBench (question answering, code generation, summarization)

**Accuracy Tradeoffs:**
- TurboQuant achieves "near-optimal distortion rates" in a data-oblivious manner
- 4-bit TurboQuant achieves up to 8× performance increase over 32-bit unquantized keys on H100
- On LongBench: TurboQuant achieves optimal scoring performance vs dot product distortion and recall
- On needle tasks: perfect results across all benchmarks with 6× memory reduction

### ACE Implications

**Memory/Providers Unit — ADOPT (with caveats):**
- The data-oblivious property aligns with ACE's goal of universal compression
- 6× memory reduction directly addresses the KV cache bottleneck for long contexts
- However: TurboQuant's d×d rotation matrix (16,384 parameters for d=128) is compute-heavy
- RotorQuant (see Section 5) improves on this with 44× fewer parameters
- **Recommendation:** Implement block-diagonal rotation (PlanarQuant/IsoQuant) instead of full d×d WHT for ACE's Providers unit — achieves better PPL, 28% faster decode, 5.3× faster prefill

**Key insight:** The two-stage approach (strong per-coordinate quantization + lightweight error correction) is the right architecture. ACE should use a similar pattern for memory compression: aggressive token reduction + residual tracking.

---

## 2. RLM / RISE (Recursive Language Models + Recursive IntroSpEction)

### Technical Summary

**Core Innovation:** RLMs are thin wrappers around LLMs that can spawn recursive sub-calls to handle unbounded context. The model decomposes the input context into a variable in a Python REPL environment, then can recursively query subsets of that context through additional LLM calls.

**Multi-turn MDP Formulation:** RISE frames fine-tuning as a multi-turn Markov Decision Process where:
- State = current context window + REPL state
- Actions = tool calls (including recursive RLM calls)
- Reward = task success signal
- Policy = learned via online imitation learning + reward-weighted supervised learning

**Self-improvement trajectory:** RLM uses GPT-5-mini as the root LM but outperforms GPT-5 by allowing recursive sub-calls. The model learns when to decompose context, when to peek, when to grep, when to summarize.

**Model sizes tested:**
- GPT-5 (frontier model)
- GPT-5-mini (smaller model used in RLM wrapper)

**Tasks:**
- OOLONG benchmark (challenging long-context reasoning, ~3000-6000 rows of entries)
- BrowseComp-Plus (100K documents, multi-hop queries requiring 10M+ tokens)
- LoCoDiff (long git diff tracking, 75k+ token histories)

**Results:**
- On OOLONG (132k context): RLM(GPT-5-mini) outperforms GPT-5 by **34 points** (~114% increase), at roughly the same API cost
- On OOLONG (263k context): RLM(GPT-5-mini) outperforms GPT-5 by **15 points** (~49% increase)
- On BrowseComp-Plus: RLM(GPT-5) maintains perfect performance at 1000-document scale; ReAct+BM25 degrades significantly
- **No performance degradation at 10M+ tokens** — the key breakthrough

**Key behaviors that emerge:**
1. **Peeking:** Root LM grabs first N characters to understand structure
2. **Grepping:** Uses regex/keyword search to narrow context
3. **Partition + Map:** Chunks context, runs recursive LM calls on each chunk
4. **Summarization:** Summarizes subsets for outer LM decisions

### ACE Implications

**Long-term Memory Architecture — ADAPT:**
- RLM demonstrates that unbounded context is achievable through recursive decomposition rather than monolithic context windows
- The REPL environment pattern (context as variable) is directly applicable to ACE's L2/L3 summarization tiers
- **Recommendation:** ACE should implement a "context decomposition layer" where large memories are recursively processed rather than retrieved wholesale
- The multi-turn MDP formulation is relevant to ACE's Learning Loop: online imitation learning + reward-weighted supervision could inform how ACE updates its internal policies

**Key architectural insight:** "Context rot" is the fundamental problem. RLMs solve it by never showing the entire context to any single model call — the root LM only sees the query + a REPL environment that manages context access.

---

## 3. Meta-Harness (Stanford/MIT, March 2026)

### Technical Summary

**Core Innovation:** Outer-loop search system that optimizes "harnesses" (the code wrapping an LLM that determines what to store, retrieve, and present) using a coding agent proposer with filesystem access to all prior candidates' code, scores, and execution traces.

**Key insight:** Raw execution traces are the critical ingredient. Ablation shows:
- Scores Only: **34.6** median accuracy
- Scores + Summary: **34.9** median accuracy
- Full Meta-Harness (with raw traces): **50.0** median accuracy (+15 points)

**Harness definition:** A stateful program wrapping a fixed LLM that determines context construction at each step. The goal is finding the harness H that maximizes expected reward across task distribution.

**Search loop:**
1. Coding agent (Claude Code with Opus-4.6) reads filesystem containing all prior candidates' source code, execution traces, and scores
2. Agent proposes k new harnesses
3. Each harness is evaluated on task distribution
4. All logs stored in filesystem for next iteration
5. Typical run: ~60 harnesses over 20 iterations

**Filesystem-access pattern:** The proposer queries the filesystem through terminal tools (grep, cat) rather than ingesting as a single prompt. This is critical because a single evaluation can produce **up to 10,000,000 tokens** of diagnostic information.

**Accuracy numbers:**
- TerminalBench-2 (agentic coding): **37.6%** pass rate on Haiku 4.5 (ranks #1 among Haiku agents), **76.4%** on Opus 4.6 (ranks #2 among Opus agents)
- Online text classification: **48.6%** accuracy (+7.7 points over ACE, +8.6 over MCE) using only 11.4K context tokens vs 50.8K for ACE
- IMO-level math problems: **+4.7 points** average improvement across 5 held-out models

**Search run duration:** A few hours of wall-clock time (from Discussion section: "A search run completes in a few hours of wall-clock time").

### ACE Implications

**Learning Loop Unit — ADOPT:**
- The filesystem-as-feedback-channel pattern is directly applicable to ACE's trajectory storage
- **Raw execution traces > summaries.** ACE should store complete tool call sequences and intermediate results, not just final outcomes
- The agentic proposer (coding agent reading filesystem) mirrors how ACE's Learning Loop should operate: retrieve diagnostic experience, form hypotheses, modify code
- **Multi-objective search** (accuracy + context cost) aligns with ACE's goal of balancing capability vs efficiency

**Key insight from ablation:** The entire advantage of Meta-Harness comes from full trace access. Without traces, even adding LLM-generated summaries barely helps. ACE's Learning Loop must preserve complete execution histories.

**Recommendation:** Implement a "harness store" in ACE where each Learning Loop iteration writes complete diagnostic logs (prompt construction, retrieval calls, state updates, scores) to persistent storage. Future iterations read this store to inform improvement.

---

## 4. AlphaEvolve (Google DeepMind, June 2025)

### Technical Summary

**Core Innovation:** Evolutionary coding agent that orchestrates LLMs (Gemini 2.0 Flash + Pro ensemble) to iteratively improve algorithms through mutation/crossover guided by automatic evaluators.

**Pipeline (evolutionary loop):**
1. **Prompt sampler** builds rich prompts from program database (top programs, rendered evaluation results, system instructions)
2. **LLMs generate** code modifications as diff blocks (SEARCH/REPLACE format)
3. **Apply diff** to parent program → child program
4. **Evaluators** execute and score child program
5. **Program database** stores promising solutions for next iteration

**Evaluator design:**
- User provides `evaluate(solution) → dict[scalar]` function
- Supports evaluation cascade (hypothesis testing: easy → hard test stages)
- Supports LLM-generated feedback for qualities hard to capture in scalar metrics
- Supports parallelized evaluation (100s of compute-hours per solution)
- Supports multiple simultaneous metrics

**Population maintenance:**
- MAP-elites inspired algorithm combined with island-based population models
- Balances exploration (diversity) vs exploitation (best programs)
- Stores programs with scores and outputs attached

**Concrete results:**

*Matrix multiplication:*
- Improved 14 matrix multiplication algorithms
- For 4×4 complex matrices: **rank 48** (first improvement over Strassen's rank 49 in 56 years)

*Data center scheduling:*
- Discovered more efficient scheduling heuristics for Google's cluster management

*Circuit design:*
- Found functionally equivalent simplification in TPU accelerator circuit design

*LLM training acceleration:*
- Discovered optimizations for the attention runtime in Transformers

*Mathematics:*
- 50+ open mathematical problems attempted
- Matched best known constructions on ~75%
- Surpassed SOTA on ~20% (new provably better constructions)
- Examples: Minimum Overlap Problem (Erdős), Kissing Numbers (11 dimensions)

**Mutation/crossover strategy:**
- Diff-based: LLM proposes changes as SEARCH/REPLACE blocks
- Can evolve entire code files (vs FunSearch's 10-20 line functions)
- Supports full program rewrites when appropriate
- Multiple models in ensemble: Flash (high throughput) + Pro (high quality)

### ACE Implications

**Learning Loop Unit — ADAPT:**
- AlphaEvolve's evaluator pool pattern is directly relevant: ACE needs multiple evaluation perspectives (correctness, efficiency, style, etc.)
- The program database with rich context (past trials + ideas) mirrors ACE's trajectory memory
- **Key difference:** AlphaEvolve operates on code artifacts; ACE operates on agent behaviors. The evolutionary principle still applies.
- **Recommendation:** ACE's Learning Loop should maintain a population of "agent strategies" (prompt templates, tool selection policies, memory management heuristics) and evolve them through LLM-guided mutation

**Evaluator design is critical:** AlphaEvolve's success hinges on having machine-gradable evaluation functions. ACE needs clearly defined success metrics for its learning objectives.

**Population diversity matters:** The combination of MAP-elites (quality diversity) + island models (parallel search) prevents premature convergence. ACE should maintain diverse strategy populations, not just converge to single best approach.

---

## 5. RotorQuant (Scrya, March 2026)

### Technical Summary

**Core Innovation:** Replaces TurboQuant's d×d random orthogonal rotation (WHT butterfly network, 16,384 params for d=128) with Clifford algebra Cl(3,0) rotors. Each rotor has only 4 non-zero multivector components, reducing parameters 44× and enabling fused GPU kernels with 10-31× speedup.

**Method:**
1. Chunk d-dimensional vector into groups of 3 dimensions
2. Each group embedded as grade-1 multivector
3. Per-group rotor sandwich: R_g × v_g × R̃_g (only ~56 FMAs per group)
4. Grade-aware Lloyd-Max quantization (different codebooks per grade)
5. Inverse rotation to reconstruct

**Deferred quantization:** K-cache allocates as FP16 during prefill (zero error compounding). Decode tokens get quantized on insertion. This gives 3× better PPL than roundtrip quantization — and makes decode faster than FP16 baseline (no dequant overhead in flash attention).

**Hardware tested:**
- NVIDIA RTX PRO 4000 Blackwell (CUDA): 10-19× speedup over TurboQuant
- Apple M4 (Metal): 9-31× speedup over TurboQuant
- RTX 5090: Llama 3.1 8B benchmarks (140 tok/s decode FP16 → 119 tok/s iso3)

**Benchmark numbers (Llama 3.1 8B Instruct Q4_K_M, RTX 5090):**

| Config | Decode tok/s | Prefill tok/s | PPL (wiki-2) | vs FP16 | Compression |
|--------|-------------|---------------|--------------|---------|-------------|
| f16/f16 | 140 | 6,156 | 6.63 | baseline | 1× |
| iso3/iso3 | 118 | 3,397 | **6.91** | +4.2% | 10.3× |
| planar3/planar3 | 119 | 3,822 | 7.05 | +6.3% | 10.3× |
| turbo3/turbo3 | 93 | 722 | 7.07 | +6.6% | 10.3× |
| planar3/f16 | 134 | — | ~6.63 | ~0% | 5.1× |

**vs TurboQuant (same 10.3× compression):**
- PPL: iso3 6.91 vs turbo3 7.07 — **better quality**
- Decode: 119 tok/s vs 93 tok/s — **28% faster**
- Prefill: 3,822 tok/s vs 722 tok/s — **5.3× faster**
- Parameters: 128 vs 16,384 — **44× fewer**

**VRAM Savings (3-bit symmetric, 10.3× compression):**
- 8K context: 288 MB → 28 MB (saves 260 MB)
- 32K context: 1,152 MB → 112 MB (saves 1.04 GB)
- 128K context: 4,608 MB → 447 MB (saves 4.16 GB)

**llama.cpp integration:** Production-ready via `johndpope/llama-cpp-turboquant` fork with feature/planarquant-kv-cache branch. Cache types: `planar3`, `iso3`, `planar4`, `iso4`.

### Comparison: TurboQuant vs RotorQuant

| Dimension | TurboQuant | RotorQuant | Winner |
|-----------|-----------|------------|--------|
| **Parameters** | 16,384 (d=128) | 372 (d=128) | RotorQuant (44× fewer) |
| **FMAs** | 16,384 | ~2,400 | RotorQuant |
| **PPL (3-bit)** | 7.07 | 6.91 (iso3) | RotorQuant |
| **Decode speed** | 93 tok/s | 119 tok/s | RotorQuant (+28%) |
| **Prefill speed** | 722 tok/s | 3,822 tok/s | RotorQuant (5.3×) |
| **Hardware** | H100 (JAX) | RTX 5090, M4 (CUDA/Metal) | Tie |
| **Production ready** | llama.cpp (TheTom fork) | llama.cpp (johndpope fork) | Tie |
| **Theoretical MSE** | Lower (exact decorrelation) | Higher (block-diagonal) | TurboQuant |
| **Real model fidelity** | 0.990 cos sim | 0.990 cos sim | Tie |
| **Algebraic structure** | None (random matrix) | Preserves geometric structure | RotorQuant |

**Key insight:** On synthetic random vectors, TurboQuant wins on MSE. On real KV cache data (which lives on low-rank manifolds), RotorQuant matches or beats TurboQuant because Clifford rotors preserve directional structure that matters for attention.

### ACE Implications

**Memory/Providers Unit — ADOPT (superior alternative to TurboQuant):**
- RotorQuant dominates TurboQuant on every practical axis: better PPL, faster decode, 5.3× faster prefill, 44× fewer parameters
- **Recommendation:** ACE's Providers unit should implement PlanarQuant/IsoQuant (the production-ready descendants of RotorQuant) rather than TurboQuant
- The deferred quantization pattern (FP16 during prefill, quantize on decode insertion) is essential for maintaining quality during long context processing
- **For 128K context: saves 4.16 GB VRAM**, enabling larger batch sizes or longer contexts on the same hardware

**Integration path:** The llama.cpp fork is already available. ACE can adopt it by:
1. Using the existing llama.cpp integration with `--cache-type-k iso3 --cache-type-v iso3`
2. Implementing custom KV cache management if using raw CUDA kernels

---

## 6. Cross-Paper Synthesis

### Theme 1: Memory/Compression (TurboQuant + RotorQuant + MSA)

**Convergence point:** All three approaches address the KV cache bottleneck through compression, but with different tradeoffs:

| Method | Approach | Compression | Quality Impact | Speed |
|--------|----------|-------------|----------------|-------|
| TurboQuant | Full d×d WHT + QJL | 10× | Near-zero loss | 8× speedup |
| RotorQuant | Block-diagonal rotors + QJL | 10× | Better PPL than TurboQuant | 5.3× prefill, 28% decode |
| MSA | Differentiable attention + chunk-mean pooling | Variable (100M tokens) | N/A (different mechanism) | N/A |

**ACE alignment:** RotorQuant's block-diagonal rotation pattern is the best fit for ACE's Providers unit. The key properties:
1. Data-oblivious (no calibration needed)
2. Production-ready (llama.cpp integration)
3. Superior on real model data (PPL, speed)
4. Deferred quantization preserves prefill quality

**What MSA adds:** MSA's insight about differentiable attention for memory retrieval is complementary — RotorQuant compresses what's stored, MSA improves what's retrieved. ACE should consider combining both.

### Theme 2: Learning Loops (AlphaEvolve + Meta-Harness)

**Convergence point:** Both systems use evolutionary search with rich feedback, but at different levels:

| System | What evolves | Feedback | Duration |
|--------|-------------|----------|---------|
| AlphaEvolve | Algorithms (code) | Automatic evaluators + program DB | Days |
| Meta-Harness | Harnesses (context management) | Raw execution traces via filesystem | Hours |

**ACE alignment:** Both patterns are relevant to ACE's Learning Loop:

1. **Meta-Harness's filesystem-as-feedback-channel** is the most directly applicable. ACE should store complete execution traces and allow its learning components to read them selectively.

2. **AlphaEvolve's evaluator pool + population maintenance** provides the right abstraction for ACE's multi-objective learning (correctness vs efficiency vs cost).

3. **The critical ingredient is raw traces, not summaries.** Both papers confirm that compression of feedback destroys diagnostic signal. ACE must preserve complete histories.

**Divergence:** AlphaEvolve operates on code artifacts with machine-gradable objectives. ACE operates on agent behaviors with potentially softer objectives. The evolutionary principle applies, but ACE needs to handle subjective evaluation.

### Theme 3: Long-Term Memory (RLM + MSA)

**Convergence point:** Both RLM and MSA propose alternatives to monolithic context windows:

- **RLM:** Recursive decomposition via REPL environment — never shows entire context to any single model call
- **MSA:** Native sparse attention + chunk-mean pooling — bypasses KV cache entirely for retrieval

**ACE alignment:** RLM's "context as variable" pattern is directly implementable in ACE's L2/L3 tiers. Large memories would be decomposed recursively rather than retrieved wholesale.

**The "context rot" problem is fundamental.** All systems that rely on monolithic context windows suffer degradation. The solution is structural (recursive processing) not incremental (better retrieval).

### Theme 4: Self-Improvement Trajectories

| Paper | Improvement Mechanism | What improves | How fast |
|-------|---------------------|---------------|----------|
| RLM/RISE | Online imitation + reward-weighted RL | Task performance on long context | 2× improvement over base model |
| Meta-Harness | Outer-loop search over harnesses | Context management | +15 points from traces |
| AlphaEvolve | Evolutionary program search | Algorithms | 56-year Strassen improvement |

**Pattern:** Self-improvement comes from richer feedback channels, not from smarter base models. RLM improves by decomposing context access. Meta-Harness improves by reading raw traces. AlphaEvolve improves by maintaining diverse populations.

**ACE implication:** The path to better ACE performance is not just better models — it's better mechanisms for experience utilization.

---

## 7. Prioritized Recommendations for ACE Units

### Priority 1: Providers Unit — Implement RotorQuant/IsoQuant

**Action:** Adopt PlanarQuant/IsoQuant from the `johndpope/llama-cpp-turboquant` fork (llama.cpp integration).

**Rationale:**
- 10.3× KV cache compression at 3-bit with better PPL than TurboQuant
- 28% faster decode, 5.3× faster prefill vs TurboQuant
- Production-ready in llama.cpp
- Saves 4.16 GB at 128K context

**Specific changes:**
- Add `--cache-type-k iso3 --cache-type-v iso3` support to Providers
- Implement deferred quantization pattern (FP16 during prefill, quantize on decode)
- Evaluate on real workloads to determine optimal K-only vs K+V compression

### Priority 2: Learning Loop — Implement Trace Storage

**Action:** Implement filesystem-based trace storage for all Learning Loop iterations.

**Rationale:** Meta-Harness ablation proves raw execution traces are the critical ingredient — scores only: 34.6, scores+summary: 34.9, full traces: 50.0 (+15 points).

**Specific changes:**
- Every tool call, state update, and intermediate result written to persistent storage
- Storage format: filesystem with per-iteration directories containing code, traces, scores
- Future iterations read this store via terminal tools (grep, cat) rather than monolithic prompts
- Implement multi-objective tracking (accuracy + context cost)

### Priority 3: Memory Architecture — Implement Context Decomposition

**Action:** Add recursive context processing to L2/L3 summarization tiers.

**Rationale:** RLM demonstrates that unbounded context is achievable through recursive decomposition. ACE's current retrieval approach (fetch entire context) will hit the same "context rot" wall.

**Specific changes:**
- Implement "context decomposition layer" — large memories recursively processed before presentation to model
- Add REPL-like environment where context is stored as variable
- Model can recursively query subsets rather than receiving entire context
- Add "peek", "grep", "partition+map", "summarize" as first-class memory operations

### Priority 4: Learning Loop — Implement Population-Based Search

**Action:** Maintain a population of agent strategies evolved through LLM-guided mutation.

**Rationale:** AlphaEvolve's success with MAP-elites + island models shows population diversity prevents premature convergence. ACE needs to explore strategy space, not just converge to single best.

**Specific changes:**
- Store multiple strategy variants (prompt templates, tool selection policies, memory heuristics)
- Implement evaluator pool with multiple perspectives (correctness, efficiency, style)
- Use LLM to propose mutations based on population diversity
- Track Pareto frontier across multiple objectives

### Priority 5: Compaction — Revisit with Deferred Quantization Insights

**Action:** Reconsider the compaction strategy based on RotorQuant's deferred quantization pattern.

**Rationale:** RotorQuant shows that keeping FP16 during prefill and quantizing on decode insertion gives 3× better PPL. ACE's current compaction may be premature.

**Specific changes:**
- During prefill: use FP16 KV cache (or lossless compression)
- On decode: quantize newly generated tokens for cache
- This eliminates compaction cycles while maintaining memory efficiency

---

## Appendix: Quick Reference Table

| Paper | Core contribution | ACE unit | Recommendation |
|-------|-------------------|----------|----------------|
| TurboQuant | Two-stage quantization (PolarQuant + QJL) | Providers | Superseded by RotorQuant |
| RLM/RISE | Recursive context decomposition | Memory | ADAPT context decomposition |
| Meta-Harness | Filesystem traces > summaries | Learning Loop | ADOPT trace storage |
| AlphaEvolve | Evolutionary program search | Learning Loop | ADAPT population search |
| RotorQuant | Block-diagonal rotors, 44× fewer params | Providers | ADOPT immediately |

---

**Sources:**
- TurboQuant: https://research.google/blog/turboquant-redefining-ai-efficiency-with-extreme-compression/ (arXiv:2504.19874)
- RLM/RISE: https://alexzhang13.github.io/blog/2025/rlm/ (arXiv:2512.24601v1)
- Meta-Harness: https://arxiv.org/html/2603.28052v1
- AlphaEvolve: https://arxiv.org/abs/2506.13131
- RotorQuant: https://github.com/scrya-com/rotorquant (rotorquant.tex + rotorquant_README.md)
