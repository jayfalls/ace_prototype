# Problem Space

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: The ACE Framework has zero LLM provider infrastructure. The NATS messaging layer defines `ace.llm.*.request` and `ace.llm.*.response` subjects, and the telemetry layer tracks LLM usage events, but there is no way to configure a provider, store an API key, select a model, form a provider group, or make an actual LLM API call. Every future cognitive engine feature — layer loops, chat, memory consolidation, tool use — depends on being able to route an LLM request to a configured provider and get a response back. Without this unit, the cognitive engine cannot run.

The specific problems being solved are:

1. **No provider configuration** — There is no database table, API, or UI for storing provider endpoints, API keys (encrypted), model lists, or provider-specific parameters (context limits, pricing, supported features)
2. **No model catalog** — There is no system for managing which models a provider offers, their context windows, pricing per token, or which features they support (streaming, function calling, vision, etc.)
3. **No provider groups** — There is no concept of grouping providers/models together with routing strategies (round-robin, sequential, failover) so that an agent can use multiple models for resilience or cost optimization
4. **No LLM routing** — The existing NATS `ace.llm.*.request` subjects have no consumer. There is no LLM gateway that receives a request, selects a provider+model from a group, makes the API call, handles errors/retries, and returns the response
5. **No usage or cost tracking at the provider level** — While `usage_events` exist, they are populated from telemetry, not from actual provider response data. There is no way to know how many tokens a specific provider call consumed, what it cost, or track usage against rate limits
6. **No rate limiting** — There is no mechanism to enforce per-provider, per-group, or per-user rate limits on LLM API calls
7. **No provider testing** — Users have no way to verify that a provider configuration is working without making a full agent run
8. **No frontend for provider management** — There is no UI page for configuring providers, models, groups, or viewing usage/cost

**Q: Who are the users?**
A:
- **ACE operators / admins** — Configure providers (API keys, endpoints), set up provider groups with routing strategies, manage global rate limits, monitor usage and costs
- **ACE agent users** — Select which provider group their agent uses for different cognitive tasks (per-layer groups, chat, memory, etc.) — though this selection happens in the agent config (future unit), the group definitions must exist first
- **The cognitive engine** (future unit) — The primary consumer of the LLM routing layer. It sends request messages on `ace.llm.*.request` and expects responses on `ace.llm.*.response`
- **Developers / self-hosters** — Need to point ACE at local models (Ollama, llama.cpp), configure custom OpenAI-compatible endpoints, and test that everything works

**Q: What are the success criteria?**
A:
1. Users can configure providers via the UI (API key, base URL, models, provider-specific parameters) for all supported provider types
2. API keys are encrypted at rest in the database using AES-GCM with a master key from the main config
3. Users can create named provider groups and assign providers to them with routing strategies (round-robin, sequential, failover)
4. Users can test a provider configuration from the UI by sending "Respond with the word 'Working' and nothing else." and seeing the response
5. The LLM gateway reads provider group config, resolves the next provider/model based on the group's strategy, makes the API call, and returns the response on NATS
6. Rate limits are enforced at three levels: per-provider, per-group, and per-user
7. Usage tracking captures input tokens, output tokens, cached tokens, and cost per LLM call, attributed to provider/model
8. Provider models can be fetched automatically where the provider supports it (OpenAI, Anthropic, etc.) or entered manually
9. Model configurations include predetermined settings (context limit, top_p, temperature) with the ability to manually override context limits
10. The system supports all target providers: OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, xAI (Grok), Groq, Together AI, Mistral AI, Cohere, DeepSeek, Alibaba (Qwen/Tongyi), Baidu (ERNIE), ByteDance (Doubao/ARK), Zhipu (GLM), 01.AI (Yi), NVIDIA, OpenRouter, Ollama, llama.cpp, and Custom OpenAI-compatible endpoints

**Q: What constraints exist (budget, timeline, tech stack)?**
A:
- **Tech Stack**: Go backend (Chi router, SQLC, PostgreSQL/SQLite), SvelteKit/TypeScript frontend, NATS messaging, embedded SQLite
- **Handler → Service → Repository pattern** — All provider logic follows the established layered architecture
- **SQLC for database access** — All queries type-safe, no raw SQL in Go code
- **No `interface{}` or `any`** — Explicit types throughout
- **No else chains** — Early returns only
- **NATS for LLM request/response** — `ace.llm.*.request` / `ace.llm.*.response` subjects already defined; the LLM gateway must subscribe to requests and publish responses
- **Usage events are non-negotiable** — Every LLM call emits a usage event with input/output/cached token counts and cost, attributed to agentId, provider, and model
- **API keys encrypted at rest** — AES-GCM encryption with master key from env (alongside JWT config), never stored in plaintext
- **Existing response package** — Use `response.OK()`, `response.BadRequest()`, etc.
- **All operations go through the Makefile** — No direct docker/go/npm commands
- **Pre-commit hooks are mandatory** — All code passes quality gates

## Iterative Exploration

### Provider Architecture

#### 1. Provider Types and Capabilities
**Q: What provider types are supported and how do they differ?**
A: Providers fall into categories with different API formats, auth mechanisms, and capabilities:

- **OpenAI-compatible (REST API)**: OpenAI, Together AI, Groq, DeepSeek, OpenRouter, Custom, Ollama, llama.cpp, xAI (Grok), Mistral AI, Cohere, NVIDIA, Alibaba (Qwen), Zhipu (GLM), 01.AI (Yi), ByteDance (Doubao/ARK) — all use OpenAI-compatible chat completions endpoints with minor variations in auth (Bearer token vs API key header) and parameter naming
- **Anthropic**: Uses its own Messages API format with `x-api-key` header auth, different request/response schema, and native thinking tokens
- **Google Gemini**: Uses its own API format with API key in query string or OAuth, different schema for multi-turn and multimodal
- **AWS Bedrock**: Uses AWS Signature V4 signing, requires AWS credentials (access key, secret key, region), InvokeModel API with provider-specific request bodies
- **Azure OpenAI**: OpenAI-compatible but with a different base URL pattern (`{resource}.openai.azure.com`) and API version in query string, uses `api-key` header
- **Baidu (ERNIE)**: Uses Baidu's own API format with OAuth 2.0 token-based auth (access token from API key/secret), different schema
- **Ollama**: OpenAI-compatible but typically runs locally; model list endpoint available; no API key needed

Each provider type needs its own adapter implementation that translates a normalized LLM request into the provider's specific format and normalizes the response back.

#### 2. Encrypted API Key Storage
**Q: How should API keys be encrypted at rest?**
A: **AES-256-GCM with a master key from environment config** — The master encryption key (`PROVIDER_ENCRYPTION_KEY`) lives in the main env config alongside `JWT_SECRET`, requiring 32 bytes (256 bits). On write, the API key is encrypted with AES-GCM using a random nonce; the nonce is stored alongside the ciphertext in the database. On read, the ciphertext is decrypted using the nonce and master key. Encryption/decryption happens in the service layer, not the repository layer — the repository stores and retrieves opaque `encrypted_key` and `key_nonce` byte columns. The master key is never logged, exposed in API responses, or stored in the database.

#### 3. Provider Group Routing Strategies
**Q: What routing strategies should be supported for provider groups?**
A: **To be determined in research phase** — Possible strategies to explore include (not exhaustive):

- **Round Robin** — Distributes requests evenly across providers in the group. Each request advances a counter and selects the next provider. Simple load balancing across multiple API keys or endpoints.
- **Sequential** — Always selects the first provider in the group list. If it fails (error, rate limit, timeout), tries the next. Useful for primary/fallback patterns (e.g., try GPT-4 first, fall back to GPT-4-mini on rate limit).
- **Failover** — Always uses the primary provider. On any failure, marks it as degraded and switches to the next provider for a cooldown period, then retries the primary. More sophisticated than sequential — adds circuit-breaker semantics.
- **Weighted** — Providers are assigned weights (e.g., based on cost, speed, or reliability); requests are distributed proportionally.
- **Latency-based** — Routes to the provider with the lowest recent observed latency. Requires tracking per-provider response time metrics.
- **Cost-optimized** — Routes to the cheapest eligible provider that meets the request's requirements (context window size, capability requirements).
- **Least-loaded** — Routes to the provider with the fewest current in-flight requests. Requires active request tracking.
- **Priority + failover** — Strict priority order with conditional fallback rules (e.g., "use provider A if under 50% rate limit usage, otherwise B, then C").

The research phase should investigate these and any other relevant strategies, evaluate tradeoffs (complexity vs usefulness, state requirements, observability), and recommend a set to implement. Strategies with state (counters, health status, latency history) must be tracked in-memory per group, with eventual consistency for multi-replica deployments via NATS messages.

#### 4. Provider Group Usage Contexts
**Q: Where are provider groups selected?**
A: Provider groups are referenced at multiple points in the agent configuration (to be built in the Cognitive Engine unit):
- Per cognitive layer (L1-L6) — different providers for aspirational vs task prosecution layers
- Chat interface — which provider handles user chat interactions
- Memory consolidator — which provider summarizes and consolidates memory
- Loops — which provider handles loop iterations
- Tools/function calling — which provider handles tool use

Each configuration point selects a provider group by ID. Default: the first created group. Manual overrides allowed per-config point. The providers unit defines the groups; the cognitive engine unit selects them.

#### 5. Model Catalog Strategy
**Q: Static catalog vs dynamic fetching vs user input?**
A: **Hybrid** — For providers with model listing APIs (OpenAI, Anthropic, Google, Together AI, Groq, Ollama, etc.), fetch models on provider creation and periodically refresh. Store the fetched models with their declared context windows and supported features. For providers without model listing (Custom endpoints, some Chinese providers), allow manual model entry. Pricing data is fetched from known public sources where available, user-entered otherwise (defaults to $0). The model catalog is per-provider, with a global model cache that aggregates across providers for deduplication and comparison.

#### 6. Rate Limiting Architecture
**Q: How do the three levels of rate limiting work together?**
A: Rate limits are checked in order: per-provider → per-group → per-user.

- **Per-provider**: RPM (requests per minute) and TPM (tokens per minute) limits reflecting the actual API limits of that provider+model combination. This prevents hitting provider-imposed rate limits.
- **Per-group**: Aggregate RPM/TPM across all providers in a group. This prevents a single agent from dominating a shared group.
- **Per-user**: Global RPM/TPM across all LLM calls by a user. This enforces plan/usage limits.

All three use sliding window counters. The lowest remaining budget across the three levels determines whether a request is allowed. Rate limit state is in-memory with NATS-synced coordination for multi-replica deployments.

#### 7. Cost and Usage Tracking
**Q: What cost and usage data must be captured?**
A: Every LLM call captures:
- **Token counts**: input tokens, output tokens, cached tokens (from provider response or calculated)
- **Cost**: calculated from token counts × model pricing (fetched, user-entered, or $0 default)
- **Provider and model**: which provider and model served the request
- **Agent attribution**: agentId, cycleId, sessionId, layerId
- **Timing**: request duration, time-to-first-token (for streaming)
- **Status**: success, error type, rate-limited, retry count

This data is stored in the `usage_events` table (which already has `model`, `input_tokens`, `output_tokens`, `cost_usd`, `duration_ms` columns) and also published on `ace.usage.*.token` and `ace.usage.*.cost` NATS subjects for real-time dashboards.

#### 8. Provider Testing
**Q: How does the test button work?**
A: The provider UI has a "Test" button per provider configuration. When clicked, the frontend sends a request to the backend which makes an actual LLM API call to the configured provider with the prompt: *"Respond with the word 'Working' and nothing else."* The backend returns the model's response text. The UI shows the response as a successful test, or shows the error message on failure. This validates: API key validity, endpoint reachability, model availability, and basic request/response flow.

### Integration with Existing Architecture

#### 9. Relationship to NATS Subjects
**Q: How does the LLM gateway integrate with the existing NATS subjects?**
A: The LLM gateway subscribes to `ace.llm.{agentId}.request` (already defined). When a cognitive engine component needs an LLM call, it publishes a request on this subject. The gateway:
1. Receives the request (which includes provider group ID, system prompt, messages, parameters)
2. Resolves the group's routing strategy to pick a provider+model
3. Checks rate limits at all three levels
4. Makes the API call via the appropriate provider adapter
5. Publishes the response on `ace.llm.{agentId}.response`
6. Emits usage event on `ace.usage.{agentId}.token` and `ace.usage.{agentId}.cost`

The gateway handles errors, retries, and fallback logic based on the group's strategy. It does NOT manage agent state or cognitive cycles — it is purely a request/response router.

#### 10. Relationship to Existing Usage Events
**Q: How does provider cost tracking extend the existing usage_events table?**
A: The existing `usage_events` table has columns for `model`, `input_tokens`, `output_tokens`, `cost_usd`, and `duration_ms`. The providers unit adds:
- `provider_id` — which provider served the request
- `provider_group_id` — which group routed the request
- `cached_tokens` — cached token count from provider
- `rate_limited` — whether the request was rate limited
- `retry_count` — how many retries were needed

A migration adds these columns. The handler that writes usage events is updated to populate them from the LLM gateway's response data.

#### 11. Relationship to Agent Config (Future Unit)
**Q: What does the providers unit expose for the cognitive engine unit?**
A: The providers unit defines the **LLM Gateway service** — a Go service within the backend binary that:
- Exposes an internal Go function `CallLLM(ctx, req LLMRequest) (*LLMResponse, error)` for programmatic use within the same process
- Also subscribes to NATS `ace.llm.{agentId}.request` for out-of-process consumers
- Exposes provider group resolution via `ResolveGroup(ctx, groupID) (*ResolvedProvider, error)` which applies the strategy and returns the selected provider+model
- Stores all provider, model, and group configuration in the database, managed through CRUD API endpoints

The cognitive engine unit imports the gateway service and calls `CallLLM`. The provider configuration (which groups exist) is managed entirely by this unit. The agent config (which group each layer uses) is managed by the cognitive engine unit.

## Key Insights

1. **Provider adapters are the crux** — Each provider has a different API format, auth mechanism, and response schema. The normalized internal LLM request/response format is the contract; adapters translate to/from each provider's native format. Adding a new provider = writing a new adapter.

2. **Encryption must be invisible to the repository** — The repository stores ciphertext+nonce. The service layer handles encrypt/decrypt. This keeps the data layer simple and the security layer auditable.

3. **Groups are the unit of selection** — Every LLM request goes through a provider group, never directly to a provider. Even a single-provider setup uses a group of size 1. This uniform indirection simplifies routing, failover, and future load-balancing.

4. **Rate limits at three levels prevent cascading failures** — Per-provider limits protect against provider API throttling. Per-group limits prevent a single agent from starving others in a shared group. Per-user limits enforce plan/usage boundaries.

5. **Cost tracking must be real-time and agent-attributed** — Every LLM call emits a usage event immediately (not batched) so that the user-facing dashboard and the agent's own cost-awareness loop have current data. agentId attribution enables cost breakdown per agent, per layer, per task.

6. **The test button is a critical UX detail** — Provider configuration is high-friction (API keys, endpoints, models). An instant "did it work?" feedback loop makes configuration iterative and debuggable.

7. **Model fetching vs manual entry is a UX continuum** — For well-known providers (OpenAI, Anthropic), fetch models automatically and populate known pricing. For custom endpoints, allow free-form entry. The system should feel intelligent where it can, transparent where it can't.

8. **The LLM gateway is not the cognitive engine** — It is a routing and translation layer only. It does not manage state, prompts, context windows, or multi-turn conversations. It receives a request, selects a provider, makes an API call, returns the response. Cognitive logic belongs in the engine layers.

## Functional Requirements

The following functional requirements will feed directly into the BSD. Each is numbered for traceability.

- **FR-1**: The system SHALL support creating, reading, updating, and deleting provider configurations via the REST API and UI
- **FR-2**: The system SHALL encrypt all provider API keys at rest using AES-256-GCM with a master key from environment configuration
- **FR-3**: The system SHALL support the following provider types: openai, anthropic, google, azure, bedrock, groq, together, mistral, cohere, xai, deepseek, alibaba, baidu, bytedance, zhipu, 01ai, nvidia, openrouter, ollama, llamacpp, custom (OpenAI-compatible)
- **FR-4**: Each provider type SHALL have an adapter that translates normalized LLM requests into the provider's native API format and normalizes responses back
- **FR-5**: The system SHALL support creating named provider groups and assigning providers to them with an order/priority
- **FR-6**: Provider groups SHALL support multiple routing strategies with the specific set to be determined during the research phase (candidates: round-robin, sequential, failover, weighted, latency-based, cost-optimized, least-loaded, priority+failover)
- **FR-7**: The system SHALL allow testing a provider configuration from the UI by sending the prompt "Respond with the word 'Working' and nothing else." and displaying the response or error
- **FR-8**: The system SHALL provide an LLM Gateway service that subscribes to `ace.llm.{agentId}.request`, resolves the provider group, makes the API call, and publishes the response on `ace.llm.{agentId}.response`
- **FR-9**: The LLM Gateway SHALL enforce rate limits at three levels: per-provider (RPM/TPM), per-group (RPM/TPM), and per-user (RPM/TPM)
- **FR-10**: Rate limits SHALL use sliding window counters with in-memory state
- **FR-11**: The system SHALL capture token usage per LLM call: input_tokens, output_tokens, cached_tokens
- **FR-12**: The system SHALL calculate and store cost per LLM call based on token counts × model pricing (fetched, user-entered, or $0 default)
- **FR-13**: The system SHALL emit usage events on the NATS subjects `ace.usage.{agentId}.token` and `ace.usage.{agentId}.cost` for every LLM call
- **FR-14**: The system SHALL support fetching available models from provider APIs where available (OpenAI, Anthropic, Google, Together AI, Groq, Ollama, etc.)
- **FR-15**: The system SHALL allow manual entry of models for providers without model listing APIs
- **FR-16**: Each model SHALL have configurable predetermined settings: context_limit, temperature, top_p, top_k, max_tokens, stop_sequences, presence_penalty, frequency_penalty
- **FR-17**: The system SHALL allow overriding the context_limit per model below its maximum (manual tune-down, but not above)
- **FR-18**: Pricing models SHALL support per-model input_token_cost and output_token_cost, settable by the user or auto-fetched
- **FR-19**: The system SHALL provide a frontend page for managing providers with CRUD operations, model lists, and test functionality
- **FR-20**: The system SHALL provide a frontend page for managing provider groups with provider assignment and strategy selection
- **FR-21**: The system SHALL provide a frontend page for viewing usage and cost data per provider, per model, per group
- **FR-22**: The default provider group SHALL be auto-created on first provider setup
- **FR-23**: The LLM Gateway SHALL handle provider API errors gracefully, with configurable retry and fallback per group strategy
- **FR-24**: The system SHALL emit traces for every LLM call with agentId, provider, model, duration, token counts as span attributes

## Non-Goals

The following are explicitly out of scope for this unit:

- **Agent configuration** — Selecting which provider group a cognitive layer, chat interface, or memory consolidator uses belongs to the Cognitive Engine unit. This unit defines the groups; the engine unit selects them.
- **Prompt management** — System prompts, message history management, and context window optimization are cognitive engine concerns, not provider concerns
- **Streaming** — Real-time streaming of LLM responses to the frontend belongs to a future unit (the LLM gateway will support it internally, but the frontend streaming integration is separate)
- **Multi-modal support** — Vision, audio, and file upload handling belong to the Senses/Inputs unit
- **Semantic caching** — Caching LLM responses by semantic similarity belongs to a future caching optimization unit
- **LLM evaluation** — Prompt quality, model comparison benchmarks, and A/B testing belong to a future evaluation unit
- **Multi-replica NATS coordination** — Synchronizing rate limit counters and routing state across multiple gateway replicas via NATS is deferred (initial implementation uses in-memory state suitable for single-replica deployments; NATS coordination is an enhancement path)
- **Billing integration** — Generating invoices, subscription management, and usage-based billing are deferred to a future unit
- **Provider analytics dashboard** — Advanced charts, trend analysis, cost forecasting, and anomaly detection belong to a future observability/analytics unit

## Dependencies Identified

- **NATS subjects** — `ace.llm.{agentId}.request` and `ace.llm.{agentId}.response` (already defined in `internal/messaging/subjects.go`)
- **NATS streams** — `COGNITIVE` stream (already includes `ace.llm.request` and `ace.llm.response` subjects)
- **Usage events table** — `usage_events` (already exists with `model`, `input_tokens`, `output_tokens`, `cost_usd`, `duration_ms`; needs migration for `provider_id`, `provider_group_id`, `cached_tokens`, `retry_count`)
- **Config package** — `internal/api/config/config.go` (needs `PROVIDER_ENCRYPTION_KEY` env var alongside JWT config)
- **Response package** — `internal/api/response/` (existing helpers)
- **Middleware** — Auth middleware for protected provider management endpoints
- **SQLC** — For type-safe database queries on providers, provider_groups, provider_group_members, and models tables

## Assumptions Made

1. New tables: `providers` (name, provider_type, base_url, encrypted_api_key, key_nonce, config JSON), `provider_models` (provider_id, model_id, display_name, context_limit, features JSON, pricing JSON), `provider_groups` (name, strategy, config JSON), and `provider_group_members` (group_id, provider_id, priority/order)
2. The existing `usage_events` table will get a migration adding `provider_id`, `provider_group_id`, `cached_tokens`, and `retry_count` columns
3. The LLM Gateway will be a Go service within the `backend/internal` package, NOT a separate binary
4. Provider adapters live in `backend/internal/llm/providers/{provider_type}/adapter.go` following an `LLMAdapter` interface
5. The normalized LLM request/response format will be defined in `backend/internal/llm/types.go`
6. Rate limiters will use Go's `sync.Map` or a similar in-memory structure with sliding window counters
7. The frontend provider pages will be under the `(app)/providers` route group
8. Model pricing auto-fetch will use known public pricing endpoints (OpenAI published prices, Anthropic published prices, OpenRouter model list with costs, etc.)
9. The master encryption key (`PROVIDER_ENCRYPTION_KEY`) will be a 32-byte hex-encoded string, required on startup, validated for minimum length
10. Default group auto-creation: when the first provider is created, a default group named "default" is created with round-robin strategy containing that provider

## Open Questions (For Research)

1. **Streaming support**: Does the LLM gateway need to support streaming responses internally (NATS request/response is inherently request-reply, not streaming)? Should streaming be a separate path?
2. **Timeout configuration**: Should timeouts be per-provider, per-model, or per-request? What are sensible defaults for each provider type?
3. **Retry strategy**: Exponential backoff? Jitter? Max retries per provider type? How does retry interact with group failover?
4. **Model feature detection**: How to determine which features a model supports (streaming, function calling, vision, thinking tokens)? Provider-declared via API, user-declared in UI, or auto-detect from error messages?
5. **Token counting**: Use the provider's returned token counts (when available), or count locally using a tokenizer library (tiktoken-go, etc.)? Both?
6. **Concurrent request handling**: Maximum concurrent LLM requests per provider? Per group? Per user? Queue/buffer behavior when limits are reached?
7. **Health checking**: Should the gateway periodically health-check providers (heartbeat pings) or only detect failures during actual requests?
8. **Custom provider validation**: For custom OpenAI-compatible endpoints, how to validate the endpoint is actually working before saving? (Options: test on save with the "Working" prompt, or allow save without test)
9. **NATS message format**: What fields go in the `ace.llm.{agentId}.request` and `ace.llm.{agentId}.response` message bodies? The envelope headers are defined; the payload schema needs design.
10. **Pricing auto-fetch schedule**: How often to refresh pricing data? On provider update? On a schedule? Manual trigger?
11. **Encryption key rotation**: Does the master key support rotation without re-encrypting all stored keys? (A wrapping key pattern — each API key encrypted with a random data key, data key encrypted with master key — enables key rotation without re-encrypting all values.)
12. **Routing strategies**: Which strategies beyond the examples (round-robin, sequential, failover) are worth implementing? Weighted, latency-based, cost-optimized, least-loaded, priority+failover — evaluate tradeoffs and recommend a set.

## Next Steps

1. Proceed to BSD (Behavioral System Design) with the problem space clarified
2. Research phase should evaluate:
   - Provider group routing strategies — research, compare, and recommend the final set beyond the initial examples
   - Streaming support in the LLM gateway
   - Retry and timeout strategies per provider type
   - Token counting approach (provider-reported vs local tokenization)
   - NATS message payload schema for LLM requests/responses
   - Encryption key rotation strategy
   - Model feature detection patterns
3. Design the database schema for providers, models, groups, and group members
4. Design the LLM Gateway architecture and provider adapter interface
5. Define the normalized LLM request/response format
6. Design the rate limiter architecture (three-level sliding window)
7. Design the frontend UI pages for providers, models, groups, and usage/cost
8. Design the provider test endpoint
