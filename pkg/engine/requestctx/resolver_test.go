package requestctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]interface{}
		template  string
		expected  string
		wantErr   bool
	}{
		{
			name:      "simple variable",
			variables: map[string]interface{}{"name": "test"},
			template:  "Hello {{ .name }}",
			expected:  "Hello test",
		},
		{
			name:      "nested variable",
			variables: map[string]interface{}{"user": map[string]interface{}{"name": "John", "age": 30}},
			template:  "{{ .user.name }} is {{ .user.age }}",
			expected:  "John is 30",
		},
		{
			name:      "no variables",
			variables: map[string]interface{}{},
			template:  "static text",
			expected:  "static text",
		},
		{
			name:      "missing variable returns empty",
			variables: map[string]interface{}{},
			template:  "Hello {{ .missing }}",
			expected:  "Hello ",
		},
		{
			name:      "with hash function",
			variables: map[string]interface{}{"value": "test"},
			template:  "{{ hash .value }}",
			expected:  "098f6bcd4621d373cade4e832627b4f6",
		},
		{
			name:      "invalid template syntax",
			variables: map[string]interface{}{},
			template:  "{{ .invalid syntax",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTestContext()
			err := AddRequestVariables(ctx, tt.variables, "")
			require.NoError(t, err)

			rc, ok := FromContext(ctx)
			require.True(t, ok)

			result, err := rc.Resolve(ctx, tt.template)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveBatch(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]interface{}
		templates []string
		expected  []string
		wantErr   bool
	}{
		{
			name:      "empty templates",
			variables: map[string]interface{}{},
			templates: []string{},
			expected:  []string{},
		},
		{
			name:      "single template",
			variables: map[string]interface{}{"x": "y"},
			templates: []string{"{{ .x }}"},
			expected:  []string{"y"},
		},
		{
			name:      "multiple templates",
			variables: map[string]interface{}{"a": "value_a", "b": "value_b", "c": "value_c"},
			templates: []string{"{{ .a }}", "{{ .b }}", "{{ .c }}"},
			expected:  []string{"value_a", "value_b", "value_c"},
		},
		{
			name:      "mixed static and dynamic",
			variables: map[string]interface{}{"name": "John"},
			templates: []string{"static", "Hello {{ .name }}", "{{ .name }} Doe"},
			expected:  []string{"static", "Hello John", "John Doe"},
		},
		{
			name:      "invalid template in batch",
			variables: map[string]interface{}{},
			templates: []string{"valid", "{{ .invalid syntax"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTestContext()
			err := AddRequestVariables(ctx, tt.variables, "")
			require.NoError(t, err)

			rc, ok := FromContext(ctx)
			require.True(t, ok)

			results, err := rc.ResolveBatch(ctx, tt.templates...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, results)
		})
	}
}

func TestActionTemplateFunction(t *testing.T) {
	tests := []struct {
		name          string
		actionOutputs map[string]interface{}
		template      string
		expected      string
	}{
		{
			name: "simple action output",
			actionOutputs: map[string]interface{}{
				"fetch_user": map[string]interface{}{
					"id":   123,
					"name": "John",
				},
			},
			template: `{{ (action "fetch_user").name }}`,
			expected: "John",
		},
		{
			name: "string action output",
			actionOutputs: map[string]interface{}{
				"get_token": "abc123",
			},
			template: `Token: {{ action "get_token" }}`,
			expected: "Token: abc123",
		},
		{
			name:          "missing action returns empty",
			actionOutputs: map[string]interface{}{},
			template:      `{{ action "nonexistent" }}`,
			expected:      "", // missingkey=zero returns empty string
		},
		{
			name: "action with nested access",
			actionOutputs: map[string]interface{}{
				"api_response": map[string]interface{}{
					"data": map[string]interface{}{
						"items": []interface{}{"a", "b", "c"},
					},
				},
			},
			template: `{{ index ((action "api_response").data).items 0 }}`,
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTestContext()

			// Add action outputs to context
			err := AddRequestVariables(ctx, tt.actionOutputs, "")
			require.NoError(t, err)

			rc, ok := FromContext(ctx)
			require.True(t, ok)

			result, err := rc.Resolve(ctx, tt.template)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveWithRequestFunctions(t *testing.T) {
	// Test that custom request functions (like param, header) work in Resolve
	ctx := NewTestContext()

	rc, ok := FromContext(ctx)
	require.True(t, ok)

	// Add a custom request function
	rc.AddRequestTemplateFunctions(map[string]any{
		"customFunc": func(s string) string {
			return "custom:" + s
		},
	})

	result, err := rc.Resolve(ctx, `{{ customFunc "test" }}`)
	require.NoError(t, err)
	assert.Equal(t, "custom:test", result)
}

func TestResolveConcurrency(t *testing.T) {
	// Test that Resolve is safe to call concurrently
	ctx := NewTestContext()
	err := AddRequestVariables(ctx, map[string]interface{}{
		"value": "test",
	}, "")
	require.NoError(t, err)

	rc, ok := FromContext(ctx)
	require.True(t, ok)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result, err := rc.Resolve(context.Background(), "{{ .value }}")
			assert.NoError(t, err)
			assert.Equal(t, "test", result)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
