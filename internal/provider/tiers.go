package provider

import (
	"context"
	"database/sql"
	"sort"
)

// Route resolves a role to its ordered fallback list (spec 9.2):
//   - main is [main] alone — it is used until its quota is spent and NEVER
//     falls back to a free model;
//   - free falls back only to free (catalogue price_in==0 && price_out==0);
//   - fast is ordered by measured p50 latency from usage_events;
//   - other roles (reasoning/general/efficient/tool) resolve through the
//     layer-role config, defaulting to main's group.
func (r *Registry) Route(role Role) []ModelRef {
	switch role {
	case RoleMain:
		return []ModelRef{r.main}
	case RoleFast:
		return r.fastOrdered()
	default:
		if ref, ok := r.roleRoutes[role]; ok {
			return []ModelRef{ref}
		}
		return []ModelRef{r.main}
	}
}

// fastOrdered returns the fast-role list sorted by measured p50 latency
// (spec: fast is ordered by measured p50). With no measurements yet the
// declared order stands.
func (r *Registry) fastOrdered() []ModelRef {
	if len(r.fast) <= 1 || r.db == nil {
		return r.fast
	}
	p50 := map[string]float64{}
	for _, m := range r.fast {
		var v sql.NullFloat64
		rows, err := r.db.QueryContext(context.Background(), `SELECT duration_ms FROM usage_events
			WHERE provider=? AND model=? ORDER BY id DESC LIMIT 50`, m.Group, m.Model)
		if err != nil {
			continue
		}
		var ds []float64
		for rows.Next() {
			var d float64
			if rows.Scan(&d) == nil {
				ds = append(ds, d)
			}
		}
		rows.Close()
		if len(ds) > 0 {
			sort.Float64s(ds)
			v.Float64, v.Valid = ds[len(ds)/2], true
		}
		if v.Valid {
			p50[m.Group+"/"+m.Model] = v.Float64
		}
	}
	out := make([]ModelRef, len(r.fast))
	copy(out, r.fast)
	sort.SliceStable(out, func(i, j int) bool {
		a, aok := p50[out[i].Group+"/"+out[i].Model]
		b, bok := p50[out[j].Group+"/"+out[j].Model]
		if aok != bok {
			return aok // measured before unmeasured
		}
		return a < b
	})
	return out
}

// resolve maps a request model id to its ordered refs: a group declaring the
// model (or "*") in priority order; main-role rules apply only to role
// routing — an explicit model id honours exactly that model.
func (r *Registry) resolve(modelID string) []ModelRef {
	var out []ModelRef
	for _, g := range r.groups {
		if len(g.Models) == 0 {
			continue
		}
		for _, m := range g.Models {
			if m == "*" || m == modelID {
				out = append(out, ModelRef{Group: g.Name, Model: modelID, Billing: g.Billing})
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return r.priority[out[i].Group] < r.priority[out[j].Group] })
	// Tier segmentation (spec 9.2): the chain never crosses the free line in
	// either direction — main never falls to free, free never falls to paid.
	// The catalogue (price_in==0 && price_out==0) decides when it knows the
	// model; otherwise the highest-priority serving group declares the tier.
	free := r.isFreeModel(modelID)
	if !free && len(out) > 0 && out[0].Billing == "free" {
		free = true
	}
	keep := out[:0]
	for _, ref := range out {
		if (ref.Billing == "free") == free {
			keep = append(keep, ref)
		}
	}
	return keep
}

func (r *Registry) isFreeModel(modelID string) bool {
	if r.db == nil {
		return false
	}
	var pin, pout float64
	err := r.db.QueryRow(`SELECT price_in, price_out FROM models_catalogue WHERE id=? AND removed_at='' LIMIT 1`, modelID).Scan(&pin, &pout)
	return err == nil && pin == 0 && pout == 0
}
