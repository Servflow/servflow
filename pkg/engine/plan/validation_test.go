package plan

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerTestAction(t *testing.T, name string, info actions.ActionRegistrationInfo) {
	t.Helper()
	err := actions.RegisterAction(name, info)
	if err != nil && err.Error() != "action type "+name+" already registered" {
		require.NoError(t, err)
	}
}

func TestAPIConfig_Validate(t *testing.T) {
	registerTestAction(t, "http", actions.ActionRegistrationInfo{
		Name:        "HTTP Action",
		Description: "HTTP action for testing",
		Fields: map[string]actions.FieldInfo{
			"url": {
				Type:     actions.FieldTypeString,
				Label:    "URL",
				Required: true,
			},
			"method": {
				Type:     actions.FieldTypeString,
				Label:    "Method",
				Required: false,
			},
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil
		},
	})

	registerTestAction(t, "database", actions.ActionRegistrationInfo{
		Name:        "Database Action",
		Description: "Database action for testing",
		Fields: map[string]actions.FieldInfo{
			"query": {
				Type:     actions.FieldTypeString,
				Label:    "Query",
				Required: true,
			},
			"table": {
				Type:     actions.FieldTypeString,
				Label:    "Table",
				Required: false,
			},
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil
		},
	})

	validConfig := apiconfig.APIConfig{
		ID: "test-api",
		Actions: map[string]apiconfig.Action{
			"action1": {
				Name: "action1",
				Type: "http",
				Config: map[string]interface{}{
					"url":    "http://example.com",
					"method": "GET",
				},
				Next: "action2",
			},
		},
		Conditionals: map[string]apiconfig.Conditional{
			"cond1": {
				Name:       "cond1",
				OnTrue:     "action1",
				OnFalse:    "action2",
				Expression: "request.id == 'test'",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Name: "success",
				Code: 200,
				Type: "template",
				Object: apiconfig.ResponseObject{
					Value: "result",
				},
			},
		},
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/api/test",
			Method:     "POST",
			Next:       "action1",
		},
		McpTool: apiconfig.MCPToolConfig{
			Enabled:     true,
			Name:        "test-tool",
			Description: "A test tool",
			Args: map[string]apiconfig.ArgType{
				"input": {
					Name: "input",
					Type: "string",
				},
			},
			Result: "output",
			Start:  "action1",
		},
	}
	tests := []struct {
		name       string
		config     func() apiconfig.APIConfig
		wantError  bool
		errMessage string
	}{
		{
			name: "valid minimal config",
			config: func() apiconfig.APIConfig {
				return apiconfig.APIConfig{
					ID: "test-api",
				}
			},
		},
		{
			name: "valid complete config",
			config: func() apiconfig.APIConfig {
				return validConfig
			},
		},
		{
			name: "valid config with invalid action type",
			config: func() apiconfig.APIConfig {
				newConfig := validConfig
				newConfig.Actions = map[string]apiconfig.Action{
					"action1": {
						Name: "action1",
						Type: "invalid-action-type",
						Next: "action2",
					},
				}
				return newConfig
			},
			errMessage: "invalid action type: invalid-action-type",
		},
		{
			name: "invalid config - missing required field",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]apiconfig.Action{
					"action1": {
						Name: "action1",
						Type: "database",
						Config: map[string]interface{}{
							"table": "users",
							// missing required "query" field
						},
						Next: "action2",
					},
				}
				return cfg
			},
			errMessage: "action 'action1': missing required field \"query\"",
		},
		{
			name: "invalid config - empty required field",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]apiconfig.Action{
					"action1": {
						Name: "action1",
						Type: "database",
						Config: map[string]interface{}{
							"query": "",
							"table": "users",
						},
						Next: "action2",
					},
				}
				return cfg
			},
			errMessage: "action 'action1': missing required field \"query\"",
		},
		{
			name: "valid config - all required fields present",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]apiconfig.Action{
					"action1": {
						Name: "action1",
						Type: "database",
						Config: map[string]interface{}{
							"query": "SELECT * FROM users",
							"table": "users",
						},
						Next: "action2",
					},
				}
				return cfg
			},
		},
		{
			name: "invalid config - empty ID",
			config: func() apiconfig.APIConfig {
				return apiconfig.APIConfig{
					ID: "",
					Actions: map[string]apiconfig.Action{
						"action1": {
							Name: "action1",
							Type: "http",
							Config: map[string]interface{}{
								"url": "http://example.com",
							},
						},
					},
				}
			},
			errMessage: "field \"id\": minLength: got 0, want 1",
		},
		{
			name: "invalid config - invalid HTTP method",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				cfg.HttpConfig.Method = "test"
				return cfg
			},
			errMessage: "field \"http.method\" has an invalid value; allowed:",
		},
		{
			name: "invalid config - invalid response code",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				cfg.Responses = map[string]apiconfig.ResponseConfig{
					"success": {
						Name: "success",
						Code: 900,
					},
				}
				return cfg
			},
			errMessage: "response \"success\" field \"code\": maximum: got 900, want 599",
		},
		{
			name: "valid config with structured conditional",
			config: func() apiconfig.APIConfig {
				return apiconfig.APIConfig{
					ID: "test-structured-conditional",
					HttpConfig: apiconfig.HttpConfig{
						ListenPath: "/test",
						Method:     "POST",
						Next:       "cond1",
					},
					Conditionals: map[string]apiconfig.Conditional{
						"cond1": {
							Name: "cond1",
							Type: "structured",
							Structure: [][]apiconfig.ConditionItem{
								{
									{Content: ".email", Function: "email", Title: "Email"},
									{Content: ".status", Comparison: "\"active\"", Function: "eq"},
								},
							},
							OnTrue:  "response1",
							OnFalse: "response2",
						},
					},
					Responses: map[string]apiconfig.ResponseConfig{
						"response1": {Name: "response1", Code: 200, Type: "template", Template: "Success"},
						"response2": {Name: "response2", Code: 400, Type: "template", Template: "Failed"},
					},
				}
			},
		},
		{
			name: "valid mcp config only",
			config: func() apiconfig.APIConfig {
				cfg := validConfig
				validConfig.McpTool = apiconfig.MCPToolConfig{
					Enabled:     true,
					Name:        "data-processor",
					Description: "Processes incoming data",
					Args: map[string]apiconfig.ArgType{
						"input_data": {
							Name: "input_data",
							Type: "string",
						},
						"format": {
							Name: "format",
							Type: "string",
						},
					},
					Result: "processed_data",
					Start:  "process-action",
				}
				validConfig.HttpConfig = apiconfig.HttpConfig{}
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config()
			err := Validate(&cfg)
			if tt.errMessage != "" {
				assert.ErrorContains(t, err, tt.errMessage)
			}
		})
	}
}

func TestActionConfigError_Extraction(t *testing.T) {
	registerTestAction(t, "test-action", actions.ActionRegistrationInfo{
		Name:        "Test Action",
		Description: "Test action for error extraction",
		Fields: map[string]actions.FieldInfo{
			"required_field": {
				Type:     actions.FieldTypeString,
				Label:    "Required Field",
				Required: true,
			},
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil
		},
	})

	tests := []struct {
		name             string
		config           apiconfig.APIConfig
		expectedActionID string
		expectedMessage  string
	}{
		{
			name: "invalid action type returns ActionConfigError with action ID",
			config: apiconfig.APIConfig{
				ID: "test-api",
				Actions: map[string]apiconfig.Action{
					"my-action": {
						Name: "my-action",
						Type: "nonexistent-type",
						Next: "response",
					},
				},
			},
			expectedActionID: "my-action",
			expectedMessage:  "invalid action type: nonexistent-type",
		},
		{
			name: "missing required field returns ActionConfigError with action ID",
			config: apiconfig.APIConfig{
				ID: "test-api",
				Actions: map[string]apiconfig.Action{
					"failing-action": {
						Name:   "failing-action",
						Type:   "test-action",
						Config: map[string]interface{}{},
						Next:   "response",
					},
				},
			},
			expectedActionID: "failing-action",
			expectedMessage:  "missing required field \"required_field\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.config)
			require.Error(t, err)

			var validationErrs *ValidationErrors
			require.True(t, errors.As(err, &validationErrs), "expected ValidationErrors type")

			actionErrors := validationErrs.GetActionConfigErrors()
			require.NotEmpty(t, actionErrors, "expected at least one ActionConfigError")

			found := false
			for _, actionErr := range actionErrors {
				if actionErr.ActionID == tt.expectedActionID {
					found = true
					assert.Contains(t, actionErr.Message, tt.expectedMessage)
				}
			}
			assert.True(t, found, "expected to find ActionConfigError with ActionID %s", tt.expectedActionID)
		})
	}
}

func TestSchemaValidationError_Extraction(t *testing.T) {
	tests := []struct {
		name            string
		config          apiconfig.APIConfig
		expectedPath    string
		expectedMessage string
	}{
		{
			name: "empty ID returns SchemaValidationError with path",
			config: apiconfig.APIConfig{
				ID: "",
			},
			expectedPath:    "/id",
			expectedMessage: "minLength: got 0, want 1",
		},
		{
			name: "invalid HTTP method returns SchemaValidationError with path",
			config: apiconfig.APIConfig{
				ID: "test-api",
				HttpConfig: apiconfig.HttpConfig{
					Method: "INVALID",
				},
			},
			expectedPath:    "/http/method",
			expectedMessage: "has an invalid value; allowed:",
		},
		{
			name: "invalid response code returns SchemaValidationError with path",
			config: apiconfig.APIConfig{
				ID: "test-api",
				Responses: map[string]apiconfig.ResponseConfig{
					"error": {
						Name: "error",
						Code: 999,
					},
				},
			},
			expectedPath:    "/responses/error/code",
			expectedMessage: "maximum: got 999, want 599",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.config)
			require.Error(t, err)

			var validationErrs *ValidationErrors
			require.True(t, errors.As(err, &validationErrs), "expected ValidationErrors type")

			schemaErrors := validationErrs.GetSchemaValidationErrors()
			require.NotEmpty(t, schemaErrors, "expected at least one SchemaValidationError")

			found := false
			for _, schemaErr := range schemaErrors {
				if schemaErr.Path == tt.expectedPath {
					found = true
					assert.Contains(t, schemaErr.Message, tt.expectedMessage)
				}
			}
			assert.True(t, found, "expected to find SchemaValidationError with Path %s", tt.expectedPath)
		})
	}
}

func TestResponseConfigError_UnknownKind(t *testing.T) {
	cfg := apiconfig.APIConfig{
		ID: "test-api",
		Responses: map[string]apiconfig.ResponseConfig{
			"bad": {
				Name: "bad",
				Kind: "not-a-real-kind",
				Code: 200,
			},
			"ok": {
				Name: "ok",
				Code: 200, // no kind -> defaults to "http" (registered)
			},
		},
	}

	err := Validate(&cfg)
	require.Error(t, err)

	var validationErrs *ValidationErrors
	require.True(t, errors.As(err, &validationErrs), "expected ValidationErrors type")

	respErrs := validationErrs.GetResponseConfigErrors()
	require.NotEmpty(t, respErrs, "expected at least one ResponseConfigError")

	found := false
	for _, re := range respErrs {
		if re.ResponseID == "bad" {
			found = true
			assert.Contains(t, re.Message, "invalid response kind: not-a-real-kind")
		}
		assert.NotEqual(t, "ok", re.ResponseID, "a default-http response must validate")
	}
	assert.True(t, found, "expected a ResponseConfigError for response 'bad'")
}

func TestValidationErrors_CollectsBothSchemaAndActionErrors(t *testing.T) {
	registerTestAction(t, "combined-test-action", actions.ActionRegistrationInfo{
		Name:        "Combined Test Action",
		Description: "Test action for combined error collection",
		Fields: map[string]actions.FieldInfo{
			"required_field": {
				Type:     actions.FieldTypeString,
				Label:    "Required Field",
				Required: true,
			},
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil
		},
	})

	config := apiconfig.APIConfig{
		ID: "",
		Actions: map[string]apiconfig.Action{
			"my-action": {
				Name:   "my-action",
				Type:   "combined-test-action",
				Config: map[string]interface{}{},
				Next:   "response",
			},
		},
	}

	err := Validate(&config)
	require.Error(t, err)

	var validationErrs *ValidationErrors
	require.True(t, errors.As(err, &validationErrs), "expected ValidationErrors type")

	schemaErrors := validationErrs.GetSchemaValidationErrors()
	actionErrors := validationErrs.GetActionConfigErrors()

	require.NotEmpty(t, schemaErrors, "expected at least one SchemaValidationError")
	require.NotEmpty(t, actionErrors, "expected at least one ActionConfigError")

	foundSchemaError := false
	for _, schemaErr := range schemaErrors {
		if schemaErr.Path == "/id" {
			foundSchemaError = true
			assert.Contains(t, schemaErr.Message, "minLength: got 0, want 1")
		}
	}
	assert.True(t, foundSchemaError, "expected to find SchemaValidationError for empty ID")

	foundActionError := false
	for _, actionErr := range actionErrors {
		if actionErr.ActionID == "my-action" {
			foundActionError = true
			assert.Contains(t, actionErr.Message, "missing required field \"required_field\"")
		}
	}
	assert.True(t, foundActionError, "expected to find ActionConfigError for missing required field")
}
