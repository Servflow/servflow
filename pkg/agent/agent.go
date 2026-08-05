//go:generate mockgen -source agent.go -destination agent_mock.go -package agent
package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

//go:embed new_instructions.md
var instructions []byte

type ToolManager interface {
	CallTool(ctx context.Context, toolName string, params map[string]any) ([]mcp.Content, error)
	ToolListDescription(ctx context.Context) (string, error)
	ToolList(ctx context.Context) []ToolInfo
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
	toolManager        ToolManager
	llm                LLmProvider
	messages           []any
	conversation       *requestctx.Conversation
	customInstructions string
	llmResponses       []LLMResponse
}

type Option func(*Session) error

func WithToolManager(toolManager ToolManager) Option {
	return func(a *Session) error {
		a.toolManager = toolManager
		return nil
	}
}

// WithConversation binds the session to the request's shared conversation
// thread. The session seeds its working context with the thread so far (so it
// sees what earlier agents in the run said) and appends every message it
// produces back to the thread. A nil conversation is a no-op — the session
// keeps a purely local, unshared history.
func WithConversation(conv *requestctx.Conversation) Option {
	return func(a *Session) error {
		a.conversation = conv
		if conv == nil {
			return nil
		}
		for _, m := range conv.Messages() {
			a.messages = append(a.messages, m)
		}
		return nil
	}
}

func WithInstructions(instructions string) Option {
	return func(a *Session) error {
		a.customInstructions = instructions
		return nil
	}
}

func NewSession(developerInstructions string, llm LLmProvider, options ...Option) (*Session, error) {
	agent := &Session{
		llm:          llm,
		messages:     make([]any, 0),
		llmResponses: make([]LLMResponse, 0),
	}
	agent.customInstructions = developerInstructions

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

// Query runs the agent loop for a turn. Every message produced along the way is
// appended to the session's conversation, which is where an agent's output is
// consumed from; the returned string is the concatenated assistant text, kept
// for callers that want the reply inline (the agent action discards it).
func (a *Session) Query(ctx context.Context, query string, file *requestctx.FileValue) (string, error) {
	logger := logging.WithContextEnriched(ctx).With(zap.String("module", "agent"))
	if query != "" || file != nil {
		a.addToMessages(logger, MessageTypeContent{
			Message:     Message{Type: MessageTypeText},
			Role:        RoleTypeUser,
			Content:     query,
			FileContent: file,
		}, nil)
	}

	var strBuilder strings.Builder
	respChan := a.startLoop(ctx)
	for r := range respChan {
		if r.err != nil {
			return "", r.err
		}
		strBuilder.WriteString(r.response)
		strBuilder.WriteString("\n")
	}
	return strBuilder.String(), nil
}

// GetMetadata returns the metadata collected during the session
func (a *Session) GetMetadata() SessionMetadata {
	var total Usage
	for _, r := range a.llmResponses {
		total = total.Add(r.Usage)
	}
	return SessionMetadata{
		LLMResponses: a.llmResponses,
		TotalUsage:   total,
	}
}

// maxAgentIterations bounds the agent's tool-calling loop so a model that keeps
// calling tools (e.g. repeatedly requesting non-existent files) cannot run
// unbounded. On the final permitted iteration the tools are withheld so the
// model must produce a text answer from what it already has.
const maxAgentIterations = 40

func (a *Session) startLoop(ctx context.Context) chan agentOutput {
	logger := logging.FromContext(ctx).With(zap.String("module", "agent"))
	out := make(chan agentOutput)

	toolList := a.toolManager.ToolList(ctx)
	go func() {
		endTurn := false
		iterations := 0
		for !endTurn {
			iterations++
			systemMessage := string(instructions)
			// On the final permitted iteration, withhold tools so the model has to
			// answer from what it already gathered rather than calling more tools.
			reqTools := toolList
			forceFinish := iterations >= maxAgentIterations
			if forceFinish {
				reqTools = nil
				logger.Warn("agent reached max iterations; forcing a final response without tools",
					zap.Int("max_iterations", maxAgentIterations))
			}
			r, err := a.llm.ProvideResponse(ctx, LLMRequest{
				Tools:         reqTools,
				Messages:      a.messages,
				SystemMessage: systemMessage,
				Instruction:   a.customInstructions,
			})
			if err != nil {
				out <- agentOutput{err: fmt.Errorf("error from llm: %w", err)}
				break
			}

			// collect LLM response for metadata
			a.llmResponses = append(a.llmResponses, r)

			// process content output
			for _, c := range r.Content {
				logger.Info("llm response", zap.String("text", c.Text))
				a.addToMessages(logger, MessageTypeContent{
					Message: Message{Type: MessageTypeText},
					Role:    RoleTypeAssistant,
					Content: c.Text,
				}, out)
			}

			if forceFinish || len(r.Tools) == 0 {
				endTurn = true
				continue
			}

			for _, tool := range r.Tools {
				a.addToMessages(logger, MessageToolCall{
					Message:   Message{Type: MessageTypeToolCall},
					ID:        tool.ToolID,
					Name:      tool.Name,
					Arguments: tool.Input,
				}, out)
			}

			// TODO call tools in parallel
			for _, tool := range r.Tools {
				logger.Info("attempting to execute tool", zap.String("tool", tool.Name), zap.Any("params", tool.Input))
				toolResp, err := a.toolManager.CallTool(ctx, tool.Name, tool.Input)
				if err != nil {
					a.addToMessages(logger, MessageToolCallResponse{
						Message: Message{Type: MessageTypeToolResponse},
						Text:    "error running tool",
						ID:      tool.ToolID,
					}, out)
					logger.Error("failed to execute tool", zap.String("tool", tool.Name), zap.Error(err))
					continue
				}
				responses, err := createToolResponseFromMCPContent(tool.ToolID, toolResp)
				if err != nil {
					logger.Error("failed to create tool response", zap.String("tool", tool.Name), zap.Error(err))
					continue
				}
				for i := range responses {
					response := responses[i]
					a.addToMessages(logger, response, out)
				}
				logger.Info("successfully executed tool", zap.String("tool", tool.Name))
			}
		}
		close(out)
	}()

	return out
}

func createToolResponseFromMCPContent(callID string, contentList []mcp.Content) ([]MessageToolCallResponse, error) {
	resp := make([]MessageToolCallResponse, len(contentList))
	for i, content := range contentList {
		switch v := content.(type) {
		case mcp.TextContent:
			resp[i] = MessageToolCallResponse{
				Message:          Message{Type: MessageTypeToolResponse},
				ToolResponseType: ToolResponseTypeText,
				ID:               callID,
				Text:             v.Text,
			}
		case mcp.ImageContent:
			resp[i] = MessageToolCallResponse{
				Message:          Message{Type: MessageTypeToolResponse},
				ToolResponseType: ToolResponseTypeImage,
				ID:               callID,
				ImageData:        []byte(v.Data),
				ImageMimeType:    v.MIMEType,
			}
		default:
			return nil, fmt.Errorf("unsupported content type")
		}
	}

	return resp, nil
}

// TODO: think of context management strategy for image responses, they can cause bloat

func (a *Session) addToMessages(logger *zap.Logger, message any, output chan agentOutput) {
	switch message := message.(type) {
	case MessageTypeContent:
		a.messages = append(a.messages, message)
		if output != nil {
			output <- agentOutput{
				response: message.Content,
			}
		}
	case MessageToolCall:
		a.messages = append(a.messages, message)
	case MessageToolCallResponse:
		a.messages = append(a.messages, message)
	default:
		logger.Warn("received message of unknown type", zap.Any("message", message))
		return
	}

	// Contribute the message to the request's shared conversation thread so
	// other agents in the run — and, when persistence is enabled, later runs —
	// can see it. The concrete message types satisfy ConversationMessage via
	// value receivers.
	if a.conversation != nil {
		if cm, ok := message.(ConversationMessage); ok {
			a.conversation.Append(cm)
		}
	}
}
