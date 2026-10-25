// Package plan holds the v0.9 mission-plan hygiene passes: strategy
// deduplication and the stale-node sweep (spec 9.0).
package plan

import (
	"regexp"

	"ace/internal/science"
)

// ArtefactRe extracts artefact identifiers (module files, dataset assets,
// c_sNN_* modules) from strategy text. The 8.3 word-shingle check missed
// differently-worded branches targeting the same module — artefact-set
// overlap catches them (spec 9.0).
var ArtefactRe = regexp.MustCompile(`[a-z0-9_]+\.(?:py|json)|c_s\d+_[a-z]+`)

// Strategy is the minimal view of an existing L2 branch needed for the
// duplicate check — the caller maps mission_dag rows onto it (plan must not
// import cognitive).
type Strategy struct {
	ID        string
	Title     string
	Directive string
}

func artefacts(title, directive string) map[string]bool {
	out := map[string]bool{}
	for _, m := range ArtefactRe.FindAllString(title+" "+directive, -1) {
		out[m] = true
	}
	return out
}

func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	union := len(a) + len(b) - n
	return float64(n) / float64(union)
}

// DuplicateOf returns the ID of an existing strategy the candidate duplicates
// ("" when novel). A candidate is a duplicate when word-shingle Jaccard on
// title+directive reaches 0.8, or the artefact sets overlap by >= 0.5.
func DuplicateOf(title, directive string, existing []Strategy) (string, bool) {
	cand := title + " " + directive
	candArts := artefacts(title, directive)
	for _, ex := range existing {
		if science.ShingleJaccard(cand, ex.Title+" "+ex.Directive) >= 0.8 {
			return ex.ID, true
		}
		if overlap(candArts, artefacts(ex.Title, ex.Directive)) >= 0.5 {
			return ex.ID, true
		}
	}
	return "", false
}
