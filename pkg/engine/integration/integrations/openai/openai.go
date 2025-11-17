package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

type Config struct {
	APIKey         string `json:"api_key"`
	OrganizationID string `json:"organization_id"`
	ModelID        string `json:"model_id"`
}

type MessageInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
		case agent.ContentMessage:
			r.Input = append(r.Input, MessageInput{
				Role:    agentRoleToRoleMapping[val.Role],
				Content: val.Content,
			})
		case agent.ToolCallOutputMessage:
			r.Input = append(r.Input, FunctionCallOutput{
				Type:   FunctionCallOutputType,
				CallID: val.ID,
				Output: val.Output,
			})
		case agent.ToolCallMessage:
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

type Client struct {
	client *http.Client
	apiKey string
	model  string
}

func (c *Client) Type() string {
	return "openai"
}

func New(apiKey string, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("no API key provided")
	}

	if model == "" {
		model = defaultModel
	}

	return &Client{
		client: &http.Client{},
		apiKey: apiKey,
		model:  model,
	}, nil
}

var (
	endpoint     = "https://api.openai.com/v1/responses"
	defaultModel = "gpt-4.1"
)

func (c *Client) ProvideResponse(ctx context.Context, agentReq agent.LLMRequest) (resp agent.LLMResponse, err error) {
	logger := logging.WithContextEnriched(ctx)
	input := convertAgentRequestToRequest(logger, &agentReq, c.model)

	inputJson, err := json.Marshal(input)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(inputJson))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	r, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected status OK, got %d", r.StatusCode)
		body, _ := io.ReadAll(r.Body)
		logger.Error("error from openai", zap.String("response", string(body)))
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}

	var response Response
	if err = json.Unmarshal(body, &response); err != nil {
		return
	}

	resp = convertResponseToAgentResponse(&response, logger)
	return resp, nil
}

func init() {
	if err := integration.RegisterFactory("openai", func(m map[string]any) (integration.Integration, error) {
		apikey, ok := m["api_key"].(string)
		if !ok {
			return nil, errors.New("api_key required in config")
		}
		model, ok := m["model"].(string)
		if !ok {
			model = defaultModel
		}
		return New(apikey, model)
	}); err != nil {
		panic(err)
	}
}
