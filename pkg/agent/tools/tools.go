package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var (
	errInitializingClient = errors.New("error initializing client")
)

type functionExec func(ctx context.Context, params map[string]any) ([]mcp.Content, error)

type ClientOption func(manager *Manager) error
type Manager struct {
	toolsExec        map[string]functionExec
	toolDescriptions map[string]toolDescription
	failedConfig     []ServerConfig
}

type ServerConfig struct {
	Endpoint  string            `json:"endpoint"`
	ToolsList []string          `json:"toolsList,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type WorkflowToolConfig struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Params      []string            `json:"params,omitempty"`
	ReturnValue string              `json:"returnValue"`
	ReturnFile  apiconfig.FileInput `json:"returnFile"`
	Start       string              `json:"start"`
	Type        string              `json:"type"`
}

const (
	workflowToolResponseString = "string"
	workflowToolResponseFile   = "file"
)

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
				logging.FromContext(context.Background()).Warn("Failed to add server config", zap.Error(err))
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
		if config.Type == "" {
			config.Type = workflowToolResponseString
		}
		if config.Type != workflowToolResponseString && config.Type != workflowToolResponseFile {
			return fmt.Errorf("invalid workflow tool type: %s", config.Type)
		}
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

		manager.toolsExec[config.Name] = generateWorkflowToolExec(&config)

		return nil
	}
}

// toolParamString coerces a model-supplied tool argument to the string form the
// workflow templates expect. The model may send a value typed per its own JSON
// (a number, bool, etc.) even though tool params are declared as strings, so a
// plain string type-assertion would drop non-string args (e.g. an installation
// id sent as a number) and yield "". Numbers are formatted without scientific
// notation so large integer-valued ids render as "144147277", not "1.44e+08".
func toolParamString(v any) string {
	switch p := v.(type) {
	case nil:
		return ""
	case string:
		return p
	case float64:
		return strconv.FormatFloat(p, 'f', -1, 64)
	case json.Number:
		return p.String()
	case bool:
		return strconv.FormatBool(p)
	default:
		return fmt.Sprintf("%v", p)
	}
}

// sensitiveToolParamKey reports whether a tool-call argument name looks like it
// carries a secret and so should be redacted before being placed on a trace span.
func sensitiveToolParamKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range []string{"token", "secret", "password", "pem", "apikey", "api_key", "authorization", "credential"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// marshalToolParams renders the model-supplied tool arguments for a trace span,
// redacting values whose key looks sensitive and capping the total size (on a
// UTF-8 boundary) so a large argument such as a comment body cannot bloat the
// span. It never returns an error; tracing must not break tool execution.
func marshalToolParams(params map[string]any) string {
	const maxLen = 2048
	safe := make(map[string]any, len(params))
	for k, v := range params {
		if sensitiveToolParamKey(k) {
			safe[k] = "[redacted]"
			continue
		}
		safe[k] = v
	}
	b, err := json.Marshal(safe)
	if err != nil {
		return fmt.Sprintf("<unmarshalable tool params: %v>", err)
	}
	if len(b) <= maxLen {
		return string(b)
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(b[cut]) {
		cut--
	}
	return string(b[:cut]) + "…(truncated)"
}

func generateWorkflowToolExec(config *WorkflowToolConfig) functionExec {
	return func(ctx context.Context, params map[string]any) ([]mcp.Content, error) {
		logging.DebugContext(ctx,
			"Executing workflow",
			zap.Any("params", params),
			zap.String("tool", config.Name),
			zap.String("start", config.Start),
			zap.String("returnValue", config.ReturnValue),
		)
		reqCtx, ok := requestctx.FromContext(ctx)
		if !ok {
			return nil, errors.New("invalid error")
		}
		reqCtx.AddRequestTemplateFunctions(template.FuncMap{
			"tool_param": func(key string) string {
				return toolParamString(params[key])
			},
		}, true)

		ctx, span := tracing.StartAgentTool(ctx, config.Name)
		defer span.End()
		// Record the model-supplied arguments on the tool-call span so a missing
		// or empty argument (e.g. an omitted installation_id) is visible directly
		// in the trace rather than inferred from a downstream action.
		span.SetAttributes(attribute.String(tracing.AttrToolParams, marshalToolParams(params)))

		if _, err := plan.ExecuteFromContext(ctx, config.Start); err != nil {
			return nil, err
		}

		// The workflow tool renders its own result from the request context once
		// the workflow has run, rather than terminating in a response step.
		switch config.Type {
		case workflowToolResponseFile:
			fileVal, err := requestctx.GetFileFromContext(ctx, config.ReturnFile)
			if err != nil {
				return nil, err
			}
			data, err := fileVal.GetContent()
			if err != nil {
				return nil, err
			}
			mimeType, err := fileVal.GetMimeType()
			if err != nil {
				return nil, err
			}
			base64Content := base64.StdEncoding.EncodeToString(data)
			return []mcp.Content{mcp.NewImageContent(base64Content, mimeType)}, nil
		case workflowToolResponseString:
			body, err := requestctx.ExecuteTemplateString(ctx, config.ReturnValue)
			if err != nil {
				return nil, err
			}
			return []mcp.Content{mcp.NewTextContent(body)}, nil
		default:
			return []mcp.Content{mcp.NewTextContent("")}, nil
		}
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
			logging.FromContext(context.Background()).Error("Failed to add server config", zap.Error(err))
			continue
		}
		m.failedConfig = append(m.failedConfig[:i], m.failedConfig[i+1:]...)
	}
}

func (m *Manager) CallTool(ctx context.Context, toolName string, params map[string]any) ([]mcp.Content, error) {
	exec, ok := m.toolsExec[toolName]
	if !ok {
		m.addFailedConfigs()
		exec, ok = m.toolsExec[toolName]
		if !ok {
			return nil, fmt.Errorf("tool %s not found", toolName)
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

// createMCPExecute creates an execute function that executes and calls an mcp server with the specified tool.
func createMCPExecute(toolName string, mcpClient *client.Client) functionExec {
	return func(ctx context.Context, params map[string]any) ([]mcp.Content, error) {
		ctx, span := tracing.StartMCPTool(ctx, toolName)
		defer span.End()
		span.SetAttributes(attribute.String(tracing.AttrToolParams, marshalToolParams(params)))

		resp, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: params,
			},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("call tool %s failed: %v", toolName, err)
		}

		if resp.IsError {
			err := fmt.Errorf("error calling tool %s", toolName)
			span.RecordError(err)
			return nil, err
		}

		return resp.Content, nil
	}
}
