package openai

import (
	"encoding/json"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

type MessageInput struct {
	Role    string                `json:"role"`
	Content []ContentInputWrapper `json:"content"`
}

type ContentInputWrapper struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type FunctionCallOutput struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
	Type   string `json:"type"`
}

type FunctionCall struct {
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

type RequestBody struct {
	Model        string               `json:"model"`
	Input        []interface{}        `json:"input"`
	Instructions string               `json:"instructions"`
	Tools        []ToolsRequestConfig `json:"tools"`
}

type ToolsRequestConfig struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  mcp.ToolInputSchema `json:"parameters"`
	Type        string              `json:"type"`
}

const (
	InputRoleSystem    = "system"
	InputRoleAssistant = "assistant"
	InputRoleDeveloper = "developer"
	InputRoleUser      = "user"

	OutputTypeMessage      = "message"
	OutputTypeFunctionCall = "function_call"

	FunctionCallOutputType = "function_call_output"
	FunctionCallType       = "function_call"

	ToolTypeFunction = "function"

	InputTypeText  = "input_text"
	InputTypeImage = "input_image"
)

var agentRoleToRoleMapping = map[agent.RoleType]string{
	agent.RoleTypeSystem:    InputRoleSystem,
	agent.RoleTypeAssistant: InputRoleAssistant,
	agent.RoleTypeDeveloper: InputRoleDeveloper,
	agent.RoleTypeUser:      InputRoleUser,
}

type Response struct {
	Output []OutputObject `json:"output"`
}

type OutputObject struct {
	Type      string          `json:"type"`
	Content   []ContentObject `json:"content"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	CallID    string          `json:"call_id"`
}

type ContentObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func convertResponseToAgentResponse(resp *Response, logger *zap.Logger) agent.LLMResponse {
	r := agent.LLMResponse{
		Content: make([]agent.ContentResponse, 0),
		Tools:   make([]agent.ToolResponseObject, 0),
	}

	for _, o := range resp.Output {
		switch o.Type {
		case OutputTypeMessage:
			if len(o.Content) > 0 {
				r.Content = append(r.Content, agent.ContentResponse{
					Text: o.Content[0].Text,
				})
			}
		case OutputTypeFunctionCall:
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(o.Arguments), &args); err != nil {
				logger.Warn("Failed to unmarshal arguments", zap.Error(err))
				continue
			}
			r.Tools = append(r.Tools, agent.ToolResponseObject{
				Name:   o.Name,
				ToolID: o.CallID,
				Input:  args,
			})
		}
	}
	return r
}

func convertAgentRequestToRequest(logger *zap.Logger, req *agent.LLMRequest, model string) RequestBody {
	r := RequestBody{
		Model:        model,
		Instructions: req.SystemMessage,
		Input:        make([]interface{}, 0),
		Tools:        make([]ToolsRequestConfig, 0),
	}

	for _, m := range req.Messages {
		switch val := m.(type) {
		case agent.MessageContent:
			contents := make([]ContentInputWrapper, 0)
			contents = append(contents, ContentInputWrapper{
				Type: InputTypeText,
				Text: val.Content,
			})
			if val.FileContent != nil {
				c, err := val.FileContent.GenerateContentString()
				if err != nil {
					logger.Warn("Failed to generate content string", zap.Error(err))
				}
				contents = append(contents, ContentInputWrapper{
					Type:     InputTypeImage,
					ImageURL: c,
				})
			}
			r.Input = append(r.Input, MessageInput{
				Role:    agentRoleToRoleMapping[val.Role],
				Content: contents,
			})
		case agent.MessageToolCallResponse:
			r.Input = append(r.Input, FunctionCallOutput{
				Type:   FunctionCallOutputType,
				CallID: val.ID,
				Output: val.Output,
			})
		case agent.MessageToolCall:
			arguments, err := json.Marshal(val.Arguments)
			if err != nil {
				logger.Warn("Failed to marshal arguments", zap.Error(err))
				continue
			}
			r.Input = append(r.Input, FunctionCall{
				Arguments: string(arguments),
				CallID:    val.ID,
				Name:      val.Name,
				Type:      FunctionCallType,
			})
		}
	}

	for _, t := range req.Tools {
		r.Tools = append(r.Tools, ToolsRequestConfig{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
			Type:        ToolTypeFunction,
		})
	}

	return r
}
