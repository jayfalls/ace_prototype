package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ace/internal/config"
	"ace/internal/senses"
)

// CollectSensoriumSnapshot dynamically probes all registered Proprioceptive + Exteroceptive sensor plugins
// available on the current system and aggregates their readings (§7.1–7.2).
func (e *Engine) CollectSensoriumSnapshot(ctx context.Context) senses.SensoriumSnapshot {
	snap := senses.SensoriumSnapshot{
		Goroutines:     runtime.NumGoroutine(),
		CurrentHourUTC: time.Now().UTC().Hour(),
	}

	for _, plugin := range senses.BuiltinPluggableSensors(e.sensorDeps()) {
		if !plugin.ProbeAvailable(ctx) {
			continue
		}
		reading, err := plugin.Read(ctx)
		if err != nil {
			continue
		}
		snap.Readings = append(snap.Readings, reading)
		if reading.SensorID == "proprio.host_compute" {
			if v, ok := reading.Values["load_avg_1m"].(float64); ok {
				snap.LoadAvg1m = v
			}
			if v, ok := reading.Values["mem_used_mb"].(int64); ok {
				snap.MemUsedMB = v
			}
			if v, ok := reading.Values["mem_avail_mb"].(int64); ok {
				snap.MemAvailMB = v
			}
			if v, ok := reading.Values["disk_free_mb"].(int64); ok {
				snap.DiskFreeMB = v
			}
		} else if reading.SensorID == "extero.human_circadian" {
			if v, ok := reading.Values["prob_active"].(float64); ok {
				snap.HumanProbActiveNow = v
			}
			if v, ok := reading.Values["expected_reply_sec"].(float64); ok {
				snap.ExpectedReplySec = v
			}
			if v, ok := reading.Values["observed_events"].(int); ok {
				snap.ObservedHumanEvents = v
			}
		}
	}
	return snap
}

// RecordHumanInteractionEvent records a real human timestamp (from Cockpit chat, Telegram, or git commits)
// into `human_events` so the 24-bin circadian model learns purely from empirical data (§7.2).
func (e *Engine) RecordHumanInteractionEvent(ctx context.Context, source string, ts time.Time, replyLatencySec float64) {
	db := e.getDB()
	if db == nil {
		return
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO human_events (source, event_ts, hour_utc, reply_latency_sec)
		VALUES (?, ?, ?, ?)
	`, source, ts, ts.Hour(), replyLatencySec)
}

// EstimateHumanCircadianAvailability implements the 24-bin hourly Beta(1 + k_h, 1 + N_h - k_h) estimator
// with 3-hour circular smoothing over observed human interaction timestamps (§7.2).
// Never hardcodes a sleep window.
func (e *Engine) EstimateHumanCircadianAvailability(ctx context.Context, nowUTC time.Time) (probActive float64, expectedReplySec float64, totalEvents int) {
	var hourHits [24]float64
	daysSet := make(map[string]bool)

	db := e.getDB()
	if db != nil {
		rows, err := db.QueryContext(ctx, `SELECT event_ts, hour_utc FROM human_events ORDER BY event_ts DESC LIMIT 2000`)
		if err == nil {
			for rows.Next() {
				var ts time.Time
				var hr int
				if rows.Scan(&ts, &hr) == nil && hr >= 0 && hr < 24 {
					dayKey := ts.UTC().Format("2006-01-02")
					daysSet[dayKey] = true
					hourHits[hr] += 1.0
					totalEvents++
				}
			}
			rows.Close()
		}

		crows, cerr := db.QueryContext(ctx, `SELECT created_at FROM chat_messages WHERE role = 'user' ORDER BY created_at DESC LIMIT 500`)
		if cerr == nil {
			for crows.Next() {
				var ts time.Time
				if crows.Scan(&ts) == nil {
					u := ts.UTC()
					daysSet[u.Format("2006-01-02")] = true
					hourHits[u.Hour()] += 1.0
					totalEvents++
				}
			}
			crows.Close()
		}
	}

	if totalEvents < 10 {
		out, err := exec.CommandContext(ctx, "git", "log", "-n", "120", "--format=%aI").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if t, perr := time.Parse(time.RFC3339, strings.TrimSpace(line)); perr == nil {
					u := t.UTC()
					daysSet[u.Format("2006-01-02")] = true
					hourHits[u.Hour()] += 1.0
					totalEvents++
				}
			}
		}
	}

	numDays := float64(len(daysSet))
	if numDays < 3.0 {
		numDays = 3.0
	}

	hNow := nowUTC.UTC().Hour()
	weights := []struct {
		offset int
		w      float64
	}{
		{-1, 0.25},
		{0, 0.50},
		{1, 0.25},
	}
	var smoothedProb float64
	for _, wk := range weights {
		h := (hNow + wk.offset + 24) % 24
		kh := math.Min(hourHits[h], numDays)
		p := (1.0 + kh) / (2.0 + numDays)
		smoothedProb += wk.w * p
	}
	probActive = math.Max(0.05, math.Min(0.95, smoothedProb))
	expectedReplySec = math.Round(300.0 / math.Max(0.08, probActive))
	return probActive, expectedReplySec, totalEvents
}

func (e *Engine) FormatSensoriumPromptLine(ctx context.Context, layer int) string {
	s := e.CollectSensoriumSnapshot(ctx)
	// Spec 9.3: senses.layer.<n> allowlists cap which sensor readings a layer
	// may see. Absent/empty allowlist = unrestricted.
	allow := e.layerKnobs.Load().(config.LayerKnobs).SensesAllowlist[layer]
	var proprio, extero []string
	for _, r := range s.Readings {
		if len(allow) > 0 {
			ok := false
			for _, prefix := range allow {
				if strings.HasPrefix(r.SensorID, prefix) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if r.Domain == senses.SensorProprioceptive {
			proprio = append(proprio, fmt.Sprintf("%s(%s)", r.Modality, r.Summary))
		} else {
			extero = append(extero, fmt.Sprintf("%s(%s)", r.Modality, r.Summary))
		}
	}
	return fmt.Sprintf("Sensorium [Proprio: %s | Extero: %s]", strings.Join(proprio, "; "), strings.Join(extero, "; "))
}

// DispatchHumanContradictionAlert sends a plain-text contradiction message to Telegram
// (or queues it into the overnight digest if P(human_active_now) < 0.25).
func (e *Engine) DispatchHumanContradictionAlert(ctx context.Context, cycleID string, nodeID string, fourLineMessage string) {
	probActive, _, _ := e.EstimateHumanCircadianAvailability(ctx, time.Now().UTC())
	db := e.getDB()

	if db != nil {
		var state string
		if db.QueryRowContext(ctx, `SELECT state FROM connectome_nodes WHERE id = ?`, nodeID).Scan(&state) == nil {
			if state == "QUARANTINED" {
				return
			}
		}
		// 8.4: L1 already ruled — the node is quarantined, the human gets a
		// report, not a question.
		_, _ = db.ExecContext(ctx, `UPDATE connectome_nodes SET state = 'QUARANTINED', updated_at = ? WHERE id = ?`, time.Now().UTC(), nodeID)
	}

	if probActive < 0.25 && e.memory != nil {
		prev, _ := e.memory.LoadEngineMeta(ctx, "overnight_contradiction_digest")
		entry := fmt.Sprintf("[%s] %s", nodeID, strings.ReplaceAll(fourLineMessage, "\n", " | "))
		if prev != "" {
			entry = prev + "\n" + entry
		}
		_ = e.memory.SaveEngineMeta(ctx, "overnight_contradiction_digest", entry)
		return
	}

	e.sendTelegramTextEscalation(ctx, fourLineMessage)
}

func (e *Engine) sendTelegramTextEscalation(ctx context.Context, text string) {
	db := e.getDB()
	if db == nil {
		return
	}
	var token, allowedIDs string
	var lastChatID int64
	err := db.QueryRowContext(ctx, `SELECT token, allowed_chat_ids, last_chat_id FROM channel_settings WHERE channel = 'telegram' AND enabled = 1`).Scan(&token, &allowedIDs, &lastChatID)
	if err != nil || strings.TrimSpace(token) == "" {
		return
	}
	chatID := lastChatID
	if chatID == 0 && strings.TrimSpace(allowedIDs) != "" {
		first := strings.Split(allowedIDs, ",")[0]
		chatID, _ = strconv.ParseInt(strings.TrimSpace(first), 10, 64)
	}
	if chatID == 0 {
		return
	}
	if len(text) > 3900 {
		text = text[:3900]
	}
	payload, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", strings.TrimSpace(token)), strings.NewReader(string(payload)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
}

// sensorDeps wires the moved senses package back to the engine (spec 9.1).
func (e *Engine) sensorDeps() senses.Deps {
	return senses.Deps{
		WorkspaceDir: func() string { wsDir, _ := e.resolveSandboxDirs(); return wsDir },
		Circadian:    e.EstimateHumanCircadianAvailability,
	}
}
