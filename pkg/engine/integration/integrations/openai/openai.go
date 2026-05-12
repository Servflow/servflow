package openai

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"go.uber.org/zap"
)

type Config struct {
	APIKey         string `json:"api_key"`
	OrganizationID string `json:"organization_id"`
	ModelID        string `json:"model_id"`
}

type Client struct {
	integration.BaseIntegration
	client *openai.Client
	model  string
}

func (c *Client) Type() string {
	return "openai"
}

var defaultModel = "gpt-4.1"

func New(apiKey string, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("no API key provided")
	}

	if model == "" {
		model = defaultModel
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &Client{
		client: &client,
		model:  model,
	}, nil
}

func (c *Client) ProvideResponse(ctx context.Context, agentReq agent.LLMRequest) (resp agent.LLMResponse, err error) {
	logger := logging.WithContextEnriched(ctx)

	params := convertAgentRequestToSDKParams(logger, &agentReq, c.model)

	response, err := c.client.Responses.New(ctx, params)
	if err != nil {
		logger.Error("error from openai", zap.Error(err))
		return
	}

	resp = convertSDKResponseToAgentResponse(response, logger)
	return resp, nil
}

func init() {
	fields := map[string]integration.FieldInfo{
		"api_key": {
			Type:        integration.FieldTypePassword,
			Label:       "API Key",
			Placeholder: "sk-...",
			Required:    true,
		},
		"model": {
			Type:        integration.FieldTypeString,
			Label:       "Model",
			Placeholder: "gpt-4.1",
			Required:    false,
			Default:     defaultModel,
		},
	}

	if err := integration.RegisterIntegration("openai", integration.RegistrationInfo{
		Name:        "OpenAI",
		Description: "OpenAI LLM provider for AI agent capabilities",
		ImageURL:    "https://d2ojax9k5fldtt.cloudfront.net/openai.png",
		Fields:      fields,
		Constructor: func(m map[string]any) (integration.Integration, error) {
			apikey, ok := m["api_key"].(string)
			if !ok {
				return nil, errors.New("api_key required in config")
			}
			model, ok := m["model"].(string)
			if !ok {
				model = defaultModel
			}
			return New(apikey, model)
		},
	}); err != nil {
		panic(err)
	}
}

func convertAgentRequestToSDKParams(logger *zap.Logger, req *agent.LLMRequest, model string) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model:        model,
		Instructions: openai.String(req.SystemMessage),
	}

	inputItems := make([]responses.ResponseInputItemUnionParam, 0)
	if req.Instruction != "" {
		inputItems = append(inputItems, buildInstructionInput(req.Instruction))
	}

	for _, m := range req.Messages {
		switch val := m.(type) {
		case agent.MessageTypeContent:
			inputItems = append(inputItems, buildMessageInput(logger, val))
		case agent.MessageToolCallResponse:
			inputItems = append(inputItems, buildFunctionCallOutput(val))
		case agent.MessageToolCall:
			inputItems = append(inputItems, buildFunctionCallInput(logger, val))
		}
	}

	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}

	if len(req.Tools) > 0 {
		tools := make([]responses.ToolUnionParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			schemaBytes, err := json.Marshal(t.InputSchema)
			if err != nil {
				logger.Warn("Failed to marshal input schema", zap.Error(err))
				continue
			}
			var schemaMap map[string]interface{}
			if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
				logger.Warn("Failed to unmarshal input schema", zap.Error(err))
				continue
			}

			tools = append(tools, responses.ToolParamOfFunction(t.Name, schemaMap, false))
			// Set description if available
			if t.Description != "" {
				tools[len(tools)-1].OfFunction.Description = openai.String(t.Description)
			}
		}
		params.Tools = tools
	}

	return params
}

func buildInstructionInput(instruction string) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: responses.EasyInputMessageRole("developer"),
			Content: responses.EasyInputMessageContentUnionParam{
				OfInputItemContentList: []responses.ResponseInputContentUnionParam{
					{
						OfInputText: &responses.ResponseInputTextParam{
							Type: "input_text",
							Text: instruction,
						},
					},
				},
			},
			Type: "message",
		},
	}
}

func buildMessageInput(logger *zap.Logger, val agent.MessageTypeContent) responses.ResponseInputItemUnionParam {
	role := mapAgentRoleToSDKRole(val.Role)

	// For assistant messages, we use OutputMessage format
	if role == "assistant" {
		content := make([]responses.ResponseOutputMessageContentUnionParam, 0)
		if val.Content != "" {
			content = append(content, responses.ResponseOutputMessageContentUnionParam{
				OfOutputText: &responses.ResponseOutputTextParam{
					Type: "output_text",
					Text: val.Content,
				},
			})
		}
		return responses.ResponseInputItemUnionParam{
			OfOutputMessage: &responses.ResponseOutputMessageParam{
				Type:    "message",
				Role:    "assistant",
				Content: content,
				Status:  "completed",
			},
		}
	}

	// For user/system/developer messages
	contentParts := make([]responses.ResponseInputContentUnionParam, 0)

	if val.Content != "" {
		contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
			OfInputText: &responses.ResponseInputTextParam{
				Type: "input_text",
				Text: val.Content,
			},
		})
	}

	if val.FileContent != nil {
		contentStr, err := val.FileContent.GenerateContentString()
		if err != nil {
			logger.Warn("Failed to generate content string", zap.Error(err))
		} else {
			contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
				OfInputImage: &responses.ResponseInputImageParam{
					Type:     "input_image",
					ImageURL: openai.String(contentStr),
				},
			})
		}
	}

	return responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role:    responses.EasyInputMessageRole(role),
			Content: responses.EasyInputMessageContentUnionParam{OfInputItemContentList: contentParts},
			Type:    "message",
		},
	}
}

func buildFunctionCallOutput(val agent.MessageToolCallResponse) responses.ResponseInputItemUnionParam {
	content, _, outputType := val.GenerateContent()

	switch outputType {
	case agent.ToolCallOutputTypeImage:
		outputItems := responses.ResponseFunctionCallOutputItemListParam{
			{
				OfInputImage: &responses.ResponseInputImageContentParam{
					ImageURL: openai.String(content),
				},
			},
		}
		return responses.ResponseInputItemParamOfFunctionCallOutput(val.ID, outputItems)
	default:
		return responses.ResponseInputItemParamOfFunctionCallOutput(val.ID, content)
	}
}

func buildFunctionCallInput(logger *zap.Logger, val agent.MessageToolCall) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemParamOfFunctionCall(marshalArguments(logger, val.Arguments), val.ID, val.Name)
}

func mapAgentRoleToSDKRole(role agent.RoleType) string {
	switch role {
	case agent.RoleTypeSystem:
		return "system"
	case agent.RoleTypeUser:
		return "user"
	case agent.RoleTypeAssistant:
		return "assistant"
	case agent.RoleTypeDeveloper:
		return "developer"
	default:
		return "user"
	}
}

func convertSDKResponseToAgentResponse(resp *responses.Response, logger *zap.Logger) agent.LLMResponse {
	r := agent.LLMResponse{
		Content: make([]agent.ContentResponse, 0),
		Tools:   make([]agent.ToolResponseObject, 0),
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msg := item.AsMessage()
			for _, content := range msg.Content {
				if content.Type == "output_text" {
					textContent := content.AsOutputText()
					r.Content = append(r.Content, agent.ContentResponse{
						Text: textContent.Text,
					})
				}
			}
		case "function_call":
			funcCall := item.AsFunctionCall()
			args := unmarshalArguments(logger, funcCall.Arguments)
			r.Tools = append(r.Tools, agent.ToolResponseObject{
				Name:   funcCall.Name,
				ToolID: funcCall.CallID,
				Input:  args,
			})
		}
	}

	return r
}
