# v0.9 Bifrost spike — `github.com/maximhq/bifrost/core` v1.13.1

Spike branch `ace-bifrost-spike`, measured 2026-10-25.

## Gate: ADOPTED

Build clean + live streaming call returns provider-reported usage.

## Measurements

| metric | baseline (ace-do @545d8d2c) | with bifrost |
|---|---|---|
| `go list -m all \| wc -l` | 33 | **162** (+129: AWS SDK, Azure SDK, fasthttp, sonic, mcp-go, starlark…) |
| `go` directive | 1.26 | **1.27.3** (host has go1.27.3 — spec's "host has 1.27.1" note is stale; no toolchain download needed) |
| `cmd/ace` binary | 19,686,420 B | 19,686,665 B (unused — adapter import links it) → **49,689,791 B (49.7 MB)** once the adapter is actually wired (measured 2026-10-26, `go list -m all`=162) |
| provider test binary | — | 41,908,377 B vs 11,874,295 B plain (watchdog test bin) ⇒ **≈ +30 MB linked** |
| `go build ./... && go test ./...` | — | clean, all packages pass |

## Live call

`ACE_BIFROST_SPIKE=1 go test -tags spike ./internal/provider/ -run TestBifrostSpike`:

- `bifrost.Init` with a minimal `Account` (key from `ACE_LLM_API_KEY` via
  `.env`) → `ChatCompletionStreamRequest` on provider `opencode-go`, model
  `deepseek-v4.1-flash`.
- `text="ok"`, `usage={PromptTokens:36, CompletionTokens:9, TotalTokens:45,
  Cost:0x…}` — **provider-reported prompt, cached-detail, completion and cost
  fields all populated**.
- Notes found while wiring:
  - `x-opencode-session` header is required (`MissingSessionID` 400 without
    it) → `NetworkConfig.ExtraHeaders`.
  - `-free` suffixed ids are not in the paid-key catalogue — model ids must
    come from the catalogue/config, not be guessed.
- Bifrost carries a **native `opencode-go` provider** plus
  anthropic/azure/bedrock/gemini/groq/deepseek/mistral/openai/openrouter/etc.
  inside `core@v1.13.1/providers/` — one import gives the whole adapter
  surface; `openai_compat.go` stays only as the thin fallback for custom
  OpenAI-compatible base_urls.

## Cost accepted

+129 modules and ≈+30 MB binary for: per-provider key pools, request
normalization across 25 providers, native retry/failure classification, and a
maintained catalogue. Reported per spec (size and module growth reported, not
gated). If the footprint ever becomes a problem the seam is `internal/provider`
— the registry contract (`core.LLMProvider`) is unchanged.
