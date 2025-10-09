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
	orchestrator, err := NewOrchestrator("You are an agent for a restaurant review system", nil, WithToolManager(mockToolManager))
	require.NoError(t, err)

	expectedMessages := []any{
		ContentMessage{
			Role:    RoleTypeDeveloper,
			Content: "You are an agent for a restaurant review system",
		},
	}
	assert.Equal(t, expectedMessages, orchestrator.thoughtMessages)
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
					ContentMessage{
						Role:    RoleTypeDeveloper,
						Content: "You are an agent for a restaurant review system",
					},
					ContentMessage{
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
					ContentMessage{
						Role:    RoleTypeDeveloper,
						Content: "You are an agent for a restaurant review system",
					},
					ContentMessage{
						Role:    RoleTypeUser,
						Content: "What's the weather like in Lagos?",
					},
					ContentMessage{
						Role:    RoleTypeAssistant,
						Content: "I'll check the weather for Lagos",
					},
					ToolCallMessage{
						ID:        "test",
						Name:      "get_weather",
						Arguments: map[string]interface{}{"location": "lagos"},
					},
					ToolCallOutputMessage{
						ID:     "test",
						Output: "Temperature: 28°C, Condition: Sunny",
					},
				},
			}).
			Return(finalResponse, nil),
	)

	// Create agent and run query
	agent, err := NewOrchestrator(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
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

	agent, err := NewOrchestrator(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
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

	agent, err := NewOrchestrator(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
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

	agent, err := NewOrchestrator(systemPrompt, mockLLmHandler, WithToolManager(mockToolManager))
	require.NoError(t, err)

	result, err := agent.Query(context.Background(), testQuery)
	require.NoError(t, err)
	assert.Contains(t, result, "I'm unable to complete the weather request due to an error")
}
