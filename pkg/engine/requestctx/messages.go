package requestctx

import (
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/storage"
)

// ConversationMessage is a single entry in a conversation thread. Every message
// type can serialize itself for persistence and render a short human-readable
// summary — the summary is what lets a thread be inspected or previewed at any
// point of a run without re-interpreting each concrete message type.
//
// The concrete types (MessageTypeContent, MessageToolCall,
// MessageToolCallResponse) implement this interface on VALUE receivers so a
// stored value satisfies ConversationMessage and still matches the LLM
// integrations' type switches over []any (case MessageTypeContent: ...).
type ConversationMessage interface {
	storage.Serializable // Serialize() ([]byte, error)
	// Summary returns a short, single-line human-readable description of the
	// message for thread previews and logging. It never returns file bytes or
	// image data.
	Summary() string
}

// summaryLimit bounds the length of the free-text portion of a message summary.
const summaryLimit = 120

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

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

// Message is the common header embedded in every concrete message type.
type Message struct {
	Type MessageType `json:"type"`
}

type MessageTypeContent struct {
	Message
	Role        RoleType
	Content     string
	FileContent *FileValue `json:"-"`
}

func (c MessageTypeContent) Serialize() ([]byte, error) {
	return json.Marshal(c)
}

func (c *MessageTypeContent) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, c)
}

func (c MessageTypeContent) Summary() string {
	s := fmt.Sprintf("%s: %s", c.Role, truncate(c.Content, summaryLimit))
	if c.FileContent != nil {
		s += " [file]"
	}
	return s
}

type MessageToolCall struct {
	Message
	ID        string
	Name      string
	Arguments map[string]interface{}
}

func (t MessageToolCall) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MessageToolCall) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, t)
}

func (t MessageToolCall) Summary() string {
	args, err := json.Marshal(t.Arguments)
	if err != nil {
		args = []byte("{}")
	}
	return fmt.Sprintf("tool call %s(%s)", t.Name, truncate(string(args), summaryLimit))
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

func (t MessageToolCallResponse) GenerateContent() (content string, mimeType string, outputType ToolCallOutputType) {
	switch t.ToolResponseType {
	case ToolResponseTypeText:
		return t.Text, "", ToolCallOutputTypeText
	case ToolResponseTypeImage:
		return fmt.Sprintf("data:%s;base64,%s", t.ImageMimeType, t.ImageData), t.ImageMimeType, ToolCallOutputTypeImage
	default:
		return t.Text, "", ToolCallOutputTypeText
	}
}

// Serialize renders the message for persistence. Image payloads are NOT stored:
// the bytes are dropped and the message is persisted as a text tool result
// carrying only the metadata (mime type and original size).
//
// The response type is downgraded to text deliberately. A stored message that
// kept ToolResponseTypeImage with no bytes would, once a later request resumed
// the thread, be rebuilt by the integrations as an image block with empty
// base64 data and rejected by the provider — breaking every subsequent turn.
// As text the resumed thread stays coherent and honest about what was dropped.
//
// The value receiver means this mutates only the copy being serialized: the
// live in-memory message keeps its bytes, so the model still receives the real
// image during the request that produced it.
func (t MessageToolCallResponse) Serialize() ([]byte, error) {
	if t.ToolResponseType == ToolResponseTypeImage {
		size := len(t.ImageData)
		t.ImageData = nil
		t.ToolResponseType = ToolResponseTypeText
		t.Text = fmt.Sprintf("[image omitted: %s, %d bytes]", t.ImageMimeType, size)
	}
	return json.Marshal(t)
}

func (t *MessageToolCallResponse) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, t)
}

func (t MessageToolCallResponse) Summary() string {
	if t.ToolResponseType == ToolResponseTypeImage {
		return fmt.Sprintf("tool result %s: [image %s]", t.ID, t.ImageMimeType)
	}
	return fmt.Sprintf("tool result %s: %s", t.ID, truncate(t.Text, summaryLimit))
}
