package claude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"go.uber.org/zap"
)

type Config struct {
	APIKey  string `json:"api_key"`
	ModelID string `json:"model_id"`
}

type Client struct {
	integration.BaseIntegration
	client    anthropic.Client
	model     string
	maxTokens int64
}

const (
	defaultModel     = "claude-sonnet-4-5-20250929"
	defaultMaxTokens = int64(2048)
)

var supportedImageMIMETypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/gif":  {},
	"image/webp": {},
}

func (c *Client) Type() string {
	return "claude"
}

func New(apiKey string, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("no API key provided")
	}
	if model == "" {
		model = defaultModel
	}

	return &Client{
		client:    anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     model,
		maxTokens: defaultMaxTokens,
	}, nil
}

func (c *Client) ProvideResponse(ctx context.Context, agentReq agent.LLMRequest) (resp agent.LLMResponse, err error) {
	logger := logging.WithContextEnriched(ctx)
	params := convertAgentRequestToSDKParams(logger, &agentReq, c.model, c.maxTokens)

	response, err := c.client.Messages.New(ctx, params)
	if err != nil {
		logger.Error("error from claude", zap.Error(err))
		return resp, err
	}

	return convertSDKResponseToAgentResponse(response, logger), nil
}

func convertAgentRequestToSDKParams(logger *zap.Logger, req *agent.LLMRequest, model string, maxTokens int64) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  make([]anthropic.MessageParam, 0, len(req.Messages)),
	}

	systemPrompt := buildSystemPrompt(req.SystemMessage, req.Instruction)
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}

	for _, m := range req.Messages {
		switch val := m.(type) {
		case agent.MessageTypeContent:
			msg, ok := buildMessageParam(logger, val)
			if ok {
				params.Messages = append(params.Messages, msg)
			}
		case agent.MessageToolCall:
			params.Messages = append(params.Messages, buildToolUseMessage(val))
		case agent.MessageToolCallResponse:
			msg, ok := buildToolResultMessage(logger, val)
			if ok {
				params.Messages = append(params.Messages, msg)
			}
		}
	}

	if len(req.Tools) > 0 {
		params.Tools = buildTools(logger, req.Tools)
	}

	return params
}

func buildSystemPrompt(systemMessage, instruction string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(systemMessage) != "" {
		parts = append(parts, systemMessage)
	}
	if strings.TrimSpace(instruction) != "" {
		parts = append(parts, instruction)
	}
	return strings.Join(parts, "\n\n")
}

func buildMessageParam(logger *zap.Logger, msg agent.MessageTypeContent) (anthropic.MessageParam, bool) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, 2)
	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}

	if msg.FileContent != nil {
		imageBlock, err := buildImageBlockFromFile(msg.FileContent)
		if err != nil {
			logger.Warn("failed to build claude image block", zap.Error(err))
		} else {
			blocks = append(blocks, imageBlock)
		}
	}

	if len(blocks) == 0 {
		return anthropic.MessageParam{}, false
	}

	switch msg.Role {
	case agent.RoleTypeAssistant:
		return anthropic.NewAssistantMessage(blocks...), true
	case agent.RoleTypeDeveloper, agent.RoleTypeSystem:
		return anthropic.MessageParam{}, false
	default:
		return anthropic.NewUserMessage(blocks...), true
	}
}

func buildToolUseMessage(msg agent.MessageToolCall) anthropic.MessageParam {
	return anthropic.NewAssistantMessage(
		anthropic.NewToolUseBlock(msg.ID, msg.Arguments, msg.Name),
	)
}

func buildToolResultMessage(logger *zap.Logger, msg agent.MessageToolCallResponse) (anthropic.MessageParam, bool) {
	switch msg.ToolResponseType {
	case agent.ToolResponseTypeImage:
		content, err := buildToolResultImageContent(msg)
		if err != nil {
			logger.Warn("failed to build claude tool result image", zap.Error(err))
			return anthropic.MessageParam{}, false
		}
		return anthropic.NewUserMessage(anthropic.ContentBlockParamUnion{
			OfToolResult: &anthropic.ToolResultBlockParam{
				ToolUseID: msg.ID,
				Content:   []anthropic.ToolResultBlockParamContentUnion{content},
			},
		}), true
	default:
		return anthropic.NewUserMessage(
			anthropic.NewToolResultBlock(msg.ID, msg.Text, false),
		), true
	}
}

func buildToolResultImageContent(msg agent.MessageToolCallResponse) (anthropic.ToolResultBlockParamContentUnion, error) {
	if err := validateImageMIMEType(msg.ImageMimeType); err != nil {
		return anthropic.ToolResultBlockParamContentUnion{}, err
	}

	return anthropic.ToolResultBlockParamContentUnion{
		OfImage: &anthropic.ImageBlockParam{
			Source: anthropic.ImageBlockParamSourceUnion{
				OfBase64: &anthropic.Base64ImageSourceParam{
					Data:      base64.StdEncoding.EncodeToString(msg.ImageData),
					MediaType: anthropic.Base64ImageSourceMediaType(msg.ImageMimeType),
					Type:      "base64",
				},
			},
		},
	}, nil
}

func buildImageBlockFromFile(file interface {
	GetMimeType() (string, error)
	GetContent() ([]byte, error)
}) (anthropic.ContentBlockParamUnion, error) {
	mimeType, err := file.GetMimeType()
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}
	if err := validateImageMIMEType(mimeType); err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}

	content, err := file.GetContent()
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}

	return anthropic.NewImageBlockBase64(mimeType, base64.StdEncoding.EncodeToString(content)), nil
}

func validateImageMIMEType(mimeType string) error {
	if _, ok := supportedImageMIMETypes[mimeType]; !ok {
		return fmt.Errorf("unsupported image mime type %q", mimeType)
	}
	return nil
}

func buildTools(logger *zap.Logger, tools []agent.ToolInfo) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			logger.Warn("failed to marshal input schema", zap.Error(err))
			continue
		}

		var schema map[string]any
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			logger.Warn("failed to unmarshal input schema", zap.Error(err))
			continue
		}

		toolParam := anthropic.ToolParam{
			Name: tool.Name,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: schema["properties"],
				Type:       "object",
			},
		}
		if required, ok := schema["required"].([]interface{}); ok {
			toolParam.InputSchema.Required = toStringSlice(required)
		}
		if tool.Description != "" {
			toolParam.Description = anthropic.String(tool.Description)
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &toolParam})
	}
	return out
}

func toStringSlice(values []interface{}) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func convertSDKResponseToAgentResponse(resp *anthropic.Message, logger *zap.Logger) agent.LLMResponse {
	r := agent.LLMResponse{
		Content: make([]agent.ContentResponse, 0),
		Tools:   make([]agent.ToolResponseObject, 0),
	}

	for _, block := range resp.Content {
		switch val := block.AsAny().(type) {
		case anthropic.TextBlock:
			r.Content = append(r.Content, agent.ContentResponse{Text: val.Text})
		case anthropic.ToolUseBlock:
			r.Tools = append(r.Tools, agent.ToolResponseObject{
				Name:   val.Name,
				ToolID: val.ID,
				Input:  unmarshalArguments(logger, string(val.Input)),
			})
		}
	}

	return r
}

func init() {
	fields := map[string]integration.FieldInfo{
		"api_key": {
			Type:        integration.FieldTypePassword,
			Label:       "API Key",
			Placeholder: "sk-ant-...",
			Required:    true,
		},
		"model": {
			Type:        integration.FieldTypeString,
			Label:       "Model",
			Placeholder: defaultModel,
			Required:    false,
			Default:     defaultModel,
		},
	}

	if err := integration.RegisterIntegration("claude", integration.RegistrationInfo{
		Name:        "Claude",
		Description: "Anthropic Claude LLM provider for AI agent capabilities",
		ImageURL:    "https://d2ojax9k5fldtt.cloudfront.net/anthropic.png",
		Fields:      fields,
		Constructor: func(m map[string]any) (integration.Integration, error) {
			apiKey, ok := m["api_key"].(string)
			if !ok {
				return nil, errors.New("api_key required in config")
			}
			model, ok := m["model"].(string)
			if !ok {
				model = defaultModel
			}
			return New(apiKey, model)
		},
	}); err != nil {
		panic(err)
	}
}
