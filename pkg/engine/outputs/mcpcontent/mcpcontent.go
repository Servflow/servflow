// Package mcpcontent renders a run's output as MCP tool content.
//
// It lives in its own package for the same reason responses/http does: the core
// outputs package is protocol-free, and MCP is a protocol. Both MCP-shaped
// callers — the engine's MCP tool handler and the agent's workflow tool — share
// this single rendering so a new Result kind cannot be handled by one and
// forgotten by the other.
package mcpcontent

import (
	"encoding/base64"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/outputs"
	"github.com/Servflow/servflow/pkg/engine/responses"
	"github.com/mark3labs/mcp-go/mcp"
)

// Render turns a run's output into MCP tool content. A run that produced no
// output yields empty text rather than an error: a tool whose workflow returns
// nothing is a valid shape, and empty text is what it returned before output
// handlers existed.
func Render(result responses.Result) ([]mcp.Content, error) {
	switch r := result.(type) {
	case nil:
		return []mcp.Content{mcp.NewTextContent("")}, nil
	case outputs.TextResult:
		return []mcp.Content{mcp.NewTextContent(r.Text)}, nil
	case outputs.FileResult:
		data, err := r.File.GetContent()
		if err != nil {
			return nil, err
		}
		mimeType, err := r.File.GetMimeType()
		if err != nil {
			return nil, err
		}
		return []mcp.Content{mcp.NewImageContent(base64.StdEncoding.EncodeToString(data), mimeType)}, nil
	default:
		return nil, fmt.Errorf("an mcp tool cannot return a %q result", result.Kind())
	}
}
