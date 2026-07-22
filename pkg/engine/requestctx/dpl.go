//go:generate mockgen -source dpl.go -destination dpl_mocks.go -package requestctx
package requestctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// TODO move to same package as requestctx

var (
	ErrInvalidVariablePath = errors.New("invalid variable path")
	ErrMissingVariable     = errors.New("variable not found")
)

type DataType int

const noValue = "<no value>"

// Request-variable namespace constants. Unlike the step-reference prefixes
// (whose canonical home is the apiconfig package), these describe how request
// and action outputs are keyed in the request-variable map that templates
// render against, so their home is here in the request layer.
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

const (
	// regexMatchString is for replacing escaped quotes in templates due to json parsing
	// https://regex101.com/r/MRJoD1/1
	regexMatchString = `{{[^"}]+\\"[^"}]*\\"[^}]*}}`

	// https://regex101.com/r/2eSmua/2
	regexMatchTemplate = `\{\{ ([^{}]+) \}\}`
)

var (
	quoteRegex    *regexp.Regexp
	templateRegex *regexp.Regexp
)

func init() {
	quoteRegex = regexp.MustCompile(regexMatchString)
	templateRegex = regexp.MustCompile(regexMatchTemplate)

}

func normalizeActionVariables(in string) string {
	return strings.ReplaceAll(in, "."+VariableActionPrefix, ".")
}

// replaceEscapedQuotes replaces escaped quotes (\") with
// (") in input and returns the replaced string.
// the function should only replace escaped strings in templates
// e.g. {{ secret \"test\"}}
func replaceEscapedQuotes(input string) string {
	return quoteRegex.ReplaceAllStringFunc(input, func(s string) string {
		return strings.ReplaceAll(s, `\"`, `"`)
	})
}

func WrapWithFunction(template, funcWrap string) string {
	replaced := templateRegex.ReplaceAllStringFunc(template, func(s string) string {
		submatches := templateRegex.FindStringSubmatch(s)
		if strings.Contains(submatches[1], funcWrap) {
			return s
		}
		return fmt.Sprintf("{{ %s (%s) }}", funcWrap, submatches[1])
	})
	return replaced
}

// CreateTextTemplate is what should be called in config items to create a template;
// it also loads up the template variables stored in the request context.
// This is the v1 entry point - uses RequestContext.getFuncMap for all template functions.
func CreateTextTemplate(reqCtx context.Context, config string, funcMap template.FuncMap) (*template.Template, error) {
	rCtx, ok := FromContext(reqCtx)
	if !ok || rCtx == nil {
		return nil, ErrNoContext
	}
	if funcMap == nil {
		funcMap = template.FuncMap{}
	}
	// Merge any additional functions provided by caller
	for k, v := range rCtx.TemplateFunctions() {
		funcMap[k] = v
	}
	// Use RequestContext's createTemplate which includes all functions (base + request-scoped)
	return rCtx.createTemplate(config, funcMap)
}

func ExecuteTemplateFromContext(ctx context.Context, tmpl *template.Template) (string, error) {
	values, err := GetAllRequestVariables(ctx)
	if err != nil {
		return "", fmt.Errorf("error executing template:: %w", err)
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, values); err != nil {
		return "", fmt.Errorf("error processing template: %w", err)
	}

	return strings.ReplaceAll(buff.String(), noValue, ""), nil
}

// ExecuteTemplateString parses config as a template against the request context
// and renders it in one step. It is the common case of CreateTextTemplate
// followed by ExecuteTemplateFromContext; callers needing a custom funcMap or
// template reuse should use those directly.
func ExecuteTemplateString(ctx context.Context, config string) (string, error) {
	tmpl, err := CreateTextTemplate(ctx, config, nil)
	if err != nil {
		return "", err
	}
	return ExecuteTemplateFromContext(ctx, tmpl)
}
