-- name: CreateProvider :one
-- Creates a new provider configuration.
INSERT INTO providers (
    id, name, provider_type, base_url, encrypted_api_key, api_key_nonce,
    encrypted_dek, dek_nonce, encryption_version, config_json, is_enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProvider :one
-- Gets a single provider by ID.
SELECT * FROM providers WHERE id = ?;

-- name: ListProviders :many
-- Lists all providers ordered by creation time (newest first).
SELECT * FROM providers ORDER BY created_at DESC;

-- name: UpdateProvider :one
-- Updates a provider configuration.
UPDATE providers
SET name = ?, base_url = ?, encrypted_api_key = ?, api_key_nonce = ?,
    encrypted_dek = ?, dek_nonce = ?, encryption_version = ?, config_json = ?,
    is_enabled = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProvider :exec
-- Deletes a provider by ID.
DELETE FROM providers WHERE id = ?;

-- name: CountProviders :one
-- Counts all providers.
SELECT COUNT(*) AS count FROM providers;

-- name: CreateProviderModel :one
-- Creates a new provider model entry.
INSERT INTO provider_models (
    id, provider_id, model_id, display_name, context_limit, features_json,
    pricing_json, parameters_json, is_user_edited, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProviderModel :one
-- Gets a single provider model by ID.
SELECT * FROM provider_models WHERE id = ?;

-- name: ListProviderModels :many
-- Lists all models for a given provider, ordered by model_id.
SELECT * FROM provider_models WHERE provider_id = ? ORDER BY model_id;

-- name: UpdateProviderModel :one
-- Updates a provider model's metadata.
UPDATE provider_models
SET display_name = ?, context_limit = ?, features_json = ?,
    pricing_json = ?, parameters_json = ?, is_user_edited = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProviderModel :exec
-- Deletes a provider model by ID.
DELETE FROM provider_models WHERE id = ?;

-- name: UpsertProviderModel :one
-- Inserts or updates a provider model (on conflict by provider_id + model_id).
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
-- Creates a new provider group.
INSERT INTO provider_groups (
    id, name, strategy, config_json, is_default, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProviderGroup :one
-- Gets a single provider group by ID.
SELECT * FROM provider_groups WHERE id = ?;

-- name: ListProviderGroups :many
-- Lists all provider groups ordered by creation time (newest first).
SELECT * FROM provider_groups ORDER BY created_at DESC;

-- name: UpdateProviderGroup :one
-- Updates a provider group's metadata (not is_default).
UPDATE provider_groups
SET name = ?, strategy = ?, config_json = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProviderGroup :exec
-- Deletes a provider group by ID.
DELETE FROM provider_groups WHERE id = ?;

-- name: GetDefaultProviderGroup :one
-- Gets the provider group marked as default.
SELECT * FROM provider_groups WHERE is_default = 1 LIMIT 1;

-- name: ClearDefaultProviderGroup :exec
-- Clears the is_default flag from all provider groups.
UPDATE provider_groups SET is_default = 0;

-- name: SetDefaultProviderGroup :exec
-- Sets a specific provider group as the default.
UPDATE provider_groups SET is_default = 1 WHERE id = ?;

-- name: CreateGroupMember :one
-- Adds a provider to a group.
INSERT INTO provider_group_members (
    id, group_id, provider_id, priority, created_at
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGroupMembers :many
-- Lists all members of a group with their provider name (JOIN).
SELECT pgm.*, p.name as provider_name
FROM provider_group_members pgm
JOIN providers p ON pgm.provider_id = p.id
WHERE pgm.group_id = ?
ORDER BY pgm.priority ASC, pgm.created_at ASC;

-- name: UpdateGroupMemberPriority :one
-- Updates a group member's priority.
UPDATE provider_group_members
SET priority = ?
WHERE id = ?
RETURNING *;

-- name: DeleteGroupMember :exec
-- Removes a provider from a group.
DELETE FROM provider_group_members WHERE id = ?;

-- name: ListUsageEventsWithProvider :many
-- Lists usage events with optional provider/group/model/time filters and pagination.
SELECT * FROM usage_events
WHERE (? = '' OR provider_id = ?)
  AND (? = '' OR provider_group_id = ?)
  AND (? = '' OR model = ?)
  AND (? = '' OR created_at >= ?)
  AND (? = '' OR created_at <= ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountUsageEventsWithProvider :one
-- Counts usage events matching provider-aware filters.
SELECT COUNT(*) AS count FROM usage_events
WHERE (? = '' OR provider_id = ?)
  AND (? = '' OR provider_group_id = ?)
  AND (? = '' OR model = ?)
  AND (? = '' OR created_at >= ?)
  AND (? = '' OR created_at <= ?);

-- name: GetUsageEventsSummary :many
-- Aggregates usage events grouped by provider, model, and group.
SELECT
    provider_id,
    model,
    provider_group_id,
    SUM(input_tokens) as total_input_tokens,
    SUM(output_tokens) as total_output_tokens,
    SUM(cost_usd) as total_cost_usd,
    COUNT(*) as total_calls
FROM usage_events
WHERE (? = '' OR provider_id = ?)
  AND (? = '' OR provider_group_id = ?)
  AND (? = '' OR model = ?)
  AND (? = '' OR created_at >= ?)
  AND (? = '' OR created_at <= ?)
GROUP BY provider_id, model, provider_group_id;
