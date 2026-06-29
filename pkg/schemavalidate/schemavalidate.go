// Package schemavalidate holds generic JSON-Schema validation helpers shared by
// the engine config validator and the pro layer's WorkflowConfig validation.
// It is deliberately free of any apiconfig/registry dependency so both layers
// can import it one-way.
package schemavalidate

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// schemaPrinter is used only for the fallback path (kinds we don't phrase
// ourselves), where v6's own localized string is good enough.
var schemaPrinter = message.NewPrinter(language.English)

// SchemaValidationError is a single humanized JSON-schema failure.
type SchemaValidationError struct {
	// Path is the instance location of the offending value (JSON-pointer style).
	Path string
	// Keyword is the failing schema keyword (e.g. "required", "enum", "type").
	Keyword string
	// Message is the humanized, domain-aware message.
	Message string
}

func (e *SchemaValidationError) Error() string {
	return e.Message
}

// MustCompileSchema compiles a JSON Schema document (as a JSON string) with the
// santhosh-tekuri/jsonschema v6 compiler, panicking on failure. Schemas are
// static, embedded build artifacts, so a compile failure is a programming error.
func MustCompileSchema(name, schemaJSON string) *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	if err != nil {
		panic(fmt.Sprintf("schemavalidate: parse schema %s: %v", name, err))
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(name, doc); err != nil {
		panic(fmt.Sprintf("schemavalidate: add schema %s: %v", name, err))
	}
	sch, err := c.Compile(name)
	if err != nil {
		panic(fmt.Sprintf("schemavalidate: compile schema %s: %v", name, err))
	}
	return sch
}

// ValidateInstance validates an already-JSON-marshaled document against a
// compiled schema, returning humanized errors located via the given locator.
// It is the shared entry point both the engine and the pro layer use so schema
// validation behaves identically everywhere.
func ValidateInstance(schema *jsonschema.Schema, jsonDoc []byte, locate func([]string) string) ([]*SchemaValidationError, error) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonDoc))
	if err != nil {
		return nil, fmt.Errorf("failed to parse document JSON: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			return HumanizeValidationError(verr, locate), nil
		}
		return nil, err
	}
	return nil, nil
}

// HumanizeValidationError walks a v6 validation error tree and produces one
// humanized SchemaValidationError per leaf. locate turns an instance location
// (path tokens) into a domain noun, e.g. ["actions","createUser"] -> action
// "createUser". It is exported so callers can reuse the translation with their
// own (schema-aware) locator.
func HumanizeValidationError(verr *jsonschema.ValidationError, locate func([]string) string) []*SchemaValidationError {
	var out []*SchemaValidationError
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		out = append(out, &SchemaValidationError{
			Path:    "/" + strings.Join(e.InstanceLocation, "/"),
			Keyword: keywordOf(e.ErrorKind),
			Message: humanizeKind(e.ErrorKind, locate(e.InstanceLocation)),
		})
	}
	walk(verr)
	return out
}

func keywordOf(k jsonschema.ErrorKind) string {
	p := k.KeywordPath()
	if len(p) == 0 {
		return ""
	}
	return p[len(p)-1]
}

// humanizeKind phrases the common keyword failures in a domain-friendly way and
// falls back to v6's own localized string for the rest. "where" is the located
// noun for the offending instance location.
func humanizeKind(k jsonschema.ErrorKind, where string) string {
	switch t := k.(type) {
	case *kind.Required:
		if len(t.Missing) == 1 {
			return fmt.Sprintf("%s is missing required field %q", where, t.Missing[0])
		}
		return fmt.Sprintf("%s is missing required fields %s", where, strings.Join(quoteAll(t.Missing), ", "))
	case *kind.Type:
		return fmt.Sprintf("%s should be %s", where, strings.Join(t.Want, " or "))
	case *kind.Enum:
		return fmt.Sprintf("%s has an invalid value; allowed: %s", where, joinAny(t.Want))
	case *kind.AdditionalProperties:
		return fmt.Sprintf("%s has unexpected field(s) %s", where, strings.Join(quoteAll(t.Properties), ", "))
	default:
		return fmt.Sprintf("%s: %s", where, k.LocalizedString(schemaPrinter))
	}
}

func quoteAll(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}

func joinAny(vals []any) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, ", ")
}
