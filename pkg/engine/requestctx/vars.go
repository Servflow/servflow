package requestctx

import (
	"context"
	"regexp"
	"strings"
)

// Request-variable namespace constants. They describe how request and action
// outputs are keyed in the request-variable map that templates render against.
const (
	// BareVariablesPrefixStripped prefixes every entry in the request-variable
	// map; templates address them as "{{ .variable_... }}".
	BareVariablesPrefixStripped = "variable_"
	// VariableActionPrefix is the request-variable prefix under which an
	// action's stored output lives ("variable_actions_<id>").
	VariableActionPrefix = BareVariablesPrefixStripped + "actions_"
	// ErrorTagStripped is the request-variable key under which conditional
	// validation errors are collected.
	ErrorTagStripped = "error"
)

// noValue is what text/template prints for a missing key.
const noValue = "<no value>"

// escapedQuotes matches a template action holding escaped quotes, which JSON
// parsing leaves behind, e.g. {{ secret \"test\" }}.
// https://regex101.com/r/MRJoD1/1
var escapedQuotes = regexp.MustCompile(`{{[^"}]+\\"[^"}]*\\"[^}]*}}`)

// PrepareTemplate rewrites a template before it is parsed: it unescapes quotes
// inside template actions and lets ".variable_actions_<id>" be written as
// ".<id>". Every RequestContext implementation calls it, so a template means
// the same thing to each.
func PrepareTemplate(template string) string {
	out := escapedQuotes.ReplaceAllStringFunc(template, func(s string) string {
		return strings.ReplaceAll(s, `\"`, `"`)
	})
	return strings.ReplaceAll(out, "."+VariableActionPrefix, ".")
}

// StripNoValue removes the marker text/template prints for a missing key, so
// a missing variable renders as empty. Every RequestContext implementation
// calls it on a rendered template.
func StripNoValue(rendered string) string {
	return strings.ReplaceAll(rendered, noValue, "")
}

// AddRequestVariables stores variables on the request ctx carries, each key
// prefixed with prefix.
func AddRequestVariables(ctx context.Context, variables map[string]any, prefix string) error {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return err
	}
	rc.AddVariables(variables, prefix)
	return nil
}

// GetRequestVariable returns one request variable.
func GetRequestVariable(ctx context.Context, key string) (any, error) {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	return rc.Variables()[key], nil
}

// GetAllRequestVariables returns the request variables.
func GetAllRequestVariables(ctx context.Context) (map[string]any, error) {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	return rc.Variables(), nil
}
