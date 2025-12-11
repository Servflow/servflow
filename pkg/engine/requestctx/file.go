package requestctx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

type FileInputType int

const (
	FileInputTypeRequest FileInputType = iota
	FileInputTypeAction
)

const (
	fileKeyActionPrefix  = "action."
	fileKeyRequestPrefix = "request."
)

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
				rc.availableFiles[fieldName] = &FileValue{
					File: file,
					Name: fileHeader.Filename,
				}
			}
		}
	}

	return nil
}

type FileValue struct {
	File     io.ReadCloser
	Name     string
	MimeType string
}

func GetFileFromContext(ctx context.Context, inputType FileInputType, identifier string) (*FileValue, error) {
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}

	var key string
	switch inputType {
	case FileInputTypeRequest:
		key = fileKeyRequestPrefix + identifier
	case FileInputTypeAction:
		key = fileKeyActionPrefix + identifier
	default:
		return nil, fmt.Errorf("invalid file input type: %d", inputType)
	}

	file, ok := reqCtx.availableFiles[key]
	if !ok {
		return nil, fmt.Errorf("file '%s' not found", identifier)
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

func (f *FileValue) GenerateContentString() []byte {
	
}

func (f *FileValue) mimeType() (string, error) {
	if f.MimeType != "" {
		return f.MimeType, nil
	}

	bufferedReader := bufio.NewReader(f.File)

	peek, err := bufferedReader.Peek(512)
	if err != nil && err != io.EOF {
		return "", err
	}

	mtype := mimetype.Detect(peek)
	f.MimeType = mtype.String()
	return f.MimeType, nil
}
