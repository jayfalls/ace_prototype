package skills

// skills.go holds the wiki-skill model moved from
// internal/cognitive/wiki_skills.go (spec 9.6): Beta(α,β) posterior
// records, lifecycle index, trigger extraction.

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ace/internal/contextsys"
)

var skillWordTokenRe = contextsys.ConnectomeWordTokenRe

type WikiSkillRecord struct {
	SkillID      string    `json:"skill_id"`
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	Domain       string    `json:"domain"`
	Version      int       `json:"version"`
	WikiPath     string    `json:"wiki_path"`
	Alpha        float64   `json:"alpha"`
	Beta         float64   `json:"beta"`
	Observations int       `json:"observations"`
	Triggers     []string  `json:"triggers"`
	Procedure    []string  `json:"procedure"`
	Verification string    `json:"verification"`
	SourceCycle  string    `json:"source_cycle"`
	Retired      bool      `json:"retired"`
	SupersededBy string    `json:"superseded_by,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s WikiSkillRecord) PosteriorMean() float64 {
	if s.Alpha+s.Beta <= 0 {
		return 0.5
	}
	return s.Alpha / (s.Alpha + s.Beta)
}

type WikiSkillLifecycleIndex struct {
	UpdatedAt       string            `json:"updated_at"`
	ActiveCount     int               `json:"active_count"`
	RetiredCount    int               `json:"retired_count"`
	Skills          []WikiSkillRecord `json:"skills"`
	RetirementLog   []string          `json:"retirement_log,omitempty"`
	LastRoutedSkill string            `json:"last_routed_skill,omitempty"`
}

var SkillSlugNonAlnumRe = regexp.MustCompile(`[^a-z0-9_]+`)

func FirstNonEmptyInterface(vals ...interface{}) interface{} {
	for _, v := range vals {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return nil
}

func InferSkillDomainAndKey(milestone string, files []string) (domain string, key string, title string) {
	lower := strings.ToLower(milestone)
	switch {
	case strings.Contains(lower, "baseline") || strings.Contains(lower, "comparison") || strings.Contains(lower, "competitor") || strings.Contains(lower, "ablation"):
		domain = "comparative_evaluation"
	case strings.Contains(lower, "sensorium") || strings.Contains(lower, "circadian") || strings.Contains(lower, "replica") || strings.Contains(lower, "proprio"):
		domain = "sensorium_embodiment"
	case strings.Contains(lower, "skill") || strings.Contains(lower, "wiki"):
		domain = "skill_lifecycle"
	case strings.Contains(lower, "connectome") || strings.Contains(lower, "router") || strings.Contains(lower, "context"):
		domain = "connectome_routing"
	case strings.Contains(lower, "swe") || strings.Contains(lower, "terminal") || strings.Contains(lower, "code"):
		domain = "software_repair"
	case strings.Contains(lower, "finance") || strings.Contains(lower, "math") || strings.Contains(lower, "gpqa") || strings.Contains(lower, "research"):
		domain = "deep_research_synthesis"
	default:
		domain = "autonomous_engineering"
	}

	baseSlug := ""
	for _, f := range files {
		if strings.HasSuffix(f, ".py") || strings.HasSuffix(f, ".go") {
			b := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			baseSlug = strings.TrimSpace(SkillSlugNonAlnumRe.ReplaceAllString(strings.ToLower(b), "_"))
			break
		}
	}
	if baseSlug == "" {
		id := contextsys.MilestoneIdentifier(milestone)
		baseSlug = strings.Trim(SkillSlugNonAlnumRe.ReplaceAllString(strings.ToLower(id), "_"), "_")
		if baseSlug == "" {
			baseSlug = "procedural_skill"
		}
	}
	key = domain + "/" + baseSlug
	title = strings.TrimSpace(milestone)
	if idx := strings.Index(title, ":"); idx > 0 {
		title = strings.TrimSpace(strings.TrimPrefix(title[:idx], "- [ ]"))
		title = strings.TrimSpace(strings.TrimPrefix(title, "- [x]"))
		title = strings.Trim(title, "* ")
	}
	if len(title) > 90 {
		title = title[:90]
	}
	return domain, key, title
}
func RecomputeSkillCounts(idx *WikiSkillLifecycleIndex) {
	act, ret := 0, 0
	for _, s := range idx.Skills {
		if s.Retired {
			ret++
		} else {
			act++
		}
	}
	idx.ActiveCount = act
	idx.RetiredCount = ret
}

func ExtractSkillTokens(text string, limit int) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range skillWordTokenRe.FindAllString(strings.ToLower(text), -1) {
		if len(m) < 4 || IsSkillStopword(m) || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func IsSkillStopword(w string) bool {
	switch w {
	case "that", "this", "with", "from", "your", "have", "will", "must", "each", "task", "write", "code", "test", "using", "into", "should", "which", "when", "where", "workspace", "benchmarks", "results", "datasets", "build", "construct":
		return true
	}
	return false
}

// RouteTopWikiSkill selects the highest-scoring active Wiki-Skill for the current milestone using
// token overlap + Beta(alpha, beta) posterior mean (Harness^2 C6 On-Demand Skill Routing).
