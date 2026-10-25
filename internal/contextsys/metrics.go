package contextsys

import (
	"context"
	"database/sql"
	"time"
)

// CacheHitRate is the spec's standing metric: Σ cached_tokens ÷ Σ
// prompt_tokens over usage_events, provider-reported. Per layer when
// layer is non-empty, all layers otherwise.
func CacheHitRate(ctx context.Context, db *sql.DB, layer string, since time.Time) float64 {
	where := "1=1"
	args := []any{}
	if layer != "" {
		where += " AND layer=?"
		args = append(args, layer)
	}
	if !since.IsZero() {
		where += " AND created_at>=?"
		args = append(args, since.Format(tsFmt))
	}
	var prompt, cached float64
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(cached_tokens),0)
		 FROM usage_events WHERE `+where, args...).Scan(&prompt, &cached)
	if err != nil || prompt <= 0 {
		return 0
	}
	return cached / prompt
}

// TurnsToFirstAction counts messages before the first mutating action in
// a trace (spec: turns to first useful action). Mutating actions are the
// registry's mutating kinds; callers pass them.
func TurnsToFirstAction(turns []Turn, mutating map[string]bool) int {
	for i, t := range turns {
		if mutating[t.Role] {
			return i + 1
		}
	}
	return -1
}

// RetrievalMiss is one late read of a file turn 0 could have injected —
// the spec's retrieval-miss-rate signal.
type RetrievalMiss struct {
	Path     string
	TurnIdx  int
	Detected time.Time
}

// TokensPerSuccess is the measured tokens/success metric: prompt+completion
// tokens summed over usage_events within a window, divided by successful
// task count the caller supplies (never fabricated — 0 when no successes).
func TokensPerSuccess(ctx context.Context, db *sql.DB, since time.Time, successes int) float64 {
	if successes <= 0 {
		return 0
	}
	var tokens float64
	where := "1=1"
	args := []any{}
	if !since.IsZero() {
		where = "created_at>=?"
		args = append(args, since.Format(tsFmt))
	}
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0) FROM usage_events WHERE `+where,
		args...).Scan(&tokens)
	return tokens / float64(successes)
}
