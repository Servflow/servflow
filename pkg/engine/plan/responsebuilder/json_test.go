package responsebuilder

import (
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONResponseBuilder_BuildResponse(t *testing.T) {
	testcases := []struct {
		name      string
		template  string
		expected  string
		code      int
		variables map[string]interface{}
	}{
		{
			name:     "simple field",
			template: `{"test": "{{ .test }}"}`,
			expected: `{"test": "testvalue"}`,
			variables: map[string]interface{}{
				"test": "testvalue",
			},
			code: 200,
		},
		{
			name: "multiple fields",
			template: `{
				"field1": "{{ .value1 }}",
				"field2": "{{ .value2 }}"
			}`,
			expected: `{
				"field1": "first value",
				"field2": "second value"
			}`,
			variables: map[string]interface{}{
				"value1": "first value",
				"value2": "second value",
			},
			code: 200,
		},
		{
			name:     "nested variable",
			template: `{"nested": "{{ .parent.child }}"}`,
			expected: `{"nested": "child value"}`,
			variables: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "child value",
				},
			},
			code: 200,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewJsonResponseBuilder(tc.code, tc.template)

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, tc.variables, "")
			require.NoError(t, err)

			response, err := builder.BuildResponse(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tc.code, response.Code)
			assert.Equal(t, "application/json", response.Headers.Get("Content-Type"))
			assert.JSONEq(t, tc.expected, string(response.Body))
		})
	}
}
