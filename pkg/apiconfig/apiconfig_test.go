package apiconfig

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIConfig_Validate(t *testing.T) {
	// Register all actions needed for the tests beforehand
	err := actions.RegisterAction("http", actions.ActionRegistrationInfo{
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
	require.NoError(t, err)

	err = actions.RegisterAction("database", actions.ActionRegistrationInfo{
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
	require.NoError(t, err)

	validConfig := APIConfig{
		ID: "test-api",
		Actions: map[string]Action{
			"action1": {
				Type: "http",
				Config: map[string]interface{}{
					"url":    "http://example.com",
					"method": "GET",
				},
				Next: "action2",
			},
		},
		Conditionals: map[string]Conditional{
			"cond1": {
				OnTrue:     "action1",
				OnFalse:    "action2",
				Expression: "request.id == 'test'",
			},
		},
		Responses: map[string]ResponseConfig{
			"success": {
				Code: 200,
				Type: "template",
				Object: ResponseObject{
					Value: "result",
				},
			},
		},
		HttpConfig: HttpConfig{
			ListenPath: "/api/test",
			Method:     "POST",
			Next:       "action1",
		},
		McpTool: MCPToolConfig{
			Enabled:     true,
			Name:        "test-tool",
			Description: "A test tool",
			Args: map[string]ArgType{
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
		config     func() APIConfig
		wantError  bool
		errMessage string
	}{
		{
			name: "valid minimal config",
			config: func() APIConfig {
				return APIConfig{
					ID: "test-api",
				}
			},
		},
		{
			name: "valid complete config",
			config: func() APIConfig {
				return validConfig
			},
		},
		{
			name: "valid config with invalid action type",
			config: func() APIConfig {
				newConfig := validConfig
				newConfig.Actions = map[string]Action{
					"action1": {
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
			config: func() APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]Action{
					"action1": {
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
			errMessage: "action 'action1': field query is required",
		},
		{
			name: "invalid config - empty required field",
			config: func() APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]Action{
					"action1": {
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
			errMessage: "action 'action1': field query is required",
		},
		{
			name: "valid config - all required fields present",
			config: func() APIConfig {
				cfg := validConfig
				cfg.Actions = map[string]Action{
					"action1": {
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
			config: func() APIConfig {
				return APIConfig{
					ID: "",
					Actions: map[string]Action{
						"action1": {
							Type: "http",
							Config: map[string]interface{}{
								"url": "http://example.com",
							},
						},
					},
				}
			},
			errMessage: "path 'id': String length must be greater than or equal to 1",
		},
		{
			name: "invalid config - invalid HTTP method",
			config: func() APIConfig {
				cfg := validConfig
				cfg.HttpConfig.Method = "test"
				return cfg
			},
			errMessage: "path 'http.method': http.method must be one of the following:",
		},
		{
			name: "invalid config - invalid response code",
			config: func() APIConfig {
				cfg := validConfig
				cfg.Responses = map[string]ResponseConfig{
					"success": {
						Code: 900,
					},
				}
				return cfg
			},
			errMessage: "path 'responses.success.code': Must be less than or equal to 599",
		},
		{
			name: "valid config with structured conditional",
			config: func() APIConfig {
				return APIConfig{
					ID: "test-structured-conditional",
					HttpConfig: HttpConfig{
						ListenPath: "/test",
						Method:     "POST",
						Next:       "cond1",
					},
					Conditionals: map[string]Conditional{
						"cond1": {
							Type: "structured",
							Structure: [][]ConditionItem{
								{
									{Content: ".email", Function: "email", Title: "Email"},
									{Content: ".status", Comparison: "\"active\"", Function: "eq"},
								},
							},
							OnTrue:  "response1",
							OnFalse: "response2",
						},
					},
					Responses: map[string]ResponseConfig{
						"response1": {Code: 200, Type: "template", Template: "Success"},
						"response2": {Code: 400, Type: "template", Template: "Failed"},
					},
				}
			},
		},
		{
			name: "valid mcp config only",
			config: func() APIConfig {
				cfg := validConfig
				validConfig.McpTool = MCPToolConfig{
					Enabled:     true,
					Name:        "data-processor",
					Description: "Processes incoming data",
					Args: map[string]ArgType{
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
				validConfig.HttpConfig = HttpConfig{}
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config()
			err := cfg.Validate()
			if tt.errMessage != "" {
				assert.ErrorContains(t, err, tt.errMessage)
			}
		})
	}
}

func TestActionConfigError_Extraction(t *testing.T) {
	err := actions.RegisterAction("test-action", actions.ActionRegistrationInfo{
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
	if err != nil && err.Error() != "action type test-action already registered" {
		require.NoError(t, err)
	}

	tests := []struct {
		name             string
		config           APIConfig
		expectedActionID string
		expectedMessage  string
	}{
		{
			name: "invalid action type returns ActionConfigError with action ID",
			config: APIConfig{
				ID: "test-api",
				Actions: map[string]Action{
					"my-action": {
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
			config: APIConfig{
				ID: "test-api",
				Actions: map[string]Action{
					"failing-action": {
						Type:   "test-action",
						Config: map[string]interface{}{},
						Next:   "response",
					},
				},
			},
			expectedActionID: "failing-action",
			expectedMessage:  "field required_field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
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
		config          APIConfig
		expectedPath    string
		expectedMessage string
	}{
		{
			name: "empty ID returns SchemaValidationError with path",
			config: APIConfig{
				ID: "",
			},
			expectedPath:    "id",
			expectedMessage: "String length must be greater than or equal to 1",
		},
		{
			name: "invalid HTTP method returns SchemaValidationError with path",
			config: APIConfig{
				ID: "test-api",
				HttpConfig: HttpConfig{
					Method: "INVALID",
				},
			},
			expectedPath:    "http.method",
			expectedMessage: "http.method must be one of the following",
		},
		{
			name: "invalid response code returns SchemaValidationError with path",
			config: APIConfig{
				ID: "test-api",
				Responses: map[string]ResponseConfig{
					"error": {
						Code: 999,
					},
				},
			},
			expectedPath:    "responses.error.code",
			expectedMessage: "Must be less than or equal to 599",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
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

func TestValidationErrors_CollectsBothSchemaAndActionErrors(t *testing.T) {
	err := actions.RegisterAction("combined-test-action", actions.ActionRegistrationInfo{
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
	if err != nil && err.Error() != "action type combined-test-action already registered" {
		require.NoError(t, err)
	}

	config := APIConfig{
		ID: "",
		Actions: map[string]Action{
			"my-action": {
				Type:   "combined-test-action",
				Config: map[string]interface{}{},
				Next:   "response",
			},
		},
	}

	err = config.Validate()
	require.Error(t, err)

	var validationErrs *ValidationErrors
	require.True(t, errors.As(err, &validationErrs), "expected ValidationErrors type")

	schemaErrors := validationErrs.GetSchemaValidationErrors()
	actionErrors := validationErrs.GetActionConfigErrors()

	require.NotEmpty(t, schemaErrors, "expected at least one SchemaValidationError")
	require.NotEmpty(t, actionErrors, "expected at least one ActionConfigError")

	foundSchemaError := false
	for _, schemaErr := range schemaErrors {
		if schemaErr.Path == "id" {
			foundSchemaError = true
			assert.Contains(t, schemaErr.Message, "String length must be greater than or equal to 1")
		}
	}
	assert.True(t, foundSchemaError, "expected to find SchemaValidationError for empty ID")

	foundActionError := false
	for _, actionErr := range actionErrors {
		if actionErr.ActionID == "my-action" {
			foundActionError = true
			assert.Contains(t, actionErr.Message, "field required_field is required")
		}
	}
	assert.True(t, foundActionError, "expected to find ActionConfigError for missing required field")
}
