package contextsys

import "sort"

// ClosureItem is an open item — unviewed file, unanswered question,
// untested assumption — ranked by value of information (spec 9.5
// closure.go). Items are worked within the exploration budget (10% of
// effort) and never block work.
type ClosureItem struct {
	ID    string
	Kind  string // unviewed_file | open_question | untested_assumption | unverified_claim
	Ref   string
	VOI   float64 // value of information, 0..1
	Cost  float64 // estimated effort units to close
	Title string
}

// ExplorationBudget is the spec's 10% cap on closure work per cycle.
const ExplorationBudget = 0.10

// RankClosure sorts items by value of information per unit cost
// (VOI/cost), ties broken by id for determinism. Items with zero cost
// rank as pure VOI.
func RankClosure(items []ClosureItem) []ClosureItem {
	out := append([]ClosureItem(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		si := score(out[i])
		sj := score(out[j])
		if si != sj {
			return si > sj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func score(it ClosureItem) float64 {
	if it.Cost <= 0 {
		return it.VOI
	}
	return it.VOI / it.Cost
}

// WithinBudget picks the highest-ranked items whose summed cost fits
// inside budgetUnits (already scaled to the 10% share by the caller).
// Returns the chosen items plus the budget left over — never more than
// the budget, never blocks work on unaffordable items.
func WithinBudget(ranked []ClosureItem, budgetUnits float64) (chosen []ClosureItem, remaining float64) {
	remaining = budgetUnits
	for _, it := range ranked {
		if it.Cost <= remaining {
			chosen = append(chosen, it)
			remaining -= it.Cost
		}
	}
	return chosen, remaining
}
