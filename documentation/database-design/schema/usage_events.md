# usage_events

Schema: `public`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | uuid | gen_random_uuid() | NO |
| `timestamp` | timestamp with time zone | - | NO |
| `agent_id` | uuid | - | NO |
| `cycle_id` | uuid | - | NO |
| `session_id` | uuid | - | NO |
| `service_name` | character varying | - | NO |
| `operation_type` | character varying | - | NO |
| `resource_type` | character varying | - | NO |
| `cost_usd` | numeric | - | YES |
| `duration_ms` | bigint | - | YES |
| `token_count` | bigint | - | YES |
| `metadata` | jsonb | - | YES |
| `created_at` | timestamp with time zone | now() | YES |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `usage_events_agent_id_not_null` | CHECK | `NOT NULL agent_id` |
| `usage_events_cycle_id_not_null` | CHECK | `NOT NULL cycle_id` |
| `usage_events_id_not_null` | CHECK | `NOT NULL id` |
| `usage_events_operation_type_not_null` | CHECK | `NOT NULL operation_type` |
| `usage_events_pkey` | PRIMARY KEY | `PRIMARY KEY (id)` |
| `usage_events_resource_type_not_null` | CHECK | `NOT NULL resource_type` |
| `usage_events_service_name_not_null` | CHECK | `NOT NULL service_name` |
| `usage_events_session_id_not_null` | CHECK | `NOT NULL session_id` |
| `usage_events_timestamp_not_null` | CHECK | `NOT NULL "timestamp"` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_usage_events_agent_id` | No | `CREATE INDEX idx_usage_events_agent_id ON public.usage_events USING btree (agent_id)` |
| `idx_usage_events_cycle_id` | No | `CREATE INDEX idx_usage_events_cycle_id ON public.usage_events USING btree (cycle_id)` |
| `idx_usage_events_operation_type` | No | `CREATE INDEX idx_usage_events_operation_type ON public.usage_events USING btree (operation_type)` |
| `idx_usage_events_service_name` | No | `CREATE INDEX idx_usage_events_service_name ON public.usage_events USING btree (service_name)` |
| `idx_usage_events_session_id` | No | `CREATE INDEX idx_usage_events_session_id ON public.usage_events USING btree (session_id)` |
| `idx_usage_events_timestamp` | No | `CREATE INDEX idx_usage_events_timestamp ON public.usage_events USING btree ("timestamp" DESC)` |

