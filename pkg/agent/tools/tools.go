package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

var (
	errInitializingClient = errors.New("error initializing client")
)

type functionExec func(ctx context.Context, params map[string]any) (string, error)

type ClientOption func(manager *Manager) error
type Manager struct {
	toolsExec        map[string]functionExec
	toolDescriptions map[string]toolDescription
	failedConfig     []ServerConfig
}

type ServerConfig struct {
	Endpoint  string            `json:"endpoint"`
	ToolsList []string          `json:"toolsList"`
	Headers   map[string]string `json:"headers"`
}

type WorkflowToolConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Params      []string `json:"params"`
	ReturnValue string   `json:"returnValue"`
	Start       string   `json:"start"`
}

type toolDescription struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	InputSchema mcp.ToolInputSchema `json:"inputSchema,omitempty"`
}

var defaultInitializeRequest = mcp.InitializeRequest{
	Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo: mcp.Implementation{
			Name:    "Servflow client",
			Version: "1.0.0",
		},
	},
}

func NewManager(options ...ClientOption) (*Manager, error) {
	cl := &Manager{
		toolsExec:        make(map[string]functionExec),
		toolDescriptions: make(map[string]toolDescription),
		failedConfig:     make([]ServerConfig, 0),
	}
	for _, opt := range options {
		if err := opt(cl); err != nil {
			return nil, err
		}
	}
	return cl, nil
}

func WithServerConfig(config ServerConfig) ClientOption {
	return func(manager *Manager) error {
		err := manager.addServerConfig(config)
		if err != nil {
			if errors.Is(err, errInitializingClient) {
				logging.GetLogger().Warn("Failed to add server config", zap.Error(err))
				manager.failedConfig = append(manager.failedConfig, config)
				return nil
			}
			return err
		}
		return nil
	}
}

func WithWorkflowToolConfig(config WorkflowToolConfig) ClientOption {
	return func(manager *Manager) error {
		properties := make(map[string]any)
		for _, v := range config.Params {
			properties[v] = map[string]string{
				"type": "string",
			}
		}
		manager.toolDescriptions[config.Name] = toolDescription{
			Name:        config.Name,
			Description: config.Description,
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Required:   config.Params,
				Properties: properties,
			},
		}

		manager.toolsExec[config.Name] = func(ctx context.Context, params map[string]any) (string, error) {
			logging.GetRequestLogger(ctx).Debug(
				"Executing workflow",
				zap.Any("params", params),
				zap.String("tool", config.Name),
				zap.String("start", config.Start),
				zap.String("returnValue", config.ReturnValue),
			)
			reqCtx, ok := requestctx.FromContext(ctx)
			if !ok {
				return "", errors.New("invalid error")
			}
			reqCtx.AddRequestTemplateFunctions(template.FuncMap{
				"tool_param": func(key string) string {
					p, ok := params[key].(string)
					if !ok {
						return ""
					}
					return p
				},
			})
			resp, err := plan.ExecuteFromContext(ctx, config.Start, config.ReturnValue)
			if err != nil {
				return "", err
			}
			return string(resp.Body), nil
		}

		return nil
	}
}

func (m *Manager) ToolListDescription() (string, error) {
	m.addFailedConfigs()
	return m.generateToolDescription()
}

func (m *Manager) ToolList() []agent.ToolInfo {
	m.addFailedConfigs()
	toolList := make([]agent.ToolInfo, 0)
	for _, config := range m.toolDescriptions {
		toolList = append(toolList, agent.ToolInfo{
			Name:        config.Name,
			Description: config.Description,
			InputSchema: config.InputSchema,
		})
	}
	return toolList
}

func (m *Manager) generateToolDescription() (string, error) {
	toolList, err := json.Marshal(m.toolDescriptions)
	if err != nil {
		return "", err
	}

	return string(toolList), nil
}

func (m *Manager) addFailedConfigs() {
	for i, config := range m.failedConfig {
		err := m.addServerConfig(config)
		if err != nil {
			logging.GetLogger().Error("Failed to add server config", zap.Error(err))
			continue
		}
		m.failedConfig = append(m.failedConfig[:i], m.failedConfig[i+1:]...)
	}
}

func (m *Manager) CallTool(ctx context.Context, toolName string, params map[string]any) (string, error) {
	exec, ok := m.toolsExec[toolName]
	if !ok {
		m.addFailedConfigs()
		exec, ok = m.toolsExec[toolName]
		if !ok {
			return "", fmt.Errorf("tool %s not found", toolName)
		}
	}

	return exec(ctx, params)
}

func (m *Manager) addServerConfig(config ServerConfig) error {
	cl, err := client.NewStreamableHttpClient(config.Endpoint, transport.WithHTTPHeaders(config.Headers))
	if err != nil {
		return err
	}
	_, err = cl.Initialize(context.Background(), defaultInitializeRequest)
	if err != nil {
		return fmt.Errorf("%w: %v", errInitializingClient, err)
	}

	toolsResp, err := cl.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("list tools failed: %v", err)
	}
	for _, t := range config.ToolsList {
		var found bool
		for _, serverTool := range toolsResp.Tools {
			if serverTool.Name != t {
				continue
			}
			tool := toolDescription{
				Name:        t,
				Description: serverTool.Description,
				InputSchema: serverTool.InputSchema,
			}
			m.toolDescriptions[tool.Name] = tool

			m.toolsExec[tool.Name] = createMCPExecute(tool.Name, cl)
			found = true
		}
		if !found {
			return fmt.Errorf("tool %s not found", t)
		}
	}
	return nil
}

func createMCPExecute(toolName string, mcpClient *client.Client) functionExec {
	return func(ctx context.Context, params map[string]any) (string, error) {
		resp, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: params,
			},
		})
		if err != nil {
			return "", fmt.Errorf("call tool %s failed: %v", toolName, err)
		}

		if resp.IsError {
			return "", fmt.Errorf("error calling tool %s", toolName)
		}

		// only support text response for now
		for _, content := range resp.Content {
			textContent, ok := content.(mcp.TextContent)
			if !ok {
				continue
			}
			return textContent.Text, nil
		}
		return "", fmt.Errorf("response content is empty")
	}
}
