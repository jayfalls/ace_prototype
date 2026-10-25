package contextsys

// connectome.go is the spec-9.5 problem-space map model, moved here from
// internal/cognitive/connectome.go (which now holds only engine-bound
// orchestration methods calling into this code).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultSliceMaxBytes = 8192

type ConnectomeNode struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"` // FILE_MODULE, SYMBOL, DATASET, TEST, SPEC_CLAUSE, METRIC_BELIEF, CITATION, UNCERTAINTY, CONTRADICTION
	Path           string    `json:"path"`
	Symbol         string    `json:"symbol"`
	Summary        string    `json:"summary"`
	Hash           string    `json:"hash"`
	SizeBytes      int       `json:"size_bytes"`
	State          string    `json:"state"` // UNEXPLORED, INSPECTED, IMPLEMENTED, VERIFIED, OPEN, PROBING, CLOSED, QUARANTINED, DEFAULTED
	OwnerLayer     int       `json:"owner_layer"`
	BlockingWeight float64   `json:"blocking_weight"`
	InfoValue      float64   `json:"info_value"`
	ExpectedCost   float64   `json:"expected_cost"`
	EvidenceRef    string    `json:"evidence_ref"`
	CycleEpoch     int64     `json:"cycle_epoch"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (n ConnectomeNode) VOI() float64 {
	cost := n.ExpectedCost
	if cost <= 0.05 {
		cost = 0.05
	}
	return (n.BlockingWeight * n.InfoValue) / cost
}

type ConnectomeEdge struct {
	Src    string  `json:"src"`
	Dst    string  `json:"dst"`
	Kind   string  `json:"kind"` // IMPORTS, CALLS, MENTIONS, CO_CHANGED, READS_DATA, PRODUCES, TESTED_BY, SPECIFIES, DEPENDS_ON, CONTRADICTS
	Weight float64 `json:"weight"`
}

var (
	ConnectomePathTokenRe = regexp.MustCompile(`(?:/workspace/|/wiki/)?[\w./-]*[\w-]+\.(?:jsonl|json|parquet|py|go|ts|js|sh|sql|db|csv|md|txt)\b`)
	ConnectomeWordTokenRe = regexp.MustCompile(`[a-z][a-z0-9]{3,}`)
	PyDefClassLineRe      = regexp.MustCompile(`^\s*(def|class)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(\([^)]*\))?`)
	GoFuncTypeLineRe      = regexp.MustCompile(`^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)|^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:struct|interface)`)
)

func SliceBudgetMaxBytes() int {
	if raw := strings.TrimSpace(os.Getenv("ACE_SLICE_MAX_BYTES")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 512 && v <= 65536 {
			return v
		}
	}
	return DefaultSliceMaxBytes
}

// RefreshConnectomeGraph deterministically indexes /workspace and /wiki files, builds compact summaries,
func ClassifyConnectomeFileKind(vpath string) string {
	lower := strings.ToLower(vpath)
	switch {
	case strings.HasPrefix(lower, "/wiki/specs/"):
		return "SPEC_CLAUSE"
	case strings.Contains(lower, "test"):
		return "TEST"
	case strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonl") || strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".parquet"):
		return "DATASET"
	default:
		return "FILE_MODULE"
	}
}

func SummarizeFileForConnectome(vpath string, hpath string, sz int) (summary string, rawSample string, hashHex string) {
	f, err := os.Open(hpath)
	if err != nil {
		return fmt.Sprintf("%s (%dB)", vpath, sz), "", ""
	}
	defer f.Close()

	buf := make([]byte, 32768)
	n, _ := f.Read(buf)
	sample := buf[:n]
	sum := sha256.Sum256(sample)
	hashHex = hex.EncodeToString(sum[:8])

	ext := strings.ToLower(filepath.Ext(vpath))
	if ext == ".db" || ext == ".sqlite" || ext == ".parquet" {
		return fmt.Sprintf("%s (%dB) binary dataset/sqlite", vpath, sz), "", hashHex
	}

	rawSample = string(sample)
	lines := strings.Split(rawSample, "\n")

	switch ext {
	case ".py":
		var defs []string
		for _, line := range lines {
			if PyDefClassLineRe.MatchString(line) {
				clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ":"))
				if len(clean) > 72 {
					clean = clean[:72]
				}
				defs = append(defs, clean)
				if len(defs) >= 5 {
					break
				}
			}
		}
		if len(defs) > 0 {
			return fmt.Sprintf("%s (%dB) defs: %s", vpath, sz, strings.Join(defs, "; ")), rawSample, hashHex
		}
	case ".go":
		var defs []string
		for _, line := range lines {
			if GoFuncTypeLineRe.MatchString(line) {
				clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "{"))
				if len(clean) > 72 {
					clean = clean[:72]
				}
				defs = append(defs, clean)
				if len(defs) >= 5 {
					break
				}
			}
		}
		if len(defs) > 0 {
			return fmt.Sprintf("%s (%dB) symbols: %s", vpath, sz, strings.Join(defs, "; ")), rawSample, hashHex
		}
	case ".json", ".jsonl":
		var obj map[string]any
		if json.Unmarshal(sample, &obj) == nil && len(obj) > 0 {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 8 {
				keys = keys[:8]
			}
			return fmt.Sprintf("%s (%dB) keys=[%s]", vpath, sz, strings.Join(keys, ", ")), rawSample, hashHex
		}
	case ".md", ".txt":
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if len(trimmed) > 85 {
					trimmed = trimmed[:85]
				}
				return fmt.Sprintf("%s (%dB) %s", vpath, sz, trimmed), rawSample, hashHex
			}
		}
	}
	return fmt.Sprintf("%s (%dB)", vpath, sz), rawSample, hashHex
}
func ResolveConnectomePathToken(tok string, files map[string]string, byBase map[string][]string) []string {
	clean := strings.TrimSpace(tok)
	if _, ok := files[clean]; ok {
		return []string{clean}
	}
	trimmed := strings.TrimLeft(clean, "/")
	for _, pref := range []string{"/workspace/", "/wiki/"} {
		if _, ok := files[pref+trimmed]; ok {
			return []string{pref + trimmed}
		}
	}
	return byBase[filepath.Base(clean)]
}

// RankConnectomeCandidates executes Reciprocal Rank Fusion (RRF, k=60) over:
// 1. Personalized PageRank (alpha=0.6, 25 iterations) seeded from directive + spec paths, and
// 2. Lexical IDF overlap between (directive + goal + spec) tokens and node path/summary tokens.
func RankConnectomeCandidates(nodes map[string]ConnectomeNode, adj map[string][]string, directive string, goalText string, specText string) []ConnectomeNode {
	files := make(map[string]string, len(nodes))
	byBase := make(map[string][]string)
	for _, n := range nodes {
		if n.Path != "" {
			files[n.Path] = n.ID
			byBase[filepath.Base(n.Path)] = append(byBase[filepath.Base(n.Path)], n.Path)
		}
	}

	// Extract explicit path seeds from directive + spec
	seedSet := make(map[string]bool)
	for _, match := range ConnectomePathTokenRe.FindAllString(directive+"\n"+specText, -1) {
		for _, vp := range ResolveConnectomePathToken(match, files, byBase) {
			if id, ok := files[vp]; ok {
				seedSet[id] = true
			}
		}
	}

	// 1. Personalized PageRank over undirected graph
	ppr := make(map[string]float64)
	if len(seedSet) > 0 {
		seedWeight := 1.0 / float64(len(seedSet))
		for s := range seedSet {
			ppr[s] = seedWeight
		}
		const alpha = 0.6
		for iter := 0; iter < 25; iter++ {
			next := make(map[string]float64, len(nodes))
			for u, val := range ppr {
				nbs := adj[u]
				if len(nbs) == 0 {
					next[u] += alpha * val
					continue
				}
				share := (alpha * val) / float64(len(nbs))
				for _, v := range nbs {
					next[v] += share
				}
			}
			for s := range seedSet {
				next[s] += (1.0 - alpha) * seedWeight
			}
			ppr = next
		}
	}

	// 2. Lexical IDF scoring over path + summary tokens
	nodeToks := make(map[string]map[string]bool, len(nodes))
	df := make(map[string]int)
	for id, n := range nodes {
		raw := strings.ToLower(strings.NewReplacer("/", " ", "_", " ", ".", " ", "-", " ").Replace(n.Path + " " + n.Summary))
		tset := make(map[string]bool)
		for _, w := range ConnectomeWordTokenRe.FindAllString(raw, -1) {
			tset[w] = true
		}
		nodeToks[id] = tset
		for w := range tset {
			df[w]++
		}
	}

	queryToks := make(map[string]bool)
	qRaw := strings.ToLower(strings.NewReplacer("/", " ", "_", " ", ".", " ", "-", " ").Replace(directive + " " + goalText + " " + specText))
	for _, w := range ConnectomeWordTokenRe.FindAllString(qRaw, -1) {
		queryToks[w] = true
	}

	nTotal := float64(len(nodes) + 1)
	lex := make(map[string]float64, len(nodes))
	for id, tset := range nodeToks {
		var score float64
		for w := range queryToks {
			if tset[w] && df[w] > 0 {
				score += math.Log(nTotal / float64(df[w]))
			}
		}
		if score > 0 {
			lex[id] = score
		}
	}

	// 3. Reciprocal Rank Fusion (k=60)
	rrf := make(map[string]float64, len(nodes))
	addRankList := func(scores map[string]float64) {
		ids := make([]string, 0, len(scores))
		for id, sc := range scores {
			if sc > 0 {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			if scores[ids[i]] == scores[ids[j]] {
				return ids[i] < ids[j]
			}
			return scores[ids[i]] > scores[ids[j]]
		})
		if len(ids) > 100 {
			ids = ids[:100]
		}
		for rank, id := range ids {
			rrf[id] += 1.0 / float64(60+rank)
		}
	}
	addRankList(ppr)
	addRankList(lex)

	// Boost explicit seed paths so direct anchors are always top-ranked
	for s := range seedSet {
		rrf[s] += 0.05
	}

	all := make([]ConnectomeNode, 0, len(nodes))
	for _, n := range nodes {
		if strings.HasPrefix(n.ID, "file:") {
			all = append(all, n)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		si, sj := rrf[all[i].ID], rrf[all[j].ID]
		if si == sj {
			return all[i].ID < all[j].ID
		}
		return si > sj
	})
	return all
}
func IsHarnessMintedEvidenceRef(ref string) bool {
	for _, pref := range []string{"tht_", "spn_", "span_", "cyc_", "test_", "human_", "selftest_"} {
		if strings.HasPrefix(ref, pref) && len(ref) > len(pref)+3 {
			return true
		}
	}
	return false
}

func SanitizeTaskID(raw string) string {
	clean := regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(strings.TrimSpace(raw), "_")
	if len(clean) > 48 {
		clean = clean[:48]
	}
	if clean == "" {
		return "default_task"
	}
	return clean
}
