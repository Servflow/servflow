package agent

import "github.com/mark3labs/mcp-go/mcp"

type RoleType int

const (
	RoleTypeUnknown RoleType = iota
	RoleTypeSystem
	RoleTypeUser
	RoleTypeAssistant
	RoleTypeDeveloper
)

type ContentType int

const (
	ContentTypeText ContentType = iota
	ContentTypeToolCall
	ContentTypeToolResponse
)

type Content struct {
	Type ContentType
	Text string
}

type LLMRequest struct {
	SystemMessage string
	Messages      []any
	Tools         []ToolInfo `json:"tools"`
}

type ContentMessage struct {
	Role    RoleType
	Content string
}

type ToolCallMessage struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

type ToolCallOutputMessage struct {
	ID     string
	Output string
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
