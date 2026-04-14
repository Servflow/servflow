package javascript

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/gorilla/mux"
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
	assert.Equal(t, "function servflowRun() { return 1; }", exec.Config())
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

			result, _, err := exec.Execute(ctx, tt.config.Script)
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

	_, _, err = exec.Execute(context.Background(), exec.Config())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get request variables")
}

func TestExecutable_Execute_RequestBody(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		body     string
		expected interface{}
	}{
		{
			name:     "access raw request body",
			script:   "function servflowRun(vars, request_body) { return request_body; }",
			body:     `{"message":"hello"}`,
			expected: `{"message":"hello"}`,
		},
		{
			name:     "parse JSON request body",
			script:   "function servflowRun(vars, request_body) { var data = JSON.parse(request_body); return data.message; }",
			body:     `{"message":"hello world"}`,
			expected: "hello world",
		},
		{
			name:     "empty body returns empty string",
			script:   "function servflowRun(vars, request_body) { return request_body === ''; }",
			body:     "",
			expected: true,
		},
		{
			name:     "access body with nested JSON",
			script:   "function servflowRun(vars, request_body) { var data = JSON.parse(request_body); return data.user.name; }",
			body:     `{"user":{"name":"Alice","age":30}}`,
			expected: "Alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutable(Config{Script: tt.script})
			require.NoError(t, err)

			ctx := requestctx.NewTestContext()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			ctx = plan.WithRequest(ctx, req)

			result, _, err := exec.Execute(ctx, tt.script)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutable_Execute_Params(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		setupReq func() *http.Request
		expected interface{}
	}{
		{
			name:   "access query params",
			script: "function servflowRun(vars, request_body, params) { return params.id; }",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test?id=123", nil)
			},
			expected: "123",
		},
		{
			name:   "access multiple query params",
			script: "function servflowRun(vars, request_body, params) { return params.name + '-' + params.age; }",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test?name=John&age=30", nil)
			},
			expected: "John-30",
		},
		{
			name:   "access URL path params",
			script: "function servflowRun(vars, request_body, params) { return params.userId; }",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users/456", nil)
				req = mux.SetURLVars(req, map[string]string{"userId": "456"})
				return req
			},
			expected: "456",
		},
		{
			name:   "URL path params take precedence over query params",
			script: "function servflowRun(vars, request_body, params) { return params.id; }",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test?id=query", nil)
				req = mux.SetURLVars(req, map[string]string{"id": "path"})
				return req
			},
			expected: "path",
		},
		{
			name:   "empty params returns empty object",
			script: "function servflowRun(vars, request_body, params) { return Object.keys(params).length; }",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test", nil)
			},
			expected: int64(0),
		},
		{
			name:   "combined path and query params",
			script: "function servflowRun(vars, request_body, params) { return params.userId + '-' + params.format; }",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users/789?format=json", nil)
				req = mux.SetURLVars(req, map[string]string{"userId": "789"})
				return req
			},
			expected: "789-json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutable(Config{Script: tt.script})
			require.NoError(t, err)

			ctx := requestctx.NewTestContext()
			req := tt.setupReq()
			ctx = plan.WithRequest(ctx, req)

			result, _, err := exec.Execute(ctx, tt.script)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutable_Execute_AllParameters(t *testing.T) {
	script := `function servflowRun(vars, request_body, params) {
		var body = JSON.parse(request_body);
		return {
			varName: vars.name,
			bodyMessage: body.message,
			paramId: params.id
		};
	}`

	exec, err := NewExecutable(Config{Script: script})
	require.NoError(t, err)

	ctx := requestctx.NewTestContext()
	err = requestctx.AddRequestVariables(ctx, map[string]interface{}{"name": "TestUser"}, "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/test?id=999", strings.NewReader(`{"message":"hello"}`))
	ctx = plan.WithRequest(ctx, req)

	result, _, err := exec.Execute(ctx, script)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"varName":     "TestUser",
		"bodyMessage": "hello",
		"paramId":     "999",
	}
	assert.Equal(t, expected, result)
}

func TestExecutable_Execute_NoRequestInContext(t *testing.T) {
	script := "function servflowRun(vars, request_body, params) { return { body: request_body, paramsCount: Object.keys(params).length }; }"

	exec, err := NewExecutable(Config{Script: script})
	require.NoError(t, err)

	ctx := requestctx.NewTestContext()

	result, _, err := exec.Execute(ctx, script)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"body":        "",
		"paramsCount": int64(0),
	}
	assert.Equal(t, expected, result)
}

func TestExecutable_Execute_BackwardCompatibility(t *testing.T) {
	script := "function servflowRun(vars) { return vars.value; }"

	exec, err := NewExecutable(Config{Script: script})
	require.NoError(t, err)

	ctx := requestctx.NewTestContext()
	err = requestctx.AddRequestVariables(ctx, map[string]interface{}{"value": "test"}, "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/test?id=123", strings.NewReader(`{"data":"ignored"}`))
	ctx = plan.WithRequest(ctx, req)

	result, _, err := exec.Execute(ctx, script)
	require.NoError(t, err)
	assert.Equal(t, "test", result)
}
