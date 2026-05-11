# Providers Unit — Architecture Design

## 1. Executive Summary

This document defines the architecture for the **Providers** unit, the LLM routing, configuration, and cost-tracking layer of the ACE Framework. It is foundational infrastructure that all future cognitive engine features depend upon.

The unit introduces four major backend subsystems — **Provider Configuration**, **LLM Gateway**, **Rate Limiter**, and **Crypto** — plus three frontend pages. All components run inside the existing single binary and follow the established Handler → Service → Repository pattern with SQLC-generated database access.

Key architectural decisions:
- **Envelope encryption** (DEK-per-provider wrapped by a master KEK) for API key storage.
- **Normalized LLM request/response types** with provider-specific adapters translating to/from native APIs.
- **Three routing strategies**: Round-Robin, Sequential, and Failover with circuit-breaker.
- **Three-level sliding-window rate limiting**: per-provider, per-group, per-user.
- **Internal streaming only** — adapters use provider streaming APIs but buffer complete responses before returning to the gateway.
- **NATS request-reply** for LLM calls from the cognitive engine, with fire-and-forget usage events.

---

## 2. Architectural Principles

1. **Provider adapters are the only provider-aware code.** The gateway, rate limiter, usage tracker, and frontend know providers only by UUID and type string. All API-format specifics live in adapter packages.
2. **Encryption happens in the service layer, never in the repository.** The repository stores opaque ciphertext+nonce bytes. The service layer calls `crypto.EncryptField` / `DecryptField`.
3. **Groups are the unit of selection.** Every LLM request goes through a provider group, even for single-provider setups. This uniform indirection enables failover, load balancing, and future routing enhancements without changing consumer code.
4. **Rate limits are checked at the gateway boundary, not inside adapters.** Adapters return normalized token counts; the gateway applies them to sliding-window counters before emitting usage events.
5. **In-memory state for routing and rate limits.** Multi-replica NATS coordination is explicitly deferred. The initial implementation targets single-replica deployments.
6. **No `any` or `interface{}`.** All LLM request/response envelopes, config JSON, and adapter internals use explicit Go structs. Configuration that varies by provider type is stored as typed JSONB with strict unmarshal validation.
7. **Usage events are emitted synchronously within the gateway call path.** Every LLM call produces a `usage_events` row and two NATS publishes (`ace.usage.{agentId}.token`, `ace.usage.{agentId}.cost`) before the gateway returns.

---

## 3. System Topology

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ACE Single Binary                               │
│                                                                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────────────┐  │
│  │   Frontend  │    │   SQLite    │    │         LLM Gateway             │  │
│  │  (SvelteKit)│◄──►│  (embedded) │◄──►│  ┌───────────────────────────┐  │  │
│  │             │    │             │    │  │  Provider Group Resolver  │  │  │
│  │ /providers  │    │ providers   │    │  │  (Round-Robin / Sequential│  │  │
│  │ /groups     │    │ provider_   │    │  │   / Failover)             │  │  │
│  │ /usage      │    │ groups      │    │  └───────────┬───────────────┘  │  │
│  └─────────────┘    │ provider_   │    │              │                   │  │
│         ▲           │ group_      │    │              ▼                   │  │
│         │           │ members     │    │  ┌───────────────────────────┐  │  │
│         │           │ provider_   │    │  │  Rate Limiter (3 levels)  │  │  │
│         │           │ models      │    │  │  per-provider / group /   │  │  │
│         │           │ usage_events│    │  │  user sliding window      │  │  │
│         │           └─────────────┘    │  └───────────┬───────────────┘  │  │
│         │                   ▲          │              │                   │  │
│         │                   │          │              ▼                   │  │
│         │              ┌────┴────┐     │  ┌───────────────────────────┐  │  │
│         │              │ SQLC    │     │  │  Provider Adapter         │  │  │
│         │              │ Queries │     │  │  (OpenAI, Anthropic, ...) │  │  │
│         │              └─────────┘     │  └───────────┬───────────────┘  │  │
│         │                              │              │                   │  │
│         │                              │         ┌────┴────┐              │  │
│         │                              │         │  NATS   │              │  │
│         │                              │         └────┬────┘              │  │
│         │                              │              │                   │  │
│         └──────────────────────────────┼──────────────┘                   │  │
│                                        │                                    │  │
│                              ┌─────────┴──────────┐                        │  │
│                              │  ace.llm.{id}.req  │                        │  │
│                              │  ace.llm.{id}.resp │                        │  │
│                              │  ace.usage.{id}.*  │                        │  │
│                              └────────────────────┘                        │  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Component Breakdown

### 4.1 Provider Configuration Subsystem

**Responsibility:** CRUD operations for providers, models, and provider groups. API key encryption/decryption. Model fetching and capability management.

**Backend components:**
- `ProviderHandler` — HTTP handlers for `/api/providers`, `/api/providers/{id}/test`, `/api/provider-groups`, `/api/provider-groups/{id}/members`.
- `ProviderService` — Business logic: encrypt/decrypt API keys, orchestrate model fetching, auto-create default group, validate provider configs.
- `ProviderRepository` (SQLC) — Database queries for `providers`, `provider_models`, `provider_groups`, `provider_group_members`.

**Frontend components:**
- `ProvidersPage` — List, create, edit, delete providers; test button; model list with capabilities.
- `ProviderGroupsPage` — List, create, edit, delete groups; assign/reorder members; select routing strategy.
- `UsagePage` — Aggregated usage/cost per provider, model, group.

### 4.2 LLM Gateway

**Responsibility:** Receive LLM requests (NATS or in-process), resolve provider group, enforce rate limits, call adapter, emit usage events, return response.

**Backend components:**
- `Gateway` service — Core orchestrator. Single instance per process, initialized at startup.
- `GroupResolver` — Applies routing strategy to select provider+model from a group.
- `ProviderAdapter` interface — One implementation per provider type.
- `RetryExecutor` — Per-provider retry with exponential backoff + full jitter.

**NATS integration:**
- Subscribes to `ace.llm.{agentId}.request` (JetStream consumer on `COGNITIVE` stream).
- Publishes responses to `ace.llm.{agentId}.response` via `messaging.ReplyTo`.
- Publishes usage events to `ace.usage.{agentId}.token` and `ace.usage.{agentId}.cost`.

**In-process integration:**
- Exposes `CallLLM(ctx context.Context, req LLMRequest) (*LLMResponse, error)` for direct use by the cognitive engine.

### 4.3 Rate Limiter

**Responsibility:** Enforce RPM and TPM limits at three levels using sliding-window counters.

**Backend components:**
- `RateLimiter` service — In-memory sliding window counter manager.
- `WindowCounter` — Per-key (provider_id, group_id, user_id) request/token counters with TTL.

### 4.4 Crypto Subsystem

**Responsibility:** Envelope encryption for provider API keys.

**Backend components:**
- `crypto.EncryptField(plaintext, masterKey) -> EncryptedField` — Generates DEK, encrypts plaintext with DEK, encrypts DEK with master KEK.
- `crypto.DecryptField(field, masterKey) -> plaintext` — Decrypts DEK with KEK, decrypts plaintext with DEK.

---

## 5. Package Structure

All new code lives under the existing `backend/internal/` tree. No new top-level packages are created.

```
backend/internal/
├── crypto/
│   └── field_encryption.go          # Envelope encryption (DEK + KEK)
│
├── llm/
│   ├── types.go                      # Normalized LLMRequest, LLMResponse, ChatMessage, LLMParameters, TokenUsage, LLMError
│   ├── gateway.go                    # Gateway service: CallLLM, NATS subscriber setup
│   ├── resolver.go                   # GroupResolver: Round-Robin / Sequential / Failover logic
│   ├── rate_limiter.go               # RateLimiter: 3-level sliding window
│   ├── retry.go                      # RetryExecutor: exponential backoff + jitter
│   └── providers/
│       ├── adapter.go                # LLMAdapter interface
│       ├── capabilities.json         # Static model capability catalog
│       ├── openai/
│       │   └── adapter.go            # OpenAI-compatible adapter (covers most providers)
│       ├── anthropic/
│       │   └── adapter.go            # Anthropic Messages API adapter
│       ├── google/
│       │   └── adapter.go            # Gemini adapter
│       ├── azure/
│       │   └── adapter.go            # Azure OpenAI adapter
│       ├── bedrock/
│       │   └── adapter.go            # AWS Bedrock adapter
│       ├── ollama/
│       │   └── adapter.go            # Ollama adapter
│       └── custom/
│           └── adapter.go            # Custom OpenAI-compatible adapter
│
├── api/
│   ├── handler/
│   │   └── provider_handler.go       # ProviderHandler, ProviderGroupHandler, UsageHandler
│   ├── service/
│   │   └── provider_service.go       # ProviderService: CRUD, encryption, model fetch
│   ├── model/
│   │   └── provider.go               # Provider, ProviderGroup, ProviderModel request/response DTOs
│   └── repository/queries/
│       └── providers.sql             # SQLC queries for provider tables
```

---

## 6. Database Schema

Four new tables are created via Goose Go migrations. The existing `usage_events` table receives a migration adding provider-attribution columns.

### 6.1 New Tables

```sql
-- providers: one row per configured LLM endpoint
CREATE TABLE providers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    provider_type       TEXT NOT NULL,
    base_url            TEXT NOT NULL,
    encrypted_api_key   BLOB NOT NULL,
    api_key_nonce       BLOB NOT NULL,
    encrypted_dek       BLOB NOT NULL,
    dek_nonce           BLOB NOT NULL,
    encryption_version  INTEGER NOT NULL DEFAULT 1,
    config_json         TEXT NOT NULL DEFAULT '{}',
    is_enabled          INTEGER NOT NULL DEFAULT 1,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE INDEX idx_providers_type ON providers(provider_type);
CREATE INDEX idx_providers_enabled ON providers(is_enabled);

-- provider_models: one row per model offered by a provider
CREATE TABLE provider_models (
    id                  TEXT PRIMARY KEY,
    provider_id         TEXT NOT NULL,
    model_id            TEXT NOT NULL,
    display_name        TEXT NOT NULL,
    context_limit       INTEGER,
    features_json       TEXT NOT NULL DEFAULT '{}',
    pricing_json        TEXT NOT NULL DEFAULT '{}',
    parameters_json     TEXT NOT NULL DEFAULT '{}',
    is_user_edited      INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

CREATE INDEX idx_provider_models_provider_id ON provider_models(provider_id);

-- provider_groups: named groups with a routing strategy
CREATE TABLE provider_groups (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    strategy        TEXT NOT NULL,
    config_json     TEXT NOT NULL DEFAULT '{}',
    is_default      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX idx_provider_groups_default ON provider_groups(is_default);

-- provider_group_members: ordered list of providers in a group
CREATE TABLE provider_group_members (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (group_id) REFERENCES provider_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(group_id, provider_id)
);

CREATE INDEX idx_provider_group_members_group_id ON provider_group_members(group_id);
CREATE INDEX idx_provider_group_members_priority ON provider_group_members(group_id, priority);
```

### 6.2 Migration to Existing Table

```sql
ALTER TABLE usage_events ADD COLUMN provider_id TEXT;
ALTER TABLE usage_events ADD COLUMN provider_group_id TEXT;
ALTER TABLE usage_events ADD COLUMN cached_tokens INTEGER DEFAULT 0;
ALTER TABLE usage_events ADD COLUMN retry_count INTEGER DEFAULT 0;

CREATE INDEX idx_usage_events_provider_id ON usage_events(provider_id);
CREATE INDEX idx_usage_events_provider_group_id ON usage_events(provider_group_id);
```

---

## 7. Core Go Interfaces and Types

### 7.1 Normalized LLM Types (`internal/llm/types.go`)

```go
package llm

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type ChatMessage struct {
    Role    Role   `json:"role"`
    Content string `json:"content"`
}

type LLMParameters struct {
    Temperature      *float32 `json:"temperature,omitempty"`
    TopP             *float32 `json:"top_p,omitempty"`
    TopK             *int32   `json:"top_k,omitempty"`
    MaxTokens        *int32   `json:"max_tokens,omitempty"`
    StopSequences    []string `json:"stop_sequences,omitempty"`
    PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
}

type LLMRequest struct {
    SchemaVersion   string            `json:"schema_version"`
    ProviderGroupID string            `json:"provider_group_id"`
    ModelOverride   *string           `json:"model_override,omitempty"`
    SystemPrompt    string            `json:"system_prompt"`
    Messages        []ChatMessage     `json:"messages"`
    Parameters      LLMParameters     `json:"parameters"`
    Stream          bool              `json:"stream"`
    Metadata        map[string]string `json:"metadata,omitempty"`
}

type TokenUsage struct {
    InputTokens  int32   `json:"input_tokens"`
    OutputTokens int32   `json:"output_tokens"`
    CachedTokens int32   `json:"cached_tokens"`
    CostUSD      float64 `json:"cost_usd"`
}

type LLMError struct {
    Code      string `json:"code"`
    Message   string `json:"message"`
    Retriable bool   `json:"retriable"`
}

type LLMResponse struct {
    SchemaVersion   string      `json:"schema_version"`
    Success         bool        `json:"success"`
    Text            string      `json:"text,omitempty"`
    Model           string      `json:"model"`
    ProviderID      string      `json:"provider_id"`
    ProviderGroupID string      `json:"provider_group_id"`
    Usage           TokenUsage  `json:"usage,omitempty"`
    DurationMs      int64       `json:"duration_ms"`
    RetryCount      int         `json:"retry_count"`
    Error           *LLMError   `json:"error,omitempty"`
}
```

### 7.2 Adapter Interface (`internal/llm/providers/adapter.go`)

```go
package providers

import (
    "context"
    "time"
    "ace/internal/llm"
)

type AdapterConfig struct {
    TimeoutMs    int               `json:"timeout_ms"`
    DefaultModel string            `json:"default_model"`
    ExtraHeaders map[string]string `json:"extra_headers"`
}

type LLMAdapter interface {
    Name() string

    Call(ctx context.Context, req *llm.LLMRequest, apiKey string, baseURL string, config AdapterConfig) (*llm.LLMResponse, error)

    FetchModels(ctx context.Context, apiKey string, baseURL string, config AdapterConfig) ([]ModelInfo, error)

    DefaultTimeout() time.Duration
}

type ModelInfo struct {
    ModelID      string
    DisplayName  string
    ContextLimit int32
}

type Registry struct {
    adapters map[string]LLMAdapter
}

func NewRegistry() *Registry
func (r *Registry) Register(adapter LLMAdapter)
func (r *Registry) Get(providerType string) (LLMAdapter, bool)
```

### 7.3 Gateway (`internal/llm/gateway.go`)

```go
package llm

import (
    "context"
    "ace/internal/api/repository/generated"
    "ace/internal/llm/providers"
    "ace/internal/messaging"
    "ace/internal/telemetry"
)

type Gateway struct {
    queries       *generated.Queries
    registry      *providers.Registry
    resolver      *GroupResolver
    rateLimiter   *RateLimiter
    retryExecutor *RetryExecutor
    natsClient    *messaging.Client
    telemetry     *telemetry.Telemetry
}

func NewGateway(
    queries *generated.Queries,
    registry *providers.Registry,
    rateLimiter *RateLimiter,
    natsClient *messaging.Client,
    telemetry *telemetry.Telemetry,
) (*Gateway, error)

func (g *Gateway) CallLLM(ctx context.Context, req LLMRequest) (*LLMResponse, error)

func (g *Gateway) StartNATSConsumer(ctx context.Context) error
```

### 7.4 Rate Limiter (`internal/llm/rate_limiter.go`)

```go
package llm

import "context"

type LimitConfig struct {
    RPM int32
    TPM int32
}

type RateLimiter struct {
    // windows uses sync.Map for concurrent access without global locks.
    // Key format: "{scope}:{id}:{kind}" where scope is provider|group|user, kind is requests|tokens.
}

func NewRateLimiter() *RateLimiter

func (rl *RateLimiter) Check(ctx context.Context, providerID, groupID, userID string, tokens int32, limits LimitConfig) error

func (rl *RateLimiter) Record(ctx context.Context, providerID, groupID, userID string, tokens int32)
```

### 7.5 Crypto (`internal/crypto/field_encryption.go`)

```go
package crypto

type EncryptedField struct {
    Ciphertext        []byte
    Nonce             []byte
    EncryptedDEK      []byte
    DEKNonce          []byte
    EncryptionVersion int
}

func EncryptField(plaintext string, masterKey []byte) (EncryptedField, error)

func DecryptField(field EncryptedField, masterKey []byte) (string, error)
```

---

## 8. Data Flows

### 8.1 In-Process LLM Call (Cognitive Engine → Gateway)

```
Cognitive Engine
    │
    ▼
llm.Gateway.CallLLM(ctx, req)
    │
    ├──► GroupResolver.Resolve(ctx, req.ProviderGroupID)
    │      ├──► DB: SELECT members JOIN providers
    │      ├──► Apply strategy (round-robin / sequential / failover)
    │      └──► Return selected provider + model
    │
    ├──► RateLimiter.Check(provider, group, user, estimatedTokens, limits)
    │      └──► If blocked: return LLMResponse{Error: {Code: "rate_limited"}}
    │
    ├──► RetryExecutor.Execute(ctx, func() {
    │         adapter := registry.Get(provider.Type)
    │         decrypt apiKey via crypto.DecryptField
    │         adapter.Call(ctx, req, apiKey, baseURL, config)
    │      })
    │      └──► Exponential backoff with full jitter on retriable errors
    │
    ├──► RateLimiter.Record(provider, group, user, actualTokens)
    │
    ├──► Calculate cost from usage × model pricing
    │
    ├──► telemetry.Usage.LLMCall(ctx, agentID, cycleID, sessionID, "gateway", usage, cost, duration)
    │
    ├──► DB: INSERT INTO usage_events (...)
    │
    ├──► NATS: Publish ace.usage.{agentId}.token
    │
    ├──► NATS: Publish ace.usage.{agentId}.cost
    │
    └──► Return LLMResponse to caller
```

### 8.2 NATS LLM Call (Out-of-Process Consumer)

```
NATS subject: ace.llm.{agentId}.request
    │
    ▼
Gateway NATS handler (messaging.SubscribeToStreamWithEnvelope)
    │
    ├──► Parse JSON payload into llm.LLMRequest
    │
    ├──► Inject agentId from NATS envelope into req.Metadata["agent_id"]
    │
    ├──► CallLLM(ctx, req)  [same flow as 8.1]
    │
    └──► messaging.ReplyTo(client, incomingMsg, responseJSON)
            └──► NATS subject: ace.llm.{agentId}.response
```

### 8.3 Provider Test Flow (UI → Backend)

```
Frontend: click "Test" on provider card
    │
    ▼
POST /api/providers/{id}/test
    │
    ▼
ProviderHandler.TestProvider
    │
    ▼
ProviderService.TestProvider(ctx, providerID)
    │
    ├──► DB: SELECT provider by ID
    ├──► Decrypt API key
    ├──► Build test LLMRequest:
    │      Messages: [{Role: User, Content: "Respond with the word 'Working' and nothing else."}]
    │      ProviderGroupID: temporary single-member group
    ├──► adapter.Call(ctx, testReq, apiKey, baseURL, config)
    ├──► Return response text or error
    │
    └──► Frontend displays:
         ✅ "Working"  (green)
         ❌ "401 Unauthorized — invalid API key" (red with error detail)
```

### 8.4 Model Fetch Flow

```
Frontend: click "Fetch Models" during provider creation
    │
    ▼
POST /api/providers/{id}/fetch-models
    │
    ▼
ProviderService.FetchModels(ctx, providerID)
    │
    ├──► DB: SELECT provider
    ├──► Decrypt API key
    ├──► adapter := registry.Get(provider.Type)
    ├──► models, err := adapter.FetchModels(ctx, apiKey, baseURL, config)
    ├──► For each fetched model:
    │      ├──► Look up capabilities in capabilities.json by model_id
    │      ├──► Look up pricing in capabilities.json
    │      ├──► DB: INSERT or UPDATE provider_models
    │      └──► Mark is_user_edited = 0
    └──► Return list of fetched models
```

---

## 9. Security Architecture

### 9.1 Envelope Encryption

API keys are encrypted using a two-level envelope scheme:

1. **Per-provider Data Encryption Key (DEK)** — A random 32-byte AES-256 key generated on provider creation.
2. **Master Key Encryption Key (KEK)** — Derived from the `PROVIDER_ENCRYPTION_KEY` environment variable (hex-encoded 32-byte string, validated at startup).

Encryption steps:
1. Generate random DEK (32 bytes).
2. Encrypt API key with AES-256-GCM using DEK + random nonce → `encrypted_api_key`, `api_key_nonce`.
3. Encrypt DEK with AES-256-GCM using KEK + random nonce → `encrypted_dek`, `dek_nonce`.
4. Store all four values + `encryption_version` in `providers` row.

Decryption steps:
1. Read `encrypted_dek` + `dek_nonce` from row.
2. Decrypt DEK using KEK.
3. Read `encrypted_api_key` + `api_key_nonce` from row.
4. Decrypt API key using DEK.

**Key rotation:** Increment `encryption_version` in config. The read path selects the correct KEK by version. Rotation requires a migration job that re-encrypts all DEKs with the new KEK.

### 9.2 API Key Exposure Prevention

- API keys are **never** returned in any HTTP response.
- The provider list endpoint returns `api_key_masked` (e.g. `sk-...XXXX`) computed as `last_4_chars`.
- The `ProviderService` is the only package that calls `crypto.DecryptField`.
- Decrypted keys are passed to adapters as `string` parameters and are never stored in struct fields or logs.
- The `internal/crypto` package is transport-agnostic and does not import `net/http` or NATS.

---

## 10. Rate Limiting Architecture

### 10.1 Sliding Window Counter

Each limit scope (provider, group, user) maintains two sliding windows:
- **Request window**: counts requests in the last 60 seconds.
- **Token window**: counts input+output tokens in the last 60 seconds.

A window is implemented as a circular buffer of 12 slots (5-second granularity). On each check:
1. Sum all slots within the last 60 seconds.
2. If `current + delta > limit`, reject.
3. Otherwise, add `delta` to the current slot and allow.

### 10.2 Three-Level Enforcement

```
LLMRequest arrives
    │
    ├──► Per-provider check
    │      key: "provider:{provider_id}:requests|tokens"
    │      limits: from provider_models.pricing_json or defaults
    │
    ├──► Per-group check
    │      key: "group:{group_id}:requests|tokens"
    │      limits: from provider_groups.config_json or defaults
    │
    └──► Per-user check
           key: "user:{user_id}:requests|tokens"
           limits: hardcoded defaults or from future user-plan config

If ANY level rejects → return rate_limited error.
If ALL allow → proceed to adapter call.
After successful call → Record at all three levels.
```

### 10.3 In-Memory State

All windows live in a `sync.Map` within the single process. Keys expire after 2 minutes of inactivity to prevent unbounded memory growth. Multi-replica synchronization is deferred.

---

## 11. Routing & Failover

### 11.1 GroupResolver State

```go
type GroupResolver struct {
    roundRobinCounters sync.Map // groupID -> *atomic.Uint64
    healthStates       sync.Map // providerID -> *HealthState
}

type HealthState struct {
    status     HealthStatus // healthy, degraded, unhealthy, probing
    lastFail   time.Time
    failCount  int
    cooldown   time.Duration
}
```

### 11.2 Strategy Implementations

**Round-Robin:**
1. Atomically increment counter for group.
2. Select member at index `counter % len(members)`.
3. Skip members with `healthState == unhealthy`.
4. If all members unhealthy, reset all to `healthy` and try again (fail-open).

**Sequential:**
1. Iterate members in `priority` order (lowest first).
2. Try first member. If it fails (after retries), try next.
3. No health state; purely reactive per-request.

**Failover (circuit-breaker):**
1. Always select the highest-priority healthy member.
2. On failure:
   - First failure in window → mark `degraded` (still used).
   - Second consecutive failure → mark `unhealthy`, start cooldown (default 60s).
3. During cooldown, skip to next healthy member.
4. After cooldown → mark `probing`. Route one request as test.
   - Success → mark `healthy`, reset failCount.
   - Failure → extend cooldown (exponential backoff, max 5 min), mark `unhealthy`.

---

## 12. Provider Adapter Architecture

### 12.1 OpenAI-Compatible Base (`providers/openai/adapter.go`)

Most providers use the OpenAI chat completions format. A single `OpenAIAdapter` implementation serves as the base for:
OpenAI, Groq, Together AI, Mistral, Cohere, xAI, DeepSeek, OpenRouter, Alibaba, Zhipu, 01.AI, NVIDIA, ByteDance, Custom, llama.cpp.

Provider-specific variations are handled by configuration overrides in `config_json`:
- `auth_header`: `"Authorization"` (default) or `"x-api-key"`.
- `api_version`: for Azure.
- `extra_headers`: provider-specific headers.

The base adapter:
1. Marshals `llm.LLMRequest` into OpenAI chat completions JSON.
2. Sends HTTP POST to `{baseURL}/v1/chat/completions`.
3. If `stream: true` in request, uses SSE streaming and accumulates chunks.
4. Unmarshals response, normalizes `usage` to inclusive token counts.
5. Returns `*llm.LLMResponse`.

### 12.2 Provider-Specific Adapters

**Anthropic:**
- Custom request format (Messages API with `system` field, `max_tokens` required).
- Auth header: `x-api-key` + `anthropic-version`.
- Response normalization: add `cache_read_input_tokens` + `cache_creation_input_tokens` to `input_tokens`.

**Google Gemini:**
- Custom endpoint pattern: `{baseURL}/v1beta/models/{model}:generateContent`.
- Request format: `contents` array with `role`/`parts`.
- Auth: API key in query string.

**AWS Bedrock:**
- AWS Signature V4 signing on every request.
- Request body varies by underlying model provider (Anthropic, Amazon, etc.).
- Config fields: `region`, `access_key_id`, `secret_access_key` (or use instance profile).

**Azure OpenAI:**
- Base URL pattern: `https://{resource}.openai.azure.com/openai/deployments/{deployment}`.
- API version in query string.
- Auth header: `api-key` instead of `Authorization`.

**Ollama:**
- OpenAI-compatible but base URL typically `http://localhost:11434`.
- `/api/tags` for model listing.
- No API key required.

### 12.3 Adapter Registration

At startup, the main function constructs a `providers.Registry` and registers all adapters:

```go
registry := providers.NewRegistry()
registry.Register(openai.NewAdapter())
registry.Register(anthropic.NewAdapter())
registry.Register(google.NewAdapter())
registry.Register(azure.NewAdapter())
registry.Register(bedrock.NewAdapter())
registry.Register(ollama.NewAdapter())
```

---

## 13. NATS Integration

### 13.1 Consumer Setup

The gateway starts a JetStream consumer on the existing `COGNITIVE` stream:

```go
err := messaging.SubscribeToStreamWithEnvelope(
    ctx, natsClient, "COGNITIVE", "llm-gateway",
    messaging.SubjectLLMRequest.Format("*"),
    g.handleLLMRequest,
)
```

### 13.2 Handler Logic

```go
func (g *Gateway) handleLLMRequest(env *messaging.Envelope, data []byte) error {
    var req llm.LLMRequest
    if err := json.Unmarshal(data, &req); err != nil {
        return fmt.Errorf("unmarshal LLM request: %w", err)
    }

    if req.Metadata == nil {
        req.Metadata = make(map[string]string)
    }
    req.Metadata["agent_id"] = env.AgentID
    req.Metadata["correlation_id"] = env.CorrelationID

    ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
    defer cancel()

    resp, err := g.CallLLM(ctx, req)
    if err != nil {
        resp = &llm.LLMResponse{
            Success: false,
            Error: &llm.LLMError{
                Code:      "gateway_error",
                Message:   err.Error(),
                Retriable: false,
            },
        }
    }

    respBytes, _ := json.Marshal(resp)
    return messaging.ReplyTo(g.natsClient, env.RawMessage, respBytes)
}
```

### 13.3 Usage Event Publishing

After every LLM call, the gateway publishes two fire-and-forget messages:

```go
_ = messaging.Publish(g.natsClient,
    messaging.SubjectUsageToken.Format(agentID),
    correlationID, agentID, cycleID, "gateway", tokenUsagePayload)

_ = messaging.Publish(g.natsClient,
    messaging.SubjectUsageCost.Format(agentID),
    correlationID, agentID, cycleID, "gateway", costPayload)
```

Failures are logged but never propagated (usage events must not fail the LLM call).

---

## 14. Frontend Architecture

### 14.1 Page Structure

| Route | Purpose | Auth |
|---|---|---|
| `/providers` | Provider CRUD, model list, test button, fetch models | Required |
| `/provider-groups` | Group CRUD, member assignment, strategy config | Required |
| `/usage` | Usage/cost dashboard per provider, model, group | Required |

### 14.2 State Management

Providers data is fetched via standard `fetch` in Svelte 5 runes (`$state`). No global store is required because provider configuration is relatively static and page-local.

```svelte
<!-- +page.svelte for /providers -->
<script>
  let providers = $state([]);
  let selectedProvider = $state(null);
  let isLoading = $state(false);

  async function loadProviders() {
    isLoading = true;
    const res = await fetch('/api/providers');
    providers = (await res.json()).data;
    isLoading = false;
  }
</script>
```

### 14.3 Component Hierarchy

```
ProvidersPage
├── ProviderList
│   └── ProviderCard
│       ├── ProviderForm (create/edit modal)
│       ├── ModelList
│       │   └── ModelRow (with capability chips)
│       └── TestButton
│           └── TestResult (inline success/error)

ProviderGroupsPage
├── GroupList
│   └── GroupCard
│       ├── GroupForm (create/edit modal)
│       └── MemberManager
│           ├── MemberList (drag-to-reorder)
│           └── AddMemberDropdown

UsagePage
├── UsageFilters (date range, provider, group)
├── UsageSummaryCards (total cost, total tokens, call count)
└── UsageTable
    └── UsageRow
```

### 14.4 API Client Pattern

Frontend uses a thin typed wrapper around `fetch`:

```ts
export async function listProviders() {
  const res = await fetch('/api/providers');
  const data = await res.json();
  if (!data.success) throw new APIError(data.error);
  return data.data;
}
```

---

## 15. Error Handling Strategy

### 15.1 Normalized Error Codes

All provider errors are mapped to a small set of normalized codes at the adapter layer:

| Code | HTTP Equivalent | Description | Retriable |
|---|---|---|---|
| `rate_limited` | 429 | Provider rate limit hit | Yes |
| `provider_error` | 500/502/503/504 | Provider server error | Yes |
| `timeout` | 504 | Request timeout | Yes |
| `invalid_request` | 400 | Bad request (malformed, invalid model) | No |
| `auth_error` | 401 | Invalid API key or credentials | No |
| `forbidden` | 403 | Quota exceeded or content policy | No |
| `unknown` | — | Uncategorized error | No |

### 15.2 Retry Logic

Retry is governed by `RetryExecutor`:
- **Retriable codes**: `rate_limited`, `provider_error`, `timeout`
- **Base delay**: 1s
- **Cap**: 8s
- **Max retries**: 2 (3 total attempts)
- **Jitter**: full jitter (`random(0, delay)`)

Retry exhaustion triggers group-level failover (for Sequential and Failover strategies).

### 15.3 Gateway Error Response

On total failure (all providers in group exhausted), the gateway returns:

```json
{
  "schema_version": "2024-05-04/v1",
  "success": false,
  "text": "",
  "model": "",
  "provider_id": "",
  "provider_group_id": "group-uuid",
  "usage": { "input_tokens": 0, "output_tokens": 0, "cached_tokens": 0, "cost_usd": 0 },
  "duration_ms": 12500,
  "retry_count": 6,
  "error": {
    "code": "provider_error",
    "message": "All providers in group failed: [openai: timeout, anthropic: rate_limited]",
    "retriable": true
  }
}
```

---

## 16. Testing Strategy

### 16.1 Unit Tests

- **Adapter tests**: Mock HTTP server per provider type. Verify request serialization and response normalization.
- **Crypto tests**: Round-trip encrypt/decrypt with known plaintext. Verify key rotation path.
- **Resolver tests**: Simulate group members, verify counter advancement, health state transitions.
- **Rate limiter tests**: Concurrent checks, boundary conditions, window expiration.

### 16.2 Integration Tests

- **Gateway end-to-end**: Start gateway with mock adapter registry. Publish NATS request, verify response and usage_events row.
- **Handler tests**: Full HTTP round-trip for provider CRUD, test endpoint, model fetch.
- **Encryption integration**: Create provider via API, verify API key is encrypted in DB, verify test call decrypts and uses it.

### 16.3 Frontend Tests

- Component tests for ProviderCard, TestButton, MemberManager.
- API mocking via MSW (Mock Service Worker) for provider endpoints.

---

## 17. Migration & Deployment

### 17.1 Database Migrations

1. `20240511120000_create_providers.go` — Create `providers`, `provider_models`, `provider_groups`, `provider_group_members`.
2. `20240511120100_alter_usage_events_provider.go` — Add `provider_id`, `provider_group_id`, `cached_tokens`, `retry_count` to `usage_events`.

### 17.2 Application Startup Sequence

1. Load `PROVIDER_ENCRYPTION_KEY` from env (required, min 32 bytes hex).
2. Initialize `crypto` package with master key.
3. Build `providers.Registry`, register all adapters.
4. Initialize `llm.Gateway` with registry, rate limiter, NATS client, telemetry.
5. Start NATS consumer `llm-gateway` on `COGNITIVE` stream.
6. Verify at least one provider group exists; if not, await first provider creation.

### 17.3 Default Group Auto-Creation

When the first provider is created via the API:
1. Check if any group exists.
2. If not, create group named `"default"` with `round_robin` strategy.
3. Add the new provider as the sole member with priority 0.
4. Mark the group as `is_default = 1`.

---

## 18. Performance Considerations

1. **Adapter HTTP clients** are reused per-provider type (shared `*http.Client` with connection pooling).
2. **Rate limiter windows** use 5-second granularity to balance accuracy vs memory (12 slots × 60s).
3. **Model capabilities JSON** is loaded once at startup (~50KB), not fetched per request.
4. **Provider list** is cached in Ristretto with 30-second TTL to avoid DB hits on every gateway call.
5. **Streaming internally** keeps peak memory low for large responses; the adapter writes chunks to a `strings.Builder`.

---

## 19. Deferred Enhancements

The following are explicitly out of scope for this unit but the architecture leaves clean extension points:

| Feature | Extension Point | Future Unit |
|---|---|---|
| Multi-replica rate limit sync | Replace `sync.Map` with NATS-broadcast counter deltas | Scaling / Swarm |
| Weighted / latency / cost routing | Add strategy enum values to `GroupResolver` | Providers v2 |
| External streaming (frontend) | Add `stream: true` path in gateway, chunked NATS publish | Streaming UI |
| Semantic caching | Cache layer before `GroupResolver` | Caching |
| Multi-modal (vision/audio) | Extend `ChatMessage` with `Images`/`Audio` fields | Senses |
| Tool calling | Extend `LLMRequest` with `ToolDefinitions`, `LLMResponse` with `ToolCalls` | Tools |
| Billing integration | Consume `ace.usage.*` events in billing service | Billing |

---

**Deliverable:** `design/units/providers/architecture.md`
**Status:** Complete
**Next:** `fsd.md` (Functional Specification)
