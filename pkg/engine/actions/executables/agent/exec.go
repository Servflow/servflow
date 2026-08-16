package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/agent/tools"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Config struct {
	ToolConfigs   []ToolConfig        `json:"toolConfigs" yaml:"toolConfigs"`
	SystemPrompt  string              `json:"systemPrompt" yaml:"systemPrompt"`
	UserPrompt    string              `json:"userPrompt" yaml:"userPrompt"`
	IntegrationID string              `json:"integrationID" yaml:"integrationID"`
	FileUpload    apiconfig.FileInput `json:"fileUpload" yaml:"fileUpload"`
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

// Execute is the V2 entrypoint: the action resolves its own templated fields
// individually against the request context, so text produced by a template
// (e.g. a JSON document interpolated into a prompt) can never corrupt the rest
// of the config the way the V1 whole-string resolve-then-reparse did.
//
// Only the fields consumed NOW are resolved: the prompts and the file
// identifier. ToolConfigs are deliberately left untouched — the tools manager
// was built from the raw config at construction time and resolves tool-time
// templates (returnValue, subgraph configs) when a tool is called.
//
// The conversation is no longer per-action: every agent action reads from and
// appends to the request's shared conversation thread (owned by the request
// context, selected once at plan start via the conversation id).
//
// The action deliberately returns NO output. An agent's result is the messages
// it contributed to the conversation — the session appends each one as it is
// produced — so there is nothing to hand to later steps and no
// {{ .variable_actions_<id> }} for them to read. Run output is extracted from
// the thread instead.
func (a *Agent) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request context: %w", err)
	}

	resolved, err := rc.ResolveBatch(ctx,
		a.config.SystemPrompt,
		a.config.UserPrompt,
		a.config.FileUpload.Identifier,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve agent config: %w", err)
	}
	systemPrompt, userPrompt := resolved[0], resolved[1]
	fileUpload := a.config.FileUpload
	fileUpload.Identifier = resolved[2]

	options := []agent.Option{
		agent.WithToolManager(a.toolManager),
		agent.WithConversation(rc.Conversation()),
	}
	session, err := agent.NewSession(
		systemPrompt,
		a.integration,
		options...,
	)
	if err != nil {
		return nil, nil, err
	}

	ctx, span := tracing.StartAgentInvoke(ctx, a.config.IntegrationID)
	defer span.End()

	fileInput, err := requestctx.GetFileFromContext(ctx, fileUpload)
	if err != nil && !errors.Is(err, requestctx.ErrFileNotFound) {
		return nil, nil, fmt.Errorf("%w: %v", actions.ErrorFatal, err)
	}
	// The reply is discarded on purpose: Query has already appended every message
	// it produced to the shared conversation, which is where an agent's output
	// lives now.
	if _, err := session.Query(ctx, userPrompt, fileInput); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	// Per-call token detail lives on the child chat spans as gen_ai.usage.*.
	// This span is an aggregate, so use the sf.usage.* keys — gen_ai.usage.* is
	// scoped to a single model call, and reusing it here would double-count in
	// backends that sum gen_ai.usage.* across a trace.
	metadata := session.GetMetadata()
	span.SetAttributes(
		attribute.Int64(tracing.AttrUsageInput, metadata.TotalUsage.InputTokens),
		attribute.Int64(tracing.AttrUsageOutput, metadata.TotalUsage.OutputTokens),
		attribute.Int64(tracing.AttrUsageTotal, metadata.TotalUsage.InputTokens+metadata.TotalUsage.OutputTokens),
	)

	return nil, nil, nil
}

func (a *Agent) Type() string {
	return "agent"
}

func New(config Config) (*Agent, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("IntegrationID is required")
	}

	i, err := integration.GetIntegration(context.Background(), config.IntegrationID)
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
			Type:        actions.FieldTypeString,
			Label:       "System Prompt",
			Placeholder: "System instructions for the agent",
			Required:    false,
		},
		"fileUpload": {
			Type:     actions.FieldTypeFile,
			Label:    "File To Upload",
			Required: false,
		},
		"userPrompt": {
			Type:        actions.FieldTypeString,
			Label:       "User Prompt",
			Placeholder: "User message or query",
			Required:    false,
		},
		"integrationID": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Integration ID",
			Placeholder: "AI integration identifier",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("agent", actions.ActionRegistrationInfo{
		Name:        "AI Agent",
		Description: "Interacts with AI models to process queries and execute tool functions",
		Fields:      fields,
		UseV2:       true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating agent action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
