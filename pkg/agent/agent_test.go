package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var testInstructions = "You are a test agent."

var toolList = `[
						{
								"name": "get_weather",
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
	assert.Empty(t, orchestrator.messages)
	assert.Equal(t, "You are an agent for a restaurant review system", orchestrator.customInstructions)
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
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return([]mcp.Content{mcp.TextContent{Type: "text", Text: "Temperature: 28°C, Condition: Sunny"}}, nil)

	// We expect two LLM calls - one that returns a tool call, and one that gives the final response
	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),

		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	// Create agent and run query
	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager), WithInstructions(testInstructions))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery, nil)
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
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)

	providerError := errors.New("provider error")
	mockLLmHandler.EXPECT().
		ProvideResponse(gomock.Any(), gomock.Any()).
		Return(LLMResponse{}, providerError)

	testInstructions := `# Test Agent Instructions

You are a test agent. Your role is to process requests efficiently with minimal complexity.

## Core Behavior
- Respond directly and concisely
- Use tools when explicitly requested
- Keep responses focused on the task at hand
- No need for extensive explanations unless asked

## Tool Usage
- Use available tools as needed
- Provide required parameters
- Handle tool responses appropriately

## Communication
- Be clear and direct
- Use simple language
- Focus on functionality over explanation`

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager), WithInstructions(testInstructions))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery, nil)
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
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return(nil, errors.New("tool error"))
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return([]mcp.Content{mcp.TextContent{Type: "text", Text: "Temperature: 28°C, Condition: Sunny"}}, nil)

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

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager), WithInstructions(testInstructions))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery, nil)
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
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return(nil, errors.New("tool error"))

	gomock.InOrder(
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(firstResponse, nil),
		mockLLmHandler.EXPECT().
			ProvideResponse(gomock.Any(), gomock.Any()).
			Return(finalResponse, nil),
	)

	agent, err := NewSession(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager), WithInstructions(testInstructions))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery, nil)
	require.NoError(t, err)
	assert.Contains(t, result, "I'm unable to complete the weather request due to an error")
}

// TestSession_AgentsInSameRequestShareConversation proves the property the whole
// centralisation rests on: two agents running in the SAME request context get
// the same conversation thread, so what one writes the next one reads. Unlike
// TestSession_ConversationThreadSharing (which shares a Conversation built
// directly), this sources the thread from the request context exactly as the
// agent action does — requestctx.Start → rc.Conversation().
func TestSession_AgentsInSameRequestShareConversation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, rc := requestctx.Start(context.Background(), requestctx.Options{})
	defer rc.Done()

	// Every agent action resolves the thread the same way; it must be one object.
	conv := rc.Conversation()
	require.NotNil(t, conv, "request context must carry a conversation")
	require.Same(t, conv, rc.Conversation(), "each agent must resolve the same thread")

	// Agent 1 runs and contributes to the thread.
	firstLLM := NewMockLLmProvider(ctrl)
	firstTools := NewMockToolManager(ctrl)
	firstTools.EXPECT().ToolList(gomock.Any()).Return(nil).AnyTimes()
	firstLLM.EXPECT().ProvideResponse(gomock.Any(), gomock.Any()).
		Return(LLMResponse{Content: []ContentResponse{{Text: "agent one reporting"}}}, nil)

	agentOne, err := NewSession("first agent", firstLLM,
		WithToolManager(firstTools),
		WithConversation(rc.Conversation()))
	require.NoError(t, err)
	_, err = agentOne.Query(ctx, "what did you find?", nil)
	require.NoError(t, err)

	// Agent 2 starts later in the same request: it must SEE agent 1's exchange
	// rather than beginning blank.
	secondLLM := NewMockLLmProvider(ctrl)
	secondTools := NewMockToolManager(ctrl)
	secondTools.EXPECT().ToolList(gomock.Any()).Return(nil).AnyTimes()
	// The mock only records what it was handed; the checking happens on the test
	// goroutine once Query has returned. Query only returns after the session
	// closes its response channel, which happens-after this write, so the read is
	// ordered without extra synchronisation.
	var sentToProvider LLMRequest
	secondLLM.EXPECT().ProvideResponse(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, req LLMRequest) { sentToProvider = req }).
		Return(LLMResponse{Content: []ContentResponse{{Text: "agent two building on that"}}}, nil)

	agentTwo, err := NewSession("second agent", secondLLM,
		WithToolManager(secondTools),
		WithConversation(rc.Conversation()))
	require.NoError(t, err)
	require.Len(t, agentTwo.messages, 2, "agent two should be seeded with agent one's exchange")

	_, err = agentTwo.Query(ctx, "anything to add?", nil)
	require.NoError(t, err)

	// Agent 1's turn and answer reached the provider ahead of agent 2's own query,
	// with the concrete types intact through the []any boundary — if boxing lost
	// them, the integrations' type switches would silently drop the history.
	require.Len(t, sentToProvider.Messages, 3)
	seeded, ok := sentToProvider.Messages[0].(MessageTypeContent)
	require.True(t, ok, "seeded message lost its concrete type: %T", sentToProvider.Messages[0])
	assert.Equal(t, RoleTypeUser, seeded.Role)
	assert.Equal(t, "what did you find?", seeded.Content)
	answer, ok := sentToProvider.Messages[1].(MessageTypeContent)
	require.True(t, ok, "seeded message lost its concrete type: %T", sentToProvider.Messages[1])
	assert.Equal(t, RoleTypeAssistant, answer.Role)
	assert.Equal(t, "agent one reporting", answer.Content)

	// Both agents' turns land on the one shared thread, in order.
	got := conv.Messages()
	require.Len(t, got, 4)
	want := []string{
		"what did you find?",
		"agent one reporting",
		"anything to add?",
		"agent two building on that",
	}
	for i, w := range want {
		msg, ok := got[i].(MessageTypeContent)
		require.True(t, ok, "message %d has type %T", i, got[i])
		assert.Equal(t, w, msg.Content, "message %d", i)
	}
}

func TestSession_ConversationThreadSharing(t *testing.T) {
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

	// A single shared conversation thread stands in for the request-context-owned
	// thread every agent action in a plan reads from and appends to.
	conv := requestctx.NewConversation(conversationID)

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
		mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList).AnyTimes()
		mockToolManager.EXPECT().
			CallTool(gomock.Any(), "get_weather", map[string]any{"location": "New York"}).
			Return([]mcp.Content{mcp.TextContent{Type: "text", Text: "Temperature: 22°C, Condition: Cloudy"}}, nil)

		gomock.InOrder(
			mockLLmHandler.EXPECT().
				ProvideResponse(gomock.Any(), gomock.Any()).
				Return(firstResponse, nil),

			mockLLmHandler.EXPECT().
				ProvideResponse(gomock.Any(), gomock.Any()).
				Return(finalResponse, nil),
		)

		// Create session with conversation ID
		session, err := NewSession(systemPrompt, mockLLmHandler,
			WithToolManager(mockToolManager),
			WithConversation(conv),
			WithInstructions(testInstructions))
		require.NoError(t, err)

		// Run first query
		result, err := session.Query(context.Background(), testQuery1, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "I'll check the weather for New York")
		assert.Contains(t, result, "The weather in New York is cloudy with 22°C")

		// Verify messages were stored in session
		assert.Equal(t, 5, len(session.messages))

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
			mockToolManager2.EXPECT().ToolList(gomock.Any()).Return(toolInfoList).AnyTimes()
			mockToolManager2.EXPECT().
				CallTool(gomock.Any(), "get_weather", map[string]any{"location": "Boston"}).
				Return([]mcp.Content{mcp.TextContent{Type: "text", Text: "Temperature: 25°C, Condition: Sunny"}}, nil)

			gomock.InOrder(
				mockLLmHandler2.EXPECT().
					ProvideResponse(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, req LLMRequest) {
						// Verify that the retrieved messages include all the previous conversation
						assert.Equal(t, 6, len(req.Messages))

						// Verify message types and content
						messages := req.Messages

						// First message should be first user query
						userMsg1, ok := messages[0].(MessageTypeContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeUser, userMsg1.Role)
						assert.Equal(t, testQuery1, userMsg1.Content)

						// Second message should be assistant response
						assistantMsg1, ok := messages[1].(MessageTypeContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeAssistant, assistantMsg1.Role)
						assert.Contains(t, assistantMsg1.Content, "I'll check the weather for New York")

						// Third message should be tool call
						toolCallMsg, ok := messages[2].(MessageToolCall)
						assert.True(t, ok)
						assert.Equal(t, "weather_ny_001", toolCallMsg.ID)
						assert.Equal(t, "get_weather", toolCallMsg.Name)
						assert.Equal(t, map[string]interface{}{"location": "New York"}, toolCallMsg.Arguments)

						// Fourth message should be tool response
						toolResponseMsg, ok := messages[3].(MessageToolCallResponse)
						assert.True(t, ok)
						assert.Equal(t, "weather_ny_001", toolResponseMsg.ID)
						assert.Equal(t, "Temperature: 22°C, Condition: Cloudy", toolResponseMsg.Text)

						// Fifth message should be llm second response
						assistantMsg2, ok := messages[4].(MessageTypeContent)
						assert.True(t, ok)
						assert.Equal(t, RoleTypeAssistant, assistantMsg2.Role)
						assert.Equal(t, "The weather in New York is cloudy with 22°C", assistantMsg2.Content)

						// Sixth message should be the new user query
						userMsg2, ok := messages[5].(MessageTypeContent)
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
				WithConversation(conv),
				WithInstructions(testInstructions))
			require.NoError(t, err)

			// Verify that messages were seeded from the shared thread
			assert.Equal(t, 5, len(newSession.messages))

			// Verify the loaded messages match what we expect
			messages := newSession.messages

			// Check first user message
			userMsg1, ok := messages[0].(MessageTypeContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeUser, userMsg1.Role)
			assert.Equal(t, testQuery1, userMsg1.Content)

			// Check assistant message
			assistantMsg1, ok := messages[1].(MessageTypeContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeAssistant, assistantMsg1.Role)

			// Check tool call message
			toolCallMsg, ok := messages[2].(MessageToolCall)
			assert.True(t, ok)
			assert.Equal(t, "weather_ny_001", toolCallMsg.ID)
			assert.Equal(t, "get_weather", toolCallMsg.Name)

			// Check tool response message
			toolResponseMsg, ok := messages[3].(MessageToolCallResponse)
			assert.True(t, ok)
			assert.Equal(t, "weather_ny_001", toolResponseMsg.ID)

			// check assistant response
			assistantMsg2, ok := messages[4].(MessageTypeContent)
			assert.True(t, ok)
			assert.Equal(t, RoleTypeAssistant, assistantMsg2.Role)
			assert.Equal(t, "The weather in New York is cloudy with 22°C", assistantMsg2.Content)

			// Continue the conversation with second query
			result, err := newSession.Query(context.Background(), testQuery2, nil)
			require.NoError(t, err)
			assert.Contains(t, result, "Boston")
			assert.Contains(t, result, "New York is cloudy")
			assert.Contains(t, result, "Boston is sunny")
		})
	})

	// A nil conversation is a safe no-op: the session keeps a purely local
	// history rather than erroring (conversation creation is the request
	// context's responsibility, not the session's).
	t.Run("nil conversation is a no-op", func(t *testing.T) {
		mockLLmHandler := NewMockLLmProvider(ctrl)
		session, err := NewSession(systemPrompt, mockLLmHandler, WithConversation(nil))
		require.NoError(t, err)
		assert.Nil(t, session.conversation)
		assert.Empty(t, session.messages)
	})
}

func TestCreateToolResponseFromMCPContent(t *testing.T) {
	tests := []struct {
		name           string
		callID         string
		contentList    []mcp.Content
		expectedLen    int
		expectError    bool
		errorContains  string
		validateResult func(t *testing.T, responses []MessageToolCallResponse)
	}{
		{
			name:   "text content",
			callID: "test-call-123",
			contentList: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "This is a test response",
				},
			},
			expectedLen: 1,
			expectError: false,
			validateResult: func(t *testing.T, responses []MessageToolCallResponse) {
				assert.Equal(t, ToolResponseTypeText, responses[0].ToolResponseType)
				assert.Equal(t, "test-call-123", responses[0].ID)
				assert.Equal(t, "This is a test response", responses[0].Text)
			},
		},
		{
			name:   "image content",
			callID: "test-call-456",
			contentList: []mcp.Content{
				mcp.ImageContent{
					Type:     "image",
					Data:     "base64encodeddata",
					MIMEType: "image/png",
				},
			},
			expectedLen: 1,
			expectError: false,
			validateResult: func(t *testing.T, responses []MessageToolCallResponse) {
				assert.Equal(t, ToolResponseTypeImage, responses[0].ToolResponseType)
				assert.Equal(t, "test-call-456", responses[0].ID)
				assert.Equal(t, []byte("base64encodeddata"), responses[0].ImageData)
				assert.Equal(t, "image/png", responses[0].ImageMimeType)
			},
		},
		{
			name:   "multiple content items",
			callID: "test-call-789",
			contentList: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "First response",
				},
				mcp.TextContent{
					Type: "text",
					Text: "Second response",
				},
				mcp.ImageContent{
					Type:     "image",
					Data:     "imagedata",
					MIMEType: "image/jpeg",
				},
			},
			expectedLen: 3,
			expectError: false,
			validateResult: func(t *testing.T, responses []MessageToolCallResponse) {
				// Check first text response
				assert.Equal(t, ToolResponseTypeText, responses[0].ToolResponseType)
				assert.Equal(t, "test-call-789", responses[0].ID)
				assert.Equal(t, "First response", responses[0].Text)

				// Check second text response
				assert.Equal(t, ToolResponseTypeText, responses[1].ToolResponseType)
				assert.Equal(t, "test-call-789", responses[1].ID)
				assert.Equal(t, "Second response", responses[1].Text)

				// Check image response
				assert.Equal(t, ToolResponseTypeImage, responses[2].ToolResponseType)
				assert.Equal(t, "test-call-789", responses[2].ID)
				assert.Equal(t, []byte("imagedata"), responses[2].ImageData)
				assert.Equal(t, "image/jpeg", responses[2].ImageMimeType)
			},
		},
		{
			name:        "empty content list",
			callID:      "test-call-empty",
			contentList: []mcp.Content{},
			expectedLen: 0,
			expectError: false,
			validateResult: func(t *testing.T, responses []MessageToolCallResponse) {
				// No additional validation needed
			},
		},
		{
			name:   "unsupported content type",
			callID: "test-call-unsupported",
			contentList: []mcp.Content{
				mcp.AudioContent{
					Type: "unsupported",
					Data: "test",
				},
			},
			expectedLen:   0,
			expectError:   true,
			errorContains: "unsupported content type",
			validateResult: func(t *testing.T, responses []MessageToolCallResponse) {
				// No validation needed for error case
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses, err := createToolResponseFromMCPContent(tt.callID, tt.contentList)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, responses)
			} else {
				require.NoError(t, err)
				require.Len(t, responses, tt.expectedLen)
				if tt.validateResult != nil {
					tt.validateResult(t, responses)
				}
			}
		})
	}
}

func TestWithInstructions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLmHandler := NewMockLLmProvider(ctrl)
	mockToolManager := NewMockToolManager(ctrl)

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)

	// Mock expectation to verify custom instructions are used
	mockLLmHandler.EXPECT().
		ProvideResponse(gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, req LLMRequest) {
			assert.Equal(t, string(instructions), req.SystemMessage)
			assert.Equal(t, testInstructions, req.Instruction)
		}).
		Return(LLMResponse{
			Content: []ContentResponse{{Text: "Response using custom instructions"}},
		}, nil)

	// Create session with custom instructions
	session, err := NewSession("Test system", mockLLmHandler,
		WithToolManager(mockToolManager),
		WithInstructions(testInstructions))
	require.NoError(t, err)

	result, err := session.Query(context.Background(), "Test query", nil)
	require.NoError(t, err)
	assert.Equal(t, "Response using custom instructions\n", result)
}

func TestSession_GetMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLmHandler := NewMockLLmProvider(ctrl)
	mockToolManager := NewMockToolManager(ctrl)

	var toolInfoList []ToolInfo
	if err := json.Unmarshal([]byte(toolList), &toolInfoList); err != nil {
		t.Fatal(err)
	}
	mockToolManager.EXPECT().ToolList(gomock.Any()).Return(toolInfoList)

	firstResponse := LLMResponse{
		Content: []ContentResponse{{Text: "I'll check the weather"}},
		Tools: []ToolResponseObject{
			{Name: "get_weather", Input: map[string]any{"location": "lagos"}, ToolID: "tool-1"},
		},
	}
	secondResponse := LLMResponse{
		Content: []ContentResponse{{Text: "The weather is sunny"}},
	}

	gomock.InOrder(
		mockLLmHandler.EXPECT().ProvideResponse(gomock.Any(), gomock.Any()).Return(firstResponse, nil),
		mockLLmHandler.EXPECT().ProvideResponse(gomock.Any(), gomock.Any()).Return(secondResponse, nil),
	)
	mockToolManager.EXPECT().
		CallTool(gomock.Any(), "get_weather", map[string]any{"location": "lagos"}).
		Return([]mcp.Content{mcp.TextContent{Type: "text", Text: "Sunny, 28C"}}, nil)

	session, err := NewSession("Test system", mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	_, err = session.Query(context.Background(), "What's the weather?", nil)
	require.NoError(t, err)

	metadata := session.GetMetadata()
	require.Len(t, metadata.LLMResponses, 2)
	assert.Equal(t, firstResponse, metadata.LLMResponses[0])
	assert.Equal(t, secondResponse, metadata.LLMResponses[1])
}
