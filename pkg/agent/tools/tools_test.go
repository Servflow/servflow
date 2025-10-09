package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/actions"
	plan2 "github.com/Servflow/servflow/pkg/engine/plan"
	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewClient(t *testing.T) {
	serv := server.NewMCPServer(
		"test-server",
		"1.0",
		server.WithLogging(),
		server.WithToolCapabilities(true),
	)

	serv.AddTool(mcp.NewTool(
		"test-first",
		mcp.WithString("param-1", mcp.Required()),
		mcp.WithDescription("test description"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := request.GetArguments()["param-1"].(string)
		if !ok {
			return nil, fmt.Errorf("missing param-1")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(val)},
		}, nil
	})
	serv.AddTool(mcp.NewTool("test-second"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewImageContent("test", "image/mime-type")},
		}, nil
	})
	serv.AddTool(mcp.NewTool(
		"test-third",
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("hello world")},
		}, nil
	})
	testServer := server.NewTestStreamableHTTPServer(serv)

	t.Run("valid config", func(t *testing.T) {

		manager, err := NewManager(WithServerConfig(ServerConfig{
			Endpoint:  testServer.URL,
			ToolsList: []string{"test-first"},
		}))
		require.NoError(t, err)
		require.NotNil(t, manager)

		expectedDescription := map[string]toolDescription{
			"test-first": {
				Name: "test-first",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"param-1": map[string]any{
							"type": "string",
						},
					},
					Required: []string{"param-1"},
				},
				Description: "test description",
			},
		}
		expected, err := json.Marshal(expectedDescription)
		require.NoError(t, err)

		description, err := manager.ToolListDescription()
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), description)

		t.Run("call valid tool", func(t *testing.T) {
			resp, err := manager.CallTool(context.Background(), "test-first", map[string]interface{}{"param-1": "hello world"})
			require.NoError(t, err)
			assert.Equal(t, "hello world", resp)
		})

		t.Run("call invalid tool", func(t *testing.T) {
			_, err := manager.CallTool(context.Background(), "test-first-notexist", map[string]interface{}{"param-1": "hello world"})
			assert.ErrorContains(t, err, "tool test-first-notexist not found")
		})

		t.Run("call without param", func(t *testing.T) {
			resp, err := manager.CallTool(context.Background(), "test-first", map[string]interface{}{})
			assert.Error(t, err)
			fmt.Println(resp)
		})

		t.Run("call non text content", func(t *testing.T) {
			resp, err := manager.CallTool(context.Background(), "test-second", map[string]interface{}{})
			assert.Error(t, err)
			assert.Empty(t, resp)
		})
	})

	t.Run("invalid tool config", func(t *testing.T) {
		client, err := NewManager(WithServerConfig(ServerConfig{
			Endpoint:  testServer.URL,
			ToolsList: []string{"test-first-invalid"},
		}))
		assert.ErrorContains(t, err, "tool test-first-invalid not found")
		assert.Nil(t, client)
	})

	t.Run("invalid tool config url, retry", func(t *testing.T) {
		client, err := NewManager(WithServerConfig(ServerConfig{
			Endpoint:  "http://invalid-url",
			ToolsList: []string{"test-first-invalid"},
		}), WithServerConfig(ServerConfig{
			Endpoint:  testServer.URL,
			ToolsList: []string{"test-first"},
		}))
		assert.NoError(t, err)
		assert.NotEmpty(t, client.failedConfig)

		secondServer := server.NewTestStreamableHTTPServer(serv)
		client.failedConfig[0].Endpoint = secondServer.URL
		client.failedConfig[0].ToolsList = []string{"test-third"}

		description, err := client.ToolListDescription()
		fmt.Println(description)
		require.NoError(t, err)
		assert.Empty(t, client.failedConfig)

		expectedDescription := map[string]toolDescription{
			"test-first": {
				Name: "test-first",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"param-1": map[string]any{
							"type": "string",
						},
					},
					Required: []string{"param-1"},
				},
				Description: "test description",
			},
			"test-third": {
				Name: "test-third",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]any{},
				},
			},
		}

		expected, err := json.Marshal(expectedDescription)
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), description)
	})

	t.Run("workflow action tool", func(t *testing.T) {
		// Setup mock action provider and executables
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := plan2.NewMockActionExecutable(ctrl)

		// Configure API and plan
		cfg := apiconfig.APIConfig{
			Actions: map[string]apiconfig.Action{
				"workflow_action": {
					Type: "workflow_action",
				},
			},
		}

		customRegistry := actions.NewRegistry()
		customRegistry.ReplaceActionType("workflow_action", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		// Setup mocks
		mockExec.EXPECT().Config().Return("").AnyTimes()
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(map[string]interface{}{"result": "workflow result:"}, nil)

		// Create planner and plan
		planner := plan2.NewPlannerV2(plan2.PlannerConfig{
			Actions:        cfg.Actions,
			Responses:      cfg.Responses,
			CustomRegistry: customRegistry,
		})

		testPlan, err := planner.Plan()
		require.NoError(t, err)

		// Create manager with workflow tool
		manager, err := NewManager(WithWorkflowToolConfig(WorkflowToolConfig{
			Name:        "workflow-test",
			Description: "Test workflow execution",
			Params:      []string{"param1", "param2"},
			ReturnValue: `{{ printf "%s%s" .variable_actions_workflow_action.result (tool_param "param1") }}`,
			Start:       requestctx2.ActionConfigPrefix + "workflow_action",
		}))
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Test tool description
		description, err := manager.ToolListDescription()
		require.NoError(t, err)

		expectedDesc := map[string]toolDescription{
			"workflow-test": {
				Name:        "workflow-test",
				Description: "Test workflow execution",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"param1", "param2"},
					Properties: map[string]any{
						"param1": map[string]string{
							"type": "string",
						},
						"param2": map[string]string{
							"type": "string",
						},
					},
				},
			},
		}
		expected, err := json.Marshal(expectedDesc)
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), description)

		// Test workflow execution
		t.Run("call workflow tool", func(t *testing.T) {
			// Create context with plan
			ctx := requestctx2.NewTestContext()
			ctx = context.WithValue(ctx, plan2.ContextKey, testPlan)

			// Call the tool
			resp, err := manager.CallTool(ctx, "workflow-test", map[string]interface{}{
				"param1": "value1",
				"param2": "value2",
			})
			require.NoError(t, err)
			assert.Equal(t, "workflow result:value1", resp)
		})
	})
}
