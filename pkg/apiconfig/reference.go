package apiconfig

import (
	"fmt"
	"strings"
)

// Step-reference prefixes. A workflow step refers to the next step to run by a
// prefixed string id, e.g. "action.createUser", "conditional.isValid",
// "response.ok". These constants are the canonical home for those prefixes;
// requestctx aliases them for backwards compatibility.
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
// A leading "$" is stripped (backwards compatibility). An empty reference is
// terminal: the chain ends and there is no further step (terminal=true, no
// error). An unrecognized prefix is an error.
//
// This is the single source of truth for reference resolution; both the planner
// and the config validator use it so they can never disagree on what a
// reference means.
func ParseStepRef(raw string) (kind StepKind, id string, terminal bool, err error) {
	s := strings.TrimPrefix(raw, "$")
	if s == "" {
		return StepKindUnknown, "", true, nil
	}
	switch {
	case strings.HasPrefix(s, ActionConfigPrefix):
		return StepKindAction, strings.TrimPrefix(s, ActionConfigPrefix), false, nil
	case strings.HasPrefix(s, ConditionalConfigPrefix):
		return StepKindConditional, strings.TrimPrefix(s, ConditionalConfigPrefix), false, nil
	case strings.HasPrefix(s, ResponsesConfigPrefix):
		return StepKindResponse, strings.TrimPrefix(s, ResponsesConfigPrefix), false, nil
	default:
		return StepKindUnknown, s, false, fmt.Errorf(
			"invalid step reference %q: must start with %q, %q, or %q",
			raw, ActionConfigPrefix, ConditionalConfigPrefix, ResponsesConfigPrefix)
	}
}

// CanonicalStepID returns the reference with any leading "$" stripped, i.e. the
// form used as the key in a plan's step map. Empty (terminal) refs return "".
func CanonicalStepID(raw string) string {
	return strings.TrimPrefix(raw, "$")
}
