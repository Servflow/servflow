package outputs

import (
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Kind values reported by the results an output handler produces.
const (
	KindText = "text"
	KindFile = "file"
)

// TextResult is a run's output as plain text. It is what the template and
// conversation handlers produce, and what every non-HTTP caller (trigger, MCP
// tool, workflow tool) ultimately renders. The HTTP handler writes it as a 200
// response body.
type TextResult struct {
	Text string
}

func (TextResult) Kind() string { return KindText }

// String returns the extracted text.
func (t TextResult) String() string { return t.Text }

// FileResult is a run's output as a file from the request context — an image a
// workflow tool returns to the model, or a document. Consumers read the content
// and mime type off the FileValue.
type FileResult struct {
	File *requestctx.FileValue
}

func (FileResult) Kind() string { return KindFile }
