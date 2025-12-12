package requestctx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/gabriel-vasile/mimetype"
)

const (
	fileKeyActionPrefix  = "action."
	fileKeyRequestPrefix = "request."
)

var ErrFileNotFound = errors.New("file not found")

func (rc *RequestContext) LoadRequestFiles(r *http.Request) error {
	if r == nil {
		return nil
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil
	}

	err := r.ParseMultipartForm(32 << 20) // 32 MB max memory
	if err != nil {
		return err
	}

	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for fieldName, fileHeaders := range r.MultipartForm.File {
			if len(fileHeaders) > 0 {
				fileHeader := fileHeaders[0] // Take the first file if multiple
				file, err := fileHeader.Open()
				if err != nil {
					continue
				}
				rc.AddRequestFile(fieldName, NewFileValue(file, fileHeader.Filename))
			}
		}
	}

	return nil
}

type FileValue struct {
	file     io.ReadCloser
	content  []byte
	reader   io.Reader
	Name     string
	mimeType string
	closed   bool
}

func NewFileValue(file io.ReadCloser, name string) *FileValue {
	return &FileValue{
		file:   file,
		reader: file,
		Name:   name,
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
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}

	var key string
	switch fileInput.Type {
	case apiconfig.FileInputTypeRequest:
		key = fileKeyRequestPrefix + fileInput.Identifier
	case apiconfig.FileInputTypeAction:
		key = fileKeyActionPrefix + fileInput.Identifier
	default:
		return nil, fmt.Errorf("invalid file input type: %s", fileInput.Type)
	}

	file, ok := reqCtx.availableFiles[key]
	if !ok {
		return nil, ErrFileNotFound
	}

	return file, nil
}

func (rc *RequestContext) AddRequestFile(fieldName string, file *FileValue) {
	rc.Lock()
	defer rc.Unlock()
	rc.availableFiles[fileKeyRequestPrefix+fieldName] = file
}

func (rc *RequestContext) AddActionFile(name string, file *FileValue) {
	rc.Lock()
	defer rc.Unlock()
	rc.availableFiles[fileKeyActionPrefix+name] = file
}

func (f *FileValue) GenerateContentString() (string, error) {
	mimeType, err := f.GetMimeType()
	if err != nil {
		return "", err
	}

	if f.content == nil {
		content, err := io.ReadAll(f.reader)
		if err != nil {
			return "", err
		}
		f.content = content
		f.reader = bytes.NewReader(f.content)
		f.closeOriginalFile()
	}

	base64Content := base64.StdEncoding.EncodeToString(f.content)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Content), nil
}

func (f *FileValue) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

func (f *FileValue) GetReader() io.Reader {
	return f.reader
}

func (f *FileValue) GetMimeType() (string, error) {
	if f.mimeType != "" {
		return f.mimeType, nil
	}

	bufferedReader := bufio.NewReader(f.reader)
	f.reader = bufferedReader

	peek, err := bufferedReader.Peek(512)
	if err != nil && err != io.EOF {
		return "", err
	}

	mtype := mimetype.Detect(peek)
	f.mimeType = mtype.String()
	return f.mimeType, nil
}

func (f *FileValue) closeOriginalFile() {
	if !f.closed && f.file != nil {
		f.file.Close()
		f.closed = true
	}
}
