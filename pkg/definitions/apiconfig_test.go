package apiconfig

import (
	"encoding/json"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIConfig_Validate(t *testing.T) {
	// Register all actions needed for the tests beforehand
	err := actions.RegisterAction("http", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return nil, nil
	}, map[string]actions.FieldInfo{
		"url": {
			Type:     "string",
			Label:    "URL",
			Required: true,
		},
		"method": {
			Type:     "string",
			Label:    "Method",
			Required: false,
		},
	})
	require.NoError(t, err)

	err = actions.RegisterAction("database", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return nil, nil
	}, map[string]actions.FieldInfo{
		"query": {
			Type:     "string",
			Label:    "Query",
			Required: true,
		},
		"table": {
			Type:     "string",
			Label:    "Table",
			Required: false,
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
		name      string
		config    func() APIConfig
		wantError bool
	}{
		{
			name: "valid minimal config",
			config: func() APIConfig {
				return APIConfig{
					ID: "test-api",
				}
			},
			wantError: false,
		},
		{
			name: "valid complete config",
			config: func() APIConfig {
				return validConfig
			},
			wantError: false,
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
			wantError: true,
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
			wantError: true,
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
			wantError: true,
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
			wantError: false,
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
			wantError: true,
		},
		{
			name: "invalid config - invalid HTTP method",
			config: func() APIConfig {
				cfg := validConfig
				cfg.HttpConfig.Method = "test"
				return cfg
			},
			wantError: true,
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
			wantError: true,
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
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config()
			err := cfg.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
