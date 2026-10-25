package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// CatalogueRow is one models_catalogue row (spec 9.2). Price zero on both
// input and output marks a free-tier model — replaces the -free suffix sniff.
type CatalogueRow struct {
	ID          string  `json:"id"`
	Provider    string  `json:"provider"`
	SourceURL   string  `json:"source_url"`
	PriceIn     float64 `json:"price_in"`
	PriceOut    float64 `json:"price_out"`
	PriceCached float64 `json:"price_cached"`
	Context     int     `json:"context"`
	Cutoff      string  `json:"cutoff"`
	AAIndex     float64 `json:"aa_index"`
}

// Free reports the catalogue definition of free tier: input AND output both
// priced at zero.
func (r CatalogueRow) Free() bool { return r.PriceIn == 0 && r.PriceOut == 0 }

// ChurnEvent is the provider.churn bus payload: what changed so subscribers
// can re-route and recalibrate before the model takes a role.
type ChurnEvent struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// SyncCatalogue diffs a fresh catalogue against models_catalogue, applies it,
// and returns the churn (empty when nothing moved). The caller emits
// provider.churn on the bus when non-empty.
func SyncCatalogue(ctx context.Context, db *sql.DB, rows []CatalogueRow) (ChurnEvent, error) {
	migrateUsage(db)
	cur := map[string]CatalogueRow{}
	qrows, err := db.QueryContext(ctx, `SELECT id, provider, price_in, price_out, price_cached, context, cutoff, aa_index FROM models_catalogue WHERE removed_at=''`)
	if err != nil {
		return ChurnEvent{}, err
	}
	for qrows.Next() {
		var r CatalogueRow
		if err := qrows.Scan(&r.ID, &r.Provider, &r.PriceIn, &r.PriceOut, &r.PriceCached, &r.Context, &r.Cutoff, &r.AAIndex); err != nil {
			qrows.Close()
			return ChurnEvent{}, err
		}
		cur[r.Provider+"/"+r.ID] = r
	}
	qrows.Close()

	var ev ChurnEvent
	now := time.Now().UTC().Format(time.RFC3339)
	seen := map[string]bool{}
	for _, r := range rows {
		key := r.Provider + "/" + r.ID
		seen[key] = true
		old, ok := cur[key]
		if !ok {
			ev.Added = append(ev.Added, key)
		} else if old.PriceIn != r.PriceIn || old.PriceOut != r.PriceOut || old.PriceCached != r.PriceCached || old.Context != r.Context {
			ev.Changed = append(ev.Changed, key)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO models_catalogue (id, provider, source_url, price_in, price_out, price_cached, context, cutoff, aa_index, removed_at)
			VALUES (?,?,?,?,?,?,?,?,?,'')
			ON CONFLICT(id, provider) DO UPDATE SET source_url=excluded.source_url, price_in=excluded.price_in,
				price_out=excluded.price_out, price_cached=excluded.price_cached, context=excluded.context,
				cutoff=excluded.cutoff, aa_index=excluded.aa_index, removed_at=''`,
			r.ID, r.Provider, r.SourceURL, r.PriceIn, r.PriceOut, r.PriceCached, r.Context, r.Cutoff, r.AAIndex); err != nil {
			return ev, err
		}
	}
	for key, r := range cur {
		if seen[key] {
			continue
		}
		ev.Removed = append(ev.Removed, key)
		_, _ = db.ExecContext(ctx, `UPDATE models_catalogue SET removed_at=? WHERE id=? AND provider=?`, now, r.ID, r.Provider)
	}
	return ev, nil
}

// catalogueLookup returns the row for a model+group (empty when unknown).
func catalogueLookup(ctx context.Context, db *sql.DB, group, model string) (CatalogueRow, bool) {
	if db == nil {
		return CatalogueRow{}, false
	}
	var r CatalogueRow
	err := db.QueryRowContext(ctx, `SELECT id, provider, price_in, price_out, price_cached, context FROM models_catalogue WHERE id=? AND provider=? AND removed_at=''`,
		model, group).Scan(&r.ID, &r.Provider, &r.PriceIn, &r.PriceOut, &r.PriceCached, &r.Context)
	return r, err == nil
}

// isFree reports the catalogue-based free check (spec 9.2: price fields, not
// a suffix match). Unknown rows are NOT free — a guess would break the
// "main never falls to free" rule.
func isFree(ctx context.Context, db *sql.DB, group, model string) bool {
	r, ok := catalogueLookup(ctx, db, group, model)
	return ok && r.Free()
}

// LiveProviders returns the provider names with at least one non-removed row.
func LiveProviders(ctx context.Context, db *sql.DB) []string {
	if db == nil {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT provider FROM models_catalogue WHERE removed_at=''`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			out = append(out, p)
		}
	}
	return out
}

// MarshalChurn serialises a churn event for the bus payload.
func MarshalChurn(ev ChurnEvent) string {
	b, _ := json.Marshal(ev)
	return string(b)
}

// SplitRef splits a "provider/model" catalogue key back into its parts —
// provider ids never contain '/', model ids are taken verbatim after it.
func SplitRef(key string) (provider, model string) {
	p, m, _ := strings.Cut(key, "/")
	return p, m
}
