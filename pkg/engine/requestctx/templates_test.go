package requestctx

import (
	"context"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStringEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "basic string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with quotes",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "string with backslashes",
			input:    `C:\path\to\file`,
			expected: `C:\\path\\to\\file`,
		},
		{
			name:     "string with newlines and tabs",
			input:    "hello\nworld\t!",
			expected: `hello\nworld\t!`,
		},
		{
			name:     "string with control characters",
			input:    "hello\b\f\rworld",
			expected: `hello\b\f\rworld`,
		},
		{
			name:     "string with unicode",
			input:    "hello 世界",
			expected: "hello 世界",
		},
		{
			name:     "string with all special characters",
			input:    "\\\"\n\r\t\b\ftest",
			expected: `\\\"\n\r\t\b\ftest`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringEscape(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateFunctions(t *testing.T) {
	tests := []struct {
		name          string
		templateInput string
		funcMap       template.FuncMap
		values        map[string]interface{}
		expected      string
		wantErr       bool
	}{
		{
			name:          "StripText function",
			templateInput: `Strip: {{strip .input "Strip "}}`,
			values:        map[string]interface{}{"input": "Strip this"},
			expected:      "Strip: this",
			wantErr:       false,
		},
		{
			name:          "json out and pluck function",
			templateInput: `Pluck: {{ jsonout (pluck .input "mainkey") }}`,
			values: map[string]interface{}{
				"input": []map[string]interface{}{
					{
						"mainkey":   "Pluck",
						"secondkey": "second",
					},
					{
						"mainkey":   "Item to pluck",
						"secondkey": "second",
					},
				},
			},
			expected: `Pluck: ["Pluck","Item to pluck"]`,
			wantErr:  false,
		},
		{
			name:          "json out and pluck map function",
			templateInput: `Pluck: {{ jsonout (pluck .input "mainkey") }}`,
			values: map[string]interface{}{
				"input": map[string]interface{}{
					"mainkey":   "Pluck",
					"secondkey": "second",
				},
			},
			expected: `Pluck: Pluck`,
			wantErr:  false,
		},
		{
			name:          "string escape direct usage",
			templateInput: `Escaped: {{stringescape .input}}`,
			values: map[string]interface{}{
				"input": "hello \"world\"\nwith\tspecial chars\\",
			},
			expected: `Escaped: hello \"world\"\nwith\tspecial chars\\`,
			wantErr:  false,
		},
		{
			name:          "string escape with empty string",
			templateInput: `Escaped: {{stringescape .input}}`,
			values: map[string]interface{}{
				"input": "",
			},
			expected: `Escaped: `,
			wantErr:  false,
		},
		{
			name:          "JsonOut function",
			templateInput: `Output: {{jsonout .input}}`,
			values:        map[string]interface{}{"input": map[string]interface{}{"key": "value"}},
			expected:      `Output: {"key":"value"}`,
			wantErr:       false,
		},
		{
			name:          "Jsonout and index function",
			templateInput: `Output: {{ jsonout ((index .variable_stored.choices 0).message.content) }}`,
			values: map[string]interface{}{
				"variable_stored": map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"message": map[string]interface{}{
								"content": "value",
							},
						},
					},
				},
			},
			wantErr:  false,
			expected: `Output: value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := CreateTextTemplate(context.Background(), tt.templateInput, tt.funcMap)
			require.NoError(t, err)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var result string
			var errExec error

			ctx := NewTestContext()
			err = AddRequestVariables(ctx, tt.values, "")
			require.NoError(t, err)

			result, errExec = ExecuteTemplateFromContext(ctx, tmpl)
			if tt.wantErr {
				assert.Error(t, errExec)
			} else {
				assert.NoError(t, errExec)
				assert.Equal(t, tt.expected, result)
			}
		})
	}

	t.Run("Hash function tests", func(t *testing.T) {
		hashTests := []struct {
			name          string
			templateInput string
			values        map[string]interface{}
			wantErr       bool
		}{
			{
				name:          "Hash string",
				templateInput: `{{hash .item}}`,
				values:        map[string]interface{}{"item": "test-string"},
				wantErr:       false,
			},
			{
				name:          "Hash integer",
				templateInput: `{{hash .item}}`,
				values:        map[string]interface{}{"item": 12345},
				wantErr:       false,
			},
			{
				name:          "Hash boolean",
				templateInput: `{{hash .item}}`,
				values:        map[string]interface{}{"item": true},
				wantErr:       false,
			},
			{
				name:          "Hash array",
				templateInput: `{{hash .item}}`,
				values:        map[string]interface{}{"item": []string{"a", "b", "c"}},
				wantErr:       false,
			},
			{
				name:          "Hash map",
				templateInput: `{{hash .item}}`,
				values:        map[string]interface{}{"item": map[string]interface{}{"key": "value"}},
				wantErr:       false,
			},
			{
				name:          "Hash nested structure",
				templateInput: `{{hash .item}}`,
				values: map[string]interface{}{
					"item": map[string]interface{}{
						"id":   123,
						"name": "test",
						"tags": []string{"tag1", "tag2"},
					},
				},
				wantErr: false,
			},
		}

		for _, tt := range hashTests {
			t.Run(tt.name, func(t *testing.T) {
				tmpl, err := CreateTextTemplate(context.Background(), tt.templateInput, nil)
				require.NoError(t, err)

				ctx := NewTestContext()
				err = AddRequestVariables(ctx, tt.values, "")
				require.NoError(t, err)

				result, errExec := ExecuteTemplateFromContext(ctx, tmpl)
				if tt.wantErr {
					assert.Error(t, errExec)
				} else {
					assert.NoError(t, errExec)

					// Calculate expected hash using the same logic as tmplHash
					var expected string
					item := tt.values["item"]
					expected = tmplHash(item)

					assert.Equal(t, expected, result)
				}
			})
		}
	})

	t.Run("Join function tests", func(t *testing.T) {
		joinTests := []struct {
			name          string
			templateInput string
			values        map[string]interface{}
			expected      string
			wantErr       bool
		}{
			{
				name:          "Join string array",
				templateInput: `{{join .items ","}}`,
				values:        map[string]interface{}{"items": []string{"a", "b", "c"}},
				expected:      "a,b,c",
				wantErr:       false,
			},
			{
				name:          "Join interface array",
				templateInput: `{{join .items "-"}}`,
				values:        map[string]interface{}{"items": []interface{}{"x", "y", "z"}},
				expected:      "x-y-z",
				wantErr:       false,
			},
			{
				name:          "Join with empty separator",
				templateInput: `{{join .items ""}}`,
				values:        map[string]interface{}{"items": []string{"1", "2", "3"}},
				expected:      "123",
				wantErr:       false,
			},
			{
				name:          "Join non-array returns original",
				templateInput: `{{join .item ","}}`,
				values:        map[string]interface{}{"item": "not-an-array"},
				expected:      "not-an-array",
				wantErr:       false,
			},
			{
				name:          "Join empty array",
				templateInput: `{{join .items ","}}`,
				values:        map[string]interface{}{"items": []string{}},
				expected:      "",
				wantErr:       false,
			},
		}

		for _, tt := range joinTests {
			t.Run(tt.name, func(t *testing.T) {
				tmpl, err := CreateTextTemplate(context.Background(), tt.templateInput, nil)
				require.NoError(t, err)

				ctx := NewTestContext()
				err = AddRequestVariables(ctx, tt.values, "")
				require.NoError(t, err)

				result, errExec := ExecuteTemplateFromContext(ctx, tmpl)
				if tt.wantErr {
					assert.Error(t, errExec)
				} else {
					assert.NoError(t, errExec)
					assert.Equal(t, tt.expected, result)
				}
			})
		}
	})
}
