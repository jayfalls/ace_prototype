// Package layers hosts the per-layer output contracts and the mailbox
// scheduler (spec 9.3). Each layer's decision JSON carries a "contract"
// object; these validators check the spec'd keys on every output.
package layers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Layer 2–6 contract requirements (spec 9.3). L1 has no output contract —
// it is the read-only supervisory crown.
//
// L2: beliefs {claim,p,source,credibility}, a belief summary; strategies are
// hypotheses. L3: self-model {capabilities,limits,config,state,
// measured_capacity}. L4: plan steps {id,deps[],checkpoint,success_criteria}
// and risks {risk,contingency}. L5: done_criteria per task, dwell_s, thrash,
// status {task,pct}. L6: expected_feedback (+thresholds) declared before
// acting.

// ValidateOutput checks one parsed decision's contract object for the layer.
// It returns a human-readable deficiency list (empty = valid). The engine
// retries invalid outputs with a prefilled opening, then escalates.
func ValidateOutput(layer int, contract json.RawMessage, hasAction bool) []string {
	if layer < 2 || layer > 6 {
		return nil
	}
	var c map[string]any
	if len(contract) == 0 {
		return []string{"missing contract object"}
	}
	if err := json.Unmarshal(contract, &c); err != nil {
		return []string{"contract is not a JSON object"}
	}
	var missing []string
	reqKeys := func(path string, obj map[string]any, keys ...string) {
		for _, k := range keys {
			if _, ok := obj[k]; !ok {
				missing = append(missing, path+"."+k)
			}
		}
	}
	switch layer {
	case 2:
		beliefs, _ := c["beliefs"].([]any)
		for i, b := range beliefs {
			if bm, ok := b.(map[string]any); ok {
				reqKeys(fmt.Sprintf("beliefs[%d]", i), bm, "claim", "p", "source", "credibility")
			}
		}
		if _, ok := c["belief_summary"]; !ok {
			missing = append(missing, "belief_summary")
		}
	case 3:
		sm, _ := c["self_model"].(map[string]any)
		if sm == nil {
			missing = append(missing, "self_model")
		} else {
			reqKeys("self_model", sm, "capabilities", "limits", "config", "state", "measured_capacity")
		}
	case 4:
		steps, _ := c["plan"].([]any)
		for i, s := range steps {
			if sm, ok := s.(map[string]any); ok {
				reqKeys(fmt.Sprintf("plan[%d]", i), sm, "id", "deps", "checkpoint", "success_criteria")
			}
		}
		risks, _ := c["risks"].([]any)
		for i, r := range risks {
			if rm, ok := r.(map[string]any); ok {
				reqKeys(fmt.Sprintf("risks[%d]", i), rm, "risk", "contingency")
			}
		}
		if _, ok := c["plan"]; !ok {
			missing = append(missing, "plan")
		}
	case 5:
		reqKeys("contract", c, "done_criteria", "dwell_s", "status")
		if st, ok := c["status"].(map[string]any); ok {
			reqKeys("status", st, "task", "pct")
		}
	case 6:
		// L6 must declare expected_feedback before it acts.
		if hasAction {
			if _, ok := c["expected_feedback"]; !ok {
				missing = append(missing, "expected_feedback")
			}
		}
	}
	return missing
}

// ContractHint is the prefilled opening appended to the retry prompt so the
// model resumes inside the contract object (spec 9.3: retry with prefilled
// response opening).
const ContractHint = `{"thought": "resuming with required contract fields", "contract": {`

// ContractPromptLine is the per-layer prompt requirement appended to the
// role instructions so models actually emit the contract object.
func ContractPromptLine(layer int) string {
	switch layer {
	case 2:
		return `, "contract": {"beliefs": [{"claim": "...", "p": 0.0, "source": "...", "credibility": "high|med|low"}], "belief_summary": "...", "strategies": [{"hypothesis": "..."}]}`
	case 3:
		return `, "contract": {"self_model": {"capabilities": [...], "limits": [...], "config": {...}, "state": "...", "measured_capacity": "..."}}`
	case 4:
		return `, "contract": {"plan": [{"id": "s1", "deps": [], "checkpoint": "...", "success_criteria": "..."}], "risks": [{"risk": "...", "contingency": "..."}], "inventory": "..."}`
	case 5:
		return `, "contract": {"done_criteria": "...", "dwell_s": 60, "thrash": 0, "status": {"task": "...", "pct": 0}}`
	case 6:
		return `, "contract": {"expected_feedback": "what signal confirms this action worked", "thresholds": "..."}`
	}
	return ""
}

// Deficiency formats the missing-keys list for the escalation payload.
func Deficiency(missing []string) string {
	return "layer contract deficient: " + strings.Join(missing, ", ")
}

// ContractPolicy drives the spec-9.3 retry rule: call produces one layer
// output attempt; a deficient contract retries up to three times, each
// retry carrying a note that prefills the response opening (ContractHint).
// Still deficient after three — escalate is invoked with the deficiency.
// Returns the final missing list (nil = contract satisfied) and any call
// error (which aborts retries).
func ContractPolicy(
	layer int,
	call func(note string) (contract json.RawMessage, hasAction bool, err error),
	escalate func(missing []string),
) ([]string, error) {
	var missing []string
	for attempt := 0; attempt < 3; attempt++ {
		note := ""
		if attempt > 0 {
			note = "Output contract deficient — missing: " + strings.Join(missing, ", ") +
				". Re-emit the COMPLETE JSON, opening exactly: " + ContractHint +
				" ... then the remaining fields."
		}
		contract, hasAction, err := call(note)
		if err != nil {
			return missing, err
		}
		missing = ValidateOutput(layer, contract, hasAction)
		if len(missing) == 0 {
			return nil, nil
		}
	}
	if len(missing) > 0 && escalate != nil {
		escalate(missing)
	}
	return missing, nil
}
