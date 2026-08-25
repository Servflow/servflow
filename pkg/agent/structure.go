package agent

import (
	"strings"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
)

// The conversation message types now live in requestctx (which owns the
// run-scoped conversation thread). They are re-exported here as aliases so
// existing callers — the LLM integrations' type switches, this package's
// session loop, and tests — keep referring to them as agent.* unchanged.

type RoleType = requestctx.RoleType

const (
	RoleTypeUnknown   = requestctx.RoleTypeUnknown
	RoleTypeSystem    = requestctx.RoleTypeSystem
	RoleTypeUser      = requestctx.RoleTypeUser
	RoleTypeAssistant = requestctx.RoleTypeAssistant
	RoleTypeDeveloper = requestctx.RoleTypeDeveloper
)

type MessageType = requestctx.MessageType

const (
	MessageTypeText         = requestctx.MessageTypeText
	MessageTypeToolCall     = requestctx.MessageTypeToolCall
	MessageTypeToolResponse = requestctx.MessageTypeToolResponse
)

type (
	Message                 = requestctx.Message
	MessageTypeContent      = requestctx.MessageTypeContent
	MessageToolCall         = requestctx.MessageToolCall
	MessageToolCallResponse = requestctx.MessageToolCallResponse
	ConversationMessage     = requestctx.ConversationMessage
)

type ToolResponseType = requestctx.ToolResponseType

const (
	ToolResponseTypeUnknown = requestctx.ToolResponseTypeUnknown
	ToolResponseTypeText    = requestctx.ToolResponseTypeText
	ToolResponseTypeImage   = requestctx.ToolResponseTypeImage
)

type ToolCallOutputType = requestctx.ToolCallOutputType

const (
	ToolCallOutputTypeText  = requestctx.ToolCallOutputTypeText
	ToolCallOutputTypeImage = requestctx.ToolCallOutputTypeImage
)

type LLMRequest struct {
	SystemMessage string
	Instruction   string
	Messages      []any
	Tools         []ToolInfo `json:"tools"`
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
