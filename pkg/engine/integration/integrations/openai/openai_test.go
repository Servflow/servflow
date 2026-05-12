package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/agent"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
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
		{
			name:    "valid config with model",
			apiKey:  "test-key",
			model:   "gpt-4",
			wantErr: false,
		},
		{
			name:    "valid config without model uses default",
			apiKey:  "test-key",
			model:   "",
			wantErr: false,
		},
		{
			name:    "missing API key",
			apiKey:  "",
			model:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.apiKey, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				if tt.model == "" {
					assert.Equal(t, defaultModel, client.model)
				} else {
					assert.Equal(t, tt.model, client.model)
				}
			}
		})
	}
}

func TestConvertSDKResponseToAgentResponse(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		response string
		expected agent.LLMResponse
	}{
		{
			name: "single message response",
			response: `{
				"output": [
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "Hello, how can I help you?"}]
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{{Text: "Hello, how can I help you?"}},
				Tools:   []agent.ToolResponseObject{},
			},
		},
		{
			name: "multiple messages response",
			response: `{
				"output": [
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "First message"}]
					},
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "Second message"}]
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{
					{Text: "First message"},
					{Text: "Second message"},
				},
				Tools: []agent.ToolResponseObject{},
			},
		},
		{
			name: "single function call response",
			response: `{
				"output": [
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{\"location\": \"New York\", \"unit\": \"celsius\"}",
						"call_id": "call_123"
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_123",
						Input:  map[string]interface{}{"location": "New York", "unit": "celsius"},
					},
				},
			},
		},
		{
			name: "multiple function calls response",
			response: `{
				"output": [
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{\"location\": \"Paris\"}",
						"call_id": "call_789"
					},
					{
						"type": "function_call",
						"name": "get_time",
						"arguments": "{\"timezone\": \"UTC\"}",
						"call_id": "call_101112"
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_789",
						Input:  map[string]interface{}{"location": "Paris"},
					},
					{
						Name:   "get_time",
						ToolID: "call_101112",
						Input:  map[string]interface{}{"timezone": "UTC"},
					},
				},
			},
		},
		{
			name: "mixed message and function call response",
			response: `{
				"output": [
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "I'll check the weather for you."}]
					},
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{\"location\": \"London\"}",
						"call_id": "call_456"
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{
					{Text: "I'll check the weather for you."},
				},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_456",
						Input:  map[string]interface{}{"location": "London"},
					},
				},
			},
		},
		{
			name: "empty output response",
			response: `{
				"output": []
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools:   []agent.ToolResponseObject{},
			},
		},
		{
			name: "invalid function arguments returns empty map",
			response: `{
				"output": [
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{invalid json}",
						"call_id": "call_invalid"
					}
				]
			}`,
			expected: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_invalid",
						Input:  map[string]interface{}{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := openai.NewClient(
				option.WithAPIKey("test-key"),
				option.WithBaseURL(server.URL),
			)

			resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
				Model: "gpt-4",
				Input: responses.ResponseNewParamsInputUnion{
					OfString: openai.String("test"),
				},
			})
			require.NoError(t, err)

			result := convertSDKResponseToAgentResponse(resp, logger)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAgentRequestToSDKParams(t *testing.T) {
	logger := zap.NewNop()

	t.Run("basic request with text messages", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "You are a helpful assistant.",
			Instruction:   "Be concise.",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeUser,
					Content: "Hello, how are you?",
				},
			},
			Tools: []agent.ToolInfo{},
		}

		params := convertAgentRequestToSDKParams(logger, req, "gpt-4")

		assert.Equal(t, "gpt-4", params.Model)
		assert.Equal(t, "You are a helpful assistant.", params.Instructions.Value)
		require.Len(t, params.Input.OfInputItemList, 2)
		instructionItem := params.Input.OfInputItemList[0]
		require.NotNil(t, instructionItem.OfMessage)
		assert.Equal(t, responses.EasyInputMessageRole("developer"), instructionItem.OfMessage.Role)
		require.Len(t, instructionItem.OfMessage.Content.OfInputItemContentList, 1)
		assert.Equal(t, "Be concise.", instructionItem.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
		inputItem := params.Input.OfInputItemList[1]
		require.NotNil(t, inputItem.OfMessage)
		assert.Equal(t, responses.EasyInputMessageRole("user"), inputItem.OfMessage.Role)
		require.Len(t, inputItem.OfMessage.Content.OfInputItemContentList, 1)
		assert.Equal(t, "Hello, how are you?", inputItem.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
	})

	t.Run("request with tools", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "You have access to weather tools.",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeUser,
					Content: "What's the weather like?",
				},
			},
			Tools: []agent.ToolInfo{
				{
					Name:        "get_weather",
					Description: "Get the current weather for a location",
					InputSchema: mcp.ToolInputSchema{
						Type: "object",
						Properties: map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
						},
						Required: []string{"location"},
					},
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, "gpt-4")

		assert.Equal(t, "gpt-4", params.Model)
		require.Len(t, params.Tools, 1)
		tool := params.Tools[0]
		require.NotNil(t, tool.OfFunction)
		assert.Equal(t, "get_weather", tool.OfFunction.Name)
		assert.Equal(t, "Get the current weather for a location", tool.OfFunction.Description.Value)
		assert.Equal(t, "object", tool.OfFunction.Parameters["type"])
		props := tool.OfFunction.Parameters["properties"].(map[string]interface{})
		locProp := props["location"].(map[string]interface{})
		assert.Equal(t, "string", locProp["type"])
	})

	t.Run("request with multiple tools", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "You have access to multiple tools.",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeUser,
					Content: "Help me with weather and time",
				},
			},
			Tools: []agent.ToolInfo{
				{
					Name:        "get_weather",
					Description: "Get weather information",
					InputSchema: mcp.ToolInputSchema{
						Type: "object",
						Properties: map[string]interface{}{
							"location": map[string]interface{}{"type": "string"},
						},
						Required: []string{"location"},
					},
				},
				{
					Name:        "get_time",
					Description: "Get current time",
					InputSchema: mcp.ToolInputSchema{
						Type: "object",
						Properties: map[string]interface{}{
							"timezone": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, "gpt-4")

		require.Len(t, params.Tools, 2)
		assert.Equal(t, "get_weather", params.Tools[0].OfFunction.Name)
		assert.Equal(t, "Get weather information", params.Tools[0].OfFunction.Description.Value)
		assert.Equal(t, "get_time", params.Tools[1].OfFunction.Name)
		assert.Equal(t, "Get current time", params.Tools[1].OfFunction.Description.Value)
	})

	t.Run("empty request", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "",
			Instruction:   "",
			Messages:      []any{},
			Tools:         []agent.ToolInfo{},
		}

		params := convertAgentRequestToSDKParams(logger, req, defaultModel)

		assert.Equal(t, defaultModel, params.Model)
		assert.Equal(t, "", params.Instructions.Value)
		assert.Empty(t, params.Input.OfInputItemList)
		assert.Empty(t, params.Tools)
	})

	t.Run("instruction is prepended as developer message", func(t *testing.T) {
		req := &agent.LLMRequest{
			SystemMessage: "Base system prompt",
			Instruction:   "Action-level instruction",
			Messages: []any{
				agent.MessageTypeContent{
					Role:    agent.RoleTypeUser,
					Content: "Hello",
				},
			},
		}

		params := convertAgentRequestToSDKParams(logger, req, "gpt-4")

		require.Len(t, params.Input.OfInputItemList, 2)
		require.NotNil(t, params.Input.OfInputItemList[0].OfMessage)
		assert.Equal(t, responses.EasyInputMessageRole("developer"), params.Input.OfInputItemList[0].OfMessage.Role)
		assert.Equal(t, "Action-level instruction", params.Input.OfInputItemList[0].OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
		require.NotNil(t, params.Input.OfInputItemList[1].OfMessage)
		assert.Equal(t, responses.EasyInputMessageRole("user"), params.Input.OfInputItemList[1].OfMessage.Role)
	})
}

func TestBuildMessageInput(t *testing.T) {
	logger := zap.NewNop()

	t.Run("user message", func(t *testing.T) {
		msg := agent.MessageTypeContent{
			Role:    agent.RoleTypeUser,
			Content: "Hello",
		}

		result := buildMessageInput(logger, msg)
		require.NotNil(t, result.OfMessage)
		assert.Nil(t, result.OfOutputMessage)
		assert.Equal(t, responses.EasyInputMessageRole("user"), result.OfMessage.Role)
		assert.Equal(t, "message", string(result.OfMessage.Type))
		require.Len(t, result.OfMessage.Content.OfInputItemContentList, 1)
		assert.Equal(t, "input_text", string(result.OfMessage.Content.OfInputItemContentList[0].OfInputText.Type))
		assert.Equal(t, "Hello", result.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
	})

	t.Run("assistant message", func(t *testing.T) {
		msg := agent.MessageTypeContent{
			Role:    agent.RoleTypeAssistant,
			Content: "Hello, I'm here to help.",
		}

		result := buildMessageInput(logger, msg)
		require.NotNil(t, result.OfOutputMessage)
		assert.Nil(t, result.OfMessage)
		assert.Equal(t, "assistant", string(result.OfOutputMessage.Role))
		assert.Equal(t, "message", string(result.OfOutputMessage.Type))
		assert.Equal(t, "completed", string(result.OfOutputMessage.Status))
		require.Len(t, result.OfOutputMessage.Content, 1)
		assert.Equal(t, "output_text", string(result.OfOutputMessage.Content[0].OfOutputText.Type))
		assert.Equal(t, "Hello, I'm here to help.", result.OfOutputMessage.Content[0].OfOutputText.Text)
	})

	t.Run("system message", func(t *testing.T) {
		msg := agent.MessageTypeContent{
			Role:    agent.RoleTypeSystem,
			Content: "System message",
		}

		result := buildMessageInput(logger, msg)
		require.NotNil(t, result.OfMessage)
		assert.Nil(t, result.OfOutputMessage)
		assert.Equal(t, responses.EasyInputMessageRole("system"), result.OfMessage.Role)
		assert.Equal(t, "message", string(result.OfMessage.Type))
		require.Len(t, result.OfMessage.Content.OfInputItemContentList, 1)
		assert.Equal(t, "System message", result.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
	})

	t.Run("developer message", func(t *testing.T) {
		msg := agent.MessageTypeContent{
			Role:    agent.RoleTypeDeveloper,
			Content: "Developer message",
		}

		result := buildMessageInput(logger, msg)
		require.NotNil(t, result.OfMessage)
		assert.Nil(t, result.OfOutputMessage)
		assert.Equal(t, responses.EasyInputMessageRole("developer"), result.OfMessage.Role)
		assert.Equal(t, "message", string(result.OfMessage.Type))
		require.Len(t, result.OfMessage.Content.OfInputItemContentList, 1)
		assert.Equal(t, "Developer message", result.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
	})

	t.Run("message with file content", func(t *testing.T) {
		msg := agent.MessageTypeContent{
			Role:        agent.RoleTypeUser,
			Content:     "Analyze this file",
			FileContent: requestctx.NewFileValue(io.NopCloser(strings.NewReader("test content")), "test.txt"),
		}

		result := buildMessageInput(logger, msg)
		require.NotNil(t, result.OfMessage)
		assert.Equal(t, responses.EasyInputMessageRole("user"), result.OfMessage.Role)
		require.Len(t, result.OfMessage.Content.OfInputItemContentList, 2)
		assert.Equal(t, "Analyze this file", result.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)
		require.NotNil(t, result.OfMessage.Content.OfInputItemContentList[1].OfInputImage)
		assert.Equal(t, "input_image", string(result.OfMessage.Content.OfInputItemContentList[1].OfInputImage.Type))
	})
}

func TestBuildFunctionCallOutput(t *testing.T) {
	t.Run("text response", func(t *testing.T) {
		val := agent.MessageToolCallResponse{
			ID:               "call_123",
			ToolResponseType: agent.ToolResponseTypeText,
			Text:             `{"temperature": 22, "condition": "sunny"}`,
		}

		result := buildFunctionCallOutput(val)
		require.NotNil(t, result.OfFunctionCallOutput)
		assert.Equal(t, "call_123", result.OfFunctionCallOutput.CallID)
		assert.Equal(t, `{"temperature": 22, "condition": "sunny"}`, result.OfFunctionCallOutput.Output.OfString.Value)
	})

	t.Run("image response", func(t *testing.T) {
		val := agent.MessageToolCallResponse{
			ID:               "call_456",
			ToolResponseType: agent.ToolResponseTypeImage,
			ImageData:        []byte("dGVzdCBpbWFnZSBkYXRh"),
			ImageMimeType:    "image/png",
		}

		result := buildFunctionCallOutput(val)
		require.NotNil(t, result.OfFunctionCallOutput)
		assert.Equal(t, "call_456", result.OfFunctionCallOutput.CallID)
		require.Len(t, result.OfFunctionCallOutput.Output.OfResponseFunctionCallOutputItemArray, 1)
		imageItem := result.OfFunctionCallOutput.Output.OfResponseFunctionCallOutputItemArray[0]
		require.NotNil(t, imageItem.OfInputImage)
		assert.Contains(t, imageItem.OfInputImage.ImageURL.Value, "data:image/png;base64,")
	})
}

func TestBuildFunctionCallInput(t *testing.T) {
	logger := zap.NewNop()

	t.Run("simple arguments", func(t *testing.T) {
		val := agent.MessageToolCall{
			ID:   "call_123",
			Name: "get_weather",
			Arguments: map[string]interface{}{
				"location": "New York",
				"unit":     "celsius",
			},
		}

		result := buildFunctionCallInput(logger, val)
		require.NotNil(t, result.OfFunctionCall)
		assert.Equal(t, "call_123", result.OfFunctionCall.CallID)
		assert.Equal(t, "get_weather", result.OfFunctionCall.Name)
		var args map[string]interface{}
		err := json.Unmarshal([]byte(result.OfFunctionCall.Arguments), &args)
		require.NoError(t, err)
		assert.Equal(t, "New York", args["location"])
		assert.Equal(t, "celsius", args["unit"])
	})

	t.Run("complex nested arguments", func(t *testing.T) {
		val := agent.MessageToolCall{
			ID:   "call_complex",
			Name: "process_data",
			Arguments: map[string]interface{}{
				"data": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{"id": 1, "active": true},
						map[string]interface{}{"id": 2, "active": false},
					},
				},
				"format": "json",
			},
		}

		result := buildFunctionCallInput(logger, val)
		require.NotNil(t, result.OfFunctionCall)
		assert.Equal(t, "call_complex", result.OfFunctionCall.CallID)
		assert.Equal(t, "process_data", result.OfFunctionCall.Name)
		var args map[string]interface{}
		err := json.Unmarshal([]byte(result.OfFunctionCall.Arguments), &args)
		require.NoError(t, err)
		assert.Equal(t, "json", args["format"])
		data := args["data"].(map[string]interface{})
		items := data["items"].([]interface{})
		assert.Len(t, items, 2)
	})

	t.Run("empty arguments", func(t *testing.T) {
		val := agent.MessageToolCall{
			ID:        "call_empty",
			Name:      "get_time",
			Arguments: map[string]interface{}{},
		}

		result := buildFunctionCallInput(logger, val)
		require.NotNil(t, result.OfFunctionCall)
		assert.Equal(t, "call_empty", result.OfFunctionCall.CallID)
		assert.Equal(t, "get_time", result.OfFunctionCall.Name)
		assert.Equal(t, "{}", result.OfFunctionCall.Arguments)
	})
}

func TestMapAgentRoleToSDKRole(t *testing.T) {
	tests := []struct {
		role     agent.RoleType
		expected string
	}{
		{agent.RoleTypeSystem, "system"},
		{agent.RoleTypeUser, "user"},
		{agent.RoleTypeAssistant, "assistant"},
		{agent.RoleTypeDeveloper, "developer"},
		{agent.RoleTypeUnknown, "user"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := mapAgentRoleToSDKRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarshalArguments(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		args     map[string]interface{}
		expected string
	}{
		{
			name:     "nil arguments",
			args:     nil,
			expected: "{}",
		},
		{
			name:     "empty arguments",
			args:     map[string]interface{}{},
			expected: "{}",
		},
		{
			name: "simple arguments",
			args: map[string]interface{}{
				"location": "New York",
			},
			expected: `{"location":"New York"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := marshalArguments(logger, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnmarshalArguments(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name      string
		arguments string
		expected  map[string]interface{}
	}{
		{
			name:      "empty string",
			arguments: "",
			expected:  map[string]interface{}{},
		},
		{
			name:      "invalid json",
			arguments: "{invalid}",
			expected:  map[string]interface{}{},
		},
		{
			name:      "valid json",
			arguments: `{"location": "New York", "unit": "celsius"}`,
			expected: map[string]interface{}{
				"location": "New York",
				"unit":     "celsius",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unmarshalArguments(logger, tt.arguments)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_ProvideResponse(t *testing.T) {
	tests := []struct {
		name             string
		serverResponse   string
		statusCode       int
		request          agent.LLMRequest
		expectedResponse agent.LLMResponse
		expectedError    bool
	}{
		{
			name: "successful response with message only",
			serverResponse: `{
				"output": [
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "Hello! How can I help you today?"}]
					}
				]
			}`,
			statusCode: http.StatusOK,
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageTypeContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expectedResponse: agent.LLMResponse{
				Content: []agent.ContentResponse{
					{Text: "Hello! How can I help you today?"},
				},
				Tools: []agent.ToolResponseObject{},
			},
			expectedError: false,
		},
		{
			name: "successful response with function call",
			serverResponse: `{
				"output": [
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{\"location\": \"New York\", \"unit\": \"celsius\"}",
						"call_id": "call_123"
					}
				]
			}`,
			statusCode: http.StatusOK,
			request: agent.LLMRequest{
				SystemMessage: "You have access to weather tools.",
				Messages: []any{
					agent.MessageTypeContent{
						Role:    agent.RoleTypeUser,
						Content: "What's the weather in New York?",
					},
				},
				Tools: []agent.ToolInfo{
					{
						Name:        "get_weather",
						Description: "Get current weather for a location",
						InputSchema: mcp.ToolInputSchema{
							Type: "object",
							Properties: map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "City name",
								},
							},
							Required: []string{"location"},
						},
					},
				},
			},
			expectedResponse: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_123",
						Input:  map[string]interface{}{"location": "New York", "unit": "celsius"},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "successful response with mixed message and function call",
			serverResponse: `{
				"output": [
					{
						"type": "message",
						"content": [{"type": "output_text", "text": "I'll check the weather for you."}]
					},
					{
						"type": "function_call",
						"name": "get_weather",
						"arguments": "{\"location\": \"London\"}",
						"call_id": "call_456"
					}
				]
			}`,
			statusCode: http.StatusOK,
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful weather assistant.",
				Messages: []any{
					agent.MessageTypeContent{
						Role:    agent.RoleTypeUser,
						Content: "What's the weather in London?",
					},
				},
				Tools: []agent.ToolInfo{
					{
						Name:        "get_weather",
						Description: "Get weather information",
						InputSchema: mcp.ToolInputSchema{Type: "object"},
					},
				},
			},
			expectedResponse: agent.LLMResponse{
				Content: []agent.ContentResponse{
					{Text: "I'll check the weather for you."},
				},
				Tools: []agent.ToolResponseObject{
					{
						Name:   "get_weather",
						ToolID: "call_456",
						Input:  map[string]interface{}{"location": "London"},
					},
				},
			},
			expectedError: false,
		},
		{
			name:           "unauthorized error",
			serverResponse: `{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`,
			statusCode:     http.StatusUnauthorized,
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageTypeContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
			},
			expectedError: true,
		},
		{
			name:           "server error",
			serverResponse: `{"error": {"message": "Internal server error", "type": "server_error"}}`,
			statusCode:     http.StatusInternalServerError,
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageTypeContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var reqBody map[string]interface{}
				err = json.Unmarshal(body, &reqBody)
				require.NoError(t, err)

				assert.NotEmpty(t, reqBody["model"])

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			sdkClient := openai.NewClient(
				option.WithAPIKey("test-api-key"),
				option.WithBaseURL(server.URL),
			)

			client := &Client{
				client: &sdkClient,
				model:  "gpt-4",
			}

			response, err := client.ProvideResponse(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResponse, response)
			}
		})
	}
}

func TestClientType(t *testing.T) {
	client := &Client{}
	assert.Equal(t, "openai", client.Type())
}

func TestCompleteConversationFlow(t *testing.T) {
	logger := zap.NewNop()

	req := &agent.LLMRequest{
		SystemMessage: "Handle complete conversation flow.",
		Messages: []any{
			agent.MessageTypeContent{
				Role:    agent.RoleTypeUser,
				Content: "Get weather for NY and LA",
			},
			agent.MessageToolCall{
				ID:   "call_ny",
				Name: "get_weather",
				Arguments: map[string]interface{}{
					"location": "New York",
					"unit":     "celsius",
				},
			},
			agent.MessageToolCall{
				ID:   "call_la",
				Name: "get_weather",
				Arguments: map[string]interface{}{
					"location": "Los Angeles",
				},
			},
			agent.MessageToolCallResponse{
				ID:               "call_ny",
				ToolResponseType: agent.ToolResponseTypeText,
				Text:             `{"temperature": 18, "condition": "sunny"}`,
			},
			agent.MessageToolCallResponse{
				ToolResponseType: agent.ToolResponseTypeText,
				ID:               "call_la",
				Text:             `{"temperature": 25, "condition": "clear"}`,
			},
			agent.MessageTypeContent{
				Role:    agent.RoleTypeAssistant,
				Content: "Weather retrieved for both cities.",
			},
		},
		Tools: []agent.ToolInfo{},
	}

	params := convertAgentRequestToSDKParams(logger, req, "gpt-4")

	assert.Equal(t, "gpt-4", params.Model)
	assert.Equal(t, "Handle complete conversation flow.", params.Instructions.Value)

	require.Len(t, params.Input.OfInputItemList, 6)

	// Verify user message
	userMsg := params.Input.OfInputItemList[0]
	require.NotNil(t, userMsg.OfMessage)
	assert.Equal(t, responses.EasyInputMessageRole("user"), userMsg.OfMessage.Role)
	require.Len(t, userMsg.OfMessage.Content.OfInputItemContentList, 1)
	assert.Equal(t, "Get weather for NY and LA", userMsg.OfMessage.Content.OfInputItemContentList[0].OfInputText.Text)

	// Verify first function call (NY)
	funcCallNY := params.Input.OfInputItemList[1]
	require.NotNil(t, funcCallNY.OfFunctionCall)
	assert.Equal(t, "call_ny", funcCallNY.OfFunctionCall.CallID)
	assert.Equal(t, "get_weather", funcCallNY.OfFunctionCall.Name)
	var argsNY map[string]interface{}
	err := json.Unmarshal([]byte(funcCallNY.OfFunctionCall.Arguments), &argsNY)
	require.NoError(t, err)
	assert.Equal(t, "New York", argsNY["location"])
	assert.Equal(t, "celsius", argsNY["unit"])

	// Verify second function call (LA)
	funcCallLA := params.Input.OfInputItemList[2]
	require.NotNil(t, funcCallLA.OfFunctionCall)
	assert.Equal(t, "call_la", funcCallLA.OfFunctionCall.CallID)
	assert.Equal(t, "get_weather", funcCallLA.OfFunctionCall.Name)
	var argsLA map[string]interface{}
	err = json.Unmarshal([]byte(funcCallLA.OfFunctionCall.Arguments), &argsLA)
	require.NoError(t, err)
	assert.Equal(t, "Los Angeles", argsLA["location"])

	// Verify first function call output (NY response)
	funcOutputNY := params.Input.OfInputItemList[3]
	require.NotNil(t, funcOutputNY.OfFunctionCallOutput)
	assert.Equal(t, "call_ny", funcOutputNY.OfFunctionCallOutput.CallID)
	assert.Equal(t, `{"temperature": 18, "condition": "sunny"}`, funcOutputNY.OfFunctionCallOutput.Output.OfString.Value)

	// Verify second function call output (LA response)
	funcOutputLA := params.Input.OfInputItemList[4]
	require.NotNil(t, funcOutputLA.OfFunctionCallOutput)
	assert.Equal(t, "call_la", funcOutputLA.OfFunctionCallOutput.CallID)
	assert.Equal(t, `{"temperature": 25, "condition": "clear"}`, funcOutputLA.OfFunctionCallOutput.Output.OfString.Value)

	// Verify assistant message
	assistantMsg := params.Input.OfInputItemList[5]
	require.NotNil(t, assistantMsg.OfOutputMessage)
	assert.Equal(t, "assistant", string(assistantMsg.OfOutputMessage.Role))
	assert.Equal(t, "completed", string(assistantMsg.OfOutputMessage.Status))
	require.Len(t, assistantMsg.OfOutputMessage.Content, 1)
	assert.Equal(t, "Weather retrieved for both cities.", assistantMsg.OfOutputMessage.Content[0].OfOutputText.Text)
}
