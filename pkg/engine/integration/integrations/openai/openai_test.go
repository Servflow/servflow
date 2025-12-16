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
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			cfg: Config{
				APIKey: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.cfg.APIKey, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("New() returned nil client for valid config")
			}
		})
	}
}

func TestConvertResponseToAgentResponse(t *testing.T) {
	logger := zap.NewNop()

	// Helper functions for building test data
	createMessage := func(text string) OutputObject {
		return OutputObject{
			Type:    OutputTypeMessage,
			Content: []ContentObject{{Type: "text", Text: text}},
		}
	}

	createFunctionCall := func(name, arguments, callID string) OutputObject {
		return OutputObject{
			Type:      OutputTypeFunctionCall,
			Name:      name,
			Arguments: arguments,
			CallID:    callID,
		}
	}

	createExpectedContent := func(texts ...string) []agent.ContentResponse {
		content := make([]agent.ContentResponse, len(texts))
		for i, text := range texts {
			content[i] = agent.ContentResponse{Text: text}
		}
		return content
	}

	createExpectedTool := func(name, toolID string, input map[string]interface{}) agent.ToolResponseObject {
		return agent.ToolResponseObject{
			Name:   name,
			ToolID: toolID,
			Input:  input,
		}
	}

	t.Run("message responses", func(t *testing.T) {
		tests := []struct {
			name     string
			response *Response
			expected agent.LLMResponse
		}{
			{
				name: "single message",
				response: &Response{
					Output: []OutputObject{createMessage("Hello, how can I help you?")},
				},
				expected: agent.LLMResponse{
					Content: createExpectedContent("Hello, how can I help you?"),
					Tools:   []agent.ToolResponseObject{},
				},
			},
			{
				name: "multiple messages",
				response: &Response{
					Output: []OutputObject{
						createMessage("First message"),
						createMessage("Second message"),
					},
				},
				expected: agent.LLMResponse{
					Content: createExpectedContent("First message", "Second message"),
					Tools:   []agent.ToolResponseObject{},
				},
			},
			{
				name: "message with empty content",
				response: &Response{
					Output: []OutputObject{{Type: OutputTypeMessage, Content: []ContentObject{}}},
				},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools:   []agent.ToolResponseObject{},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := convertResponseToAgentResponse(tt.response, logger)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("function call responses", func(t *testing.T) {
		tests := []struct {
			name     string
			response *Response
			expected agent.LLMResponse
		}{
			{
				name: "single function call",
				response: &Response{
					Output: []OutputObject{
						createFunctionCall("get_weather", `{"location": "New York", "unit": "celsius"}`, "call_123"),
					},
				},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools: []agent.ToolResponseObject{
						createExpectedTool("get_weather", "call_123", map[string]interface{}{
							"location": "New York",
							"unit":     "celsius",
						}),
					},
				},
			},
			{
				name: "multiple function calls",
				response: &Response{
					Output: []OutputObject{
						createFunctionCall("get_weather", `{"location": "Paris"}`, "call_789"),
						createFunctionCall("get_time", `{"timezone": "UTC"}`, "call_101112"),
					},
				},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools: []agent.ToolResponseObject{
						createExpectedTool("get_weather", "call_789", map[string]interface{}{"location": "Paris"}),
						createExpectedTool("get_time", "call_101112", map[string]interface{}{"timezone": "UTC"}),
					},
				},
			},
			{
				name: "complex function arguments",
				response: &Response{
					Output: []OutputObject{
						createFunctionCall("complex_function", `{"nested": {"key": "value"}, "array": [1, 2, 3], "boolean": true, "number": 42}`, "call_complex"),
					},
				},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools: []agent.ToolResponseObject{
						createExpectedTool("complex_function", "call_complex", map[string]interface{}{
							"nested":  map[string]interface{}{"key": "value"},
							"array":   []interface{}{1.0, 2.0, 3.0},
							"boolean": true,
							"number":  42.0,
						}),
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := convertResponseToAgentResponse(tt.response, logger)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("mixed responses", func(t *testing.T) {
		t.Run("message and function call", func(t *testing.T) {
			response := &Response{
				Output: []OutputObject{
					createMessage("I'll get the weather for you."),
					createFunctionCall("get_weather", `{"location": "London"}`, "call_456"),
				},
			}
			expected := agent.LLMResponse{
				Content: createExpectedContent("I'll get the weather for you."),
				Tools: []agent.ToolResponseObject{
					createExpectedTool("get_weather", "call_456", map[string]interface{}{"location": "London"}),
				},
			}

			result := convertResponseToAgentResponse(response, logger)
			assert.Equal(t, expected, result)
		})
	})

	t.Run("edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			response *Response
			expected agent.LLMResponse
		}{
			{
				name:     "empty response",
				response: &Response{Output: []OutputObject{}},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools:   []agent.ToolResponseObject{},
				},
			},
			{
				name: "invalid JSON arguments - should skip invalid, keep valid",
				response: &Response{
					Output: []OutputObject{
						createFunctionCall("invalid_function", `{invalid json}`, "call_invalid"),
						createFunctionCall("valid_function", `{"valid": "json"}`, "call_valid"),
					},
				},
				expected: agent.LLMResponse{
					Content: []agent.ContentResponse{},
					Tools: []agent.ToolResponseObject{
						createExpectedTool("valid_function", "call_valid", map[string]interface{}{"valid": "json"}),
					},
				},
			},
			{
				name: "unknown output type - should ignore unknown, process known",
				response: &Response{
					Output: []OutputObject{
						{
							Type:    "unknown_type",
							Content: []ContentObject{{Type: "text", Text: "This should be ignored"}},
						},
						createMessage("This should be included"),
					},
				},
				expected: agent.LLMResponse{
					Content: createExpectedContent("This should be included"),
					Tools:   []agent.ToolResponseObject{},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := convertResponseToAgentResponse(tt.response, logger)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestConvertAgentRequestToRequest(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		request  *agent.LLMRequest
		expected RequestBody
	}{
		{
			name: "basic request with text messages",
			request: &agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello, how are you?",
					},
					agent.MessageContent{
						Role:    agent.RoleTypeAssistant,
						Content: "I'm doing great, thank you!",
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "You are a helpful assistant.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Hello, how are you?"},
						},
					},
					MessageInput{
						Role: "assistant",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "I'm doing great, thank you!"},
						},
					},
				},
				Tools: []ToolsRequestConfig{},
			},
		},
		{
			name: "request with tools",
			request: &agent.LLMRequest{
				SystemMessage: "You have access to weather tools.",
				Messages: []any{
					agent.MessageContent{
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
								"unit": map[string]interface{}{
									"type": "string",
									"enum": []string{"celsius", "fahrenheit"},
								},
							},
							Required: []string{"location"},
						},
					},
				},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "You have access to weather tools.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "What's the weather like?"},
						},
					},
				},
				Tools: []ToolsRequestConfig{
					{
						Name:        "get_weather",
						Description: "Get the current weather for a location",
						Parameters: mcp.ToolInputSchema{
							Type: "object",
							Properties: map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "The city and state, e.g. San Francisco, CA",
								},
								"unit": map[string]interface{}{
									"type": "string",
									"enum": []string{"celsius", "fahrenheit"},
								},
							},
							Required: []string{"location"},
						},
						Type: ToolTypeFunction,
					},
				},
			},
		},
		{
			name: "complete conversation with all message types",
			request: &agent.LLMRequest{
				SystemMessage: "Handle complete conversation flow.",
				Messages: []any{
					agent.MessageContent{
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
					agent.MessageContent{
						Role:    agent.RoleTypeAssistant,
						Content: "Weather retrieved for both cities.",
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "Handle complete conversation flow.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Get weather for NY and LA"},
						},
					},
					FunctionCall{
						Type:      FunctionCallType,
						CallID:    "call_ny",
						Name:      "get_weather",
						Arguments: `{"location":"New York","unit":"celsius"}`,
					},
					FunctionCall{
						Type:      FunctionCallType,
						CallID:    "call_la",
						Name:      "get_weather",
						Arguments: `{"location":"Los Angeles"}`,
					},
					FunctionCallOutput{
						Type:   FunctionCallOutputType,
						CallID: "call_ny",
						Output: `{"temperature": 18, "condition": "sunny"}`,
					},
					FunctionCallOutput{
						Type:   FunctionCallOutputType,
						CallID: "call_la",
						Output: `{"temperature": 25, "condition": "clear"}`,
					},
					MessageInput{
						Role: "assistant",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Weather retrieved for both cities."},
						},
					},
				},
				Tools: []ToolsRequestConfig{},
			},
		},
		{
			name: "request with all role types",
			request: &agent.LLMRequest{
				SystemMessage: "You are a development assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeSystem,
						Content: "System message",
					},
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "User message",
					},
					agent.MessageContent{
						Role:    agent.RoleTypeAssistant,
						Content: "Assistant message",
					},
					agent.MessageContent{
						Role:    agent.RoleTypeDeveloper,
						Content: "Developer message",
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "You are a development assistant.",
				Input: []interface{}{
					MessageInput{
						Role: "system",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "System message"},
						},
					},
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "User message"},
						},
					},
					MessageInput{
						Role: "assistant",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Assistant message"},
						},
					},
					MessageInput{
						Role: "developer",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Developer message"},
						},
					},
				},
				Tools: []ToolsRequestConfig{},
			},
		},
		{
			name: "request with multiple tools",
			request: &agent.LLMRequest{
				SystemMessage: "You have access to multiple tools.",
				Messages: []any{
					agent.MessageContent{
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
								"location": map[string]interface{}{
									"type": "string",
								},
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
								"timezone": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "You have access to multiple tools.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Help me with weather and time"},
						},
					},
				},
				Tools: []ToolsRequestConfig{
					{
						Name:        "get_weather",
						Description: "Get weather information",
						Parameters: mcp.ToolInputSchema{
							Type: "object",
							Properties: map[string]interface{}{
								"location": map[string]interface{}{
									"type": "string",
								},
							},
							Required: []string{"location"},
						},
						Type: ToolTypeFunction,
					},
					{
						Name:        "get_time",
						Description: "Get current time",
						Parameters: mcp.ToolInputSchema{
							Type: "object",
							Properties: map[string]interface{}{
								"timezone": map[string]interface{}{
									"type": "string",
								},
							},
						},
						Type: ToolTypeFunction,
					},
				},
			},
		},
		{
			name: "empty request",
			request: &agent.LLMRequest{
				SystemMessage: "",
				Messages:      []any{},
				Tools:         []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "",
				Input:        []interface{}{},
				Tools:        []ToolsRequestConfig{},
			},
		},
		{
			name: "tool calls with complex and edge case arguments",
			request: &agent.LLMRequest{
				SystemMessage: "Handle various tool argument scenarios.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "Process data and get time",
					},
					agent.MessageToolCall{
						ID:   "call_complex",
						Name: "process_data",
						Arguments: map[string]interface{}{
							"data": map[string]interface{}{
								"items": []interface{}{
									map[string]interface{}{"id": 1, "active": true},
									map[string]interface{}{"id": 2, "active": false},
								},
								"filters": map[string]interface{}{
									"status": "active",
									"count":  10,
								},
							},
							"format": "json",
						},
					},
					agent.MessageToolCall{
						ID:        "call_empty",
						Name:      "get_time",
						Arguments: map[string]interface{}{},
					},
					agent.MessageToolCall{
						ID:   "call_invalid",
						Name: "bad_call",
						Arguments: map[string]interface{}{
							"invalid": func() {}, // This should be skipped due to marshal error
						},
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "Handle various tool argument scenarios.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Process data and get time"},
						},
					},
					FunctionCall{
						Type:      FunctionCallType,
						CallID:    "call_complex",
						Name:      "process_data",
						Arguments: `{"data":{"filters":{"count":10,"status":"active"},"items":[{"active":true,"id":1},{"active":false,"id":2}]},"format":"json"}`,
					},
					FunctionCall{
						Type:      FunctionCallType,
						CallID:    "call_empty",
						Name:      "get_time",
						Arguments: `{}`,
					},
					// Note: call_invalid should be skipped due to marshal error
				},
				Tools: []ToolsRequestConfig{},
			},
		},
		{
			name: "request with file content",
			request: &agent.LLMRequest{
				SystemMessage: "You can process files.",
				Messages: []any{
					agent.MessageContent{
						Role:        agent.RoleTypeUser,
						Content:     "Analyze this file",
						FileContent: requestctx.NewFileValue(io.NopCloser(strings.NewReader("test content")), "test.txt"),
					},
				},
				Tools: []agent.ToolInfo{},
			},
			expected: RequestBody{
				Model:        defaultModel,
				Instructions: "You can process files.",
				Input: []interface{}{
					MessageInput{
						Role: "user",
						Content: []ContentInputWrapper{
							{Type: InputTypeText, Text: "Analyze this file"},
							{Type: InputTypeImage, ImageURL: ""},
						},
					},
				},
				Tools: []ToolsRequestConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAgentRequestToRequest(logger, tt.request, defaultModel)

			if tt.name == "request with file content" {
				assert.Equal(t, tt.expected.Model, result.Model)
				assert.Equal(t, tt.expected.Instructions, result.Instructions)
				assert.Equal(t, len(tt.expected.Input), len(result.Input))
				assert.Equal(t, tt.expected.Tools, result.Tools)

				if len(result.Input) > 0 {
					resultMsg, ok := result.Input[0].(MessageInput)
					assert.True(t, ok)
					expectedMsg := tt.expected.Input[0].(MessageInput)
					assert.Equal(t, expectedMsg.Role, resultMsg.Role)
					assert.Equal(t, len(expectedMsg.Content), len(resultMsg.Content))

					if len(resultMsg.Content) >= 2 {
						assert.Equal(t, InputTypeText, resultMsg.Content[0].Type)
						assert.Equal(t, expectedMsg.Content[0].Text, resultMsg.Content[0].Text)
						assert.Equal(t, InputTypeImage, resultMsg.Content[1].Type)
						assert.NotEmpty(t, resultMsg.Content[1].ImageURL)
					}
				}
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestClient_ProvideResponse(t *testing.T) {
	tests := []struct {
		name             string
		serverHandler    func(w http.ResponseWriter, r *http.Request)
		request          agent.LLMRequest
		expectedError    bool
		expectedResponse agent.LLMResponse
	}{
		{
			name: "successful response with message only",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != http.MethodPost {
					t.Errorf("Expected method POST, got %s", r.Method)
				}
				if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
					t.Errorf("Expected Content-Type: application/json, got %s", contentType)
				}
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-api-key" {
					t.Errorf("Expected Authorization: Bearer test-api-key, got %s", auth)
				}

				// Verify request body
				var reqBody RequestBody
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				mockResponse := Response{
					Output: []OutputObject{
						{
							Type: OutputTypeMessage,
							Content: []ContentObject{
								{Type: "text", Text: "Hello! How can I help you today?"},
							},
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockResponse)
			},
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageContent{
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
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				mockResponse := Response{
					Output: []OutputObject{
						{
							Type:      OutputTypeFunctionCall,
							Name:      "get_weather",
							Arguments: `{"location": "New York", "unit": "celsius"}`,
							CallID:    "call_123",
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockResponse)
			},
			request: agent.LLMRequest{
				SystemMessage: "You have access to weather tools.",
				Messages: []any{
					agent.MessageContent{
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
								"unit": map[string]interface{}{
									"type":        "string",
									"description": "Temperature unit",
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
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				mockResponse := Response{
					Output: []OutputObject{
						{
							Type: OutputTypeMessage,
							Content: []ContentObject{
								{Type: "text", Text: "I'll check the weather for you."},
							},
						},
						{
							Type:      OutputTypeFunctionCall,
							Name:      "get_weather",
							Arguments: `{"location": "London"}`,
							CallID:    "call_456",
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockResponse)
			},
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful weather assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "What's the weather in London?",
					},
				},
				Tools: []agent.ToolInfo{
					{
						Name:        "get_weather",
						Description: "Get weather information",
						InputSchema: mcp.ToolInputSchema{
							Type: "object",
							Properties: map[string]interface{}{
								"location": map[string]interface{}{
									"type": "string",
								},
							},
						},
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
			name: "response with tool response message",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Verify that tool response is properly formatted in request
				var reqBody RequestBody
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				// Check that the tool response input is correctly formatted
				found := false
				for _, input := range reqBody.Input {
					if funcOutput, ok := input.(map[string]interface{}); ok {
						if funcOutput["type"] == FunctionCallOutputType {
							found = true
							break
						}
					}
				}
				if !found {
					t.Error("Expected function call output in request")
				}

				mockResponse := Response{
					Output: []OutputObject{
						{
							Type: OutputTypeMessage,
							Content: []ContentObject{
								{Type: "text", Text: "Based on the weather data, it's sunny today."},
							},
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockResponse)
			},
			request: agent.LLMRequest{
				SystemMessage: "Process tool responses and provide helpful information.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "What's the weather like?",
					},
					agent.MessageToolCallResponse{
						ID:   "call_weather_123",
						Text: `{"temperature": 22, "condition": "sunny"}`,
					},
				},
			},
			expectedResponse: agent.LLMResponse{
				Content: []agent.ContentResponse{
					{Text: "Based on the weather data, it's sunny today."},
				},
				Tools: []agent.ToolResponseObject{},
			},
			expectedError: false,
		},
		{
			name: "unauthorized error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Invalid API key"}`))
			},
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
			},
			expectedError: true,
		},
		{
			name: "server error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Internal server error"}`))
			},
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
			},
			expectedError: true,
		},
		{
			name: "invalid JSON response",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{invalid json}`))
			},
			request: agent.LLMRequest{
				SystemMessage: "You are a helpful assistant.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "Hello",
					},
				},
			},
			expectedError: true,
		},
		{
			name: "response with invalid function call arguments",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				mockResponse := Response{
					Output: []OutputObject{
						{
							Type:      OutputTypeFunctionCall,
							Name:      "get_weather",
							Arguments: `{invalid json}`,
							CallID:    "call_invalid",
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockResponse)
			},
			request: agent.LLMRequest{
				SystemMessage: "You have access to weather tools.",
				Messages: []any{
					agent.MessageContent{
						Role:    agent.RoleTypeUser,
						Content: "What's the weather?",
					},
				},
				Tools: []agent.ToolInfo{
					{
						Name:        "get_weather",
						Description: "Get weather",
						InputSchema: mcp.ToolInputSchema{Type: "object"},
					},
				},
			},
			expectedResponse: agent.LLMResponse{
				Content: []agent.ContentResponse{},
				Tools:   []agent.ToolResponseObject{}, // Should be empty due to invalid JSON
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			client := &Client{
				client: server.Client(),
				apiKey: "test-api-key",
			}

			// Override endpoint for testing
			originalEndpoint := endpoint
			endpoint = server.URL
			defer func() { endpoint = originalEndpoint }()

			response, err := client.ProvideResponse(context.Background(), tt.request)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				assert.Equal(t, tt.expectedResponse, response)
			}
		})
	}
}
