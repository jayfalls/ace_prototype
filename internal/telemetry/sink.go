package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ace/internal/core"
	"github.com/google/uuid"
)

type Sink struct {
	db             *sql.DB
	mu             sync.RWMutex
	nextID         atomic.Uint64
	lastSnapshotNs atomic.Int64
	subs           map[uint64]func(core.LiveEvent)
}

func NewSink(db *sql.DB) (*Sink, error) {
	s := &Sink{
		db:   db,
		subs: make(map[uint64]func(core.LiveEvent)),
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Sink) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS thought_traces (
		id TEXT PRIMARY KEY,
		cycle_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		layer_id INTEGER NOT NULL,
		layer_code TEXT NOT NULL,
		loop_kind TEXT NOT NULL,
		model TEXT NOT NULL,
		trigger_summary TEXT NOT NULL,
		system_prompt TEXT NOT NULL,
		user_prompt TEXT NOT NULL,
		raw_reasoning TEXT NOT NULL,
		raw_response TEXT NOT NULL,
		thought TEXT NOT NULL,
		state_summary TEXT NOT NULL,
		southbound TEXT NOT NULL,
		northbound TEXT NOT NULL,
		action_type TEXT NOT NULL,
		action_target TEXT NOT NULL,
		action_result TEXT NOT NULL,
		confidence REAL NOT NULL,
		latency_ms INTEGER NOT NULL,
		prompt_tokens INTEGER NOT NULL,
		completion_tokens INTEGER NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_thought_traces_created ON thought_traces(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_thought_traces_layer ON thought_traces(layer_id, created_at DESC);

	CREATE TABLE IF NOT EXISTS telemetry_spans (
		span_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		parent_span_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		layer_code TEXT NOT NULL,
		operation TEXT NOT NULL,
		model TEXT NOT NULL,
		status TEXT NOT NULL,
		duration_ms INTEGER NOT NULL,
		prompt_tokens INTEGER NOT NULL,
		completion_tokens INTEGER NOT NULL,
		detail TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_telemetry_spans_created ON telemetry_spans(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_telemetry_spans_trace ON telemetry_spans(trace_id, created_at ASC);

	CREATE TABLE IF NOT EXISTS telemetry_timeseries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts_unix INTEGER NOT NULL,
		thought_count INTEGER NOT NULL,
		span_count INTEGER NOT NULL,
		prompt_tokens INTEGER NOT NULL,
		cached_tokens INTEGER NOT NULL,
		completion_tokens INTEGER NOT NULL,
		cache_hit_pct REAL NOT NULL,
		l6_avg_latency_ms INTEGER NOT NULL,
		l5_gate_latency_ms INTEGER NOT NULL,
		mode3_pass_count INTEGER NOT NULL,
		mode3_veto_count INTEGER NOT NULL,
		ltl_veto_count INTEGER NOT NULL,
		active_model TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_telemetry_timeseries_ts ON telemetry_timeseries(ts_unix DESC);
	`
	_, err := s.db.Exec(schema)
	_, _ = s.db.Exec(`ALTER TABLE thought_traces ADD COLUMN cached_tokens INTEGER NOT NULL DEFAULT 0`)
	return err
}

func cleanUTF8String(v string) string {
	if v == "" {
		return ""
	}
	v = strings.ToValidUTF8(v, "")
	if strings.IndexByte(v, 0) >= 0 {
		v = strings.ReplaceAll(v, "\x00", "")
	}
	return v
}

func (s *Sink) RecordThought(ctx context.Context, t core.ThoughtTrace) error {
	if t.ID == "" {
		t.ID = "tht_" + uuid.NewString()[:12]
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.TriggerSummary = cleanUTF8String(t.TriggerSummary)
	t.SystemPrompt = cleanUTF8String(t.SystemPrompt)
	t.UserPrompt = cleanUTF8String(t.UserPrompt)
	t.RawReasoning = cleanUTF8String(t.RawReasoning)
	t.RawResponse = cleanUTF8String(t.RawResponse)
	t.Thought = cleanUTF8String(t.Thought)
	t.StateSummary = cleanUTF8String(t.StateSummary)
	t.Southbound = cleanUTF8String(t.Southbound)
	t.Northbound = cleanUTF8String(t.Northbound)
	t.ActionTarget = cleanUTF8String(t.ActionTarget)
	t.ActionResult = cleanUTF8String(t.ActionResult)
	// Estimate token counts (~4 chars/token) when upstream free provider omits usage metadata
	if t.PromptTokens == 0 && (len(t.SystemPrompt)+len(t.UserPrompt)) > 0 {
		t.PromptTokens = (len(t.SystemPrompt) + len(t.UserPrompt)) / 4
	}
	if t.CompletionTokens == 0 && (len(t.RawReasoning)+len(t.RawResponse)) > 0 {
		t.CompletionTokens = (len(t.RawReasoning) + len(t.RawResponse)) / 4
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO thought_traces (
			id, cycle_id, agent_id, layer_id, layer_code, loop_kind, model,
			trigger_summary, system_prompt, user_prompt, raw_reasoning, raw_response,
			thought, state_summary, southbound, northbound, action_type, action_target,
			action_result, confidence, latency_ms, prompt_tokens, cached_tokens, completion_tokens, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.ID, t.CycleID, t.AgentID, t.LayerID, t.LayerCode, t.LoopKind, t.Model,
		t.TriggerSummary, t.SystemPrompt, t.UserPrompt, t.RawReasoning, t.RawResponse,
		t.Thought, t.StateSummary, t.Southbound, t.Northbound, t.ActionType, t.ActionTarget,
		t.ActionResult, t.Confidence, t.LatencyMs, t.PromptTokens, t.CachedTokens, t.CompletionTokens, t.CreatedAt,
	)
	if err != nil {
		return err
	}

	s.EmitLive("thought.recorded", t)
	s.maybeRecordTimeSeriesSnapshot(ctx, t.Model)
	return nil
}

func (s *Sink) RecentThoughts(ctx context.Context, limit int, layerFilter int) ([]core.ThoughtTrace, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}

	query := `
		SELECT id, cycle_id, agent_id, layer_id, layer_code, loop_kind, model,
		       trigger_summary, system_prompt, user_prompt, raw_reasoning, raw_response,
		       thought, state_summary, southbound, northbound, action_type, action_target,
		       action_result, confidence, latency_ms, prompt_tokens, cached_tokens, completion_tokens, created_at
		FROM thought_traces
	`
	var args []interface{}
	if layerFilter > 0 {
		query += ` WHERE layer_id = ?`
		args = append(args, layerFilter)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.ThoughtTrace
	for rows.Next() {
		var t core.ThoughtTrace
		if err := rows.Scan(
			&t.ID, &t.CycleID, &t.AgentID, &t.LayerID, &t.LayerCode, &t.LoopKind, &t.Model,
			&t.TriggerSummary, &t.SystemPrompt, &t.UserPrompt, &t.RawReasoning, &t.RawResponse,
			&t.Thought, &t.StateSummary, &t.Southbound, &t.Northbound, &t.ActionType, &t.ActionTarget,
			&t.ActionResult, &t.Confidence, &t.LatencyMs, &t.PromptTokens, &t.CachedTokens, &t.CompletionTokens, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Sink) GetThought(ctx context.Context, id string) (*core.ThoughtTrace, error) {
	var t core.ThoughtTrace
	err := s.db.QueryRowContext(ctx, `
		SELECT id, cycle_id, agent_id, layer_id, layer_code, loop_kind, model,
		       trigger_summary, system_prompt, user_prompt, raw_reasoning, raw_response,
		       thought, state_summary, southbound, northbound, action_type, action_target,
		       action_result, confidence, latency_ms, prompt_tokens, cached_tokens, completion_tokens, created_at
		FROM thought_traces WHERE id = ?
	`, id).Scan(
		&t.ID, &t.CycleID, &t.AgentID, &t.LayerID, &t.LayerCode, &t.LoopKind, &t.Model,
		&t.TriggerSummary, &t.SystemPrompt, &t.UserPrompt, &t.RawReasoning, &t.RawResponse,
		&t.Thought, &t.StateSummary, &t.Southbound, &t.Northbound, &t.ActionType, &t.ActionTarget,
		&t.ActionResult, &t.Confidence, &t.LatencyMs, &t.PromptTokens, &t.CachedTokens, &t.CompletionTokens, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Sink) RecordSpan(ctx context.Context, sp core.TelemetrySpan) error {
	if sp.SpanID == "" {
		sp.SpanID = "spn_" + uuid.NewString()[:12]
	}
	if sp.AgentID == "" {
		sp.AgentID = "ace-primary"
	}
	if sp.Status == "" {
		sp.Status = "ok"
	}
	if sp.CreatedAt.IsZero() {
		sp.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telemetry_spans (
			span_id, trace_id, parent_span_id, agent_id, layer_code, operation,
			model, status, duration_ms, prompt_tokens, completion_tokens, detail, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sp.SpanID, sp.TraceID, sp.ParentSpanID, sp.AgentID, sp.LayerCode, sp.Operation,
		sp.Model, sp.Status, sp.DurationMs, sp.PromptTokens, sp.CompletionTokens, sp.Detail, sp.CreatedAt,
	)
	if err == nil {
		s.EmitLive("telemetry.span", sp)
	}
	return err
}

func (s *Sink) RecentSpans(ctx context.Context, limit int, traceID string) ([]core.TelemetrySpan, error) {
	if limit <= 0 || limit > 250 {
		limit = 80
	}
	query := `
		SELECT span_id, trace_id, parent_span_id, agent_id, layer_code, operation,
		       model, status, duration_ms, prompt_tokens, completion_tokens, detail, created_at
		FROM telemetry_spans
	`
	var args []interface{}
	if traceID != "" {
		query += ` WHERE trace_id = ?`
		args = append(args, traceID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.TelemetrySpan
	for rows.Next() {
		var sp core.TelemetrySpan
		if err := rows.Scan(
			&sp.SpanID, &sp.TraceID, &sp.ParentSpanID, &sp.AgentID, &sp.LayerCode, &sp.Operation,
			&sp.Model, &sp.Status, &sp.DurationMs, &sp.PromptTokens, &sp.CompletionTokens, &sp.Detail, &sp.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Sink) Metrics(ctx context.Context) core.TelemetryMetrics {
	m := core.TelemetryMetrics{
		AvgLayerLatencyMs: make(map[string]int64),
		SpansByOperation:  make(map[string]int64),
	}

	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN prompt_tokens > 0 THEN prompt_tokens ELSE (LENGTH(system_prompt)+LENGTH(user_prompt))/4 END), 0),
		       COALESCE(SUM(cached_tokens), 0),
		       COALESCE(SUM(CASE WHEN completion_tokens > 0 THEN completion_tokens ELSE (LENGTH(raw_reasoning)+LENGTH(raw_response))/4 END), 0)
		FROM thought_traces
	`).Scan(&m.TotalSpans, &m.TotalPromptTokens, &m.TotalCachedTokens, &m.TotalCompletionToks)

	// Compute rolling layer latency over the most recent 25 invocations per layer
	// so architectural upgrades (like the L5 4-Bit GPU Gate and 1-pass L4) are immediately reflected.
	for _, code := range []string{"L1", "L2", "L3", "L4", "L5", "L6", "CHAT"} {
		var avg int64
		if err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(CAST(AVG(latency_ms) AS INTEGER), 0)
			FROM (SELECT latency_ms FROM thought_traces WHERE layer_code = ? ORDER BY rowid DESC LIMIT 25)
		`, code).Scan(&avg); err == nil && avg > 0 {
			m.AvgLayerLatencyMs[code] = avg
		}
	}
	// Override L5 latency with the live 4-Bit GPU Gate spans if present
	var l5GpuAvg int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(CAST(AVG(duration_ms) AS INTEGER), 0)
		FROM (SELECT duration_ms FROM telemetry_spans WHERE layer_code = 'L5' AND operation LIKE 'gate.%' ORDER BY rowid DESC LIMIT 25)
	`).Scan(&l5GpuAvg); err == nil && l5GpuAvg > 0 {
		m.AvgLayerLatencyMs["L5"] = l5GpuAvg
	}

	opRows, err := s.db.QueryContext(ctx, `
		SELECT operation, status, COUNT(*)
		FROM telemetry_spans
		GROUP BY operation, status
	`)
	if err == nil {
		var spanCount int64
		for opRows.Next() {
			var op, status string
			var cnt int64
			if err := opRows.Scan(&op, &status, &cnt); err == nil {
				spanCount += cnt
				m.SpansByOperation[op] += cnt
				if op == "tool.exec" {
					m.TotalToolCalls += cnt
				}
				if op == "safety.check" {
					if status == "ok" {
						m.SafetyChecksPassed += cnt
					}
					if status == "blocked" {
						m.SafetyInterventions += cnt
					}
				}
			}
		}
		opRows.Close()
		if spanCount > 0 {
			m.TotalSpans = spanCount
		}
	}

	return m
}

func (s *Sink) maybeRecordTimeSeriesSnapshot(ctx context.Context, activeModel string) {
	nowNs := time.Now().UnixNano()
	prev := s.lastSnapshotNs.Load()
	if nowNs-prev < int64(5*time.Second) {
		return
	}
	if !s.lastSnapshotNs.CompareAndSwap(prev, nowNs) {
		return
	}
	var thoughtCount, promptTok, cachedTok, compTok int64
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(prompt_tokens), 0),
		       COALESCE(SUM(cached_tokens), 0),
		       COALESCE(SUM(completion_tokens), 0)
		FROM thought_traces
	`).Scan(&thoughtCount, &promptTok, &cachedTok, &compTok)

	var spanCount, mode3Pass, mode3Veto, ltlVeto int64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_spans`).Scan(&spanCount)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_spans WHERE operation = 'gate.mode3' AND status = 'ok'`).Scan(&mode3Pass)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_spans WHERE operation = 'gate.mode3' AND status = 'veto'`).Scan(&mode3Veto)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_spans WHERE operation = 'gate.ltl' AND status = 'veto'`).Scan(&ltlVeto)

	var l6Lat, l5Lat int64
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(CAST(AVG(latency_ms) AS INTEGER), 0)
		FROM (SELECT latency_ms FROM thought_traces WHERE layer_code = 'L6' ORDER BY rowid DESC LIMIT 25)
	`).Scan(&l6Lat)
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(CAST(AVG(duration_ms) AS INTEGER), 0)
		FROM (SELECT duration_ms FROM telemetry_spans WHERE layer_code = 'L5' AND operation LIKE 'gate.%' ORDER BY rowid DESC LIMIT 25)
	`).Scan(&l5Lat)

	hitPct := 0.0
	if promptTok > 0 {
		hitPct = (float64(cachedTok) * 100.0) / float64(promptTok)
	}
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO telemetry_timeseries (
			ts_unix, thought_count, span_count, prompt_tokens, cached_tokens, completion_tokens,
			cache_hit_pct, l6_avg_latency_ms, l5_gate_latency_ms, mode3_pass_count, mode3_veto_count,
			ltl_veto_count, active_model, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		time.Now().UTC().Unix(), thoughtCount, spanCount, promptTok, cachedTok, compTok,
		hitPct, l6Lat, l5Lat, mode3Pass, mode3Veto, ltlVeto, activeModel, time.Now().UTC(),
	)
}

func (s *Sink) RecentTimeSeries(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 500 {
		limit = 120
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts_unix, thought_count, span_count, prompt_tokens, cached_tokens, completion_tokens,
		       cache_hit_pct, l6_avg_latency_ms, l5_gate_latency_ms, mode3_pass_count, mode3_veto_count,
		       ltl_veto_count, active_model
		FROM telemetry_timeseries
		ORDER BY ts_unix DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var ts, tc, sc, pt, ct, compt, l6Lat, l5Lat, m3Pass, m3Veto, ltlVeto int64
		var hitPct float64
		var model string
		if err := rows.Scan(&ts, &tc, &sc, &pt, &ct, &compt, &hitPct, &l6Lat, &l5Lat, &m3Pass, &m3Veto, &ltlVeto, &model); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"ts_unix":            ts,
			"thought_count":      tc,
			"span_count":         sc,
			"prompt_tokens":      pt,
			"cached_tokens":      ct,
			"completion_tokens":  compt,
			"cache_hit_pct":      hitPct,
			"l6_avg_latency_ms":  l6Lat,
			"l5_gate_latency_ms": l5Lat,
			"mode3_pass_count":   m3Pass,
			"mode3_veto_count":   m3Veto,
			"ltl_veto_count":     ltlVeto,
			"active_model":       model,
		})
	}
	return out, rows.Err()
}

// FormatPrometheusMetrics renders the complete Prometheus / OpenMetrics text exposition format for ACE v0.2.
func (s *Sink) FormatPrometheusMetrics(ctx context.Context, models []core.ModelInfo, eng core.EngineStatus) string {
	m := s.Metrics(ctx)
	var b strings.Builder

	b.WriteString("# HELP ace_cycle_count_total Total cognitive cycles executed by ACE v0.2\n")
	b.WriteString("# TYPE ace_cycle_count_total counter\n")
	b.WriteString(fmt.Sprintf("ace_cycle_count_total %d\n\n", eng.TotalCycles))

	b.WriteString("# HELP ace_tokens_total Total LLM tokens processed by ACE v0.2\n")
	b.WriteString("# TYPE ace_tokens_total counter\n")
	b.WriteString(fmt.Sprintf("ace_tokens_total{kind=\"prompt\"} %d\n", m.TotalPromptTokens))
	b.WriteString(fmt.Sprintf("ace_tokens_total{kind=\"cached_prefix\"} %d\n", m.TotalCachedTokens))
	b.WriteString(fmt.Sprintf("ace_tokens_total{kind=\"uncached_prompt\"} %d\n", m.TotalPromptTokens-m.TotalCachedTokens))
	b.WriteString(fmt.Sprintf("ace_tokens_total{kind=\"completion\"} %d\n\n", m.TotalCompletionToks))

	hitRatio := 0.0
	if m.TotalPromptTokens > 0 {
		hitRatio = float64(m.TotalCachedTokens) / float64(m.TotalPromptTokens)
	}
	b.WriteString("# HELP ace_kv_cache_hit_ratio Cumulative KV prefix cache hit ratio [0.0, 1.0]\n")
	b.WriteString("# TYPE ace_kv_cache_hit_ratio gauge\n")
	b.WriteString(fmt.Sprintf("ace_kv_cache_hit_ratio %.4f\n\n", hitRatio))

	// Rolling 20-thought KV cache hit ratio
	var recPrompt, recCached int64
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(cached_tokens), 0)
		FROM (SELECT prompt_tokens, cached_tokens FROM thought_traces ORDER BY rowid DESC LIMIT 20)
	`).Scan(&recPrompt, &recCached)
	recRatio := 0.0
	if recPrompt > 0 {
		recRatio = float64(recCached) / float64(recPrompt)
	}
	b.WriteString("# HELP ace_kv_cache_hit_ratio_rolling20 Rolling 20-trace KV prefix cache hit ratio [0.0, 1.0]\n")
	b.WriteString("# TYPE ace_kv_cache_hit_ratio_rolling20 gauge\n")
	b.WriteString(fmt.Sprintf("ace_kv_cache_hit_ratio_rolling20 %.4f\n\n", recRatio))

	b.WriteString("# HELP ace_layer_avg_latency_ms Rolling 25-trace average latency per cognitive layer in milliseconds\n")
	b.WriteString("# TYPE ace_layer_avg_latency_ms gauge\n")
	for _, code := range []string{"L1", "L2", "L3", "L4", "L5", "L6", "CHAT"} {
		b.WriteString(fmt.Sprintf("ace_layer_avg_latency_ms{layer=%q} %d\n", code, m.AvgLayerLatencyMs[code]))
	}
	b.WriteString("\n")

	b.WriteString("# HELP ace_layer_invocations_total Total invocations per cognitive layer\n")
	b.WriteString("# TYPE ace_layer_invocations_total counter\n")
	for _, l := range eng.Layers {
		b.WriteString(fmt.Sprintf("ace_layer_invocations_total{layer=%q,name=%q} %d\n", l.Code, l.Name, l.TotalInvocations))
	}
	b.WriteString("\n")

	b.WriteString("# HELP ace_spans_by_operation_total Total telemetry spans grouped by operation and status\n")
	b.WriteString("# TYPE ace_spans_by_operation_total counter\n")
	rows, err := s.db.QueryContext(ctx, `SELECT operation, status, COUNT(*) FROM telemetry_spans GROUP BY operation, status`)
	if err == nil {
		for rows.Next() {
			var op, st string
			var cnt int64
			if err := rows.Scan(&op, &st, &cnt); err == nil {
				b.WriteString(fmt.Sprintf("ace_spans_by_operation_total{operation=%q,status=%q} %d\n", op, st, cnt))
			}
		}
		rows.Close()
	}
	b.WriteString("\n")

	b.WriteString("# HELP ace_model_requests_total Total LLM requests per model and outcome\n")
	b.WriteString("# TYPE ace_model_requests_total counter\n")
	for _, mdl := range models {
		b.WriteString(fmt.Sprintf("ace_model_requests_total{model=%q,outcome=\"success\",reasoning=\"%v\"} %d\n", mdl.ID, mdl.Reasoning, mdl.SuccessCount))
		b.WriteString(fmt.Sprintf("ace_model_requests_total{model=%q,outcome=\"failure\",reasoning=\"%v\"} %d\n", mdl.ID, mdl.Reasoning, mdl.FailureCount))
	}
	b.WriteString("\n")

	b.WriteString("# HELP ace_model_avg_latency_ms Average model completion latency in milliseconds\n")
	b.WriteString("# TYPE ace_model_avg_latency_ms gauge\n")
	for _, mdl := range models {
		b.WriteString(fmt.Sprintf("ace_model_avg_latency_ms{model=%q} %d\n", mdl.ID, mdl.AvgLatencyMs))
	}
	b.WriteString("\n")

	b.WriteString("# HELP ace_model_circuit_open Circuit breaker state per model (0=closed/healthy, 1=open/cooldown)\n")
	b.WriteString("# TYPE ace_model_circuit_open gauge\n")
	for _, mdl := range models {
		openVal := 0
		if mdl.CircuitState == "open" {
			openVal = 1
		}
		b.WriteString(fmt.Sprintf("ace_model_circuit_open{model=%q} %d\n", mdl.ID, openVal))
	}

	return b.String()
}

func (s *Sink) EmitLive(eventType string, payload interface{}) {
	evt := core.LiveEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
	s.mu.RLock()
	handlers := make([]func(core.LiveEvent), 0, len(s.subs))
	for _, fn := range s.subs {
		handlers = append(handlers, fn)
	}
	s.mu.RUnlock()

	for _, fn := range handlers {
		fn(evt)
	}
}

func (s *Sink) SubscribeLive(handler func(core.LiveEvent)) func() {
	id := s.nextID.Add(1)
	s.mu.Lock()
	s.subs[id] = handler
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}
}
