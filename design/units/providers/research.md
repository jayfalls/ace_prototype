# Research Report: Providers Unit

## 1. Executive Summary

This document evaluates the seven open architectural questions identified in the Providers problem space. Each section presents research findings, compares real-world implementations, analyses trade-offs, and issues a binding recommendation for the ACE Providers unit. The recommendations prioritise operational simplicity within ACE's single-binary, embedded-infrastructure model while leaving upgrade paths for multi-replica and swarm-scale deployments.

**Key Recommendations:**
1. **Routing strategies:** Implement Round-Robin, Sequential (priority-ordered failover), and Failover with circuit-breaker semantics. Defer weighted, latency-based, and cost-optimized strategies to a future enhancement cycle.
2. **Streaming:** Support streaming internally via chunked NATS messages on a dedicated subject suffix (`.response.stream`), but do not expose streaming to the frontend in this unit. The cognitive engine receives complete responses; streaming is an internal transport optimisation.
3. **Retry/Timeout:** Per-provider configurable timeout (default 60s, 120s for Bedrock/Azure). Exponential backoff with full jitter (base 1s, max 8s, 2 retries). Group Sequential/Failover strategies treat retry exhaustion as a group-level failover trigger.
4. **Token counting:** Trust provider-reported token counts exclusively. Do not import tiktoken or similar local tokenizers. Normalise all provider counts to an inclusive model (input_tokens includes cached; output_tokens includes reasoning) at the adapter layer.
5. **NATS payload:** Normalised request/response structs defined in Go, serialised as JSON. Request includes provider_group_id, messages, parameters, and streaming flag. Response includes text, usage, duration, and error details.
6. **Encryption:** Envelope encryption (per-provider DEK wrapped by master KEK) despite single-binary deployment. The marginal complexity is justified by key rotation without re-encrypting all API keys, and it future-proofs against external KMS integration.
7. **Feature detection:** Hybrid static catalog + provider fetch. Each provider adapter ships a static capability map for well-known models. Dynamic fetching populates model lists where APIs support it. Users manually override when static data is wrong or missing.

---

## 2. Provider Group Routing Strategies

### 2.1 Strategy Landscape

| Strategy | Complexity | State Required | ACE Suitability | Notes |
|---|---|---|---|---|
| **Round-Robin** | Low | Atomic counter | High | Even distribution. Ignores provider health unless combined with health checks. |
| **Sequential** | Low | None | High | Try provider A, then B, then C. Simplest failover. No health awareness — failure is detected per-request. |
| **Failover** | Medium | Health state per provider | High | Primary with circuit-breaker cooldown. Actively avoids degraded providers. Best balance of reliability and simplicity. |
| **Weighted** | Low-Medium | Weight counters | Medium | Distribute by capacity/cost ratio. Useful when providers have different rate limits. |
| **Latency-based** | High | Latency history window | Low | Requires per-provider RTT tracking and decay. Overkill for single-binary; valuable at swarm scale. |
| **Cost-optimized** | High | Pricing + capability matrix | Low | Routes to cheapest model meeting requirements. Requires dynamic pricing feeds and capability matching. |
| **Least-loaded** | Medium | In-flight counters | Medium | Tracks concurrent requests per provider. Complex in multi-replica without NATS coordination. |
| **Priority + Failover** | Medium | Health + priority queue | Medium | Conditional rules ("use A if under 50% rate limit, else B"). Powerful but complex configuration surface. |

### 2.2 Real-World Evidence

**LLMGateway** uses a weighted scoring system (uptime 50%, throughput 20%, price 20%, latency 10%) with exponential penalty for degraded providers and epsilon-greedy exploration (1% random traffic) to prevent metric staleness. Automatic fallback triggers when uptime drops below 90%.

**NeuralRouting** advocates health-aware routing over per-request failover: "maintain a health status for each provider and route based on current health... the user never waits for a timeout on a dead provider." They recommend combining cost-weighted, latency-weighted, and capability-weighted routing for optimal results.

**CC-Relay** defaults to `failover` strategy with parallel racing of remaining providers on failure. It also supports `round_robin`, `weighted_round_robin`, and `model_based` routing. Their failover triggers on 429/500/502/503/504, timeouts, and connection errors.

**Coverge's gateway comparison** notes that Portkey and LiteLLM both support weighted round-robin and latency-based routing, but these require sustained traffic volumes to produce meaningful metrics. For smaller deployments, simple failover chains are preferred.

### 2.3 Analysis

**Latency-based and cost-optimized routing** require metric windows, normalisation, and exploration/exploitation trade-offs that are difficult to tune without production traffic volumes. In ACE's initial deployment (single binary, single operator), these strategies add more configuration surface than value.

**Weighted routing** is valuable for capacity-based distribution (e.g., Groq API key with 120 RPM vs OpenAI key with 60 RPM), but can be approximated by Sequential ordering with per-provider rate limits in the initial release.

**Round-Robin** is trivial to implement but dangerous without health checks — a failed provider receives 1/N of traffic until a request times out.

**Sequential** is the simplest correct strategy: always try the first provider, fall back on error. Its weakness is slow degradation detection (every request to a bad provider waits for timeout).

**Failover with circuit-breaker** is the best production default: it adds health state (healthy/degraded/unhealthy) with automatic recovery probes, preventing requests from hitting known-bad providers. The state machine is simple: healthy → degraded (on first failure) → unhealthy (after threshold) → probing (after cooldown) → healthy (on success).

### 2.4 Recommendation

Implement **three strategies** in this unit:

1. **Round-Robin** — atomic counter modulo N. Combined with per-provider health checks: unhealthy providers are skipped. Simple load-balancing across equivalent providers.
2. **Sequential** — ordered list, try each in order on failure. No health state; purely reactive. Suitable for primary/backup patterns where failover latency is acceptable.
3. **Failover** — priority-ordered with circuit-breaker. Primary provider receives all traffic while healthy. On failure, marks degraded, tries next provider, and enters cooldown before re-promoting primary. Best for production reliability.

Defer **Weighted**, **Latency-based**, **Cost-optimized**, **Least-loaded**, and **Priority+Failover** to a future routing enhancement unit. These require operational telemetry that ACE will not generate at sufficient volume in the initial release.

**Multi-replica note:** Health state and round-robin counters are in-memory per process. In a multi-replica deployment, each replica maintains independent state. This is acceptable for initial release; NATS-broadcast state synchronisation is explicitly deferred (see Non-Goals in problem space).

---

## 3. Streaming Support in the LLM Gateway

### 3.1 The Problem

NATS request-reply (`messaging.RequestReply`) is fundamentally a request-response pattern: one message in, one message out, with a timeout. LLM streaming produces a sequence of chunks (SSE events in OpenAI's case). Two design questions arise:

1. Does the LLM gateway support streaming internally (gateway ↔ provider)?
2. Does the LLM gateway support streaming externally (cognitive engine ↔ gateway via NATS)?

### 3.2 Design Options

**Option A: No streaming anywhere**
The gateway always uses non-streaming provider APIs and returns complete responses. Simplest implementation. Sacrifices time-to-first-token (TTFT) latency, which matters for chat UX but not for cognitive engine loops.

**Option B: Internal streaming only**
The gateway uses streaming provider APIs to reduce TTFT and memory pressure for large responses, but buffers the full response before publishing to NATS. The cognitive engine still receives one complete message. This optimises provider-side behaviour without changing the NATS contract.

**Option C: External streaming via NATS**
Publish chunks as a sequence of NATS messages on a dedicated subject (e.g., `ace.llm.{agentId}.response.chunk`), with a final `ace.llm.{agentId}.response.done` message. Requires the consumer to handle out-of-order delivery, reassembly, and timeout. Complex but enables real-time frontend streaming.

**Option D: Streaming over NATS request-reply**
Abuse the reply subject to send multiple messages. NATS request-reply does not natively support this; it would require custom protocol logic on top of core NATS. Not recommended.

### 3.3 Recommendation

Implement **Option B: Internal streaming only** in this unit.

**Rationale:**
- The cognitive engine's primary consumer is layer loops, which need complete responses to parse tool calls, reasoning, and decisions. Streaming provides no value to the consumer.
- The frontend streaming integration is explicitly out of scope for this unit (see Non-Goals: "Streaming — Real-time streaming of LLM responses to the frontend belongs to a future unit").
- Internal streaming reduces memory pressure when providers return large responses (e.g., JSON tool outputs, long reasoning chains) and improves cancellation responsiveness (abort the stream if the NATS context is cancelled).
- The gateway's provider adapters should use the provider's streaming endpoint where available, accumulate chunks into a `strings.Builder`, and return the full text. Usage metadata (token counts) is typically only available at the end of the stream anyway.

**Future path:** When the frontend streaming unit is built, extend the gateway with Option C: a `stream: true` flag in the NATS request triggers chunked responses on `ace.llm.{agentId}.response.chunk` with a terminal `done` message. The infrastructure for this (adapters already support streaming) will already exist.

---

## 4. Retry and Timeout Strategies

### 4.1 Timeout Configuration

| Provider Type | Default Timeout | Rationale |
|---|---|---|
| OpenAI-compatible (incl. Groq, Together, DeepSeek, Mistral, xAI) | 60s | Standard API latency; Groq is faster but 60s covers edge cases |
| Anthropic | 60s | Messages API typically responds within 5-30s |
| Google Gemini | 60s | Variable latency; 60s safe default |
| Azure OpenAI | 120s | Regional latency + enterprise throttling |
| AWS Bedrock | 120s | InvokeModel latency plus AWS Signature V4 overhead |
| Ollama / llama.cpp | 300s | Local inference can be slow on CPU; user-configurable |
| Custom | 60s | Assumed OpenAI-compatible unless overridden |

Timeouts should be configurable per-provider (stored in `providers.config_json`), with the above defaults applied by the adapter factory.

### 4.2 Retry Strategy

**Exponential backoff with full jitter** is the industry standard for LLM APIs:

```
delay = min(cap, base * 2^attempt) + random(0, base * 2^attempt)
```

Where `base = 1s`, `cap = 8s`, `max_attempts = 2` (3 total tries including initial). This yields retry delays of approximately 1-2s and 2-4s.

**Why full jitter?** AWS and Google recommend full jitter over equal jitter or no jitter to prevent thundering herd after a provider outage. LLM providers (especially OpenAI, Anthropic) explicitly request exponential backoff with jitter in their 429 error documentation.

**Retryable errors:**
- 429 Too Many Requests (rate limit)
- 500 Internal Server Error
- 502 Bad Gateway
- 503 Service Unavailable
- 504 Gateway Timeout
- Network errors (connection refused, DNS failure, timeout)
- Context deadline exceeded (if timeout < cap)

**Non-retryable errors:**
- 400 Bad Request (invalid model, malformed request)
- 401 Unauthorized (bad API key)
- 403 Forbidden (quota exceeded, content policy)
- 404 Not Found (invalid endpoint)

### 4.3 Interaction with Group Failover

Retry and group failover operate at different scopes:

- **Retry** is per-provider, per-request. It attempts the same provider again with backoff.
- **Group failover** is per-group, per-request. It occurs when the primary provider (after retries) definitively fails.

**Rules:**
1. The gateway attempts the selected provider with retries.
2. If all retries are exhausted, the error is treated as a provider failure.
3. The group's routing strategy decides the next provider.
4. Round-Robin: advance to next provider. Sequential: try next in list. Failover: mark current as degraded, use next.
5. The same retry logic applies to the fallback provider.

**Circuit-breaker integration (Failover strategy only):**
- First failure: mark provider as `degraded` (still used, but logged).
- Second consecutive failure within window: mark `unhealthy` (skipped for cooldown period, default 60s).
- Recovery probe: after cooldown, route a single request to test health. On success → `healthy`. On failure → extend cooldown (exponential backoff, max 5 min).

### 4.4 Recommendation

- **Timeouts:** Per-provider configurable with type-specific defaults (60s/120s/300s).
- **Retry:** Exponential backoff with full jitter, base 1s, cap 8s, max 2 retries (3 attempts total).
- **Retryable status codes:** 429, 500, 502, 503, 504, network errors.
- **Group failover trigger:** Only after all retries on a provider are exhausted.
- **Circuit-breaker:** Part of Failover strategy. Cooldown 60s, exponential extension up to 5 min.

---

## 5. Token Counting Approach

### 5.1 The Problem

LLM providers report token usage inconsistently:

- **Inclusive model** (OpenAI, Gemini, Mistral, Groq, Cohere, xAI, DeepSeek): `input_tokens` is the grand total including cached tokens. Sub-categories are subsets.
- **Additive model** (Anthropic, AWS Bedrock): `input_tokens` covers only non-cached tokens. Cached tokens are separate top-level fields that must be added.

Similarly for output: most providers include reasoning/thinking tokens inside `output_tokens`, but Google Vertex AI reports them separately.

Local tokenizers (tiktoken, Anthropic's tokenizer, etc.) cannot accurately count tokens for all providers, and discrepancies of 30-50% are common when tool calls or special formatting is involved.

### 5.2 Options

**Option 1: Trust provider-reported counts exclusively**
Use the `usage` object from the provider response. Normalise to inclusive model at the adapter layer.

**Option 2: Local tokenization + provider confirmation**
Count tokens locally before sending, then validate against provider response. Catches discrepancies but requires importing multiple tokenizer libraries.

**Option 3: Local tokenization only**
Never trust provider counts. Count locally using tiktoken (OpenAI-compatible) or provider-specific libraries. Simple but inaccurate for providers with different tokenizers.

### 5.3 Recommendation

**Implement Option 1: Trust provider-reported counts exclusively.**

**Rationale:**
- Provider counts are the billing source of truth. Any local count that disagrees with the provider is irrelevant for cost tracking.
- Local tokenizers add significant binary size and dependency complexity (tiktoken is a Python library; Go equivalents like `github.com/pkoukk/tiktoken-go` require embedding 5MB+ of BPE vocab files).
- Anthropic, Gemini, and Bedrock use tokenizers that are not compatible with tiktoken. Using tiktoken for these providers produces estimates off by 30-50%.
- The `input_tokens` field in OTEL conventions (gen_ai.usage.input_tokens v1.37+) is explicitly defined as inclusive: "SHOULD include all types of input tokens, including cached tokens."

**Normalisation rules (adapter layer responsibility):**
- For inclusive providers (OpenAI, Gemini, etc.): use `input_tokens` and `output_tokens` directly. Extract `cached_tokens` from sub-fields if available.
- For additive providers (Anthropic, Bedrock): compute `input_tokens_total = input_tokens + cache_read_input_tokens + cache_creation_input_tokens`.
- For output: `output_tokens_total = output_tokens + reasoning_tokens` (only if provider reports them separately, e.g. Vertex AI).
- Store both `input_tokens` (normalised inclusive) and `cached_tokens` (subtracted or reported) in usage events.

**Rate limiting:** TPM limits must use the normalised total, not the provider's raw non-cached count. The adapter must return normalised counts to the gateway before rate limit checks.

---

## 6. NATS Message Payload Schema

### 6.1 Design Constraints

- NATS message size limit is ~8MB. LLM responses can exceed this for long outputs, but initial ACE use cases (cognitive loops, tool calls) rarely generate >100KB responses.
- The envelope headers (message_id, correlation_id, agent_id, cycle_id, source_service, timestamp, schema_version) are already mandated by `internal/messaging`.
- The payload schema must be versioned (schema_version in envelope).

### 6.2 Request Payload

```go
// LLMRequest is the normalised request sent on ace.llm.{agentId}.request
type LLMRequest struct {
    SchemaVersion string            `json:"schema_version"` // "2024-05-04/v1"
    ProviderGroupID string          `json:"provider_group_id"` // UUID of the group
    ModelOverride   *string         `json:"model_override,omitempty"` // Optional: force specific model
    SystemPrompt    string          `json:"system_prompt"`
    Messages        []ChatMessage   `json:"messages"`
    Parameters      LLMParameters  `json:"parameters"`
    Stream          bool            `json:"stream"` // false in this unit; reserved for future
    Metadata        map[string]string `json:"metadata,omitempty"` // agentId, layerId, cycleId duplicates for payload convenience
}

type ChatMessage struct {
    Role    string `json:"role"`    // system, user, assistant, tool
    Content string `json:"content"`
    // Future: ToolCalls, ToolCallID, Images (deferred to Senses unit)
}

type LLMParameters struct {
    Temperature      *float32 `json:"temperature,omitempty"`
    TopP             *float32 `json:"top_p,omitempty"`
    TopK             *int32   `json:"top_k,omitempty"`
    MaxTokens        *int32   `json:"max_tokens,omitempty"`
    StopSequences    []string `json:"stop_sequences,omitempty"`
    PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
    // Future: ToolDefinitions, ResponseFormat (deferred to Tools/Engine units)
}
```

### 6.3 Response Payload

```go
// LLMResponse is the normalised response sent on ace.llm.{agentId}.response
type LLMResponse struct {
    SchemaVersion string      `json:"schema_version"`
    Success       bool        `json:"success"`
    Text          string      `json:"text,omitempty"`      // Generated text (empty if error)
    Model         string      `json:"model"`               // Actual model served
    ProviderID    string      `json:"provider_id"`         // UUID of the provider
    ProviderGroupID string    `json:"provider_group_id"`   // UUID of the group
    Usage         TokenUsage  `json:"usage,omitempty"`
    DurationMs    int64       `json:"duration_ms"`
    RetryCount    int         `json:"retry_count"`
    Error         *LLMError   `json:"error,omitempty"`
}

type TokenUsage struct {
    InputTokens  int32   `json:"input_tokens"`
    OutputTokens int32   `json:"output_tokens"`
    CachedTokens int32   `json:"cached_tokens"`
    CostUSD      float64 `json:"cost_usd"`
}

type LLMError struct {
    Code    string `json:"code"`    // rate_limited, provider_error, timeout, invalid_request, auth_error, unknown
    Message string `json:"message"`
    Retriable bool `json:"retriable"`
}
```

### 6.4 Usage Event NATS Messages

In addition to the request-response cycle, the gateway emits usage events:

- `ace.usage.{agentId}.token`: `TokenUsage` payload (same shape as above) for real-time dashboards
- `ace.usage.{agentId}.cost`: `{ "cost_usd": float64, "provider_id": string, "model": string }`

These are fire-and-forget publishes, not request-reply.

### 6.5 Recommendation

Adopt the schema above. Key design decisions:
- **JSON encoding** for human readability and debugging. NATS messages in ACE already use JSON for other payloads.
- **ModelOverride** allows the cognitive engine to force a specific model within a group for A/B testing or capability requirements.
- **Error codes** are normalised across all providers so the cognitive engine can handle `rate_limited` uniformly regardless of whether OpenAI, Anthropic, or Bedrock returned it.
- **Schema versioning** enables future extensions (tool calls, multimodal, streaming chunks) without breaking existing consumers.

---

## 7. Encryption Key Rotation Strategy

### 7.1 The Problem

The problem space specifies AES-256-GCM with a master key from environment config (`PROVIDER_ENCRYPTION_KEY`). The open question asks whether to support key rotation without re-encrypting all stored API keys.

### 7.2 Options

**Option A: Direct encryption**
Encrypt each API key directly with the master key. Store `ciphertext + nonce` per provider.
- **Pros:** Simplest implementation. One encryption operation per key. No extra storage.
- **Cons:** Rotating the master key requires reading every provider row, decrypting with old key, re-encrypting with new key. For N providers, this is O(N) cryptographic operations. In a large deployment, this is a downtime-risk migration.

**Option B: Envelope encryption**
Generate a unique Data Encryption Key (DEK) per provider. Encrypt the API key with the DEK. Encrypt the DEK with the master Key Encryption Key (KEK). Store `encrypted_api_key`, `api_key_nonce`, `encrypted_dek`, `dek_nonce`.
- **Pros:** Master key rotation only requires re-encrypting DEKs (same size as API keys, but still O(N) operations — however, DEKs are small and the operation is fast). Each provider has unique encryption material, limiting blast radius if one DEK is compromised. Industry standard (AWS KMS, Google Cloud KMS, HashiCorp Vault).
- **Cons:** Double the encryption operations on read/write. More storage columns. Slightly more complex implementation.

### 7.3 Research Findings

The nr-vault ADR-002, Google Cloud KMS documentation, and production AES-GCM guides all recommend envelope encryption for database field-level encryption. The Cloud KMS docs state: "Generate a new DEK every time you write the data... Rotating the KEK only requires re-encrypting DEKs, not all data."

For ACE specifically:
- The number of providers per deployment is typically small (5-20). O(N) re-encryption is not a performance bottleneck even with direct encryption.
- However, envelope encryption future-proofs against integration with external KMS (AWS KMS, HashiCorp Vault) if ACE ever supports cloud-hosted deployments.
- The additional complexity is minimal: two AES-GCM calls instead of one, and four database columns instead of two.

### 7.4 Recommendation

**Implement envelope encryption (Option B).**

**Schema:**
```sql
encrypted_api_key   BYTEA  NOT NULL,  -- API key encrypted with DEK
api_key_nonce       BYTEA  NOT NULL,  -- 12-byte nonce for API key encryption
encrypted_dek       BYTEA  NOT NULL,  -- DEK encrypted with master KEK
dek_nonce           BYTEA  NOT NULL,  -- 12-byte nonce for DEK encryption
encryption_version  INT    NOT NULL DEFAULT 1,  -- Enables algorithm migration
```

**Rotation procedure:**
1. Generate new master KEK.
2. For each provider: decrypt DEK with old KEK, re-encrypt DEK with new KEK, update `encrypted_dek`, `dek_nonce`, increment `encryption_version`.
3. Old KEK can be discarded after all rows are migrated.
4. Read path: use `encryption_version` to select the correct KEK if multiple are valid during rotation window.

**Implementation:**
- Encryption/decryption lives in a new `internal/crypto` package (transport-agnostic, as per constraints).
- The service layer calls `crypto.EncryptField(plaintext, masterKey) -> EncryptedField` and `crypto.DecryptField(field, masterKey) -> plaintext`.
- The repository stores and retrieves `EncryptedField` without inspecting its contents.

---

## 8. Model Feature Detection Patterns

### 8.1 The Problem

How does the system know which features a model supports (streaming, function calling, vision, thinking tokens, JSON mode)? Provider APIs vary in how they expose this information.

### 8.2 Options

**Option 1: Provider API discovery**
Fetch model capabilities from the provider's API (e.g., OpenAI's `/models` endpoint includes some capability hints; Anthropic's model list does not include features).
- **Pros:** Automatically correct when providers add new models.
- **Cons:** Most providers do not expose structured capability data. OpenAI returns `owned_by` and `id` only — no feature flags. Anthropic returns model IDs with no metadata.

**Option 2: Static capability catalog**
Ship a hardcoded map of known models and their capabilities in each provider adapter.
- **Pros:** Reliable, fast, works offline.
- **Cons:** Requires code updates when new models are released. Stale data until deployment.

**Option 3: Manual user declaration**
Users check boxes for features when adding a model.
- **Pros:** Always correct for the user's actual use case.
- **Cons:** High friction. Users may not know which features a model supports.

**Option 4: Error-based detection**
Attempt the feature and detect "unsupported" errors.
- **Pros:** No upfront knowledge required.
- **Cons:** Expensive and slow. Some providers charge for failed requests. Not all failures are cleanly categorised.

### 8.3 Real-World Patterns

**LiteLLM** maintains a massive static JSON file (`model_prices_and_context_window.json`) mapping model IDs to capabilities, pricing, and context limits. This file is updated frequently and shipped with the library.

**Portkey** uses a combination: static catalog for known models, with fallback to user-declared capabilities for custom models.

**OpenRouter's API** returns model metadata including context length and pricing, but not feature flags.

### 8.4 Recommendation

**Implement a hybrid: Static catalog + manual override.**

1. **Static capability map:** Each provider adapter includes a `map[string]ModelCapabilities` for well-known models (e.g., `gpt-4o`, `claude-3-5-sonnet-20240620`). Capabilities include: `Streaming`, `FunctionCalling`, `Vision`, `JSONMode`, `ThinkingTokens`.
2. **Dynamic model list fetching:** For providers with `/models` endpoints (OpenAI, Together AI, Groq, Ollama), fetch the model list and cross-reference with the static catalog. Known models get capabilities from the catalog. Unknown models get defaults based on provider type (e.g., all OpenAI-compatible models default to `Streaming=true`, `FunctionCalling=false`).
3. **Manual override:** The provider UI allows users to edit capabilities for any model. This overrides the static catalog.
4. **Context limits:** Static catalog includes known context limits. User can manually tune down (but not above the static maximum).

**Maintenance:** The static catalog should be structured as a JSON file (`backend/internal/llm/providers/capabilities.json`) loaded at startup, not hardcoded in Go structs. This allows hot-reloading or updating without recompilation in future releases.

---

## 9. Research-Derived Architectural Principles

From the above analysis, the following principles govern the Providers unit design:

1. **Prefer provider-reported truth over local approximation.** Token counts, pricing, and usage are whatever the provider says they are. Normalise at the boundary, do not duplicate provider logic locally.
2. **Health-aware routing is table stakes; predictive routing is a luxury.** Circuit-breaker failover prevents user-visible degradation. Latency-weighted and cost-optimized routing require operational maturity that ACE will grow into.
3. **Envelope encryption is cheap insurance.** The complexity delta over direct encryption is small; the future-proofing for KMS integration and key rotation is significant.
4. **NATS payloads are versioned JSON structs.** Explicit Go types, never `map[string]interface{}`. Schema versioning enables evolution without breaking the cognitive engine.
5. **Streaming is an internal optimisation, not an external contract.** The cognitive engine receives complete responses. Streaming reduces provider-side latency and memory pressure but does not change the gateway's external interface.
6. **Static catalogs with manual override beat pure discovery.** Provider APIs rarely expose structured capability metadata. A maintainable static file plus user override is the pragmatic choice.

---

## 10. Open Questions Resolved / Deferred

| Question | Resolution | Rationale |
|---|---|---|
| Routing strategies beyond examples | Implement Round-Robin, Sequential, Failover. Defer others. | Sufficient for production reliability; others require telemetry volumes unavailable at initial scale. |
| Streaming support | Internal streaming only (Option B). | External streaming deferred to frontend streaming unit. |
| Timeout configuration | Per-provider configurable with type-specific defaults. | Local inference (Ollama) needs longer timeouts than cloud APIs. |
| Retry strategy | Exponential backoff with full jitter, 2 retries. | Industry standard; prevents thundering herd. |
| Token counting | Provider-reported only, normalised to inclusive model. | Local tokenizers are inaccurate and heavy. |
| NATS payload schema | Versioned JSON structs (LLMRequest / LLMResponse). | Explicit types, human-readable, extensible. |
| Encryption key rotation | Envelope encryption with DEK per provider. | Enables rotation without full re-encryption; industry standard. |
| Model feature detection | Static catalog + manual override. | Provider APIs lack structured capability metadata. |
| Concurrent request limits | Defer to rate limiting implementation. | Addressed by per-provider / per-group / per-user sliding windows. |
| Health checking | Active health checks via circuit-breaker in Failover strategy. | No separate heartbeat pings; health derived from actual request outcomes. |
| Custom provider validation | Test on save with "Working" prompt. | Validate before persisting; allow save only on success or explicit override. |
| Pricing auto-fetch schedule | Fetch on provider creation + manual refresh button. | No automatic schedule; pricing changes infrequently and user can refresh. |

---

*Deliverable: design/units/providers/research.md*
*Status: Complete*
*Next: architecture.md*
