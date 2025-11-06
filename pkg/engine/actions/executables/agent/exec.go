package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/agent/tools"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
)

type Config struct {
	ToolConfigs       []ToolConfig `json:"toolConfigs" yaml:"toolConfigs"`
	SystemPrompt      string       `json:"systemPrompt" yaml:"systemPrompt"`
	UserPrompt        string       `json:"userPrompt" yaml:"userPrompt"`
	IntegrationID     string       `json:"integrationID" yaml:"integrationID"`
	ConversationID    string       `json:"conversationID" yaml:"conversationID"`
	ReturnLastMessage bool         `json:"returnLastMessage" yaml:"returnLastMessage"`
}
type MCPServerConfig struct {
	Endpoint string   `json:"endpoint" yaml:"endpoint"`
	Tools    []string `json:"tools" yaml:"tools"`
}

type ToolConfig struct {
	Type           string                   `json:"type" yaml:"type"`
	McpConfig      tools.ServerConfig       `json:"mcpConfig" yaml:"mcpConfig"`
	WorkflowConfig tools.WorkflowToolConfig `json:"workflowConfig" yaml:"workflowConfig"`
}

var (
	toolTypeMCP      = "mcp"
	toolTypeWorkflow = "workflow"
)

type ActionToolConfig struct {
}

type Agent struct {
	config      *Config
	integration agent.LLmProvider
	toolManager *tools.Manager
}

func (a *Agent) Config() string {
	c, err := json.Marshal(a.config)
	if err != nil {
		return ""
	}
	return string(c)
}

func (a *Agent) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var newConfig Config
	if err := json.Unmarshal([]byte(modifiedConfig), &newConfig); err != nil {
		return nil, err
	}

	options := []agent.Option{agent.WithToolManager(a.toolManager)}
	if newConfig.ConversationID != "" {
		options = append(options, agent.WithConversationID(newConfig.ConversationID))
	}
	if newConfig.ReturnLastMessage {
		options = append(options, agent.WithReturnOnlyLastMessage())
	}
	session, err := agent.NewSession(
		newConfig.SystemPrompt,
		a.integration,
		options...,
	)
	if err != nil {
		return nil, err
	}

	resp, err := session.Query(ctx, newConfig.UserPrompt)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (a *Agent) Type() string {
	return "agent"
}

func New(config Config) (*Agent, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("IntegrationID is required")
	}

	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}

	integ, ok := i.(agent.LLmProvider)
	if !ok {
		return nil, errors.New("integration is not an LLmHandler")
	}

	options := make([]tools.ClientOption, 0)
	for _, opt := range config.ToolConfigs {
		switch opt.Type {
		case toolTypeMCP:
			options = append(options, tools.WithServerConfig(opt.McpConfig))
		case toolTypeWorkflow:
			options = append(options, tools.WithWorkflowToolConfig(opt.WorkflowConfig))
		}
	}

	cl, err := tools.NewManager(options...)
	if err != nil {
		return nil, err
	}

	return &Agent{
		integration: integ,
		config:      &config,
		toolManager: cl,
	}, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"toolConfigs": {
			Type:        "array",
			Label:       "Tool Configurations",
			Placeholder: "Array of tool configurations",
			Required:    false,
		},
		"systemPrompt": {
			Type:        "string",
			Label:       "System Prompt",
			Placeholder: "System instructions for the agent",
			Required:    false,
		},
		"userPrompt": {
			Type:        "string",
			Label:       "User Prompt",
			Placeholder: "User message or query",
			Required:    true,
		},
		"integrationID": {
			Type:        "string",
			Label:       "Integration ID",
			Placeholder: "AI integration identifier",
			Required:    true,
		},
		"conversationID": {
			Type:        "string",
			Label:       "Conversation ID",
			Placeholder: "Conversation identifier",
			Required:    false,
		},
		"returnLastMessage": {
			Type:        "boolean",
			Label:       "Return Last Message",
			Placeholder: "Whether to return only the last message",
			Required:    false,
			Default:     false,
		},
	}

	if err := actions.RegisterAction("agent", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var cfg Config
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("error creating agent action: %v", err)
		}
		return New(cfg)
	}, fields); err != nil {
		panic(err)
	}
}
