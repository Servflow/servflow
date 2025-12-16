package agent

import (
	"encoding/json"
	"fmt"

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

type MessageType int

const (
	MessageTypeText MessageType = iota
	MessageTypeToolCall
	MessageTypeToolResponse
)

type LLMRequest struct {
	SystemMessage string
	Messages      []any
	Tools         []ToolInfo `json:"tools"`
}

type MessageContent struct {
	Message
	Role        RoleType
	Content     string
	FileContent *requestctx.FileValue `json:"-"`
}

func (c *MessageContent) Serialize() ([]byte, error) {
	return json.Marshal(c)
}

func (c *MessageContent) Deserialize(bytes []byte) error {
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

type MessageToolCallResponse struct {
	Message
	ToolResponseType ToolResponseType
	ID               string
	Text             string
	ImageData        []byte
	ImageMimeType    string
}

func (t *MessageToolCallResponse) GenerateContent(imageSupport bool) string {
	switch t.ToolResponseType {
	case ToolResponseTypeText:
		return t.Text
	case ToolResponseTypeImage:
		if imageSupport {
			return fmt.Sprintf("data:%s;base64,%s", t.ImageMimeType, t.ImageData)
		}
		return ""
	default:
		return ""
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
}

type ToolResponseObject struct {
	Name   string                 `json:"name"`
	Input  map[string]interface{} `json:"input"`
	ToolID string                 `json:"toolId"`
}

type ContentResponse struct {
	Text string `json:"text"`
}
