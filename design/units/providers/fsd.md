# Providers Unit — Functional Specification

## 1. Scope

This document translates the Providers problem space, research conclusions, and architecture into implementable contracts. It defines every API endpoint, NATS payload, database query, gateway algorithm, and frontend state shape required to build the unit. No new requirements are introduced; all content below is derived from the upstream `problem_space.md`, `research.md`, and `architecture.md`.

**Upstream decisions consumed:**
- Routing strategies: Round-Robin, Sequential, Failover (circuit-breaker). Others deferred.
- Streaming: internal-only (adapters buffer complete responses).
- Retry: exponential backoff + full jitter, base 1s, cap 8s, max 2 retries.
- Token counting: provider-reported only, normalised to inclusive model at adapter layer.
- Encryption: envelope encryption (per-provider DEK wrapped by master KEK).
- Feature detection: static `capabilities.json` + manual user override.

---

## 2. REST API Contract

All routes require authenticated access (existing JWT middleware). Responses use the standard envelope:
- Success: `{ "success": true, "data": <T> }`
- Error: `{ "success": false, "error": { "code": "...", "message": "...", "details": [...] } }`

### 2.1 Providers

#### `GET /api/providers`
List all provider configurations.

**Query params:** none
**Response `data`:** `[]ProviderResponse`

```json
{
  "id": "uuid",
  "name": "OpenAI Production",
  "provider_type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key_masked": "sk-...XXXX",
  "config_json": { "timeout_ms": 60000, "default_model": "gpt-4o" },
  "is_enabled": true,
  "created_at": "2024-05-11T12:00:00Z",
  "updated_at": "2024-05-11T12:00:00Z"
}
```

**Rules:**
- `api_key_masked` is computed by the service layer as `last 4 chars` prefixed by `...`. If key length ≤ 4, mask is `****`.
- `config_json` is provider-specific typed JSON. The API validates it against the provider type's schema on write.

#### `POST /api/providers`
Create a provider.

**Request body:**
```json
{
  "name": "string, required, 1-255 chars",
  "provider_type": "string, required, enum",
  "base_url": "string, required, valid URL",
  "api_key": "string, required unless provider_type=ollama",
  "config_json": { "timeout_ms": 60000, "default_model": "gpt-4o", "extra_headers": {} }
}
```

**Provider type enum:** `openai`, `anthropic`, `google`, `azure`, `bedrock`, `groq`, `together`, `mistral`, `cohere`, `xai`, `deepseek`, `alibaba`, `baidu`, `bytedance`, `zhipu`, `01ai`, `nvidia`, `openrouter`, `ollama`, `llamacpp`, `custom`.

**Validation rules:**
- `name`: non-empty, ≤ 255 chars, unique across all providers.
- `base_url`: must parse as absolute URL with `http` or `https` scheme.
- `api_key`: required for all types except `ollama` and `llamacpp`. For `bedrock`, `config_json` must contain `access_key_id`, `secret_access_key`, and `region` (these are treated as additional secrets and encrypted in the same envelope as `api_key`).
- `config_json.timeout_ms`: optional, defaults per provider type (OpenAI-compatible 60000, Azure/Bedrock 120000, Ollama 300000).
- `config_json.default_model`: optional. If omitted, the first model fetched/created for this provider becomes the implicit default.

**Side effects:**
- Service layer calls `crypto.EncryptField` on `api_key` before repository insert.
- If this is the first provider created in the system, the service auto-creates a default provider group named `"default"` with `round_robin` strategy and adds this provider as the sole member with priority `0`. The group is marked `is_default = 1`.

**Response `data`:** `ProviderResponse` (with `api_key_masked`).

#### `GET /api/providers/{id}`
Get a single provider by ID.

**Response `data`:** `ProviderResponse`.

#### `PUT /api/providers/{id}`
Update a provider.

**Request body:** same as create, but all fields optional (partial update).

**Rules:**
- If `api_key` is provided, it replaces the existing encrypted key (new DEK generated).
- If `api_key` is omitted, the existing key is preserved.
- `provider_type` is immutable after creation.
- `updated_at` is set to `NOW()` by trigger.

#### `DELETE /api/providers/{id}`
Delete a provider.

**Rules:**
- Cascades to `provider_models` and `provider_group_members` via foreign key `ON DELETE CASCADE`.
- Returns `409 Conflict` if the provider is the last member of the default group and no other groups exist. The user must create another group/provider first.

#### `POST /api/providers/{id}/test`
Test a provider configuration.

**Request body:** none (path param only).

**Backend behavior:**
1. Fetch provider row by ID.
2. Decrypt API key via `crypto.DecryptField`.
3. Build `llm.LLMRequest`:
   - `ProviderGroupID`: a transient single-member group containing only this provider.
   - `Messages`: `[{ "role": "user", "content": "Respond with the word 'Working' and nothing else." }]`
   - `Parameters`: `{}`
   - `Stream`: `false`
4. Call the provider's adapter directly (bypass group resolver/rate limiter for the test).
5. Return the model's text response or error.

**Response `data`:**
```json
{
  "success": true,
  "response_text": "Working",
  "model": "gpt-4o",
  "duration_ms": 842
}
```

If the adapter returns an error:
```json
{
  "success": false,
  "response_text": "",
  "model": "",
  "duration_ms": 0,
  "error": {
    "code": "auth_error",
    "message": "401 Unauthorized: invalid API key",
    "retriable": false
  }
}
```

**HTTP status:** `200 OK` in both cases (the HTTP call succeeded; the LLM result is inside the envelope).

#### `POST /api/providers/{id}/fetch-models`
Fetch models from the provider's API.

**Request body:** none.

**Backend behavior:**
1. Fetch provider row, decrypt API key.
2. Call `adapter.FetchModels(ctx, apiKey, baseURL, config)`.
3. For each fetched `ModelInfo`:
   - Look up `capabilities.json` by `model_id` to populate `features_json` and `context_limit`.
   - Look up `capabilities.json` pricing to populate `pricing_json`.
   - Insert or update `provider_models` row (`ON CONFLICT(provider_id, model_id) DO UPDATE`).
   - Set `is_user_edited = 0`.
4. Return the list of fetched models.

**Response `data`:** `[]ProviderModelResponse`

```json
{
  "id": "uuid",
  "provider_id": "uuid",
  "model_id": "gpt-4o",
  "display_name": "GPT-4o",
  "context_limit": 128000,
  "features_json": { "streaming": true, "function_calling": true, "vision": true, "json_mode": true },
  "pricing_json": { "input_token_cost": 5.0, "output_token_cost": 15.0, "currency": "USD" },
  "parameters_json": { "temperature": 0.7, "top_p": 1.0, "max_tokens": 4096 },
  "is_user_edited": false,
  "created_at": "...",
  "updated_at": "..."
}
```

#### `GET /api/providers/{id}/models`
List models for a provider.

**Response `data`:** `[]ProviderModelResponse`.

#### `PUT /api/providers/{id}/models/{modelId}`
Update a model's metadata (manual override).

**Request body (partial):**
```json
{
  "display_name": "string",
  "context_limit": 128000,
  "features_json": { "streaming": true, "vision": false },
  "pricing_json": { "input_token_cost": 0.0, "output_token_cost": 0.0 },
  "parameters_json": { "temperature": 0.5 }
}
```

**Rules:**
- `context_limit` cannot exceed the static catalog's known maximum for this model. If the model is unknown in the catalog, any positive integer is allowed.
- On any field change, `is_user_edited` is set to `1`.

#### `DELETE /api/providers/{id}/models/{modelId}`
Delete a model row. Allowed even if `is_user_edited = 0`. Re-fetching will recreate it.

---

### 2.2 Provider Groups

#### `GET /api/provider-groups`
List all groups with their members.

**Response `data`:** `[]ProviderGroupResponse`

```json
{
  "id": "uuid",
  "name": "default",
  "strategy": "round_robin",
  "config_json": { "rpm": 100, "tpm": 100000 },
  "is_default": true,
  "members": [
    {
      "id": "uuid",
      "provider_id": "uuid",
      "provider_name": "OpenAI Production",
      "priority": 0
    }
  ],
  "created_at": "...",
  "updated_at": "..."
}
```

#### `POST /api/provider-groups`
Create a group.

**Request body:**
```json
{
  "name": "string, required, unique",
  "strategy": "string, required, enum: round_robin|sequential|failover",
  "config_json": { "rpm": 100, "tpm": 100000 }
}
```

**Validation:**
- `name`: non-empty, ≤ 255 chars, unique.
- `strategy`: must be one of the three supported strategies.
- `config_json`: optional. If provided, may contain group-level rate limit overrides (`rpm`, `tpm`). Defaults are `0` (unlimited) at group level unless specified.

#### `GET /api/provider-groups/{id}`
Get a single group.

#### `PUT /api/provider-groups/{id}`
Update group name, strategy, or config.

**Rules:**
- `is_default` cannot be changed via this endpoint. It is managed automatically.

#### `DELETE /api/provider-groups/{id}`
Delete a group.

**Rules:**
- Returns `409 Conflict` if the group is the sole `is_default = 1` group.
- Cascades to `provider_group_members`.

#### `POST /api/provider-groups/{id}/members`
Add a provider to a group.

**Request body:**
```json
{
  "provider_id": "uuid, required",
  "priority": 0
}
```

**Rules:**
- A provider can belong to a group at most once (`UNIQUE(group_id, provider_id)`).
- `priority` determines member order for Sequential and Failover strategies. Lower values are tried first. For Round-Robin, priority is ignored but stored for consistency.

#### `PUT /api/provider-groups/{id}/members/{memberId}`
Update member priority (reordering).

**Request body:** `{ "priority": 1 }`

#### `DELETE /api/provider-groups/{id}/members/{memberId}`
Remove a provider from a group.

---

### 2.3 Usage

#### `GET /api/usage`
Query usage events.

**Query params:**
- `provider_id` (optional, filter)
- `group_id` (optional, filter)
- `model` (optional, filter)
- `start` (optional, RFC3339)
- `end` (optional, RFC3339)
- `limit` (optional, default 100, max 1000)
- `offset` (optional, default 0)

**Response `data`:** `{ "events": []UsageEventResponse, "total": 42 }`

```json
{
  "id": "uuid",
  "provider_id": "uuid",
  "provider_group_id": "uuid",
  "agent_id": "uuid",
  "model": "gpt-4o",
  "input_tokens": 150,
  "output_tokens": 42,
  "cached_tokens": 0,
  "cost_usd": 0.00126,
  "duration_ms": 842,
  "retry_count": 0,
  "created_at": "2024-05-11T12:00:00Z"
}
```

#### `GET /api/usage/summary`
Aggregated usage summary.

**Query params:** same filters as `/api/usage` except `limit`/`offset`.

**Response `data`:**
```json
{
  "total_cost_usd": 12.45,
  "total_input_tokens": 50000,
  "total_output_tokens": 12000,
  "total_calls": 340,
  "by_provider": [
    { "provider_id": "uuid", "provider_name": "OpenAI", "cost_usd": 10.00, "calls": 200 }
  ],
  "by_model": [
    { "model": "gpt-4o", "cost_usd": 10.00, "calls": 200 }
  ],
  "by_group": [
    { "group_id": "uuid", "group_name": "default", "cost_usd": 12.45, "calls": 340 }
  ]
}
```

---

## 3. Database Contract (SQLC)

### 3.1 Schema

Four new tables and one migration. All primary keys are UUID strings generated in Go (`github.com/google/uuid`). Timestamps are `TEXT` in RFC3339.

```sql
-- providers
CREATE TABLE providers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
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

-- provider_models
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

-- provider_groups
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

-- provider_group_members
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

**Migration on `usage_events`:**
```sql
ALTER TABLE usage_events ADD COLUMN provider_id TEXT;
ALTER TABLE usage_events ADD COLUMN provider_group_id TEXT;
ALTER TABLE usage_events ADD COLUMN cached_tokens INTEGER DEFAULT 0;
ALTER TABLE usage_events ADD COLUMN retry_count INTEGER DEFAULT 0;
CREATE INDEX idx_usage_events_provider_id ON usage_events(provider_id);
CREATE INDEX idx_usage_events_provider_group_id ON usage_events(provider_group_id);
```

### 3.2 SQLC Queries (`providers.sql`)

```sql
-- name: CreateProvider :one
INSERT INTO providers (
    id, name, provider_type, base_url, encrypted_api_key, api_key_nonce,
    encrypted_dek, dek_nonce, encryption_version, config_json, is_enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProvider :one
SELECT * FROM providers WHERE id = ?;

-- name: ListProviders :many
SELECT * FROM providers ORDER BY created_at DESC;

-- name: UpdateProvider :one
UPDATE providers
SET name = ?, base_url = ?, encrypted_api_key = ?, api_key_nonce = ?,
    encrypted_dek = ?, dek_nonce = ?, encryption_version = ?, config_json = ?,
    is_enabled = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProvider :exec
DELETE FROM providers WHERE id = ?;

-- name: CountProviders :one
SELECT COUNT(*) FROM providers;

-- name: CreateProviderModel :one
INSERT INTO provider_models (
    id, provider_id, model_id, display_name, context_limit, features_json,
    pricing_json, parameters_json, is_user_edited, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProviderModel :one
SELECT * FROM provider_models WHERE id = ?;

-- name: ListProviderModels :many
SELECT * FROM provider_models WHERE provider_id = ? ORDER BY model_id;

-- name: UpdateProviderModel :one
UPDATE provider_models
SET display_name = ?, context_limit = ?, features_json = ?,
    pricing_json = ?, parameters_json = ?, is_user_edited = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProviderModel :exec
DELETE FROM provider_models WHERE id = ?;

-- name: UpsertProviderModel :one
INSERT INTO provider_models (
    id, provider_id, model_id, display_name, context_limit, features_json,
    pricing_json, parameters_json, is_user_edited, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(provider_id, model_id) DO UPDATE SET
    display_name = excluded.display_name,
    context_limit = excluded.context_limit,
    features_json = excluded.features_json,
    pricing_json = excluded.pricing_json,
    parameters_json = excluded.parameters_json,
    is_user_edited = excluded.is_user_edited,
    updated_at = excluded.updated_at
RETURNING *;

-- name: CreateProviderGroup :one
INSERT INTO provider_groups (
    id, name, strategy, config_json, is_default, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProviderGroup :one
SELECT * FROM provider_groups WHERE id = ?;

-- name: ListProviderGroups :many
SELECT * FROM provider_groups ORDER BY created_at DESC;

-- name: UpdateProviderGroup :one
UPDATE provider_groups
SET name = ?, strategy = ?, config_json = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProviderGroup :exec
DELETE FROM provider_groups WHERE id = ?;

-- name: GetDefaultProviderGroup :one
SELECT * FROM provider_groups WHERE is_default = 1 LIMIT 1;

-- name: SetDefaultProviderGroup :exec
UPDATE provider_groups SET is_default = 0;
UPDATE provider_groups SET is_default = 1 WHERE id = ?;

-- name: CreateGroupMember :one
INSERT INTO provider_group_members (
    id, group_id, provider_id, priority, created_at
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGroupMembers :many
SELECT pgm.*, p.name as provider_name
FROM provider_group_members pgm
JOIN providers p ON pgm.provider_id = p.id
WHERE pgm.group_id = ?
ORDER BY pgm.priority ASC, pgm.created_at ASC;

-- name: UpdateGroupMemberPriority :one
UPDATE provider_group_members
SET priority = ?
WHERE id = ?
RETURNING *;

-- name: DeleteGroupMember :exec
DELETE FROM provider_group_members WHERE id = ?;

-- name: ListUsageEvents :many
SELECT * FROM usage_events
WHERE (COALESCE(?1, '') = '' OR provider_id = ?1)
  AND (COALESCE(?2, '') = '' OR provider_group_id = ?2)
  AND (COALESCE(?3, '') = '' OR model = ?3)
  AND (COALESCE(?4, '') = '' OR created_at >= ?4)
  AND (COALESCE(?5, '') = '' OR created_at <= ?5)
ORDER BY created_at DESC
LIMIT ?6 OFFSET ?7;

-- name: CountUsageEvents :one
SELECT COUNT(*) FROM usage_events
WHERE (COALESCE(?1, '') = '' OR provider_id = ?1)
  AND (COALESCE(?2, '') = '' OR provider_group_id = ?2)
  AND (COALESCE(?3, '') = '' OR model = ?3)
  AND (COALESCE(?4, '') = '' OR created_at >= ?4)
  AND (COALESCE(?5, '') = '' OR created_at <= ?5);

-- name: GetUsageSummary :many
SELECT
    provider_id,
    model,
    provider_group_id,
    SUM(input_tokens) as total_input_tokens,
    SUM(output_tokens) as total_output_tokens,
    SUM(cost_usd) as total_cost_usd,
    COUNT(*) as total_calls
FROM usage_events
WHERE (COALESCE(?1, '') = '' OR provider_id = ?1)
  AND (COALESCE(?2, '') = '' OR provider_group_id = ?2)
  AND (COALESCE(?3, '') = '' OR model = ?3)
  AND (COALESCE(?4, '') = '' OR created_at >= ?4)
  AND (COALESCE(?5, '') = '' OR created_at <= ?5)
GROUP BY provider_id, model, provider_group_id;
```

---

## 4. NATS Message Contract

### 4.1 LLM Request

**Subject:** `ace.llm.{agentId}.request`
**Stream:** `COGNITIVE`
**Consumer:** `llm-gateway`

Payload (JSON):
```go
package llm

type LLMRequest struct {
    SchemaVersion   string            `json:"schema_version"`   // "2024-05-04/v1"
    ProviderGroupID string            `json:"provider_group_id"` // UUID
    ModelOverride   *string           `json:"model_override,omitempty"`
    SystemPrompt    string            `json:"system_prompt"`
    Messages        []ChatMessage     `json:"messages"`
    Parameters      LLMParameters     `json:"parameters"`
    Stream          bool              `json:"stream"` // always false in this unit
    Metadata        map[string]string `json:"metadata,omitempty"`
}

type ChatMessage struct {
    Role    Role   `json:"role"`    // "system", "user", "assistant", "tool"
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
```

**Envelope header rules:**
- `agent_id` is mandatory in NATS envelope.
- `correlation_id` is mandatory.
- `cycle_id` is optional for non-engine consumers but should be present.

### 4.2 LLM Response

**Subject:** `ace.llm.{agentId}.response` (reply subject)
**Delivery:** `messaging.ReplyTo`

Payload (JSON):
```go
type LLMResponse struct {
    SchemaVersion   string     `json:"schema_version"`
    Success         bool       `json:"success"`
    Text            string     `json:"text,omitempty"`
    Model           string     `json:"model"`
    ProviderID      string     `json:"provider_id"`
    ProviderGroupID string     `json:"provider_group_id"`
    Usage           TokenUsage `json:"usage,omitempty"`
    DurationMs      int64      `json:"duration_ms"`
    RetryCount      int        `json:"retry_count"`
    Error           *LLMError  `json:"error,omitempty"`
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
```

**Error codes:**
- `rate_limited` — RPM or TPM limit exceeded at provider, group, or user level.
- `provider_error` — Provider returned 500/502/503/504 or equivalent.
- `timeout` — Request exceeded provider timeout.
- `invalid_request` — Malformed request, invalid model, or parameter out of range.
- `auth_error` — Invalid API key or credentials.
- `forbidden` — Quota exceeded or content policy violation.
- `unknown` — Uncategorized failure.

### 4.3 Usage Events

**Subjects (fire-and-forget):**
- `ace.usage.{agentId}.token`
- `ace.usage.{agentId}.cost`

Payload for `token`:
```json
{
  "input_tokens": 150,
  "output_tokens": 42,
  "cached_tokens": 0,
  "cost_usd": 0.00126
}
```

Payload for `cost`:
```json
{
  "cost_usd": 0.00126,
  "provider_id": "uuid",
  "model": "gpt-4o"
}
```

**Rule:** Publishing failures are logged but never propagated to the caller.

---

## 5. Gateway Algorithmic Specification

### 5.1 CallLLM

```
function CallLLM(ctx, req):
  1. Validate req.ProviderGroupID is non-empty.
  2. Fetch group members from DB (cached for 30s in Ristretto).
     If group has no members → return error{code: "invalid_request", message: "group empty"}
  3. Fetch group strategy from DB.
  4. resolver := GroupResolver(strategy)
  5. candidates := ordered member list from resolver
  6. For each candidate in candidates:
     a. provider := fetch provider by candidate.provider_id
     b. If provider.is_enabled == false → continue
     c. model := resolve model (req.ModelOverride ?? provider.config_json.default_model ?? first model)
     d. rateLimitErr := RateLimiter.Check(provider.id, group.id, userID, estimatedTokens=0, limits)
        If rateLimitErr != nil:
           If strategy == sequential || failover → continue to next candidate
           If strategy == round_robin → return LLMResponse{Error: {code: "rate_limited"}}
     e. adapter := registry.Get(provider.provider_type)
     f. apiKey := crypto.DecryptField(provider.encrypted_field, masterKey)
     g. config := unmarshal provider.config_json into AdapterConfig
     h. resp, err := RetryExecutor.Execute(ctx, func() {
            return adapter.Call(ctx, req, apiKey, provider.base_url, config)
        })
     i. If err == nil:
         - RateLimiter.Record(provider.id, group.id, userID, resp.Usage.InputTokens + resp.Usage.OutputTokens)
         - cost := resp.Usage.InputTokens * model.pricing.input_cost_per_1m / 1e6 +
                   resp.Usage.OutputTokens * model.pricing.output_cost_per_1m / 1e6
         - resp.Usage.CostUSD = cost
         - resp.ProviderID = provider.id
         - resp.ProviderGroupID = group.id
         - telemetry.Usage.LLMCall(ctx, agentID, cycleID, sessionID, "gateway", tokens, cost, duration)
         - DB: INSERT usage_events (...)
         - NATS: Publish token + cost events
         - Return resp
     j. If err != nil:
         - normalizedErr := adapter.NormalizeError(err)
         - resolver.RecordFailure(candidate.provider_id, normalizedErr)
         - If strategy == sequential || failover → continue to next candidate
         - If strategy == round_robin → return LLMResponse{Error: normalizedErr}
  7. All candidates exhausted:
     - Return LLMResponse{
         Success: false,
         Error: {code: "provider_error", message: "All providers in group failed", Retriable: true}
       }
```

### 5.2 GroupResolver

**State:**
- `roundRobinCounters: sync.Map<string, *atomic.Uint64>`
- `healthStates: sync.Map<string, *HealthState>`

**HealthState machine:**
```
healthy --(failure #1)--> degraded --(failure #2 within window)--> unhealthy
unhealthy --(cooldown expires)--> probing --(success)--> healthy
probing --(failure)--> unhealthy (extend cooldown exponentially, max 5min)
```

**Round-Robin:**
1. Atomically increment `counter[group.id]`.
2. `idx = counter % len(members)`.
3. Starting at `idx`, scan members circularly.
4. Skip members with `healthState.status == unhealthy`.
5. If all skipped, fail-open (reset all to `healthy`, try again from `idx`).

**Sequential:**
1. Return members ordered by `priority ASC, created_at ASC`.
2. No health state consulted.

**Failover:**
1. Return members ordered by `priority ASC`. The first member with `healthState.status != unhealthy` is primary; the rest are fallbacks.
2. On adapter failure: `RecordFailure` advances the health state machine.
3. On retry exhaustion: the gateway tries the next healthy member.

### 5.3 RateLimiter

**Data structure:**
- `windows: sync.Map<string, *Window>`
- Key format: `"{scope}:{id}:{kind}"` where `scope ∈ {provider,group,user}`, `kind ∈ {requests,tokens}`.

**Window (sliding):**
- 12 slots, 5-second granularity.
- `lastUpdate: time.Time`
- `slots: [12]int64`

**Algorithm `Check(scope, id, kind, delta, limit)`:**
1. If `limit <= 0` → allow (unlimited).
2. `window := getOrCreateWindow(key)`.
3. `now := time.Now().UTC()`
4. `elapsedSlots := int(now.Sub(window.lastUpdate).Seconds()) / 5`
5. If `elapsedSlots >= 12` → zero all slots, `window.lastUpdate = now`.
6. Else → shift slots left by `elapsedSlots`, zero new slots, `window.lastUpdate = now`.
7. `current := sum(window.slots)`
8. If `current + delta > limit` → reject (`rate_limited`).
9. Else → allow (do NOT add yet; `Record` adds).

**Algorithm `Record(scope, id, kind, delta)`:**
1. `window := getWindow(key)` (must exist from Check).
2. `window.slots[11] += delta`.

**Cleanup:** Background goroutine iterates `windows` every 2 minutes and deletes keys with `lastUpdate < now - 2 minutes`.

### 5.4 RetryExecutor

```
function Execute(ctx, operation):
  attempts = 0
  maxAttempts = 3
  baseDelay = 1s
  capDelay = 8s

  for attempts < maxAttempts:
    resp, err := operation()
    if err == nil → return resp, nil

    normErr := normalize(err)
    if !normErr.Retriable → return nil, err

    attempts++
    if attempts >= maxAttempts → break

    delay := min(capDelay, baseDelay * (1 << (attempts-1)))
    jitter := rand.Int63n(int64(delay))
    sleep(ctx, time.Duration(jitter))

  return nil, err
```

**Retriable codes:** `rate_limited`, `provider_error`, `timeout`.
**Non-retriable codes:** `invalid_request`, `auth_error`, `forbidden`, `unknown`.

---

## 6. Crypto Specification

### 6.1 Key Requirements

- **Master KEK:** 32-byte array derived from hex-encoded `PROVIDER_ENCRYPTION_KEY` env var. Validated at startup: must decode to exactly 32 bytes. Fatal error if missing or invalid.
- **Per-provider DEK:** Random 32-byte AES-256 key generated during `EncryptField`.
- **Nonces:** 12-byte random nonce for each AES-GCM operation (two per provider: one for API key, one for DEK).

### 6.2 EncryptField

```
function EncryptField(plaintext string, masterKey []byte) EncryptedField:
  dek := randomBytes(32)
  dekNonce := randomBytes(12)
  apiKeyNonce := randomBytes(12)

  dekCipher := aesgcm.Seal(nil, dekNonce, dek, nil)  // encrypt DEK with masterKey
  apiKeyCipher := aesgcm.Seal(nil, apiKeyNonce, []byte(plaintext), nil)  // encrypt plaintext with DEK

  return EncryptedField{
    Ciphertext: apiKeyCipher,
    Nonce: apiKeyNonce,
    EncryptedDEK: dekCipher,
    DEKNonce: dekNonce,
    EncryptionVersion: 1,
  }
```

### 6.3 DecryptField

```
function DecryptField(field EncryptedField, masterKey []byte) string:
  dek := aesgcm.Open(nil, field.DEKNonce, field.EncryptedDEK, nil)
  plaintext := aesgcm.Open(nil, field.Nonce, field.Ciphertext, nil)
  return string(plaintext)
```

### 6.4 Key Rotation Path

1. Introduce new master key as `PROVIDER_ENCRYPTION_KEY_V2`.
2. Migration job iterates all providers:
   - Decrypt DEK with old master key (selected by `encryption_version`).
   - Re-encrypt DEK with new master key.
   - Update `encrypted_dek`, `dek_nonce`, `encryption_version = 2`.
3. Read path uses `encryption_version` to select the correct KEK.

---

## 7. Frontend Functional Specification

### 7.1 Routes & Layout

| Route | Page Component | Layout |
|---|---|---|
| `/providers` | `ProvidersPage` | `(app)` layout with sidebar |
| `/provider-groups` | `ProviderGroupsPage` | `(app)` layout with sidebar |
| `/usage` | `UsagePage` | `(app)` layout with sidebar |

**Navigation:** Add "Providers", "Groups", "Usage" links to the existing admin sidebar under a new "LLM" section.

### 7.2 State Shapes (Svelte 5 Runes)

**`ProvidersPage`**
```ts
let providers = $state<ProviderResponse[]>([]);
let selectedProvider = $state<ProviderResponse | null>(null);
let models = $state<ProviderModelResponse[]>([]);
let isLoading = $state(false);
let isModelLoading = $state(false);
let testResult = $state<{ success: boolean; text: string; error?: APIError } | null>(null);
let showForm = $state(false);
let editingProvider = $state<ProviderResponse | null>(null);
```

**`ProviderGroupsPage`**
```ts
let groups = $state<ProviderGroupResponse[]>([]);
let providers = $state<ProviderResponse[]>([]); // for dropdown
let selectedGroup = $state<ProviderGroupResponse | null>(null);
let isLoading = $state(false);
let showForm = $state(false);
let editingGroup = $state<ProviderGroupResponse | null>(null);
let draggedMember = $state<MemberResponse | null>(null);
```

**`UsagePage`**
```ts
let filters = $state({
  provider_id: '',
  group_id: '',
  model: '',
  start: '',
  end: ''
});
let events = $state<UsageEventResponse[]>([]);
let summary = $state<UsageSummaryResponse | null>(null);
let isLoading = $state(false);
let total = $state(0);
let offset = $state(0);
let limit = $state(100);
```

### 7.3 Component Contracts

**`ProviderCard`**
- Props: `provider: ProviderResponse`
- Events: `onEdit`, `onDelete`, `onTest`, `onFetchModels`
- Displays: name, type badge, base URL, masked key, enabled toggle, model count.

**`ProviderForm`** (modal)
- Props: `provider?: ProviderResponse` (undefined = create mode)
- State: form object with validation errors.
- Validation:
  - `name`: required, max 255.
  - `provider_type`: required, select from enum.
  - `base_url`: required, must be valid URL.
  - `api_key`: required unless type is `ollama`/`llamacpp`. On edit, empty means "keep existing".
- Actions: `onSubmit`, `onCancel`.
- Dynamic fields: When `provider_type` changes, show/hide `config_json` sub-fields (e.g., `region` for Bedrock).

**`ModelList`**
- Props: `models: ProviderModelResponse[]`, `providerId: string`
- Displays: table with model_id, display_name, context_limit, capability chips (streaming, vision, etc.), pricing.
- Actions: inline edit for display_name, context_limit, pricing; delete row.

**`TestButton`**
- Props: `providerId: string`
- State: `testing = $state(false)`.
- On click: `POST /api/providers/{id}/test`. Show inline spinner.
- Result: green check + response text, or red alert with error code and message.

**`GroupCard`**
- Props: `group: ProviderGroupResponse`
- Displays: name, strategy badge, member count, default indicator.
- Events: `onEdit`, `onDelete`.

**`GroupForm`** (modal)
- Props: `group?: ProviderGroupResponse`
- Validation: `name` required unique; `strategy` required.
- Actions: `onSubmit`, `onCancel`.

**`MemberManager`**
- Props: `groupId: string`, `members: MemberResponse[]`, `availableProviders: ProviderResponse[]`
- Displays: sortable list of members with provider name and priority.
- Actions: drag-to-reorder (updates priority via `PUT`), remove member, add member dropdown.

**`UsageTable`**
- Props: `events: UsageEventResponse[]`
- Columns: timestamp, provider, model, input tokens, output tokens, cached tokens, cost, duration.

**`UsageSummaryCards`**
- Props: `summary: UsageSummaryResponse`
- Displays: total cost, total tokens, total calls. Optionally small bar chart by provider.

### 7.4 API Client Functions

```ts
// api/providers.ts
export async function listProviders(): Promise<ProviderResponse[]>;
export async function createProvider(req: ProviderCreateRequest): Promise<ProviderResponse>;
export async function updateProvider(id: string, req: Partial<ProviderCreateRequest>): Promise<ProviderResponse>;
export async function deleteProvider(id: string): Promise<void>;
export async function testProvider(id: string): Promise<TestProviderResponse>;
export async function fetchModels(id: string): Promise<ProviderModelResponse[]>;
export async function listModels(providerId: string): Promise<ProviderModelResponse[]>;
export async function updateModel(providerId: string, modelId: string, req: Partial<ProviderModelResponse>): Promise<ProviderModelResponse>;
export async function deleteModel(providerId: string, modelId: string): Promise<void>;

// api/provider-groups.ts
export async function listGroups(): Promise<ProviderGroupResponse[]>;
export async function createGroup(req: ProviderGroupCreateRequest): Promise<ProviderGroupResponse>;
export async function updateGroup(id: string, req: Partial<ProviderGroupCreateRequest>): Promise<ProviderGroupResponse>;
export async function deleteGroup(id: string): Promise<void>;
export async function addMember(groupId: string, req: { provider_id: string; priority: number }): Promise<MemberResponse>;
export async function updateMemberPriority(groupId: string, memberId: string, priority: number): Promise<MemberResponse>;
export async function removeMember(groupId: string, memberId: string): Promise<void>;

// api/usage.ts
export async function listUsage(params: UsageQueryParams): Promise<{ events: UsageEventResponse[]; total: number }>;
export async function getUsageSummary(params: UsageQueryParams): Promise<UsageSummaryResponse>;
```

All functions throw `APIError` on `success: false` responses.

---

## 8. Error Handling Matrix

### 8.1 Provider HTTP → Normalized Code Mapping

| Provider Status / Error | Normalized Code | Retriable | Notes |
|---|---|---|---|
| 429 | `rate_limited` | Yes | Check `Retry-After` if present; else use jitter backoff. |
| 500, 502, 503, 504 | `provider_error` | Yes | |
| Timeout / context deadline | `timeout` | Yes | |
| 400 | `invalid_request` | No | Includes invalid model, bad parameters. |
| 401 | `auth_error` | No | Test button exposes this immediately. |
| 403 | `forbidden` | No | Quota or content policy. |
| DNS / connection refused | `provider_error` | Yes | |
| Unknown / parse error | `unknown` | No | |

### 8.2 API Error Responses

| Scenario | HTTP Status | Error Code | Message |
|---|---|---|---|
| Validation failure | 400 | `validation_error` | Field-level details array. |
| Duplicate provider name | 409 | `duplicate_name` | "Provider name already exists." |
| Provider not found | 404 | `not_found` | "Provider not found." |
| Group not found | 404 | `not_found` | "Group not found." |
| Cannot delete default group | 409 | `default_group_protected` | "Cannot delete the default group." |
| Cannot delete last provider in default group | 409 | `last_provider_protected` | "Create another group first." |
| Encryption key missing | 500 | `config_error` | "Provider encryption key not configured." |
| LLM gateway internal error | 500 | `gateway_error` | Generic; log details server-side. |

---

## 9. Validation Rules Summary

| Entity | Field | Rule |
|---|---|---|
| Provider | `name` | Required, 1-255 chars, unique. |
| Provider | `provider_type` | Required, enum of 21 values. Immutable. |
| Provider | `base_url` | Required, absolute URL, http/https. |
| Provider | `api_key` | Required except `ollama`/`llamacpp`. |
| Provider | `config_json.timeout_ms` | Optional, positive integer. |
| ProviderModel | `context_limit` | Positive integer; cannot exceed static catalog max if known. |
| ProviderModel | `pricing_json.input_token_cost` | Non-negative float. |
| ProviderGroup | `name` | Required, 1-255 chars, unique. |
| ProviderGroup | `strategy` | Required, enum: `round_robin`, `sequential`, `failover`. |
| GroupMember | `provider_id` | Required, must exist, unique per group. |
| GroupMember | `priority` | Integer, default 0. Lower = higher priority. |
| LLMRequest | `ProviderGroupID` | Required, valid UUID. |
| LLMRequest | `Messages` | Non-empty array. Each `Role` must be valid enum. |
| LLMRequest | `Parameters.temperature` | If set, 0.0–2.0. |
| LLMRequest | `Parameters.top_p` | If set, 0.0–1.0. |
| LLMRequest | `Parameters.max_tokens` | If set, positive integer. |

---

## 10. Traceability Matrix

Maps each functional requirement from `problem_space.md` to the concrete contract(s) in this document.

| FR | Requirement | API / DB / Gateway / Frontend Contract |
|---|---|---|
| FR-1 | Provider CRUD | `POST/GET/PUT/DELETE /api/providers`; `providers.sql` CRUD queries; `ProvidersPage` |
| FR-2 | Encrypted API keys | `crypto.EncryptField/DecryptField`; `providers` table BLOB columns; never returned in API |
| FR-3 | 21 provider types | `provider_type` enum validation; `AdapterConfig` schema; adapter packages |
| FR-4 | Provider adapters | `LLMAdapter` interface; `registry.Get`; per-type adapter packages |
| FR-5 | Provider groups | `POST/GET/PUT/DELETE /api/provider-groups`; `provider_groups` table |
| FR-6 | Routing strategies | `GroupResolver`; `strategy` enum; Round-Robin / Sequential / Failover algorithms |
| FR-7 | Provider test | `POST /api/providers/{id}/test`; `TestButton` component |
| FR-8 | LLM Gateway NATS | `Gateway.CallLLM`; `ace.llm.{id}.request` consumer; `LLMRequest/LLMResponse` |
| FR-9 | Three-level rate limits | `RateLimiter.Check/Record`; per-provider/group/user sliding windows |
| FR-10 | Sliding window counters | `RateLimiter` 12-slot/5s window algorithm |
| FR-11 | Token usage capture | `TokenUsage` struct; `usage_events` columns; adapter normalization rules |
| FR-12 | Cost calculation | Gateway step 5.i cost formula; `pricing_json` input/output per 1M tokens |
| FR-13 | Usage NATS events | `messaging.Publish` to `ace.usage.{id}.token` and `ace.usage.{id}.cost` |
| FR-14 | Fetch models | `POST /api/providers/{id}/fetch-models`; `adapter.FetchModels` |
| FR-15 | Manual model entry | `PUT /api/providers/{id}/models/{modelId}`; `ModelList` inline edit |
| FR-16 | Model parameters | `parameters_json` schema; `LLMParameters` struct |
| FR-17 | Context limit override | `context_limit` validation (max static catalog); UI allows tune-down |
| FR-18 | Pricing per model | `pricing_json` schema; `GetUsageSummary` aggregation |
| FR-19 | Frontend provider management | `/providers` route; `ProvidersPage`, `ProviderCard`, `ProviderForm` |
| FR-20 | Frontend group management | `/provider-groups` route; `ProviderGroupsPage`, `MemberManager` |
| FR-21 | Usage/cost dashboard | `/usage` route; `UsagePage`, `UsageTable`, `UsageSummaryCards` |
| FR-22 | Default group auto-create | `ProviderService.CreateProvider` side effect; `SetDefaultProviderGroup` query |
| FR-23 | Error handling / retry | `RetryExecutor`; normalized error codes; group failover logic |
| FR-24 | Tracing per LLM call | `telemetry.Usage.LLMCall`; OTel span attributes in gateway |

---

**Deliverable:** `design/units/providers/fsd.md`
**Vertical Status:** 0/24 slices planned (pending `implementation_plan.md`)
**Files Affected:**
- `/home/jay/programming/ace_prototype/design/units/providers/fsd.md`
