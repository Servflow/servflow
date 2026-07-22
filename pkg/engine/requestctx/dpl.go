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

// ReplaceVariableValuesInContext scans the input string for placeholders formatted as {{key}}
// and replaces each placeholder with the corresponding value from the provided map.
// Placeholders are expected to be enclosed in double curly braces with an optional
// amount of whitespace inside the braces around the key. The function performs a lookup
// in the provided values map using the trimmed key and replaces the placeholder with the
// value found. If a placeholder's key does not exist in the map, the placeholder is left
// unchanged in the output string.
//
// Parameters:
//   - in: The input string containing placeholders to be replaced. Placeholders should
//     be in the format {{key}}, where key corresponds to a key in the values map.
//   - values: A map[string]string where each key is a placeholder found in the input string
//     without the curly braces and whitespace, and the value is the string to replace
//     the placeholder with.
//
// Returns:
//   - A string where all recognized placeholders have been replaced with corresponding
//     values from the map. If a key from the placeholder is not found in the map, the
//     placeholder remains unchanged in the returned string.
//
// Example Usage:
//
//	input := "Hello, {{name}}! Today is {{day}}."
//	values in context := map[string]string{"name": "Alice", "day": "Wednesday"}
//	result := ReplaceVariableValues(input, values)
//	// result will be "Hello, Alice! Today is Wednesday."
//
// This function is useful in templating scenarios where dynamic data needs to be inserted
// into a predefined format. It supports basic templating functionalities and is designed
// to handle common use cases with simplicity and efficiency.
//
// Note:
//   - The function is case-sensitive, which means that the keys in the values map must
//     match exactly with the case used in the placeholders.
//   - Extra spaces inside the placeholder braces are ignored, so '{{  name  }}' is treated
//     as '{{name}}'.
func ReplaceVariableValuesInContext(ctx context.Context, in string) (string, error) {
	values, err := GetAllRequestVariables(ctx)
	if err != nil {
		return in, err
	}
	return ReplaceVariableValues(in, values)
}

func ReplaceVariableValues(in string, values map[string]interface{}) (string, error) {
	tmpl, err := createTemplate(in, nil, false)
	if err != nil {
		return in, fmt.Errorf("error processing template: %w", err)
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, values); err != nil {
		return "", fmt.Errorf("error processing template: %w", err)
	}

	return strings.ReplaceAll(buff.String(), noValue, ""), nil
}

// createTemplate creates a template using base functions only (no request context).
// For v1 backward compatibility - used by ReplaceVariableValues.
func createTemplate(in string, funcMap template.FuncMap, wrapJSON bool) (*template.Template, error) {
	funcMap = getBaseFuncMap(funcMap)
	replaced := replaceEscapedQuotes(in)
	replaced = normalizeActionVariables(replaced)
	if wrapJSON {
		replaced = wrapWithJSON(replaced)
	}
	return template.New("input").Option("missingkey=zero").Funcs(funcMap).Parse(replaced)
}

// getBaseFuncMap returns base template functions without request context.
// Used for backward compatibility with code that doesn't have a RequestContext.
func getBaseFuncMap(funcMap template.FuncMap) template.FuncMap {
	m := template.FuncMap{
		"strip":        tmplStripText,
		"jsonout":      jsonOut,
		"pluck":        tmplPluck,
		"escape":       stringEscape,
		"stringescape": stringEscape,
		"jsonraw":      jsonRaw,
		"join":         tmplJoin,
		"hash":         tmplHash,
		"now":          now,
		"secret":       secret,
	}
	for k, v := range funcMap {
		m[k] = v
	}
	return m
}

func normalizeActionVariables(in string) string {
	return strings.ReplaceAll(in, "."+VariableActionPrefix, ".")
}

// DEPRECATED jsonout will be added in action generation
func wrapWithJSON(in string) string {
	replaced := strings.ReplaceAll(in, "\"{{", "{{ jsonout (")
	replaced = strings.ReplaceAll(replaced, "}}\"", ")}}")
	return replaced
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

func BaseParseTextTemplate(rawString string, funcMap template.FuncMap) (string, error) {
	tmpl, err := createTemplate(rawString, funcMap, false)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, nil); err != nil {
		return "", fmt.Errorf("error processing template: %w", err)
	}
	return strings.ReplaceAll(buff.String(), noValue, ""), nil
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
