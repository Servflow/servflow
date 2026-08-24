package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// An action's result is stored as a request variable under the action's own id,
// so a later step reads it as "{{ .stepID }}" or, when the result is a map,
// "{{ .stepID.some.path }}". Nothing in the registry used to say which of those
// two an action produced, or what paths were inside it, so the dashboard could
// only ever offer the bare id. The types here are that missing description.

// OutputKind says how much of an action's output is known before it runs.
type OutputKind string

const (
	// OutputDynamic is a shape that depends on data the action has not seen
	// yet — a remote API's response, a stored value, another workflow's reply.
	// The output is offered whole and the caller picks paths out of it. This is
	// the zero value, so an action nobody has described yet keeps working.
	OutputDynamic OutputKind = ""
	// OutputNone is an action that publishes no request variable at all.
	OutputNone OutputKind = "none"
	// OutputValue is a single value with nothing to address inside it, read as
	// "{{ .stepID }}".
	OutputValue OutputKind = "value"
	// OutputObject is a map whose keys are known, each listed in Fields and read
	// as "{{ .stepID.<path> }}".
	OutputObject OutputKind = "object"
)

// OutputField is one addressable path inside an action's output.
type OutputField struct {
	// Path is what follows the step id, dotted for nesting: "meta.title" is read
	// as "{{ .stepID.meta.title }}".
	Path        string `json:"path"`
	Type        string `json:"type"` // string, number, boolean, object, array, or any
	Description string `json:"description"`
}

// OutputInfo describes an action's output.
//
// An action whose shape is chosen by one of its own config fields — a resource
// selector, a mode — sets VariantField to that field's name and lists a shape
// per value in Variants. Variants are plain OutputInfo but only one level deep:
// a variant's own VariantField is never consulted.
type OutputInfo struct {
	Kind OutputKind `json:"kind"`
	// Description says in one line what the output holds. It is the only
	// guidance a dynamic output can offer, so it is worth naming the paths a
	// caller is likely to want.
	Description  string                `json:"description,omitempty"`
	Fields       []OutputField         `json:"fields,omitempty"`
	VariantField string                `json:"variantField,omitempty"`
	Variants     map[string]OutputInfo `json:"variants,omitempty"`
}

// MarshalJSON writes the kind out in full.
//
// OutputDynamic is the empty string so that an undescribed action is dynamic
// without anyone writing it down, but a reader of the generated catalog should
// not have to know that. On the wire the kind is always one of the four names.
func (o OutputInfo) MarshalJSON() ([]byte, error) {
	type wire OutputInfo
	out := wire(o)
	if out.Kind == OutputDynamic {
		out.Kind = "dynamic"
	}
	return json.Marshal(out)
}

// UnmarshalJSON accepts the name MarshalJSON writes as well as the empty
// string, so a value survives a round trip through JSON unchanged.
func (o *OutputInfo) UnmarshalJSON(data []byte) error {
	type wire OutputInfo
	var in wire
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	if in.Kind == "dynamic" {
		in.Kind = OutputDynamic
	}
	*o = OutputInfo(in)
	return nil
}

// For resolves the output shape of one configured instance of an action, where
// config holds that instance's config field values.
//
// A variant output falls back to the base shape while its selector is unset or
// holds a value no variant claims, which is what a half-filled form in the
// dashboard looks like.
func (o OutputInfo) For(config map[string]string) OutputInfo {
	if o.VariantField == "" {
		return o
	}
	variant, ok := o.Variants[config[o.VariantField]]
	if !ok {
		return o
	}
	return variant
}

// validate rejects a description that cannot be resolved against the action's
// own config fields. Registration is the only moment this can be caught: once
// the dashboard is reading a selector that names no field, all it can do is
// offer the wrong variables and let them resolve to empty at run time.
func (o OutputInfo) validate(fields map[string]FieldInfo) error {
	if o.VariantField == "" {
		if len(o.Variants) > 0 {
			return errors.New("output declares variants without a variantField")
		}
		return nil
	}
	field, ok := fields[o.VariantField]
	if !ok {
		return fmt.Errorf("output variantField %q is not a config field", o.VariantField)
	}
	if len(o.Variants) == 0 {
		return fmt.Errorf("output variantField %q has no variants", o.VariantField)
	}
	// When the selector has a closed set of values, every variant must name one
	// of them. A variant keyed on a value the field cannot hold is unreachable.
	if len(field.Values) > 0 {
		for value := range o.Variants {
			if !slices.Contains(field.Values, value) {
				return fmt.Errorf("output variant %q is not a value of field %q", value, o.VariantField)
			}
		}
	}
	return nil
}
