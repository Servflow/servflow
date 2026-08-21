package apiconfig

import (
	"fmt"
	"strings"
)

// Step-reference prefixes. A workflow step refers to the next step to run by a
// prefixed string id, e.g. "action.createUser", "conditional.isValid",
// "response.ok". These constants are the canonical home for those prefixes.
const (
	ActionConfigPrefix      = "action."
	ConditionalConfigPrefix = "conditional."
	ResponsesConfigPrefix   = "response."
)

// StepKind identifies which map a step reference resolves into.
type StepKind int

const (
	StepKindUnknown StepKind = iota
	StepKindAction
	StepKindConditional
	StepKindResponse
)

func (k StepKind) String() string {
	switch k {
	case StepKindAction:
		return "action"
	case StepKindConditional:
		return "conditional"
	case StepKindResponse:
		return "response"
	default:
		return "unknown"
	}
}

// ParseStepRef parses a step reference into its kind and bare id.
//
// An empty reference is terminal: the chain ends and there is no further step
// (terminal=true, no error). An unrecognized prefix is an error.
//
// This is the single source of truth for reference resolution; both the planner
// and the config validator use it so they can never disagree on what a
// reference means.
func ParseStepRef(raw string) (kind StepKind, id string, terminal bool, err error) {
	if raw == "" {
		return StepKindUnknown, "", true, nil
	}
	switch {
	case strings.HasPrefix(raw, ActionConfigPrefix):
		return StepKindAction, strings.TrimPrefix(raw, ActionConfigPrefix), false, nil
	case strings.HasPrefix(raw, ConditionalConfigPrefix):
		return StepKindConditional, strings.TrimPrefix(raw, ConditionalConfigPrefix), false, nil
	case strings.HasPrefix(raw, ResponsesConfigPrefix):
		return StepKindResponse, strings.TrimPrefix(raw, ResponsesConfigPrefix), false, nil
	default:
		return StepKindUnknown, raw, false, fmt.Errorf(
			"invalid step reference %q: must start with %q, %q, or %q",
			raw, ActionConfigPrefix, ConditionalConfigPrefix, ResponsesConfigPrefix)
	}
}
