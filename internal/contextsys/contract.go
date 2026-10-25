package contextsys

import (
	"fmt"
	"strings"
)

// Contract declares the turn-0 inputs a layer's prompt must contain.
// Inputs are marker substrings the built prompt is asserted to hold
// (ContractTest). Each layer has its own view of the shared map (spec).
type Contract struct {
	Layer  int
	Name   string
	Inputs []string
}

// LayerContracts is the spec's per-layer turn-0 declaration. Markers match
// the section headers the prompt builder emits.
var LayerContracts = []Contract{
	{1, "aspirational", []string{"CONSTITUTION", "CURRENT MISSION", "TEMPORAL CONTEXT"}},
	{2, "strategy", []string{"CURRENT MISSION", "TEMPORAL CONTEXT"}},
	{3, "agent_model", []string{"CURRENT MISSION", "TEMPORAL CONTEXT"}},
	{4, "executive", []string{"CURRENT MISSION", "TEMPORAL CONTEXT"}},
	{5, "cognitive_control", []string{"CURRENT MISSION", "TEMPORAL CONTEXT"}},
	{6, "task_prosecution", []string{"CURRENT MISSION", "ACTIVE GOALS", "TEMPORAL CONTEXT"}},
}

// Missing returns the declared inputs absent from the prompt.
func (c Contract) Missing(prompt string) []string {
	var missing []string
	for _, in := range c.Inputs {
		if !strings.Contains(prompt, in) {
			missing = append(missing, in)
		}
	}
	return missing
}

// ContractTest asserts every declared input is present in the built
// prompt. It returns an error naming the missing inputs (test helpers
// call it per layer; spec: fails when a declared input is removed).
func ContractTest(layer int, prompt string) error {
	for _, c := range LayerContracts {
		if c.Layer == layer {
			if miss := c.Missing(prompt); len(miss) > 0 {
				return fmt.Errorf("layer %d contract: missing declared inputs %v", layer, miss)
			}
			return nil
		}
	}
	return fmt.Errorf("no context contract declared for layer %d", layer)
}
