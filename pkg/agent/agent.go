//go:generate mockgen -source agent.go -destination agent_mock.go -package agent
package agent

import (
	"context"
	_ "embed"
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
	toolManager    ToolManager
	llm            LLmProvider
	messages       []any
	conversationID string
}

type Option func(*Session)

func WithToolManager(toolManager ToolManager) Option {
	return func(a *Session) {
		a.toolManager = toolManager
	}
}

func WithConversationID(id string) Option {
	return func(a *Session) {
		a.conversationID = id
		//messages, err := storage.GetLogEntriesByPrefix(conversationStoragePrefix+id, func(data []byte) (storage.Serializable, error) {
		//	return &ContentMessage{}
		//})
	}
}

func NewSession(developerInstructions string, llm LLmProvider, options ...Option) (*Session, error) {
	agent := &Session{
		llm:      llm,
		messages: make([]any, 0),
	}

	agent.messages = append(agent.messages, ContentMessage{
		Role:    RoleTypeDeveloper,
		Content: developerInstructions,
	})

	for _, option := range options {
		option(agent)
	}

	return agent, nil
}

type agentOutput struct {
	err      error
	response string
}

func (a *Session) Query(ctx context.Context, query string) (string, error) {
	a.messages = append(a.messages, ContentMessage{
		Role:    RoleTypeUser,
		Content: query,
	})

	stringBuilder := strings.Builder{}
	respChan := a.startLoop(ctx)

	for r := range respChan {
		if r.err != nil {
			return "", r.err
		}
		stringBuilder.WriteString(r.response)
		stringBuilder.WriteString("\n")
	}

	return stringBuilder.String(), nil
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
				a.messages = append(a.messages, ContentMessage{
					Role:    RoleTypeAssistant,
					Content: c.Text,
				})
				out <- agentOutput{response: c.Text}
			}

			if len(r.Tools) == 0 {
				endTurn = true
			}

			// TODO call tools in parallel
			for _, tool := range r.Tools {
				logger.Debug("attempting to execute tool", zap.String("tool", tool.Name))

				a.messages = append(a.messages, ToolCallMessage{
					ID:        tool.ToolID,
					Name:      tool.Name,
					Arguments: tool.Input,
				})
				toolResp, err := a.toolManager.CallTool(ctx, tool.Name, tool.Input)
				if err != nil {
					logger.Error("failed to execute tool", zap.String("tool", tool.Name), zap.Error(err))
					a.messages = append(a.messages, ToolCallOutputMessage{
						Output: "error running tool",
						ID:     tool.ToolID,
					})
					continue
				}
				a.messages = append(a.messages, ToolCallOutputMessage{
					Output: toolResp,
					ID:     tool.ToolID,
				})
				logger.Debug("successfully executed tool", zap.String("tool", tool.Name), zap.String("toolResp", toolResp))
			}
		}
		close(out)
	}()

	return out
}

func (a *Session) addToMessages(message any) error {
	storageKey := ""
	if a.conversationID != "" {
		storageKey = conversationStoragePrefix + a.conversationID
	}
	switch message := message.(type) {
	case ContentMessage:
		a.messages = append(a.messages, message)
		if storageKey != "" {
			if err := storage.WriteToLog(
				storageKey,
				[]storage.Serializable{&message},
			); err != nil {
				return err
			}
		}
	case ToolCallMessage:
		a.messages = append(a.messages, message)
		if storageKey != "" {
			if err := storage.WriteToLog(storageKey, []storage.Serializable{&message}); err != nil {
				return err
			}
		}
	case ToolCallOutputMessage:
		a.messages = append(a.messages, message)
		if storageKey != "" {
			if err := storage.WriteToLog(storageKey, []storage.Serializable{&message}); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown type %T", message)
	}
	return nil
}
