package apiconfig

import (
	"encoding/json"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIConfig_Validate(t *testing.T) {
	validConfig := APIConfig{
		ID: "test-api",
		Actions: map[string]Action{
			"action1": {
				Type: "http",
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
			name: "valid config with invalid action",
			config: func() APIConfig {
				newConfig := validConfig
				newConfig.Actions = map[string]Action{
					"action1": {
						Type: "http2",
						Next: "action2",
					},
				}
				return newConfig
			},
			wantError: true,
		},
		{
			name: "invalid config - empty ID",
			config: func() APIConfig {
				return APIConfig{
					ID: "",
					Actions: map[string]Action{
						"action1": {
							Type: "http",
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

	err := actions.RegisterAction("http", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return nil, nil
	})
	require.NoError(t, err)

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
