package requestctx

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/gabriel-vasile/mimetype"
)

const (
	fileKeyActionPrefix  = "action."
	fileKeyRequestPrefix = "request."
)

// TODO see if we want to limit file to reading once based on memory use and pressure

var ErrFileNotFound = errors.New("file not found")

// FileValue provides safe, consistent access to file content.
//
// Design:
// FileValue intentionally does NOT implement io.Reader or expose the underlying reader.
// This prevents inconsistent state that could occur if callers partially read from
// the stream before calling methods like GetContent() or GenerateContentString().
//
// Usage:
//   - Use GetContent() to retrieve the file's raw bytes (cached after first read)
//   - Use GenerateContentString() to get a base64-encoded data URI
//   - Use GetMimeType() to detect the file's MIME type
//
// All methods that read from the file will cache the content on first access,
// ensuring subsequent calls return consistent data. The original file handle
// is automatically closed after content is cached.
type FileValue struct {
	file     io.ReadCloser // original file handle, closed after content is cached
	content  []byte        // cached content after first read
	Name     string
	mimeType string
	closed   bool
}

func NewFileValue(file io.ReadCloser, name string) *FileValue {
	return &FileValue{
		file: file,
		Name: name,
	}
}

func (f *FileValue) Close() error {
	if !f.closed && f.file != nil {
		err := f.file.Close()
		f.closed = true
		return err
	}
	return nil
}

func GetFileFromContext(ctx context.Context, fileInput apiconfig.FileInput) (*FileValue, error) {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}

	var key string
	switch fileInput.Type {
	case apiconfig.FileInputTypeRequest:
		key = fileKeyRequestPrefix + fileInput.Identifier
	case apiconfig.FileInputTypeAction:
		key = fileKeyActionPrefix + strings.TrimPrefix(fileInput.Identifier, apiconfig.ActionConfigPrefix)
	case apiconfig.FileInputTypeStorage:
		ws := rc.Workspace()
		if ws == nil {
			return nil, ErrNoWorkspace
		}
		data, err := ws.Read(ctx, fileInput.Identifier)
		if err != nil {
			return nil, err
		}
		return NewFileValue(io.NopCloser(bytes.NewReader(data)), fileInput.Identifier), nil
	default:
		return nil, nil
	}

	file, ok := rc.File(key)
	if !ok {
		return nil, ErrFileNotFound
	}

	return file, nil
}

// AddRequestFile stores a file that arrived with the request under its form
// field name, where a "request" file input finds it.
func AddRequestFile(rc RequestContext, fieldName string, file *FileValue) {
	rc.AddFile(fileKeyRequestPrefix+fieldName, file)
}

// AddActionFile stores a file an action produced under the action's name,
// where an "action" file input finds it.
func AddActionFile(rc RequestContext, name string, file *FileValue) {
	rc.AddFile(fileKeyActionPrefix+name, file)
}

// GetContent returns the file's content as a byte slice.
// The content is cached on first read, so subsequent calls return the same data.
// The original file handle is closed after the content is cached.
func (f *FileValue) GetContent() ([]byte, error) {
	if f.content != nil {
		return f.content, nil
	}

	content, err := io.ReadAll(f.file)
	if err != nil {
		return nil, err
	}
	f.content = content
	f.closeOriginalFile()

	return f.content, nil
}

// GenerateContentString returns a base64-encoded data URI containing the file content.
// Format: data:<mime-type>;base64,<base64-encoded-content>
func (f *FileValue) GenerateContentString() (string, error) {
	mimeType, err := f.GetMimeType()
	if err != nil {
		return "", err
	}

	content, err := f.GetContent()
	if err != nil {
		return "", err
	}

	base64Content := base64.StdEncoding.EncodeToString(content)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Content), nil
}

// GetMimeType detects and returns the file's MIME type.
// The MIME type is cached after first detection.
func (f *FileValue) GetMimeType() (string, error) {
	if f.mimeType != "" {
		return f.mimeType, nil
	}

	content, err := f.GetContent()
	if err != nil {
		return "", err
	}

	mtype := mimetype.Detect(content)
	f.mimeType = mtype.String()
	return f.mimeType, nil
}

// NewReader returns a new io.Reader over the cached content.
// This can be called multiple times to get fresh readers.
// Note: GetContent() must have been called first (directly or via other methods),
// or this will trigger content caching.
func (f *FileValue) NewReader() (io.Reader, error) {
	content, err := f.GetContent()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(content), nil
}

func (f *FileValue) closeOriginalFile() {
	if !f.closed && f.file != nil {
		f.file.Close()
		f.closed = true
	}
}
