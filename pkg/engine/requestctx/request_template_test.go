package requestctx

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalTemplate(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		values   map[string]interface{}
		expected string
	}{
		{
			name:     "Email function valid",
			template: `{{email .email "email"}}`,
			values:   map[string]interface{}{"email": "test@example.com"},
			expected: "true",
		},
		{
			name:     "Email function invalid",
			template: `{{email .email "email"}}`,
			values:   map[string]interface{}{"email": "not-an-email"},
			expected: "false",
		},
		{
			name:     "Empty function with non-empty",
			template: `{{empty .value "value"}}`,
			values:   map[string]interface{}{"value": "non-empty"},
			expected: "false",
		},
		{
			name:     "Empty function with empty",
			template: `{{empty .value "value"}}`,
			values:   map[string]interface{}{"value": ""},
			expected: "true",
		},
		{
			name:     "NotEmpty function with non-empty",
			template: `{{notempty .value "value"}}`,
			values:   map[string]interface{}{"value": "non-empty"},
			expected: "true",
		},
		{
			name:     "NotEmpty function with empty",
			template: `{{notempty .value "value"}}`,
			values:   map[string]interface{}{"value": ""},
			expected: "false",
		},
		{
			name:     "Email function error enabled",
			template: `{{email .email "email"}}`,
			values:   map[string]interface{}{"email": "not-an-email"},
			expected: "false",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := NewTestContext()
			err := AddRequestVariables(ctx, testCase.values, "")
			require.NoError(t, err)

			rCtx, ok := FromContext(ctx)
			require.True(t, ok)

			tmpl, err := template.New("template").Funcs(rCtx.ConditionalTemplateFunctions()).Parse(testCase.template)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, rCtx.requestVariables)
			require.NoError(t, err)

			result := buf.Bytes()
			assert.Equal(t, testCase.expected, string(result))

			if testCase.expected == "false" {
				assert.NotEmpty(t, rCtx.validationErrors)
			}
		})
	}
}
