package agent

import (
	"encoding/json"

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

type ContentMessage struct {
	Message
	Role    RoleType
	Content string
}

func (c *ContentMessage) Serialize() ([]byte, error) {
	return json.Marshal(c)
}

func (c *ContentMessage) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, c)
}

type Message struct {
	Type MessageType `json:"type"`
}

type ToolCallMessage struct {
	Message
	ID        string
	Name      string
	Arguments map[string]interface{}
}

func (t *ToolCallMessage) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *ToolCallMessage) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, t)
}

type ToolCallOutputMessage struct {
	Message
	ID     string
	Output string
}

func (t *ToolCallOutputMessage) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *ToolCallOutputMessage) Deserialize(bytes []byte) error {
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
