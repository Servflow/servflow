package mcpcontent_test

import (
	"io"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/outputs"
	"github.com/Servflow/servflow/pkg/engine/outputs/mcpcontent"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		content, err := mcpcontent.Render(outputs.TextResult{Text: "answer"})
		require.NoError(t, err)
		require.Len(t, content, 1)
		text, ok := content[0].(mcp.TextContent)
		require.True(t, ok, "content is %T, want mcp.TextContent", content[0])
		assert.Equal(t, "answer", text.Text)
	})

	t.Run("file becomes image content", func(t *testing.T) {
		file := requestctx.NewFileValue(io.NopCloser(strings.NewReader("body")), "thing.txt")
		content, err := mcpcontent.Render(outputs.FileResult{File: file})
		require.NoError(t, err)
		require.Len(t, content, 1)
		img, ok := content[0].(mcp.ImageContent)
		require.True(t, ok, "content is %T, want mcp.ImageContent", content[0])
		assert.Equal(t, "Ym9keQ==", img.Data)
		assert.Contains(t, img.MIMEType, "text/plain")
	})

	// A workflow with no output still returns a well-formed empty tool result,
	// which is what an empty result expression produced before output handlers.
	t.Run("no output yields empty text", func(t *testing.T) {
		content, err := mcpcontent.Render(nil)
		require.NoError(t, err)
		require.Len(t, content, 1)
		text, ok := content[0].(mcp.TextContent)
		require.True(t, ok, "content is %T, want mcp.TextContent", content[0])
		assert.Empty(t, text.Text)
	})

	// A result kind MCP cannot express is an error naming the kind, not a
	// silently empty tool result.
	t.Run("an unrenderable result names the kind", func(t *testing.T) {
		_, err := mcpcontent.Render(unknownResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stream")
	})
}

type unknownResult struct{}

func (unknownResult) Kind() string { return "stream" }
