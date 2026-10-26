package config

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LayerKnobs is the spec-9.3 sectioned config surface, loaded from
// <config-dir>/config.d/*.toml (owner-set, hot-reloaded). Values absent
// from the files keep their spec defaults.
type LayerKnobs struct {
	// L1ReviewInterval is layers.l1.review_interval (default 15m).
	L1ReviewInterval time.Duration
	// ParallelReads is layers.parallel_reads (default 4).
	ParallelReads int
	// L5MinDwell is layers.l5.min_dwell (default 60s).
	L5MinDwell time.Duration
	// L5Period is layers.l5.period — L5's own clock (default 30s).
	L5Period time.Duration
	// SensesAllowlist maps layer number (senses.layer.<n>) to the sensor_id
	// prefixes that layer may see. Empty/absent = all senses allowed.
	SensesAllowlist map[int][]string
	// Turn05 is context.turn05 — the turn-0.5 retrieval router. Spec
	// default false (was ACE_TURN05 env, default "ab").
	Turn05 bool
	// RecapN is context.recap_n — turns shown post-compaction (default 5).
	RecapN int
	// ContextEnabled is features.context — the 9.5 context system master
	// switch (default true once the package exists).
	ContextEnabled bool
	// MutationMinMutants is gate.min_mutants — below this count the
	// mutation sample is inconclusive and cannot veto (default 3).
	MutationMinMutants int
}

// DefaultLayerKnobs returns the spec defaults.
func DefaultLayerKnobs() LayerKnobs {
	return LayerKnobs{
		L1ReviewInterval: 15 * time.Minute,
		ParallelReads:    4,
		L5MinDwell:       60 * time.Second,
		L5Period:         30 * time.Second,
		Turn05:           false,
		RecapN:           5,
		ContextEnabled:   true,
		MutationMinMutants: 3,
	}
}

// LoadLayerKnobs parses every .toml file in <config-dir>/config.d. The
// parser accepts flat `key = value` lines, `[section]` prefixes, quoted
// strings, bare numbers, durations ("15m"), and string arrays.
func LoadLayerKnobs() LayerKnobs {
	knobs := DefaultLayerKnobs()
	dir := filepath.Join(Dir(), "config.d")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return knobs
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".toml") {
			continue
		}
		applyKnobFile(&knobs, filepath.Join(dir, ent.Name()))
	}
	return knobs
}

func applyKnobFile(knobs *LayerKnobs, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		if section != "" {
			key = section + "." + key
		}
		val := strings.TrimSpace(v)
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		applyKnob(knobs, key, val)
	}
}

func applyKnob(knobs *LayerKnobs, key, val string) {
	str := func() string {
		return strings.Trim(val, `"'`)
	}
	dur := func() (time.Duration, bool) {
		if d, err := time.ParseDuration(str()); err == nil {
			return d, true
		}
		if n, err := strconv.Atoi(str()); err == nil {
			return time.Duration(n) * time.Second, true
		}
		return 0, false
	}
	switch key {
	case "layers.l1.review_interval":
		if d, ok := dur(); ok && d > 0 {
			knobs.L1ReviewInterval = d
		}
	case "layers.parallel_reads":
		if n, err := strconv.Atoi(str()); err == nil && n > 0 {
			knobs.ParallelReads = n
		}
	case "layers.l5.min_dwell":
		if d, ok := dur(); ok && d > 0 {
			knobs.L5MinDwell = d
		}
	case "layers.l5.period":
		if d, ok := dur(); ok && d > 0 {
			knobs.L5Period = d
		}
	case "context.turn05":
		knobs.Turn05 = str() == "true" || str() == "1" || str() == "on"
	case "context.recap_n":
		if n, err := strconv.Atoi(str()); err == nil && n > 0 {
			knobs.RecapN = n
		}
	case "gate.min_mutants":
		if n, err := strconv.Atoi(str()); err == nil && n >= 0 {
			knobs.MutationMinMutants = n
		}
	case "features.context":
		knobs.ContextEnabled = str() == "true" || str() == "1" || str() == "on"
	default:
		if strings.HasPrefix(key, "senses.layer.") {
			n, err := strconv.Atoi(strings.TrimPrefix(key, "senses.layer."))
			if err != nil || n < 1 || n > 6 {
				return
			}
			if knobs.SensesAllowlist == nil {
				knobs.SensesAllowlist = map[int][]string{}
			}
			knobs.SensesAllowlist[n] = parseStringArray(val)
		}
	}
}

func parseStringArray(v string) []string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil
	}
	var out []string
	for _, item := range strings.Split(v[1:len(v)-1], ",") {
		item = strings.Trim(strings.TrimSpace(item), `"'`)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// WatchLayerKnobs polls config.d mtimes every 5s and fires onChange with the
// freshly loaded knobs whenever the directory contents change (spec 9.3 hot
// reload — the in-band notice is published by the caller).
func WatchLayerKnobs(ctx context.Context, onChange func(LayerKnobs)) {
	dir := filepath.Join(Dir(), "config.d")
	sig := func() string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		var b strings.Builder
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			b.WriteString(name)
			b.WriteString(info.ModTime().UTC().Format(time.RFC3339Nano))
		}
		return b.String()
	}
	last := sig()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cur := sig(); cur != last {
				last = cur
				onChange(LoadLayerKnobs())
			}
		}
	}
}
