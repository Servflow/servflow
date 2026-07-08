package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
)

type RoleType int

const (
	RoleTypeUnknown RoleType = iota
	RoleTypeSystem
	RoleTypeUser
	RoleTypeAssistant
	RoleTypeDeveloper
)

func (r RoleType) String() string {
	switch r {
	case RoleTypeSystem:
		return "system"
	case RoleTypeUser:
		return "user"
	case RoleTypeAssistant:
		return "assistant"
	case RoleTypeDeveloper:
		return "developer"
	default:
		return "unknown"
	}
}

type MessageType int

const (
	MessageTypeText MessageType = iota
	MessageTypeToolCall
	MessageTypeToolResponse
)

type LLMRequest struct {
	SystemMessage string
	Instruction   string
	Messages      []any
	Tools         []ToolInfo `json:"tools"`
}

// TraceMessages renders the request messages as a compact JSON array for
// tracing (role + textual content only; no image bytes). Best-effort — returns
// "" if it cannot marshal.
func TraceMessages(messages []any) string {
	type traced struct {
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
		Tool    string `json:"tool,omitempty"`
	}
	out := make([]traced, 0, len(messages))
	for _, msg := range messages {
		switch v := msg.(type) {
		case MessageTypeContent:
			out = append(out, traced{Role: v.Role.String(), Content: v.Content})
		case MessageToolCall:
			args, _ := json.Marshal(v.Arguments)
			out = append(out, traced{Role: "assistant", Tool: v.Name, Content: string(args)})
		case MessageToolCallResponse:
			out = append(out, traced{Role: "tool", Tool: v.ID, Content: v.Text})
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

type MessageTypeContent struct {
	Message
	Role        RoleType
	Content     string
	FileContent *requestctx.FileValue `json:"-"`
}

func (c *MessageTypeContent) Serialize() ([]byte, error) {
	return json.Marshal(c)
}

func (c *MessageTypeContent) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, c)
}

type Message struct {
	Type MessageType `json:"type"`
}

type MessageToolCall struct {
	Message
	ID        string
	Name      string
	Arguments map[string]interface{}
}

func (t *MessageToolCall) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MessageToolCall) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, t)
}

type ToolResponseType int

const (
	ToolResponseTypeUnknown ToolResponseType = iota
	ToolResponseTypeText
	ToolResponseTypeImage
)

type ToolCallOutputType int

const (
	ToolCallOutputTypeText ToolCallOutputType = iota
	ToolCallOutputTypeImage
)

type MessageToolCallResponse struct {
	Message
	ToolResponseType ToolResponseType
	ID               string
	Text             string
	ImageData        []byte
	ImageMimeType    string
}

func (t *MessageToolCallResponse) GenerateContent() (content string, mimeType string, outputType ToolCallOutputType) {
	switch t.ToolResponseType {
	case ToolResponseTypeText:
		return t.Text, "", ToolCallOutputTypeText
	case ToolResponseTypeImage:
		return fmt.Sprintf("data:%s;base64,%s", t.ImageMimeType, t.ImageData), t.ImageMimeType, ToolCallOutputTypeImage
	default:
		return t.Text, "", ToolCallOutputTypeText
	}
}

// TODO consider removing image data when serializing for saving, so it won't be included in history

func (t *MessageToolCallResponse) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MessageToolCallResponse) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, t)
}

type ToolInfo struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	InputSchema mcp.ToolInputSchema `json:"inputSchema,omitempty"`
}

type LLMResponse struct {
	Content []ContentResponse    `json:"content"`
	Tools   []ToolResponseObject `json:"tools"`
	Usage   Usage                `json:"usage"`
}

// Text joins the textual content blocks of a response, for tracing/logging.
func (r LLMResponse) Text() string {
	parts := make([]string, 0, len(r.Content))
	for _, c := range r.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Usage holds the token counts reported by an LLM provider for a single call.
type Usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
}

// Add returns the element-wise sum of two Usage values.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		InputTokens:  u.InputTokens + o.InputTokens,
		OutputTokens: u.OutputTokens + o.OutputTokens,
		TotalTokens:  u.TotalTokens + o.TotalTokens,
	}
}

type ToolResponseObject struct {
	Name   string                 `json:"name"`
	Input  map[string]interface{} `json:"input"`
	ToolID string                 `json:"toolId"`
}

type ContentResponse struct {
	Text string `json:"text"`
}

// SessionMetadata contains metadata collected during an agent session
type SessionMetadata struct {
	LLMResponses []LLMResponse `json:"llmResponses"`
	TotalUsage   Usage         `json:"totalUsage"`
}
