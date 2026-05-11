# Providers Unit — Implementation Plan

## Overview

This plan breaks the Providers unit into 24 atomic vertical slices, each implementable and testable in under 15 minutes. Slices are ordered by dependency: foundation (schema, crypto, types) first, then provider configuration backend and frontend, then gateway core infrastructure, then usage dashboard, and finally integration polish.

Each slice follows the project's established patterns:
- **Backend:** Goose Go migrations → SQLC queries → Repository → Service → Handler
- **Frontend:** SvelteKit route page → API client wrapper → reusable components
- **Tests:** Go unit tests for backend logic; component-level manual verification for frontend

---

## Slice 1: Database Schema & SQLC Queries

- **Backend:**
  - Goose migration `20260321120000_create_providers_tables.go` creating `providers`, `provider_models`, `provider_groups`, `provider_group_members` with indexes and triggers.
  - Goose migration `20260321120100_alter_usage_events_provider.go` adding `provider_id`, `provider_group_id`, `cached_tokens`, `retry_count` to `usage_events`.
  - Update `backend/internal/api/repository/schema.sql` with new table definitions for sqlc inference.
  - Create `backend/internal/api/repository/queries/providers.sql` with all CRUD, upsert, list, and aggregation queries per the FSD.
  - Run `sqlc generate` to produce generated code.
- **Frontend:** N/A
- **Test:** `make test` passes; `sqlite3` CLI confirms all 4 new tables + altered `usage_events` exist; SQLC generation produces no errors.

---

## Slice 2: Envelope Encryption Package

- **Backend:**
  - Create `backend/internal/crypto/field_encryption.go` with `EncryptedField` struct, `EncryptField(plaintext, masterKey)`, and `DecryptField(field, masterKey)` using AES-256-GCM.
  - Add `ProviderEncryptionKey` to `backend/internal/app/config.go` (`AuthConfig` or top-level), loaded from env `ACE_PROVIDER_ENCRYPTION_KEY` / config file `provider_encryption_key`.
  - Validate key is hex-decoded to exactly 32 bytes in `validateConfig`.
- **Frontend:** N/A
- **Test:** Unit tests for round-trip encrypt/decrypt, tamper detection, and key rotation path (version selection).

---

## Slice 3: LLM Normalized Types

- **Backend:**
  - Create `backend/internal/llm/types.go` with `Role`, `ChatMessage`, `LLMParameters`, `LLMRequest`, `LLMResponse`, `TokenUsage`, `LLMError` structs exactly as specified in the FSD.
  - Add JSON marshaling/unmarshaling validation (no `interface{}`).
- **Frontend:** N/A
- **Test:** Unit tests for JSON round-trip, validation of `Role` enum, and `LLMParameters` bounds (temperature 0.0–2.0, top_p 0.0–1.0, max_tokens positive).

---

## Slice 4: Provider CRUD Backend

- **Backend:**
  - Create `backend/internal/api/model/provider.go` with request/response DTOs (`ProviderResponse`, `ProviderCreateRequest`, etc.).
  - Create `backend/internal/api/service/provider_service.go` with `CreateProvider`, `GetProvider`, `ListProviders`, `UpdateProvider`, `DeleteProvider`.
  - Service layer encrypts `api_key` via `crypto.EncryptField` on create/update; returns `api_key_masked` on reads.
  - Create `backend/internal/api/handler/provider_handler.go` with HTTP handlers mapped to `POST/GET/PUT/DELETE /api/providers`.
  - Wire handlers into the Chi router.
- **Frontend:** N/A
- **Test:** HTTP handler integration tests: create provider, verify API key is encrypted in DB (not plaintext), verify list returns masked key, verify delete cascades to models.

---

## Slice 5: Provider List Frontend

- **Backend:** Consumes endpoints from Slice 4.
- **Frontend:**
  - Create `frontend/src/routes/(app)/providers/+page.svelte` with `$state` for `providers`, `isLoading`.
  - Create `frontend/src/lib/components/providers/ProviderCard.svelte` displaying name, type badge, base URL, masked key, enabled toggle, model count.
  - Create `frontend/src/lib/api/providers.ts` with `listProviders()` typed wrapper.
  - Add route to admin sidebar under new "LLM" section.
- **Test:** Manual verification: providers list loads, cards render correctly, masked key shows `...XXXX` format.

---

## Slice 6: Provider Form (Create/Edit)

- **Backend:** Consumes endpoints from Slice 4.
- **Frontend:**
  - Create `frontend/src/lib/components/providers/ProviderForm.svelte` modal with fields: name, provider_type select (21 options), base_url, api_key (required except ollama/llamacpp), dynamic `config_json` sub-fields.
  - Validation: name required/max 255, base_url valid http/https, api_key required per type.
  - Integrate into `+page.svelte` with `onEdit` and create button.
- **Test:** Manual verification: create provider succeeds, edit provider preserves existing key when api_key left blank, validation errors show inline.

---

## Slice 7: Provider Test Backend

- **Backend:**
  - Extend `provider_handler.go` with `POST /api/providers/{id}/test`.
  - Extend `provider_service.go` with `TestProvider(ctx, id)`:
    - Fetch provider, decrypt API key.
    - Build transient single-member group and `llm.LLMRequest` with test prompt.
    - Call adapter directly (bypass gateway/rate limiter).
    - Return response text, model, duration, or normalized error.
- **Frontend:** N/A
- **Test:** Handler integration test with mock adapter registry: success path returns `response_text`, failure path returns `error` with normalized code.

---

## Slice 8: Provider Test Frontend

- **Backend:** Consumes endpoint from Slice 7.
- **Frontend:**
  - Create `frontend/src/lib/components/providers/TestButton.svelte`.
  - On click: `POST /api/providers/{id}/test`, show inline spinner.
  - Display green check + response text on success, red alert with error code/message on failure.
- **Test:** Manual verification: test button triggers call, success/failure states render correctly.

---

## Slice 9: Model Fetch & CRUD Backend

- **Backend:**
  - Extend `provider_handler.go` with `POST /api/providers/{id}/fetch-models`, `GET /api/providers/{id}/models`, `PUT /api/providers/{id}/models/{modelId}`, `DELETE /api/providers/{id}/models/{modelId}`.
  - Extend `provider_service.go` with `FetchModels`, `ListModels`, `UpdateModel`, `DeleteModel`.
  - `FetchModels`: decrypt key, call `adapter.FetchModels`, cross-reference `capabilities.json`, upsert into `provider_models` with `is_user_edited = 0`.
  - `UpdateModel`: partial update, set `is_user_edited = 1`; validate `context_limit` against static catalog max.
- **Frontend:** N/A
- **Test:** Handler integration tests for fetch-models (mock adapter returning 2 models, verify DB upsert), model update (context_limit validation), model delete.

---

## Slice 10: Model Management Frontend

- **Backend:** Consumes endpoints from Slice 9.
- **Frontend:**
  - Create `frontend/src/lib/components/providers/ModelList.svelte` table showing model_id, display_name, context_limit, capability chips (streaming, vision, etc.), pricing.
  - Inline edit for display_name, context_limit, pricing fields.
  - Delete row action.
  - Integrate into ProviderCard or ProvidersPage.
- **Test:** Manual verification: model list renders, inline edit updates row, delete removes row, fetch-models button populates list.

---

## Slice 11: Provider Groups CRUD Backend

- **Backend:**
  - Extend `provider_handler.go` with `POST/GET/PUT/DELETE /api/provider-groups`.
  - Extend `provider_service.go` with group CRUD.
  - `ListProviderGroups` eagerly loads members via `ListGroupMembers` join query.
  - Validation: `name` unique, `strategy` enum `round_robin|sequential|failover`.
- **Frontend:** N/A
- **Test:** Handler integration tests for create, list with members, update strategy, delete (reject if default group).

---

## Slice 12: Provider Groups List Frontend

- **Backend:** Consumes endpoints from Slice 11.
- **Frontend:**
  - Create `frontend/src/routes/(app)/provider-groups/+page.svelte` with groups list.
  - Create `frontend/src/lib/components/providers/GroupCard.svelte` displaying name, strategy badge, member count, default indicator.
  - Create `frontend/src/lib/api/provider-groups.ts` with typed wrappers.
- **Test:** Manual verification: groups list loads, cards show correct strategy badge, default group is marked.

---

## Slice 13: Group Members Backend

- **Backend:**
  - Extend `provider_handler.go` with `POST /api/provider-groups/{id}/members`, `PUT /api/provider-groups/{id}/members/{memberId}`, `DELETE /api/provider-groups/{id}/members/{memberId}`.
  - Extend `provider_service.go` with member add/update priority/remove.
  - Validation: provider not already in group (`UNIQUE(group_id, provider_id)`), priority integer.
- **Frontend:** N/A
- **Test:** Handler integration tests for add member, reorder priority, remove member.

---

## Slice 14: Member Manager Frontend

- **Backend:** Consumes endpoints from Slices 11 and 13.
- **Frontend:**
  - Create `frontend/src/lib/components/providers/MemberManager.svelte`.
  - Sortable list of members with provider name and priority.
  - Drag-to-reorder (updates priority via `PUT`).
  - Remove member button.
  - Add member dropdown (available providers not in group).
- **Test:** Manual verification: reorder sends correct priority updates, add/remove reflect immediately in UI.

---

## Slice 15: OpenAI-Compatible Base Adapter

- **Backend:**
  - Create `backend/internal/llm/providers/adapter.go` with `LLMAdapter` interface, `AdapterConfig`, `ModelInfo`, `Registry`.
  - Create `backend/internal/llm/providers/openai/adapter.go` implementing `Call` and `FetchModels` for OpenAI-compatible endpoints.
  - Handle auth header variations (Bearer vs api-key), SSE streaming accumulation into `strings.Builder`, usage normalization to inclusive model.
  - Register base adapter in registry factory.
- **Frontend:** N/A
- **Test:** Mock HTTP server tests: verify request serialization matches OpenAI chat completions schema, verify response normalization produces correct `TokenUsage`, verify streaming accumulation works.

---

## Slice 16: Specialty Adapters

- **Backend:**
  - Create `backend/internal/llm/providers/anthropic/adapter.go` (Messages API, `x-api-key`, additive usage normalization).
  - Create `backend/internal/llm/providers/google/adapter.go` (Gemini generateContent, API key in query).
  - Create `backend/internal/llm/providers/azure/adapter.go` (Azure OpenAI base URL pattern, `api-key` header).
  - Create `backend/internal/llm/providers/bedrock/adapter.go` (AWS Signature V4, InvokeModel).
  - Create `backend/internal/llm/providers/ollama/adapter.go` (OpenAI-compatible, `/api/tags` for models, no auth).
  - Register all in registry factory.
- **Frontend:** N/A
- **Test:** Mock HTTP server tests for each adapter: request format matches provider API, response normalization correct, `FetchModels` returns expected `ModelInfo` list.

---

## Slice 17: Retry Executor

- **Backend:**
  - Create `backend/internal/llm/retry.go` with `RetryExecutor`.
  - Exponential backoff with full jitter: `delay = min(8s, 1s * 2^attempt) + random(0, delay)`.
  - Max 2 retries (3 attempts total).
  - Retriable codes: `rate_limited`, `provider_error`, `timeout`.
  - Non-retriable: `invalid_request`, `auth_error`, `forbidden`, `unknown`.
- **Frontend:** N/A
- **Test:** Unit tests: success on first attempt, retry on retriable error, no retry on non-retriable, jitter distribution non-deterministic, max attempts respected.

---

## Slice 18: Sliding-Window Rate Limiter

- **Backend:**
  - Create `backend/internal/llm/rate_limiter.go` with `RateLimiter`, `WindowCounter`.
  - 12-slot circular buffer, 5-second granularity, per-key `sync.Map` storage.
  - Three-level checking: `provider:{id}`, `group:{id}`, `user:{id}` for both `requests` and `tokens`.
  - `Check` validates limit without mutating; `Record` adds delta to current slot.
  - Background cleanup goroutine evicts stale windows (>2 min inactivity).
- **Frontend:** N/A
- **Test:** Unit tests: allow under limit, reject over limit, window slides correctly after 60s, concurrent checks don't race, cleanup removes stale keys.

---

## Slice 19: Group Resolver

- **Backend:**
  - Create `backend/internal/llm/resolver.go` with `GroupResolver`, `HealthState`, `HealthStatus`.
  - **Round-Robin:** atomic counter modulo N, skip `unhealthy`, fail-open reset.
  - **Sequential:** priority-ordered list, no health state, reactive failover.
  - **Failover:** circuit-breaker state machine (`healthy → degraded → unhealthy → probing → healthy`), cooldown 60s with exponential extension up to 5 min.
- **Frontend:** N/A
- **Test:** Unit tests: RR counter advances correctly, Sequential tries next on failure, Failover marks degraded then unhealthy, probing recovery succeeds/fails correctly.

---

## Slice 20: LLM Gateway Core (CallLLM)

- **Backend:**
  - Create `backend/internal/llm/gateway.go` with `Gateway` struct and `CallLLM(ctx, req)`.
  - Integrate resolver (select provider+model), rate limiter (`Check` before, `Record` after), retry executor, adapter registry.
  - Cost calculation: `input_tokens * price_per_1m / 1e6 + output_tokens * price_per_1m / 1e6`.
  - DB insert into `usage_events`, telemetry `Usage.LLMCall`, NATS publish token+cost events.
  - Handle total failure (all candidates exhausted) with normalized error response.
- **Frontend:** N/A
- **Test:** Integration test with mock adapter registry and in-memory DB: successful call produces usage_events row, rate limit rejection returns `rate_limited`, total failure returns `provider_error`.

---

## Slice 21: NATS Consumer & Usage Publishing

- **Backend:**
  - Extend `Gateway` with `StartNATSConsumer(ctx)` subscribing to `ace.llm.{agentId}.request` via JetStream consumer on `COGNITIVE` stream.
  - Handler unmarshals payload to `llm.LLMRequest`, injects `agent_id` from envelope, calls `CallLLM`, replies via `messaging.ReplyTo`.
  - Fire-and-forget publishes to `ace.usage.{agentId}.token` and `ace.usage.{agentId}.cost`; failures logged, never propagated.
- **Frontend:** N/A
- **Test:** End-to-end test: publish NATS request, verify response on reply subject, verify usage_events row created, verify usage events published on NATS.

---

## Slice 22: Usage Query API

- **Backend:**
  - Extend `provider_handler.go` with `GET /api/usage` and `GET /api/usage/summary`.
  - Query params: `provider_id`, `group_id`, `model`, `start`, `end`, `limit`, `offset`.
  - `/api/usage` returns `{ events: []UsageEventResponse, total: number }`.
  - `/api/usage/summary` returns aggregated totals plus breakdowns by provider, model, group.
- **Frontend:** N/A
- **Test:** Handler integration tests with seeded usage_events: filter by provider_id returns correct subset, summary aggregates cost and tokens accurately.

---

## Slice 23: Usage Dashboard Frontend

- **Backend:** Consumes endpoints from Slice 22.
- **Frontend:**
  - Create `frontend/src/routes/(app)/usage/+page.svelte` with filters (date range, provider, group, model).
  - Create `frontend/src/lib/components/providers/UsageSummaryCards.svelte` showing total cost, total tokens, total calls.
  - Create `frontend/src/lib/components/providers/UsageTable.svelte` with columns: timestamp, provider, model, input tokens, output tokens, cached tokens, cost, duration.
  - Pagination (limit/offset controls).
  - Create `frontend/src/lib/api/usage.ts` with typed wrappers.
- **Test:** Manual verification: usage events load in table, summary cards update on filter change, pagination works.

---

## Slice 24: Default Group Auto-Create, Navigation, E2E Validation

- **Backend:**
  - Extend `ProviderService.CreateProvider`: if this is the first provider, auto-create group named `"default"` with `round_robin` strategy, add provider as sole member with priority 0, mark `is_default = 1`.
  - Prevent deletion of last provider in default group (409 Conflict).
- **Frontend:**
  - Add "Providers", "Groups", "Usage" links to admin sidebar under "LLM" section.
  - Ensure all three routes are guarded by auth.
- **Test:**
  - Integration test: create first provider → verify default group exists with that member.
  - Integration test: attempt to delete last provider in default group → verify 409.
  - End-to-end: full flow — create provider, test it, fetch models, create group, assign member, make LLM call via NATS, verify usage appears in dashboard.

---

## Dependency Graph

```
Slice 1 (Schema) ──────────────────────────────────────────────────────────────┐
       │                                                                         │
Slice 2 (Crypto) ──► Slice 4 (Provider CRUD Backend) ──► Slice 5-6 (Frontend)  │
       │                        │                        │                       │
       │                        ▼                        ▼                       │
       │                 Slice 7 (Test Backend) ──► Slice 8 (Test Frontend)      │
       │                        │                                                │
       │                        ▼                                                │
       │                 Slice 9 (Model Backend) ──► Slice 10 (Model Frontend)   │
       │                                                                         │
Slice 3 (LLM Types) ──► Slice 15 (OpenAI Adapter) ──► Slice 16 (Specialty)      │
       │                        │                                                │
       │                        ▼                                                │
       │                 Slice 17 (Retry) ──► Slice 18 (Rate Limiter)            │
       │                        │                        │                       │
       │                        ▼                        ▼                       │
       │                 Slice 19 (Resolver) ◄────────────┘                      │
       │                        │                                                │
       │                        ▼                                                │
       │                 Slice 20 (Gateway Core) ◄───────────────────────────────┤
       │                        │                                                │
       │                        ▼                                                │
       │                 Slice 21 (NATS Consumer)                                │
       │                                                                         │
Slice 11 (Groups Backend) ──► Slice 12-14 (Groups Frontend)                     │
       │                                                                         │
       └─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                           Slice 22 (Usage Backend) ──► Slice 23 (Usage Frontend)
                                    │
                                    ▼
                           Slice 24 (Integration)
```

---

## FR Traceability

| FR | Requirement | Covered By |
|---|---|---|
| FR-1 | Provider CRUD | Slice 4, 5, 6 |
| FR-2 | Encrypted API keys | Slice 2, 4 |
| FR-3 | 21 provider types | Slice 15, 16 |
| FR-4 | Provider adapters | Slice 15, 16 |
| FR-5 | Provider groups | Slice 11, 12, 13, 14 |
| FR-6 | Routing strategies | Slice 19 |
| FR-7 | Provider test | Slice 7, 8 |
| FR-8 | LLM Gateway NATS | Slice 20, 21 |
| FR-9 | Three-level rate limits | Slice 18 |
| FR-10 | Sliding window counters | Slice 18 |
| FR-11 | Token usage capture | Slice 20 |
| FR-12 | Cost calculation | Slice 20 |
| FR-13 | Usage NATS events | Slice 21 |
| FR-14 | Fetch models | Slice 9 |
| FR-15 | Manual model entry | Slice 9, 10 |
| FR-16 | Model parameters | Slice 9, 10 |
| FR-17 | Context limit override | Slice 9, 10 |
| FR-18 | Pricing per model | Slice 9, 10 |
| FR-19 | Frontend provider management | Slice 5, 6, 8, 10 |
| FR-20 | Frontend group management | Slice 12, 14 |
| FR-21 | Usage/cost dashboard | Slice 22, 23 |
| FR-22 | Default group auto-create | Slice 24 |
| FR-23 | Error handling / retry | Slice 17, 20 |
| FR-24 | Tracing per LLM call | Slice 20, 21 |

---

**Deliverable:** `design/units/providers/implementation_plan.md`
**Vertical Status:** 24/24 Slices Planned
**Files Affected:**
- `/home/jay/programming/ace_prototype/design/units/providers/implementation_plan.md`
- `/home/jay/programming/ace_prototype/backend/internal/api/repository/migrations/20260321120000_create_providers_tables.go`
- `/home/jay/programming/ace_prototype/backend/internal/api/repository/migrations/20260321120100_alter_usage_events_provider.go`
- `/home/jay/programming/ace_prototype/backend/internal/api/repository/schema.sql`
- `/home/jay/programming/ace_prototype/backend/internal/api/repository/queries/providers.sql`
- `/home/jay/programming/ace_prototype/backend/internal/crypto/field_encryption.go`
- `/home/jay/programming/ace_prototype/backend/internal/crypto/field_encryption_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/app/config.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/types.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/types_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/api/model/provider.go`
- `/home/jay/programming/ace_prototype/backend/internal/api/service/provider_service.go`
- `/home/jay/programming/ace_prototype/backend/internal/api/handler/provider_handler.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/capabilities.json`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/openai/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/openai/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/anthropic/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/anthropic/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/google/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/google/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/azure/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/azure/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/bedrock/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/bedrock/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/ollama/adapter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/providers/ollama/adapter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/retry.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/retry_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/rate_limiter.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/rate_limiter_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/resolver.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/resolver_test.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/gateway.go`
- `/home/jay/programming/ace_prototype/backend/internal/llm/gateway_test.go`
- `/home/jay/programming/ace_prototype/frontend/src/routes/(app)/providers/+page.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/routes/(app)/provider-groups/+page.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/routes/(app)/usage/+page.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/ProviderCard.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/ProviderForm.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/TestButton.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/ModelList.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/GroupCard.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/GroupForm.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/MemberManager.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/UsageTable.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/components/providers/UsageSummaryCards.svelte`
- `/home/jay/programming/ace_prototype/frontend/src/lib/api/providers.ts`
- `/home/jay/programming/ace_prototype/frontend/src/lib/api/provider-groups.ts`
- `/home/jay/programming/ace_prototype/frontend/src/lib/api/usage.ts`
