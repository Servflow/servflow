package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		wantErr bool
	}{
		{name: "valid config with model", apiKey: "test-key", model: "claude-3", wantErr: false},
		{name: "valid config without model uses default", apiKey: "test-key", model: "", wantErr: false},
		{name: "missing API key", apiKey: "", model: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.apiKey, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			if tt.model == "" {
				assert.Equal(t, defaultModel, client.model)
			} else {
				assert.Equal(t, tt.model, client.model)
			}
			assert.Equal(t, defaultMaxTokens, client.maxTokens)
		})
	}
}

func TestConvertAgentRequestToSDKParams(t *testing.T) {
	logger := zap.NewNop()

	t.Run("combines system message and instruction", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "Base system prompt",
			Instruction:   "Action prompt",
			Messages: []any{
				agent.MessageTypeContent{Role: agent.RoleTypeUser, Content: "Hello"},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, defaultModel, defaultMaxTokens)

		assert.Equal(t, defaultModel, params.Model)
		assert.Equal(t, defaultMaxTokens, params.MaxTokens)
		require.Len(t, params.System, 1)
		assert.Equal(t, "Base system prompt\n\nAction prompt", params.System[0].Text)
		require.Len(t, params.Messages, 1)
	})

	t.Run("tool definitions and tool history are converted", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "Base system prompt",
			Messages: []any{
				agent.MessageToolCall{
					ID:   "call_123",
					Name: "get_weather",
					Arguments: map[string]interface{}{
						"location": "Paris",
					},
				},
				agent.MessageToolCallResponse{
					ID:               "call_123",
					ToolResponseType: agent.ToolResponseTypeText,
					Text:             `{"temp":22}`,
				},
			},
			Tools: []agent.ToolInfo{
				{
					Name:        "get_weather",
					Description: "Get weather",
					InputSchema: mcp.ToolInputSchema{
						Type: "object",
						Properties: map[string]interface{}{
							"location": map[string]interface{}{"type": "string"},
						},
						Required: []string{"location"},
					},
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, defaultModel, defaultMaxTokens)

		require.Len(t, params.Tools, 1)
		require.NotNil(t, params.Tools[0].OfTool)
		assert.Equal(t, "get_weather", params.Tools[0].OfTool.Name)
		assert.Equal(t, "Get weather", params.Tools[0].OfTool.Description.Value)
		require.Len(t, params.Messages, 2)
		rawAssistant, err := json.Marshal(params.Messages[0])
		require.NoError(t, err)
		assert.Contains(t, string(rawAssistant), `"tool_use"`)
		rawUser, err := json.Marshal(params.Messages[1])
		require.NoError(t, err)
		assert.Contains(t, string(rawUser), `"tool_result"`)
	})

	t.Run("user image is converted to image block", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "Base system prompt",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeUser,
					Content: "Analyze this image",
					FileContent: requestctx.NewFileValue(io.NopCloser(bytes.NewReader([]byte{
						0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
					})), "image.png"),
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, defaultModel, defaultMaxTokens)
		require.Len(t, params.Messages, 1)
		raw, err := json.Marshal(params.Messages[0])
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"image"`)
		assert.Contains(t, string(raw), `"image/png"`)
	})

	t.Run("developer history messages are ignored", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "Base system prompt",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeDeveloper,
					Content: "Do not emit this as a message",
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, defaultModel, defaultMaxTokens)
		assert.Empty(t, params.Messages)
	})
}

func TestBuildToolResultMessage(t *testing.T) {
	logger := zap.NewNop()

	t.Run("image tool result is supported", func(t *testing.T) {
		msg := agent.MessageToolCallResponse{
			ID:               "call_img",
			ToolResponseType: agent.ToolResponseTypeImage,
			ImageData:        []byte("image-bytes"),
			ImageMimeType:    "image/png",
		}

		result, ok := buildToolResultMessage(logger, msg)
		require.True(t, ok)
		raw, err := json.Marshal(result)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"tool_result"`)
		assert.Contains(t, string(raw), `"image/png"`)
	})
}

func TestConvertSDKResponseToAgentResponse(t *testing.T) {
	logger := zap.NewNop()

	var response anthropic.Message
	err := json.Unmarshal([]byte(`{
		"id": "msg_123",
		"content": [
			{"type":"text","text":"I'll help with that."},
			{"type":"tool_use","id":"toolu_123","name":"get_weather","input":{"location":"Lagos"}}
		]
	}`), &response)
	require.NoError(t, err)

	result := convertSDKResponseToAgentResponse(&response, logger)
	assert.Equal(t, []agent.ContentResponse{{Text: "I'll help with that."}}, result.Content)
	require.Len(t, result.Tools, 1)
	assert.Equal(t, "get_weather", result.Tools[0].Name)
	assert.Equal(t, "toolu_123", result.Tools[0].ToolID)
	assert.Equal(t, "Lagos", result.Tools[0].Input["location"])
}

func TestClientProvideResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_123",
			"content": [
				{"type":"text","text":"Hello from Claude"}
			]
		}`))
	}))
	defer server.Close()

	client := &Client{
		client: anthropic.NewClient(
			option.WithAPIKey("test-key"),
			option.WithBaseURL(server.URL),
		),
		model:     defaultModel,
		maxTokens: defaultMaxTokens,
	}

	resp, err := client.ProvideResponse(context.Background(), agent.LLMRequest{
		SystemMessage: "Base system prompt",
		Instruction:   "Action prompt",
		Messages: []any{
			agent.MessageTypeContent{Role: agent.RoleTypeUser, Content: "Hello"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, []agent.ContentResponse{{Text: "Hello from Claude"}}, resp.Content)
}
