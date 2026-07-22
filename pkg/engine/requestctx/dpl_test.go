package requestctx

import (
	"regexp"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/secrets"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_wrapWithFunction(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		funcWrap  string
		expected  string
		expectErr bool
	}{
		{
			name:     "success",
			input:    `{{ .variable }}`,
			funcWrap: "jsonraw",
			expected: `{{ jsonraw (.variable) }}`,
		},
		{
			name:     "multiple replacements",
			input:    `{{ .variable }} {{ printf .variable }}`,
			funcWrap: "jsonraw",
			expected: `{{ jsonraw (.variable) }} {{ jsonraw (printf .variable) }}`,
		},
		{
			name:     "with parenthesis",
			input:    `{{ .variable }} {{ printf (.variable) }}`,
			funcWrap: "jsonraw",
			expected: `{{ jsonraw (.variable) }} {{ jsonraw (printf (.variable)) }}`,
		},
		{
			name:     "function skipped",
			input:    `{{ .variable }} {{ printf (jsonraw .variable) }}`,
			funcWrap: "jsonraw",
			expected: `{{ jsonraw (.variable) }} {{ printf (jsonraw .variable) }}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotten := WrapWithFunction(tc.input, tc.funcWrap)

			assert.Equal(t, tc.expected, gotten)
		})
	}
}

func TestNormalizeActionVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces variable_actions_ prefix",
			input:    "{{ .variable_actions_greet }}",
			expected: "{{ .greet }}",
		},
		{
			name:     "replaces nested access",
			input:    "{{ .variable_actions_greet.message }}",
			expected: "{{ .greet.message }}",
		},
		{
			name:     "replaces multiple occurrences",
			input:    "{{ .variable_actions_action1 }} and {{ .variable_actions_action2 }}",
			expected: "{{ .action1 }} and {{ .action2 }}",
		},
		{
			name:     "leaves other variables unchanged",
			input:    "{{ .variable_request }} and {{ .variable_actions_test }}",
			expected: "{{ .variable_request }} and {{ .test }}",
		},
		{
			name:     "no change when no prefix",
			input:    "{{ .greet }}",
			expected: "{{ .greet }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeActionVariables(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	ctx := NewTestContext()
	err := AddRequestVariables(ctx, map[string]interface{}{"greet": map[string]interface{}{"message": "hello"}}, "")
	require.NoError(t, err)

	result, err := ExecuteTemplateString(ctx, "{{ .variable_actions_greet.message }}")
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestExecuteTemplateStringVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		values   map[string]interface{}
		expected string
	}{
		{
			name:  "has field in variable",
			input: "Hello this is a test for {{ .varmain.field.field2 }}!",
			values: map[string]interface{}{
				"varmain": map[string]interface{}{
					"field": map[string]interface{}{
						"field2": "test",
					},
				},
			},
			expected: "Hello this is a test for test!",
		},
		{
			name:  "has array",
			input: "{{ index .main.test 0 }}",
			values: map[string]interface{}{
				"main": map[string]interface{}{
					"test": []interface{}{
						"test",
					},
				},
			},
			expected: "test",
		},
		{
			name:  "has array nested",
			input: "{{ (index .main.test 0).field1 }}",
			values: map[string]interface{}{
				"main": map[string]interface{}{
					"test": []interface{}{
						map[string]interface{}{
							"field1": "value",
						},
					},
				},
			},
			expected: "value",
		},
		{
			name:     "Basic replacement",
			input:    "Hello, {{ .name}}! Today is {{ .day }}.",
			values:   map[string]interface{}{"name": "Alice", "day": "Monday"},
			expected: "Hello, Alice! Today is Monday.",
		},
		{
			name:     "No replacements found",
			input:    "Hello, {{.Name}}! Today is {{.day}}.",
			values:   map[string]interface{}{"firstname": "Alice", "weekday": "Monday"},
			expected: "Hello, ! Today is .",
		},
		{
			name:     "Empty string input",
			input:    "",
			values:   map[string]interface{}{"name": "Alice", "day": "Monday"},
			expected: "",
		},
		{
			name:     "Extra whitespace within brackets",
			input:    "Hello, {{  .name  }}!",
			values:   map[string]interface{}{"name": "Alice"},
			expected: "Hello, Alice!",
		},
		{
			name:     "Case sensitivity",
			input:    "Hello, {{ .Name}}!",
			values:   map[string]interface{}{"name": "Alice"},
			expected: "Hello, !",
		},
		{
			name:     "Multiple occurrences",
			input:    "{{ .greeting}}, {{ .greeting }}!",
			values:   map[string]interface{}{"greeting": "Hello"},
			expected: "Hello, Hello!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTestContext()
			err := AddRequestVariables(ctx, tt.values, "")
			require.NoError(t, err)
			result, err := ExecuteTemplateString(ctx, tt.input)
			require.NoError(t, err)
			if result != tt.expected {
				t.Errorf("ExecuteTemplateString(%q, %v) = %q, want %q", tt.input, tt.values, result, tt.expected)
			}
		})
	}
}

func TestRegexDoesNotSpanMultipleTemplates(t *testing.T) {
	// This test confirms the fix: the regex should NOT match across multiple template tags.
	// The input has no escaped quotes inside the templates, so nothing should match.
	regex := regexp.MustCompile(`{{[^"}]+\\"[^"}]*\\"[^}]*}}`)
	input := `{\n  \"message\": \"{{ escape .content | js }}\",\n  \"path\": \"{{ .filepath }}\"\n}`

	matches := regex.FindAllString(input, -1)

	// The regex should match nothing because there are no escaped quotes inside the templates
	assert.Len(t, matches, 0, "regex should not match when no escaped quotes inside templates")
}

func TestReplaceEscapedSecretQuotes(t *testing.T) {
	testCases := []struct {
		name     string
		in       string
		expected string
	}{
		{
			name:     "default case",
			in:       `test {{ secret \"value\" }}`,
			expected: `test {{ secret "value" }}`,
		},
		{
			name:     "multitag case",
			in:       `{{ test \"hello\" }} {{ secret "test" }} {{ test \"ho\"}} {{ test "" }} {{ {{ secret \"\" }}`,
			expected: `{{ test "hello" }} {{ secret "test" }} {{ test "ho"}} {{ test "" }} {{ {{ secret "" }}`,
		},
		{
			name:     "json with newlines and multiple templates - escaped quotes outside should be preserved",
			in:       `{\n  \"message\": \"{{ escape .content | js }}\",\n  \"path\": \"{{ .filepath }}\"\n}`,
			expected: `{\n  \"message\": \"{{ escape .content | js }}\",\n  \"path\": \"{{ .filepath }}\"\n}`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotten := replaceEscapedQuotes(tt.in)
			assert.Equal(t, tt.expected, gotten)
		})
	}
}

func TestCreateTextTemplate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		funcMap map[string]interface{}
		wantErr bool
	}{
		{
			name:    "basic template",
			config:  "Hello {{.name}}",
			funcMap: nil,
			wantErr: false,
		},
		{
			name:    "invalid template",
			config:  "Hello {{.name",
			funcMap: nil,
			wantErr: true,
		},
		{
			name:    "with secret function",
			config:  `Secret: {{secret "mysecret"}}`,
			funcMap: nil,
			wantErr: false,
		},
		{
			name:    "with custom function",
			config:  "Custom: {{mycustom}}",
			funcMap: map[string]interface{}{"mycustom": func() string { return "value" }},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := CreateTextTemplate(NewTestContext(), tt.config, tt.funcMap)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tmpl)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tmpl)
			}
		})
	}
}

func TestExecuteTemplateStringSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "basic template",
			input:    "Hello World",
			expected: "Hello World",
			wantErr:  false,
		},
		{
			name:    "invalid template",
			input:   "Hello {{.name",
			wantErr: true,
		},
		{
			name:     "with secret function",
			input:    `Secret: {{secret "mysecret"}}`,
			expected: "Secret: test",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			wantErr:  false,
		},
	}

	// Reset the secrets manager state for this test
	secrets.Reset()

	// Add a test secret directly to environment storage
	envStorage := secrets.NewEnvStorage()
	envStorage.AddSecret("mysecret", "test")
	secrets.GetManager().AddStorage(envStorage)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteTemplateString(NewTestContext(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExecuteTemplateWithActionFunctionMap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		values   map[string]interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "Test missing key",
			input:    "{{ .missingkey }}",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "Basic template execution with strip",
			input:    `Hello, {{strip .name "Mr. "}}!`,
			values:   map[string]interface{}{"name": "Mr. John"},
			expected: "Hello, John!",
			wantErr:  false,
		},
		{
			name: "escape with json nested",
			input: `
{
  "url": "http://127.0.0.1:49244",
  "body": "{\n  \"message\": \"{{ escape .variable_content | js }}\",\n  \"path\": \"{{ .variable_filepath }}\"\n}"
}`,
			values: map[string]interface{}{
				"variable_content":  `This is a "quoted text"`,
				"variable_filepath": "path",
			},
			expected: `
{
  "url": "http://127.0.0.1:49244",
  "body": "{\n  \"message\": \"This is a \\\"quoted text\\\"\",\n  \"path\": \"path\"\n}"
}`,
			wantErr: false,
		},
		{
			name:     "Test jsonout with array of digits",
			input:    `JSON Output: {{jsonout .numbers}}`,
			values:   map[string]interface{}{"numbers": []int{1, 2, 3, 4, 5}},
			expected: "JSON Output: [1,2,3,4,5]",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := NewTestContext()
			err := AddRequestVariables(ctx, tt.values, "")
			require.NoError(t, err)

			tmpl, err := CreateTextTemplate(NewTestContext(), tt.input, nil)
			require.NoError(t, err)

			result, err := ExecuteTemplateFromContext(ctx, tmpl)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func BenchmarkExecuteTemplateFromContext(b *testing.B) {
	ctx := NewTestContext()
	err := AddRequestVariables(ctx, map[string]interface{}{"variable1": "test", "variable2": "test2"}, "")
	require.NoError(b, err)
	input := "Start {{.variable1}} middle {{.variable2}} end."
	template, err := CreateTextTemplate(NewTestContext(), input, nil)
	require.NoError(b, err)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		resp, err := ExecuteTemplateFromContext(ctx, template)
		assert.NoError(b, err)
		if resp != "Start test middle test2 end." {
			b.Fatal("invalid value")
		}
	}
}
