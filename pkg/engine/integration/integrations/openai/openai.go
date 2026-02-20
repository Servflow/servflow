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
	"go.uber.org/zap"
)

type Config struct {
	APIKey         string `json:"api_key"`
	OrganizationID string `json:"organization_id"`
	ModelID        string `json:"model_id"`
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
		body, _ := io.ReadAll(r.Body)
		err = fmt.Errorf("expected status OK, got %d, message: %s", r.StatusCode, string(body))
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
