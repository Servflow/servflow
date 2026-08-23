package actions

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputInfoFor(t *testing.T) {
	base := OutputInfo{
		Kind:         OutputDynamic,
		Description:  "whatever the resource returns",
		VariantField: "resource",
		Variants: map[string]OutputInfo{
			"diff": {Kind: OutputValue, Description: "the diff text"},
			"meta": {
				Kind:   OutputObject,
				Fields: []OutputField{{Path: "title", Type: "string"}},
			},
		},
	}

	t.Run("selected variant wins", func(t *testing.T) {
		got := base.For(map[string]string{"resource": "meta"})
		assert.Equal(t, OutputObject, got.Kind)
		require.Len(t, got.Fields, 1)
		assert.Equal(t, "title", got.Fields[0].Path)
	})

	t.Run("unset selector falls back to the base shape", func(t *testing.T) {
		got := base.For(map[string]string{})
		assert.Equal(t, OutputDynamic, got.Kind)
		assert.Equal(t, "whatever the resource returns", got.Description)
	})

	t.Run("unknown selector value falls back to the base shape", func(t *testing.T) {
		got := base.For(map[string]string{"resource": "{{ .step.resource }}"})
		assert.Equal(t, OutputDynamic, got.Kind)
	})

	t.Run("output without variants resolves to itself", func(t *testing.T) {
		plain := OutputInfo{Kind: OutputValue, Description: "command output"}
		assert.Equal(t, plain, plain.For(map[string]string{"resource": "meta"}))
	})

	t.Run("a variant's own variants are not followed", func(t *testing.T) {
		nested := OutputInfo{
			VariantField: "mode",
			Variants: map[string]OutputInfo{
				"one": {
					Kind:         OutputValue,
					VariantField: "inner",
					Variants:     map[string]OutputInfo{"deep": {Kind: OutputObject}},
				},
			},
		}
		got := nested.For(map[string]string{"mode": "one", "inner": "deep"})
		assert.Equal(t, OutputValue, got.Kind)
	})
}

func TestOutputInfoJSON(t *testing.T) {
	t.Run("dynamic is written out by name", func(t *testing.T) {
		data, err := json.Marshal(OutputInfo{Kind: OutputDynamic, Description: "a remote reply"})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"kind":"dynamic"`)
	})

	t.Run("nested variants are written out by name", func(t *testing.T) {
		data, err := json.Marshal(OutputInfo{
			VariantField: "resource",
			Variants:     map[string]OutputInfo{"meta": {Kind: OutputDynamic}},
		})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"meta":{"kind":"dynamic"}`)
	})

	t.Run("a round trip through JSON keeps the value", func(t *testing.T) {
		original := OutputInfo{
			Kind:         OutputDynamic,
			VariantField: "resource",
			Variants: map[string]OutputInfo{
				"diff": {Kind: OutputValue, Description: "diff text"},
				"meta": {Kind: OutputObject, Fields: []OutputField{{Path: "title", Type: "string"}}},
			},
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var round OutputInfo
		require.NoError(t, json.Unmarshal(data, &round))
		assert.Equal(t, original, round)
	})

	t.Run("the other kinds are unchanged", func(t *testing.T) {
		for _, kind := range []OutputKind{OutputNone, OutputValue, OutputObject} {
			data, err := json.Marshal(OutputInfo{Kind: kind})
			require.NoError(t, err)
			assert.Contains(t, string(data), fmt.Sprintf(`"kind":%q`, kind))
		}
	})
}

func TestRegisterActionValidatesOutput(t *testing.T) {
	constructor := func(config json.RawMessage) (ActionExecutable, error) {
		return &mockActionExecutable{}, nil
	}
	resourceField := map[string]FieldInfo{
		"resource": {Type: FieldTypeString, Values: []string{"diff", "meta"}},
	}

	t.Run("variantField naming no config field is rejected", func(t *testing.T) {
		err := RegisterAction("output-unknown-selector", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      resourceField,
			Output: OutputInfo{
				VariantField: "kind",
				Variants:     map[string]OutputInfo{"diff": {Kind: OutputValue}},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `variantField "kind" is not a config field`)
		assert.False(t, HasRegisteredActionType("output-unknown-selector"))
	})

	t.Run("variant outside the field's values is rejected", func(t *testing.T) {
		err := RegisterAction("output-unknown-variant", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      resourceField,
			Output: OutputInfo{
				VariantField: "resource",
				Variants:     map[string]OutputInfo{"patch": {Kind: OutputValue}},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `variant "patch" is not a value of field "resource"`)
	})

	t.Run("variantField without variants is rejected", func(t *testing.T) {
		err := RegisterAction("output-empty-variants", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      resourceField,
			Output:      OutputInfo{VariantField: "resource"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has no variants")
	})

	t.Run("variants without a variantField are rejected", func(t *testing.T) {
		err := RegisterAction("output-orphan-variants", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      resourceField,
			Output:      OutputInfo{Variants: map[string]OutputInfo{"diff": {Kind: OutputValue}}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "variants without a variantField")
	})

	t.Run("a valid variant declaration registers", func(t *testing.T) {
		err := RegisterAction("output-valid-variants", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      resourceField,
			Output: OutputInfo{
				VariantField: "resource",
				Variants: map[string]OutputInfo{
					"diff": {Kind: OutputValue},
					"meta": {Kind: OutputObject, Fields: []OutputField{{Path: "title", Type: "string"}}},
				},
			},
		})
		require.NoError(t, err)

		info, err := GetInfoForAction("output-valid-variants")
		require.NoError(t, err)
		assert.Equal(t, "resource", info.Output.VariantField)
	})

	t.Run("an undescribed action registers as dynamic", func(t *testing.T) {
		err := RegisterAction("output-undescribed", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      map[string]FieldInfo{},
		})
		require.NoError(t, err)

		info, err := GetInfoForAction("output-undescribed")
		require.NoError(t, err)
		assert.Equal(t, OutputDynamic, info.Output.Kind)
	})

	t.Run("a selector field with open values accepts any variant", func(t *testing.T) {
		err := RegisterAction("output-open-selector", ActionRegistrationInfo{
			Constructor: constructor,
			Fields:      map[string]FieldInfo{"mode": {Type: FieldTypeString}},
			Output: OutputInfo{
				VariantField: "mode",
				Variants:     map[string]OutputInfo{"anything": {Kind: OutputValue}},
			},
		})
		require.NoError(t, err)
	})
}

func TestReplaceActionTypeKeepsOutput(t *testing.T) {
	err := RegisterAction("output-replaceable", ActionRegistrationInfo{
		Constructor: func(config json.RawMessage) (ActionExecutable, error) {
			return &mockActionExecutable{config: "original"}, nil
		},
		Fields: map[string]FieldInfo{},
		Output: OutputInfo{
			Kind:   OutputObject,
			Fields: []OutputField{{Path: "title", Type: "string"}},
		},
	})
	require.NoError(t, err)

	ReplaceActionType("output-replaceable", func(config json.RawMessage) (ActionExecutable, error) {
		return &mockActionExecutable{config: "replaced"}, nil
	})

	info, err := GetInfoForAction("output-replaceable")
	require.NoError(t, err)
	assert.Equal(t, OutputObject, info.Output.Kind)
	require.Len(t, info.Output.Fields, 1)
	assert.Equal(t, "title", info.Output.Fields[0].Path)
}
