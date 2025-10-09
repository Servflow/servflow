//go:generate mockgen -source agent.go -destination agent_mock.go -package agent
package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/logging"
	"go.uber.org/zap"
)

//go:embed new_instructions.md
var instructions []byte

type ToolManager interface {
	CallTool(ctx context.Context, toolName string, params map[string]any) (string, error)
	ToolListDescription() (string, error)
	ToolList() []ToolInfo
}

type LLmProvider interface {
	ProvideResponse(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var (
	ErrParsingResponse = errors.New("error parsing response")
)

type Orchestrator struct {
	toolManager     ToolManager
	llm             LLmProvider
	thoughtMessages []any
}

type Option func(*Orchestrator)

func WithToolManager(toolManager ToolManager) Option {
	return func(a *Orchestrator) {
		a.toolManager = toolManager
	}
}

func NewOrchestrator(developerInstructions string, llm LLmProvider, options ...Option) (*Orchestrator, error) {

	agent := &Orchestrator{
		llm:             llm,
		thoughtMessages: make([]any, 0),
	}

	for _, option := range options {
		option(agent)
	}

	agent.thoughtMessages = append(agent.thoughtMessages, ContentMessage{
		Role:    RoleTypeDeveloper,
		Content: developerInstructions,
	})

	return agent, nil
}

type agentOutput struct {
	err      error
	response string
}

func (a *Orchestrator) Query(ctx context.Context, query string) (string, error) {
	a.thoughtMessages = append(a.thoughtMessages, ContentMessage{
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

func (a *Orchestrator) startLoop(ctx context.Context) chan agentOutput {
	logger := logging.GetRequestLogger(ctx).With(zap.String("module", "agent"))
	out := make(chan agentOutput)

	toolList := a.toolManager.ToolList()
	go func() {
		endTurn := false
		for !endTurn {
			r, err := a.llm.ProvideResponse(ctx, LLMRequest{
				Tools:         toolList,
				Messages:      a.thoughtMessages,
				SystemMessage: string(instructions),
			})
			if err != nil {
				out <- agentOutput{err: fmt.Errorf("error from llm: %w", err)}
				break
			}

			for _, c := range r.Content {
				a.thoughtMessages = append(a.thoughtMessages, ContentMessage{
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

				a.thoughtMessages = append(a.thoughtMessages, ToolCallMessage{
					ID:        tool.ToolID,
					Name:      tool.Name,
					Arguments: tool.Input,
				})
				toolResp, err := a.toolManager.CallTool(ctx, tool.Name, tool.Input)
				if err != nil {
					logger.Error("failed to execute tool", zap.String("tool", tool.Name), zap.Error(err))
					a.thoughtMessages = append(a.thoughtMessages, ToolCallOutputMessage{
						Output: "error running tool",
						ID:     tool.ToolID,
					})
					continue
				}
				a.thoughtMessages = append(a.thoughtMessages, ToolCallOutputMessage{
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
