package provider

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// windowSecs maps quota window names to their duration (spec 9.2: 5h, weekly).
var windowSecs = map[string]int64{
	"5h":     5 * 3600,
	"daily":  24 * 3600,
	"weekly": 7 * 24 * 3600,
}

// QuotaUsed reports the fraction (0..1) of a subscription window already
// consumed: sum of window usage_events tokens vs the window's allowance in
// provider.quota.<group>.<window> (limit is a token count; when configured
// as a fraction <=1 it is treated as a hard block threshold — see spec §5).
func QuotaUsed(ctx context.Context, db *sql.DB, group, window string, allowanceTokens float64) float64 {
	if db == nil || allowanceTokens <= 0 {
		return 0
	}
	secs, ok := windowSecs[window]
	if !ok {
		return 0
	}
	since := time.Now().UTC().Add(-time.Duration(secs) * time.Second).Format("2006-01-02 15:04:05")
	var used float64
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0) FROM usage_events
		WHERE provider=? AND replace(substr(created_at,1,19),'T',' ') >= ?`, group, since).Scan(&used)
	return used / allowanceTokens
}

// quotaBlocked reports whether a group is over any configured quota window.
// limits: window -> allowance tokens (subscription) — billing 'metered'
// groups instead compare summed cost against the limit (spend cap).
func quotaBlocked(ctx context.Context, db *sql.DB, g GroupCfg, limits map[string]float64) (bool, string) {
	for window, limit := range limits {
		if limit <= 0 {
			continue
		}
		if g.Billing == "metered" {
			secs, ok := windowSecs[window]
			if !ok {
				continue
			}
			since := time.Now().UTC().Add(-time.Duration(secs) * time.Second).Format("2006-01-02 15:04:05")
			var spent float64
			_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(cost),0) FROM usage_events
				WHERE provider=? AND replace(substr(created_at,1,19),'T',' ') >= ?`, g.Name, since).Scan(&spent)
			if spent >= limit {
				return true, "spend cap " + window + " reached (" + strconv.FormatFloat(spent, 'f', 4, 64) + " >= " + strconv.FormatFloat(limit, 'f', 4, 64) + ")"
			}
			continue
		}
		if used := QuotaUsed(ctx, db, g.Name, window, limit); used >= 1 {
			return true, "quota window " + window + " exhausted"
		}
	}
	return false, ""
}

// parseQuotaKey splits "provider.quota.<group>.<window>" — kept for
// config-file migration convenience.
func parseQuotaKey(k string) (group, window string, ok bool) {
	parts := strings.Split(k, ".")
	if len(parts) == 4 && parts[0] == "provider" && parts[1] == "quota" {
		return parts[2], parts[3], true
	}
	return "", "", false
}
