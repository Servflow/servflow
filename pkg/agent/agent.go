//go:generate mockgen -source agent.go -destination agent_mock.go -package agent
package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/internal/storage"
	"go.uber.org/zap"
)

//go:embed new_instructions.md
var instructions []byte

const conversationStoragePrefix = "agent_conversation_"

type ToolManager interface {
	CallTool(ctx context.Context, toolName string, params map[string]any) (string, error)
	ToolListDescription() (string, error)
	ToolList() []ToolInfo
}

type LLmProvider interface {
	ProvideResponse(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

type OutputMessages interface {
	storage.Serializable
}

var (
	ErrParsingResponse = errors.New("error parsing response")
)

type Session struct {
	toolManager           ToolManager
	llm                   LLmProvider
	messages              []any
	conversationID        string
	returnOnlyLastMessage bool
}

type Option func(*Session) error

func WithToolManager(toolManager ToolManager) Option {
	return func(a *Session) error {
		a.toolManager = toolManager
		return nil
	}
}

func WithConversationID(id string) Option {
	return func(a *Session) error {
		if id == "" {
			return fmt.Errorf("conversationID can not be empty")
		}
		a.conversationID = id
		messages, err := storage.GetLogEntriesByPrefix(conversationStoragePrefix+id, func(data []byte) (any, error) {
			var msg Message
			err := json.Unmarshal(data, &msg)
			if err != nil {
				return nil, err
			}
			switch msg.Type {
			case MessageTypeText:
				var contentMessage ContentMessage
				err = json.Unmarshal(data, &contentMessage)
				return contentMessage, err
			case MessageTypeToolResponse:
				var toolResponse ToolCallOutputMessage
				err = json.Unmarshal(data, &toolResponse)
				return toolResponse, err
			case MessageTypeToolCall:
				var toolCall ToolCallMessage
				err = json.Unmarshal(data, &toolCall)
				return toolCall, err
			default:
				logging.GetLogger().Warn("invalid type in log storage", zap.Int("type", int(msg.Type)))
			}
			return nil, nil
		})
		if err != nil {
			return err
		}
		a.messages = append(a.messages, messages...)
		return nil
	}
}

func WithReturnOnlyLastMessage() Option {
	return func(a *Session) error {
		a.returnOnlyLastMessage = true
		return nil
	}
}

func NewSession(developerInstructions string, llm LLmProvider, options ...Option) (*Session, error) {
	agent := &Session{
		llm:      llm,
		messages: make([]any, 0),
	}

	agent.messages = append(agent.messages, ContentMessage{
		Message: Message{Type: MessageTypeText},
		Role:    RoleTypeDeveloper,
		Content: developerInstructions,
	})

	for _, option := range options {
		if err := option(agent); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

type agentOutput struct {
	err      error
	response string
}

func (a *Session) Query(ctx context.Context, query string) (string, error) {
	logger := logging.GetRequestLogger(ctx).With(zap.String("module", "agent"))
	a.addToMessages(logger, ContentMessage{
		Message: Message{Type: MessageTypeText},
		Role:    RoleTypeUser,
		Content: query,
	}, nil)

	var (
		strBuilder  strings.Builder
		lastMessage string
	)
	respChan := a.startLoop(ctx)
	for r := range respChan {
		if r.err != nil {
			return "", r.err
		}
		if a.returnOnlyLastMessage {
			lastMessage = r.response
		} else {
			strBuilder.WriteString(r.response)
			strBuilder.WriteString("\n")
		}
	}
	if a.returnOnlyLastMessage {
		return lastMessage, nil
	} else {
		return strBuilder.String(), nil
	}
}

func (a *Session) startLoop(ctx context.Context) chan agentOutput {
	logger := logging.GetRequestLogger(ctx).With(zap.String("module", "agent"))
	out := make(chan agentOutput)

	toolList := a.toolManager.ToolList()
	go func() {
		endTurn := false
		for !endTurn {
			r, err := a.llm.ProvideResponse(ctx, LLMRequest{
				Tools:         toolList,
				Messages:      a.messages,
				SystemMessage: string(instructions),
			})
			if err != nil {
				out <- agentOutput{err: fmt.Errorf("error from llm: %w", err)}
				break
			}

			// process content output
			for _, c := range r.Content {
				a.addToMessages(logger, ContentMessage{
					Message: Message{Type: MessageTypeText},
					Role:    RoleTypeAssistant,
					Content: c.Text,
				}, out)
			}

			if len(r.Tools) == 0 {
				endTurn = true
			}

			// TODO call tools in parallel
			for _, tool := range r.Tools {
				logger.Debug("attempting to execute tool", zap.String("tool", tool.Name))

				a.addToMessages(logger, ToolCallMessage{
					Message:   Message{Type: MessageTypeToolCall},
					ID:        tool.ToolID,
					Name:      tool.Name,
					Arguments: tool.Input,
				}, out)
				toolResp, err := a.toolManager.CallTool(ctx, tool.Name, tool.Input)
				if err != nil {
					a.addToMessages(logger, ToolCallOutputMessage{
						Message: Message{Type: MessageTypeToolResponse},
						Output:  "error running tool",
						ID:      tool.ToolID,
					}, out)
					continue
				}
				a.addToMessages(logger, ToolCallOutputMessage{
					Message: Message{Type: MessageTypeToolResponse},
					Output:  toolResp,
					ID:      tool.ToolID,
				}, out)
				logger.Debug("successfully executed tool", zap.String("tool", tool.Name), zap.String("toolResp", toolResp))
			}
		}
		close(out)
	}()

	return out
}

func (a *Session) addToMessages(logger *zap.Logger, message any, output chan agentOutput) {
	storageKey := ""
	if a.conversationID != "" {
		storageKey = conversationStoragePrefix + a.conversationID
	}

	var (
		serializable storage.Serializable
	)
	switch message := message.(type) {
	case ContentMessage:
		a.messages = append(a.messages, message)
		if output != nil {
			output <- agentOutput{
				response: message.Content,
			}
		}

		serializable = &message
	case ToolCallMessage:
		a.messages = append(a.messages, message)
		serializable = &message
	case ToolCallOutputMessage:
		a.messages = append(a.messages, message)
		serializable = &message
	default:
		logger.Warn("received message of unknown type", zap.Any("message", message))
	}

	if storageKey != "" {
		if err := storage.WriteToLog(storageKey, []storage.Serializable{serializable}); err != nil {
			logger.Error("failed to write serializable message", zap.Error(err))
		}
	}
}
