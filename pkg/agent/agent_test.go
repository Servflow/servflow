package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var toolList = `
[
						{
								"name": "get_weather",
								"title": "Weather Information Provider",
								"description": "Get current weather information for a location",
								"inputSchema": {
										"type": "object",
										"properties": {
												"location": {
														"type": "string",
														"description": "City name or zip code"
												}
										},
										"required": ["location"]
								}
						}
				]`

func TestNewOrchestrator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockToolManager := NewMockToolManager(ctrl)
	orchestrator, err := NewSession("You are an agent for a restaurant review system", nil, WithToolManager(mockToolManager))
	require.NoError(t, err)

	expectedMessages := []any{
		MessageContent{
			Role:    RoleTypeDeveloper,
			Content: "You are an agent for a restaurant review system",
		},
	}
	assert.Equal(t, expectedMessages, orchestrator.messages)
}

func TestOrchestrator_TestQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	systemPrompt := "You are an agent for a restaurant review system"
	testQuery := "What's the weather like in Lagos?"

	// First LLM response with tool call
	firstResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "I'll check the weather for Lagos",
			},
		},
		Tools: []ToolResponseObject{
			{
				Name: "get_weather",
				Input: map[string]any{
					"location": "lagos",
				},
				ToolID: "test",
			},
		},
	}

	// Final LLM response without tool call
	finalResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "The weather in Lagos is sunny with 28°C",
			},
		},
	}

	mockToolManager := NewMockToolManager(ctrl)
	mockLLmHandler := NewMockLLmProvider(ctrl)

	// Setup expectations
	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList().Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return("Temperature: 28°C, Condition: Sunny", nil)

	// We expect two LLM calls - one that returns a tool call, and one that gives the final response
	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), LLMRequest{
				Tools:         toolInfoList,
				SystemMessage: string(instructions),
				Messages: []any{
					MessageContent{
						Message: Message{Type: MessageTypeText},
						Role:    RoleTypeDeveloper,
						Content: "You are an agent for a restaurant review system",
					},
					MessageContent{
						Message: Message{Type: MessageTypeText},
						Role:    RoleTypeUser,
						Content: "What's the weather like in Lagos?",
					},
				},
			}).
			Return(firstResponse, nil),

		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), LLMRequest{
				Tools:         toolInfoList,
				SystemMessage: string(instructions),
				Messages: []any{
					MessageContent{
						Message: Message{Type: MessageTypeText},
						Role:    RoleTypeDeveloper,
						Content: "You are an agent for a restaurant review system",
					},
					MessageContent{
						Message: Message{Type: MessageTypeText},
						Role:    RoleTypeUser,
						Content: "What's the weather like in Lagos?",
					},
					MessageContent{
						Message: Message{Type: MessageTypeText},
						Role:    RoleTypeAssistant,
						Content: "I'll check the weather for Lagos",
					},
					MessageToolCall{
						Message:   Message{Type: MessageTypeToolCall},
						ID:        "test",
						Name:      "get_weather",
						Arguments: map[string]interface{}{"location": "lagos"},
					},
					MessageToolCallResponse{
						Message: Message{Type: MessageTypeToolResponse},
						ID:      "test",
						Output:  "Temperature: 28°C, Condition: Sunny",
					},
				},
			}).
			Return(finalResponse, nil),
	)

	// Create agent and run query
	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery)
	require.NoError(t, err)

	// Assert the final accumulated response
	expectedResponse := "I'll check the weather for Lagos\nThe weather in Lagos is sunny with 28°C\n"
	assert.Equal(t, expectedResponse, result)
}

func TestOrchestrator_ProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	systemPrompt := "You are an agent for a restaurant review system"
	testQuery := "What's the weather like in Lagos?"

	mockToolManager := NewMockToolManager(ctrl)
	mockLLmHandler := NewMockLLmProvider(ctrl)

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList().Return(toolInfoList)

	providerError := errors.New("provider error")
	mockLLmHandler.EXPECT().
		ProvideResponse(gomock.Any(), gomock.Any()).
		Return(LLMResponse{}, providerError)

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider error")
	assert.Empty(t, result)
}

func TestOrchestrator_ToolErrorWithRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	systemPrompt := "You are an agent for a restaurant review system"
	testQuery := "What's the weather like in Lagos?"

	firstResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "I'll check the weather for Lagos",
			},
		},
		Tools: []ToolResponseObject{
			{
				Name: "get_weather",
				Input: map[string]any{
					"location": "lagos",
				},
				ToolID: "test",
			},
		},
	}

	secondResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "Let me try again",
			},
		},
		Tools: []ToolResponseObject{
			{
				Name: "get_weather",
				Input: map[string]any{
					"location": "lagos",
				},
				ToolID: "test2",
			},
		},
	}

	finalResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "The weather in Lagos is sunny with 28°C",
			},
		},
	}

	mockToolManager := NewMockToolManager(ctrl)
	mockLLmHandler := NewMockLLmProvider(ctrl)

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList().Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return("", errors.New("tool error"))
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return("Temperature: 28°C, Condition: Sunny", nil)

	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(secondResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery)
	require.NoError(t, err)
	assert.Contains(t, result, "The weather in Lagos is sunny with 28°C")
}

func TestOrchestrator_ToolErrorWithLLMWrapup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	systemPrompt := "You are an agent for a restaurant review system"
	testQuery := "What's the weather like in Lagos?"

	firstResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "I'll check the weather for Lagos",
			},
		},
		Tools: []ToolResponseObject{
			{
				Name: "get_weather",
				Input: map[string]any{
					"location": "lagos",
				},
				ToolID: "test",
			},
		},
	}

	finalResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "I'm unable to complete the weather request due to an error",
			},
		},
	}

	mockToolManager := NewMockToolManager(ctrl)
	mockLLmHandler := NewMockLLmProvider(ctrl)

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList().Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return("", errors.New("tool error"))

	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery)
	require.NoError(t, err)
	assert.Contains(t, result, "I'm unable to complete the weather request due to an error")
}

func TestSession_ConversationIDMessageRetrieval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conversationID := "test-conversation-123"
	systemPrompt := "You are a helpful assistant"
	testQuery1 := "What's the weather in New York?"
	testQuery2 := "What about Boston?"

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}

	t.Run("initial conversation with storage", func(t *testing.T) {
		mockToolManager := NewMockToolManager(ctrl)
		mockLLmHandler := NewMockLLmProvider(ctrl)

		firstResponse := LLMResponse{
			Content: []ContentResponse{
				{
					Text: "I'll check the weather for New York",
				},
			},
			Tools: []ToolResponseObject{
				{
					Name: "get_weather",
					Input: map[string]any{
						"location": "New York",
					},
					ToolID: "weather_ny_001",
				},
			},
		}

		// Final LLM response
		finalResponse := LLMResponse{
			Content: []ContentResponse{
				{
					Text: "The weather in New York is cloudy with 22°C",
				},
			},
		}

		// Setup mock expectations
		mockToolManager.EXPECT().ToolList().Return(toolInfoList).AnyTimes()
		mockToolManager.EXPECT().
			CallTool(gomock.Any(), "get_weather", map[string]any{"location": "New York"}).
			Return("Temperature: 22°C, Condition: Cloudy", nil)

		gomock.InOrder(
			mockLLmHandler.EXPECT().
				ProvideResponse(gomock.Any(), LLMRequest{
					Tools:         toolInfoList,
					SystemMessage: string(instructions),
					Messages: []any{
						MessageContent{
							Message: Message{Type: MessageTypeText},
							Role:    RoleTypeDeveloper,
							Content: systemPrompt,
						},
						MessageContent{
							Message: Message{Type: MessageTypeText},
							Role:    RoleTypeUser,
							Content: testQuery1,
						},
					},
				}).
				Return(firstResponse, nil),

			mockLLmHandler.EXPECT().
				ProvideResponse(gomock.Any(), gomock.Any()).
				Return(finalResponse, nil),
		)

		// Create session with conversation ID
		session, err := NewSession(systemPrompt, mockLLmHandler,
			WithToolManager(mockToolManager),
			WithConversationID(conversationID))
		require.NoError(t, err)

		// Run first query
		result, err := session.Query(context.Background(), testQuery1)
		require.NoError(t, err)
		assert.Contains(t, result, "I'll check the weather for New York")
		assert.Contains(t, result, "The weather in New York is cloudy with 22°C")

		// Verify messages were stored in session
		assert.Equal(t, 6, len(session.messages))

		t.Run("new session retrieves stored messages and continues conversation", func(t *testing.T) {
			mockToolManager2 := NewMockToolManager(ctrl)
			mockLLmHandler2 := NewMockLLmProvider(ctrl)

			// Second query response
			secondResponse := LLMResponse{
				Content: []ContentResponse{
					{
						Text: "I'll check Boston's weather too",
					},
				},
				Tools: []ToolResponseObject{
					{
						Name: "get_weather",
						Input: map[string]any{
							"location": "Boston",
						},
						ToolID: "weather_boston_001",
					},
				},
			}

			finalResponse2 := LLMResponse{
				Content: []ContentResponse{
					{
						Text: "Boston has sunny weather at 25°C. So New York is cloudy at 22°C while Boston is sunny at 25°C.",
					},
				},
			}

			// Setup mock expectations for second session
			mockToolManager2.EXPECT().ToolList().Return(toolInfoList).AnyTimes()
			mockToolManager2.EXPECT().
				CallTool(gomock.Any(), "get_weather", map[string]any{"location": "Boston"}).
				Return("Temperature: 25°C, Condition: Sunny", nil)

			gomock.InOrder(
				mockLLmHandler2.EXPECT().
					ProvideResponse(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, req LLMRequest) {
						// Verify that the retrieved messages include all the previous conversation
						assert.Equal(t, 7, len(req.Messages)) // Developer + User + Assistant + ToolCall + ToolResponse+ initial response + new User query

						// Verify message types and content
						messages := req.Messages

						// First message should be developer instruction
						developerMsg, ok := messages[0].(MessageContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeDeveloper, developerMsg.Role)
						assert.Equal(t, systemPrompt, developerMsg.Content)

						// Second message should be first user query
						userMsg1, ok := messages[1].(MessageContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeUser, userMsg1.Role)
						assert.Equal(t, testQuery1, userMsg1.Content)

						// Third message should be assistant response
						assistantMsg1, ok := messages[2].(MessageContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeAssistant, assistantMsg1.Role)
						assert.Contains(t, assistantMsg1.Content, "I'll check the weather for New York")

						// Fourth message should be tool call
						toolCallMsg, ok := messages[3].(MessageToolCall)
						assert.True(t, ok)
						assert.Equal(t, "weather_ny_001", toolCallMsg.ID)
						assert.Equal(t, "get_weather", toolCallMsg.Name)
						assert.Equal(t, map[string]interface{}{"location": "New York"}, toolCallMsg.Arguments)

						// Fifth message should be tool response
						toolResponseMsg, ok := messages[4].(MessageToolCallResponse)
						assert.True(t, ok)
						assert.Equal(t, "weather_ny_001", toolResponseMsg.ID)
						assert.Equal(t, "Temperature: 22°C, Condition: Cloudy", toolResponseMsg.Output)

						//sixth message should be llm second response
						assistantMsg2, ok := messages[5].(MessageContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeAssistant, assistantMsg2.Role)
						assert.Equal(t, "The weather in New York is cloudy with 22°C", assistantMsg2.Content)

						// seventh message should be the new user query
						userMsg2, ok := messages[6].(MessageContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeUser, userMsg2.Role)
						assert.Equal(t, testQuery2, userMsg2.Content)
					}).
					Return(secondResponse, nil),

				mockLLmHandler2.EXPECT().
					ProvideResponse(gomock.Any(), gomock.Any()).
					Return(finalResponse2, nil),
			)

			newSession, err := NewSession(systemPrompt, mockLLmHandler2,
				WithToolManager(mockToolManager2),
				WithConversationID(conversationID))
			require.NoError(t, err)

			// Verify that messages were loaded from storage + developer message
			assert.Equal(t, 6, len(newSession.messages))

			// Verify the loaded messages match what we expect
			messages := newSession.messages

			// Check developer message
			developerMsg, ok := messages[0].(MessageContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeDeveloper, developerMsg.Role)
			assert.Equal(t, systemPrompt, developerMsg.Content)

			// Check first user message
			userMsg1, ok := messages[1].(MessageContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeUser, userMsg1.Role)
			assert.Equal(t, testQuery1, userMsg1.Content)

			// Check assistant message
			assistantMsg1, ok := messages[2].(MessageContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeAssistant, assistantMsg1.Role)

			// Check tool call message
			toolCallMsg, ok := messages[3].(MessageToolCall)
			assert.True(t, ok)
			assert.Equal(t, "weather_ny_001", toolCallMsg.ID)
			assert.Equal(t, "get_weather", toolCallMsg.Name)

			// Check tool response message
			toolResponseMsg, ok := messages[4].(MessageToolCallResponse)
			assert.True(t, ok)
			assert.Equal(t, "weather_ny_001", toolResponseMsg.ID)

			// check assistant response
			assistantMsg2, ok := messages[5].(MessageContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeAssistant, assistantMsg2.Role)
			assert.Equal(t, "The weather in New York is cloudy with 22°C", assistantMsg2.Content)

			// Continue the conversation with second query
			result, err := newSession.Query(context.Background(), testQuery2)
			require.NoError(t, err)
			assert.Contains(t, result, "Boston")
			assert.Contains(t, result, "New York is cloudy")
			assert.Contains(t, result, "Boston is sunny")
		})
	})

	// Test error case: empty conversation ID
	t.Run("error on empty conversation ID", func(t *testing.T) {
		mockLLmHandler := NewMockLLmProvider(ctrl)
		_, err := NewSession(systemPrompt, mockLLmHandler, WithConversationID(""))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conversationID can not be empty")
	})
}

func TestSession_WithReturnOnlyLastMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	systemPrompt := "You are a helpful assistant"
	testQuery := "Tell me about the weather"

	// First LLM response with tool call
	firstResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "I'll check the weather for you",
			},
		},
		Tools: []ToolResponseObject{
			{
				Name: "get_weather",
				Input: map[string]any{
					"location": "default",
				},
				ToolID: "test",
			},
		},
	}

	// Final LLM response without tool call
	finalResponse := LLMResponse{
		Content: []ContentResponse{
			{
				Text: "The weather is sunny today",
			},
		},
	}

	mockToolManager := NewMockToolManager(ctrl)
	mockLLmHandler := NewMockLLmProvider(ctrl)

	// Setup expectations
	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList().Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "default"}).
		Return("Weather: Sunny, 25°C", nil)

	// We expect two LLM calls - one that returns a tool call, and one that gives the final response
	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	// Test with returnOnlyLastMessage enabled
	session, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager), WithReturnOnlyLastMessage())
	require.NoError(t, err)

	response, err := session.Query(context.Background(), testQuery)
	require.NoError(t, err)

	// Should only contain the final response, not the first one
	assert.Equal(t, "The weather is sunny today", response)
	assert.NotContains(t, response, "I'll check the weather for you")

	// Test without returnOnlyLastMessage (default behavior)
	session2, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	mockToolManager.EXPECT().ToolList().Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "default"}).
		Return("Weather: Sunny, 25°C", nil)

	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	response2, err := session2.Query(context.Background(), testQuery)
	require.NoError(t, err)

	// Should contain both responses concatenated with newlines
	assert.Contains(t, response2, "I'll check the weather for you")
	assert.Contains(t, response2, "The weather is sunny today")
}
