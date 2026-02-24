package javascript

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutable_Type(t *testing.T) {
	exec, err := NewExecutable(Config{Script: "function servflowRun() { return 1; }"})
	require.NoError(t, err)
	assert.Equal(t, "javascript", exec.Type())
}

func TestExecutable_Config(t *testing.T) {
	exec, err := NewExecutable(Config{Script: "function servflowRun() { return 1; }"})
	require.NoError(t, err)
	assert.Equal(t, "", exec.Config())
}

func TestNewExecutable(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty script",
			config:      Config{Script: ""},
			expectError: true,
			errorMsg:    "script is required",
		},
		{
			name:        "invalid script syntax",
			config:      Config{Script: "function servflowRun( { return 1; }"},
			expectError: true,
			errorMsg:    "failed to compile script",
		},
		{
			name:        "invalid dependencies syntax",
			config:      Config{Script: "function servflowRun() { return 1; }", Dependencies: "var x = {"},
			expectError: true,
			errorMsg:    "failed to compile dependencies",
		},
		{
			name:        "valid script",
			config:      Config{Script: "function servflowRun() { return 1; }"},
			expectError: false,
		},
		{
			name:        "valid script with dependencies",
			config:      Config{Script: "function servflowRun() { return helper(); }", Dependencies: "function helper() { return 42; }"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutable(tt.config)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, exec)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, exec)
			}
		})
	}
}

func TestExecutable_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		variables   map[string]interface{}
		expected    interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:     "simple return value",
			config:   Config{Script: "function servflowRun() { return 42; }"},
			expected: int64(42),
		},
		{
			name:     "return string",
			config:   Config{Script: "function servflowRun() { return 'hello'; }"},
			expected: "hello",
		},
		{
			name:     "return object",
			config:   Config{Script: "function servflowRun() { return {foo: 'bar', num: 123}; }"},
			expected: map[string]interface{}{"foo": "bar", "num": int64(123)},
		},
		{
			name:     "return array",
			config:   Config{Script: "function servflowRun() { return [1, 2, 3]; }"},
			expected: []interface{}{int64(1), int64(2), int64(3)},
		},
		{
			name:      "access request variables",
			config:    Config{Script: "function servflowRun(vars) { return vars.name + ' ' + vars.age; }"},
			variables: map[string]interface{}{"name": "John", "age": "30"},
			expected:  "John 30",
		},
		{
			name:      "access nested variables",
			config:    Config{Script: "function servflowRun(vars) { return vars.user.name; }"},
			variables: map[string]interface{}{"user": map[string]interface{}{"name": "Alice"}},
			expected:  "Alice",
		},
		{
			name: "use dependencies",
			config: Config{
				Script:       "function servflowRun(vars) { return multiply(vars.x, vars.y); }",
				Dependencies: "function multiply(a, b) { return a * b; }",
			},
			variables: map[string]interface{}{"x": 5, "y": 3},
			expected:  int64(15),
		},
		{
			name:        "missing servflowRun function",
			config:      Config{Script: "function otherFunction() { return 1; }"},
			expectError: true,
			errorMsg:    "servflowRun function not defined",
		},
		{
			name:        "servflowRun is not a function",
			config:      Config{Script: "var servflowRun = 42;"},
			expectError: true,
			errorMsg:    "servflowRun is not a function",
		},
		{
			name:        "runtime error in script",
			config:      Config{Script: "function servflowRun() { return undefinedVar.property; }"},
			expectError: true,
			errorMsg:    "failed to execute servflowRun",
		},
		{
			name:     "return null",
			config:   Config{Script: "function servflowRun() { return null; }"},
			expected: nil,
		},
		{
			name:     "return undefined",
			config:   Config{Script: "function servflowRun() { return undefined; }"},
			expected: nil,
		},
		{
			name:     "return boolean true",
			config:   Config{Script: "function servflowRun() { return true; }"},
			expected: true,
		},
		{
			name:     "return boolean false",
			config:   Config{Script: "function servflowRun() { return false; }"},
			expected: false,
		},
		{
			name:      "transform variables",
			config:    Config{Script: "function servflowRun(vars) { return { result: vars.items.map(function(i) { return i * 2; }) }; }"},
			variables: map[string]interface{}{"items": []interface{}{1, 2, 3}},
			expected:  map[string]interface{}{"result": []interface{}{int64(2), int64(4), int64(6)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutable(tt.config)
			require.NoError(t, err)

			ctx := requestctx.NewTestContext()
			if tt.variables != nil {
				err := requestctx.AddRequestVariables(ctx, tt.variables, "")
				require.NoError(t, err)
			}

			result, err := exec.Execute(ctx, "")
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExecutable_Execute_NoContext(t *testing.T) {
	exec, err := NewExecutable(Config{Script: "function servflowRun() { return 1; }"})
	require.NoError(t, err)

	_, err = exec.Execute(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get request variables")
}
