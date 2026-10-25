package provider

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
)

// Registry routes LLM calls to adapters by role and model id with breakers,
// classified retries, quota windows and usage accounting (spec 9.2). It
// implements core.LLMProvider so the engine port stays unchanged.
type Registry struct {
	db         *sql.DB
	adapters   map[string]Adapter // group name -> adapter
	groups     []GroupCfg         // priority-ascending
	priority   map[string]int
	cfg        Config
	breakers   *breaker
	main       ModelRef
	fast       []ModelRef
	roleRoutes map[Role]ModelRef
	onChurn    func(ChurnEvent)

	mu          sync.RWMutex
	activeModel string
}

// New builds the registry from provider.json. adapters maps group name ->
// ready adapter; model routing comes from cfg.Models.
func New(db *sql.DB, cfg Config, adapters map[string]Adapter) (*Registry, error) {
	if len(cfg.Groups) == 0 {
		return nil, fmt.Errorf("provider.groups is empty")
	}
	r := &Registry{
		db:         db,
		adapters:   adapters,
		priority:   map[string]int{},
		cfg:        cfg,
		roleRoutes: map[Role]ModelRef{},
	}
	r.groups = append([]GroupCfg(nil), cfg.Groups...)
	sort.SliceStable(r.groups, func(i, j int) bool { return r.groups[i].Priority < r.groups[j].Priority })
	for i, g := range r.groups {
		if g.Priority == 0 {
			g.Priority = i + 1
		}
		r.priority[g.Name] = g.Priority
		if _, ok := adapters[g.Name]; !ok {
			return nil, fmt.Errorf("provider group %s has no adapter", g.Name)
		}
	}
	r.breakers = newBreaker(cfg.Breaker.Cooldown, cfg.Breaker.Max)
	migrateUsage(db)

	resolveGroup := func(modelID string) ModelRef {
		refs := r.resolve(modelID)
		if len(refs) > 0 {
			return refs[0]
		}
		return ModelRef{Group: r.groups[0].Name, Model: modelID, Billing: r.groups[0].Billing}
	}
	mainModel := cfg.Models.Main
	if mainModel == "" && len(cfg.Groups) > 0 && len(cfg.Groups[0].Models) > 0 && cfg.Groups[0].Models[0] != "*" {
		mainModel = cfg.Groups[0].Models[0]
	}
	if mainModel == "" {
		return nil, fmt.Errorf("models.main is empty")
	}
	r.main = resolveGroup(mainModel)
	for _, m := range cfg.Models.Fast {
		r.fast = append(r.fast, resolveGroup(m))
	}
	for role, m := range cfg.Models.Roles {
		if m != "" {
			r.roleRoutes[role] = resolveGroup(m)
		}
	}
	r.activeModel = mainModel
	return r, nil
}

// OnChurn registers the subscriber invoked after a catalogue change — the
// engine wires T1 recalibration + re-routing here (spec 9.2).
func (r *Registry) OnChurn(f func(ChurnEvent)) { r.onChurn = f }

// SyncCatalogue applies a fresh catalogue and reports the churn — callers
// publish provider.churn on the bus. Roles pointing at removed models are
// rerouted to the best remaining model in the same group (spec 9.2).
func (r *Registry) SyncCatalogue(ctx context.Context, rows []CatalogueRow) (ChurnEvent, error) {
	ev, err := SyncCatalogue(ctx, r.db, rows)
	if err != nil {
		return ev, err
	}
	if len(ev.Added)+len(ev.Removed)+len(ev.Changed) > 0 && r.onChurn != nil {
		r.onChurn(ev)
	}
	if len(ev.Removed) > 0 {
		r.rerouteRemoved(ctx)
	}
	return ev, nil
}

// rerouteRemoved re-seats every role route whose model was removed from the
// catalogue onto the highest-aa_index model still live in the same group.
// Returns main's model after rerouting.
func (r *Registry) rerouteRemoved(ctx context.Context) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	reroute := func(ref ModelRef) ModelRef {
		if r.db == nil {
			return ref
		}
		if _, ok := catalogueLookup(ctx, r.db, ref.Group, ref.Model); ok {
			return ref
		}
		var next string
		if err := r.db.QueryRowContext(ctx, `SELECT id FROM models_catalogue WHERE provider=? AND removed_at='' ORDER BY aa_index DESC, id LIMIT 1`, ref.Group).Scan(&next); err == nil {
			return ModelRef{Group: ref.Group, Model: next, Billing: ref.Billing}
		}
		return ref
	}
	r.main = reroute(r.main)
	for k, ref := range r.roleRoutes {
		r.roleRoutes[k] = reroute(ref)
	}
	for i, ref := range r.fast {
		r.fast[i] = reroute(ref)
	}
	r.activeModel = r.main.Model
	return r.main.Model
}

func (r *Registry) Name() string { return "provider-registry" }

// Models reports catalogue-backed model info with breaker state.
func (r *Registry) Models() []core.ModelInfo {
	if r.db == nil {
		return nil
	}
	rows, err := r.db.Query(`SELECT id, provider, context FROM models_catalogue WHERE removed_at=''`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []core.ModelInfo
	for rows.Next() {
		var m core.ModelInfo
		if rows.Scan(&m.ID, &m.Provider, &m.ContextSize) != nil {
			continue
		}
		m.Name = m.ID
		if r.breakers.Open(m.Provider, m.ID) {
			m.CircuitState = "open"
		} else {
			m.CircuitState = "closed"
		}
		out = append(out, m)
	}
	return out
}

func (r *Registry) ActiveModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeModel
}

func (r *Registry) SetActiveModel(modelID string) error {
	if strings.TrimSpace(modelID) == "" {
		return fmt.Errorf("empty model id")
	}
	r.mu.Lock()
	r.activeModel = modelID
	r.main = ModelRef{Group: r.main.Group, Model: modelID, Billing: r.main.Billing}
	if refs := r.resolve(modelID); len(refs) > 0 {
		r.main = refs[0]
	}
	r.mu.Unlock()
	return nil
}

// StreamComplete routes one call: explicit model ids resolve through group
// priority; empty model means the main role (spec: [main] until quota spent,
// never to free). Every attempt is usage-recorded; only transient/timeout
// errors retry (full-jitter, then the next group).
func (r *Registry) StreamComplete(ctx context.Context, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	modelID := req.Model
	role := RoleMain
	if modelID == "" {
		modelID = r.ActiveModel()
	} else if modelID == r.fastModel() {
		role = RoleFast
	}
	return r.Call(ctx, role, modelID, req, onDelta)
}

func (r *Registry) fastModel() string {
	if len(r.fast) > 0 {
		return r.fast[0].Model
	}
	return ""
}

// Call routes a request for an explicit role/model — the engine's richer
// entry point (usage rows carry the role; layer rides in via ctx upstream).
func (r *Registry) Call(ctx context.Context, role Role, modelID string, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	if req.Temperature == 0 {
		req.Temperature = r.tempFor(role)
	}
	refs := r.routeFor(role, modelID)
	if len(refs) == 0 {
		return core.CompletionResponse{}, fmt.Errorf("no provider group serves model %q", modelID)
	}
	maxRetries := r.cfg.Retry.Max
	if maxRetries <= 0 {
		maxRetries = 2
	}
	var lastErr error
	lastClass := Class("")
	retries := 0
	for _, ref := range refs {
		adapter := r.adapters[ref.Group]
		if adapter == nil {
			continue
		}
		if r.breakers.Open(ref.Group, ref.Model) {
			continue
		}
		g := r.groupCfg(ref.Group)
		if blocked, why := quotaBlocked(ctx, r.db, g, r.cfg.Quota[g.Name]); blocked {
			lastErr = fmt.Errorf("group %s blocked: %s", g.Name, why)
			lastClass = ClassQuota
			continue
		}
		for attempt := 0; attempt <= maxRetries; attempt++ {
			start := time.Now()
			resp, err := adapter.Stream(ctx, ref.Model, Request{Req: req, Role: role, OnDelta: onDelta})
			if err == nil {
				r.breakers.Reset(ref.Group, ref.Model)
				r.recordCall(ctx, role, ref, resp, retries, start)
				return resp, nil
			}
			class := Classify(err, statusOf(err))
			if attempt < maxRetries && retryable(class) {
				retries++
				select {
				case <-ctx.Done():
					return core.CompletionResponse{}, ctx.Err()
				case <-time.After(backoff(attempt, r.cfg.Retry.Base, r.cfg.Retry.Cap)):
				}
				continue
			}
			lastErr = &Classified{Class: class, Status: statusOf(err), Reason: err.Error(), Err: err}
			lastClass = class
			if class == ClassTransient || class == ClassTimeout {
				r.breakers.Trip(ref.Group, ref.Model)
			}
			r.recordErr(ctx, role, ref, retries, start, err)
			break // next group
		}
		// quota blocks stop the chain for main; other classes fall through.
		if lastClass == ClassQuota {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all provider groups skipped for model %q", modelID)
	}
	return core.CompletionResponse{}, lastErr
}

// routeFor resolves the ordered refs for a call: main resolves its model's
// group chain (tier-segmented by resolve — never to free); fast takes the
// p50-ordered fast list; other roles and explicit ids resolve by priority.
func (r *Registry) routeFor(role Role, modelID string) []ModelRef {
	if role == RoleFast {
		if refs := r.fastOrdered(); len(refs) > 0 {
			return refs
		}
	}
	return r.resolve(modelID)
}

// tempFor applies the configured per-role temperature when the caller left
// it zero: creative for main, efficient for logic/side roles.
func (r *Registry) tempFor(role Role) float64 {
	t := r.cfg.Models.Temperature
	if role == RoleMain {
		if t.Creative > 0 {
			return t.Creative
		}
		return 0.7
	}
	if t.Efficient > 0 {
		return t.Efficient
	}
	return 0.2
}

func (r *Registry) groupCfg(name string) GroupCfg {
	for _, g := range r.groups {
		if g.Name == name {
			return g
		}
	}
	return GroupCfg{Name: name}
}

func (r *Registry) recordCall(ctx context.Context, role Role, ref ModelRef, resp core.CompletionResponse, retries int, start time.Time) {
	if r.db == nil {
		return
	}
	var tin, tc, tout *int64
	// usage_events stores provider-reported tokens only; a response that
	// reported none stores NULL (never an estimate).
	if resp.PromptTokens > 0 {
		v := int64(resp.PromptTokens)
		tin = &v
	}
	if resp.CachedTokens > 0 {
		v := int64(resp.CachedTokens)
		tc = &v
	}
	if resp.CompletionTokens > 0 {
		v := int64(resp.CompletionTokens)
		tout = &v
	}
	var cost *float64
	if tin != nil || tout != nil {
		var i, c, o int64
		if tin != nil {
			i = *tin
		}
		if tc != nil {
			c = *tc
		}
		if tout != nil {
			o = *tout
		}
		cost = usageCost(ctx, r.db, ref.Group, ref.Model, i, c, o)
	}
	recordUsage(ctx, r.db, usageEvent{
		callID: newCallID(), layer: LayerFrom(ctx), role: string(role), model: ref.Model, group: ref.Group,
		billing: ref.Billing, tokIn: tin, tokCached: tc, tokOut: tout,
		retries: retries, durationMs: time.Since(start).Milliseconds(), cost: cost,
	})
}

func (r *Registry) recordErr(ctx context.Context, role Role, ref ModelRef, retries int, start time.Time, err error) {
	if r.db == nil {
		return
	}
	recordUsage(ctx, r.db, usageEvent{
		callID: newCallID(), layer: LayerFrom(ctx), role: string(role), model: ref.Model, group: ref.Group,
		billing: ref.Billing, retries: retries, durationMs: time.Since(start).Milliseconds(),
	})
}

var callIDCounter struct {
	mu  sync.Mutex
	seq int
}

func newCallID() string {
	callIDCounter.mu.Lock()
	defer callIDCounter.mu.Unlock()
	callIDCounter.seq++
	return fmt.Sprintf("call_%d_%d", time.Now().UnixMilli(), callIDCounter.seq)
}
