package requestctx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestContext_FileManagement(t *testing.T) {
	ctx := NewTestContext()
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		t.Fatalf("Failed to get request context: %v", err)
	}

	t.Run("add and retrieve request file", func(t *testing.T) {
		fileContent := "test file content"
		file := io.NopCloser(strings.NewReader(fileContent))

		reqCtx.AddRequestFile("testfile", NewFileValue(file, "test.txt"))

		if len(reqCtx.availableFiles) != 1 {
			t.Errorf("Expected 1 file in availableFiles, got %d", len(reqCtx.availableFiles))
		}

		expectedKey := fileKeyRequestPrefix + "testfile"
		if _, exists := reqCtx.availableFiles[expectedKey]; !exists {
			t.Errorf("Expected file with key '%s' not found in availableFiles", expectedKey)
		}

		fv, err := GetFileFromContext(ctx, apiconfig.FileInput{
			Type:       apiconfig.FileInputTypeRequest,
			Identifier: "testfile",
		})
		if err != nil {
			t.Fatalf("Failed to retrieve file: %v", err)
		}

		content, err := fv.GetContent()
		if err != nil {
			t.Fatalf("Failed to get file content: %v", err)
		}

		if string(content) != fileContent {
			t.Errorf("Expected file content '%s', got '%s'", fileContent, string(content))
		}

		fv.Close()
	})

	t.Run("add and retrieve action file", func(t *testing.T) {
		fileContent := "action file content"
		file := io.NopCloser(strings.NewReader(fileContent))

		reqCtx.AddActionFile("actionfile", NewFileValue(file, "action.txt"))

		expectedKey := fileKeyActionPrefix + "actionfile"
		if _, exists := reqCtx.availableFiles[expectedKey]; !exists {
			t.Errorf("Expected file with key '%s' not found in availableFiles", expectedKey)
		}

		fv, err := GetFileFromContext(ctx, apiconfig.FileInput{
			Type:       apiconfig.FileInputTypeAction,
			Identifier: "actionfile",
		})
		if err != nil {
			t.Fatalf("Failed to retrieve action file: %v", err)
		}

		content, err := fv.GetContent()
		if err != nil {
			t.Fatalf("Failed to get file content: %v", err)
		}

		if string(content) != fileContent {
			t.Errorf("Expected file content '%s', got '%s'", fileContent, string(content))
		}

		fv.Close()
	})

	t.Run("multiple files in map", func(t *testing.T) {
		reqCtx.AddRequestFile("req1", NewFileValue(io.NopCloser(strings.NewReader("request file 1")), "req1.txt"))
		reqCtx.AddRequestFile("req2", NewFileValue(io.NopCloser(strings.NewReader("request file 2")), "req2.txt"))
		reqCtx.AddActionFile("act1", NewFileValue(io.NopCloser(strings.NewReader("action file 1")), "act1.txt"))

		if len(reqCtx.availableFiles) != 5 {
			t.Errorf("Expected 5 files in availableFiles, got %d", len(reqCtx.availableFiles))
		}

		testCases := []struct {
			inputType  string
			identifier string
			expected   string
		}{
			{apiconfig.FileInputTypeRequest, "req1", "request file 1"},
			{apiconfig.FileInputTypeRequest, "req2", "request file 2"},
			{apiconfig.FileInputTypeAction, "act1", "action file 1"},
		}

		for _, tc := range testCases {
			fv, err := GetFileFromContext(ctx, apiconfig.FileInput{
				Type:       tc.inputType,
				Identifier: tc.identifier,
			})
			if err != nil {
				t.Errorf("Failed to retrieve file '%s': %v", tc.identifier, err)
				continue
			}

			content, _ := fv.GetContent()
			if string(content) != tc.expected {
				t.Errorf("File '%s': expected content '%s', got '%s'", tc.identifier, tc.expected, string(content))
			}
			fv.Close()
		}
	})

	t.Run("file overwrite", func(t *testing.T) {
		reqCtx.AddRequestFile("overwritefile", NewFileValue(io.NopCloser(strings.NewReader("original content")), "original.txt"))
		reqCtx.AddRequestFile("overwritefile", NewFileValue(io.NopCloser(strings.NewReader("new content")), "new.txt"))

		fv, _ := GetFileFromContext(ctx, apiconfig.FileInput{
			Type:       apiconfig.FileInputTypeRequest,
			Identifier: "overwritefile",
		})
		content, _ := fv.GetContent()

		if string(content) != "new content" {
			t.Errorf("Expected overwritten content 'new content', got '%s'", string(content))
		}

		fv.Close()
	})
}

func TestRequestContext_LoadRequestFiles(t *testing.T) {
	tests := []struct {
		name          string
		setupRequest  func() *http.Request
		expectedFiles int
		expectError   bool
	}{
		{
			name: "single file upload",
			setupRequest: func() *http.Request {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				part, _ := writer.CreateFormFile("uploadfile", "test.txt")
				part.Write([]byte("file content"))
				writer.Close()

				req, _ := http.NewRequest("POST", "/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req
			},
			expectedFiles: 1,
			expectError:   false,
		},
		{
			name: "multiple files upload",
			setupRequest: func() *http.Request {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				part1, _ := writer.CreateFormFile("file1", "test1.txt")
				part1.Write([]byte("content1"))

				part2, _ := writer.CreateFormFile("file2", "test2.txt")
				part2.Write([]byte("content2"))

				writer.Close()

				req, _ := http.NewRequest("POST", "/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req
			},
			expectedFiles: 2,
			expectError:   false,
		},
		{
			name: "no multipart content type",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("POST", "/upload", strings.NewReader("plain body"))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedFiles: 0,
			expectError:   false,
		},
		{
			name: "nil request",
			setupRequest: func() *http.Request {
				return nil
			},
			expectedFiles: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCtx := NewRequestContext("test")
			req := tt.setupRequest()

			err := reqCtx.LoadRequestFiles(req)
			if (err != nil) != tt.expectError {
				t.Errorf("LoadRequestFiles() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if len(reqCtx.availableFiles) != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(reqCtx.availableFiles))
			}
		})
	}
}

func TestGetFileFromContext_Errors(t *testing.T) {
	ctx := NewTestContext()

	t.Run("file not found", func(t *testing.T) {
		_, err := GetFileFromContext(ctx, apiconfig.FileInput{
			Type:       apiconfig.FileInputTypeRequest,
			Identifier: "nonexistent",
		})
		if err == nil {
			t.Error("Expected error when retrieving non-existent file, got nil")
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("invalid input type", func(t *testing.T) {
		f, err := GetFileFromContext(ctx, apiconfig.FileInput{
			Type:       "dummy_type",
			Identifier: "somefile",
		})

		assert.Nil(t, f)
		assert.Nil(t, err)
	})
}

func TestFileValue_MimeTypeDetection(t *testing.T) {
	tests := []struct {
		name         string
		content      []byte
		expectedMime string
	}{
		{
			name:         "plain text",
			content:      []byte("hello world"),
			expectedMime: "text/plain",
		},
		{
			name:         "json content",
			content:      []byte(`{"key": "value"}`),
			expectedMime: "application/json",
		},
		{
			name:         "small content",
			content:      []byte("hi"),
			expectedMime: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := io.NopCloser(bytes.NewReader(tt.content))
			fv := NewFileValue(file, "test.txt")

			mimeType, err := fv.GetMimeType()
			if err != nil {
				t.Fatalf("GetMimeType() error = %v", err)
			}

			if !strings.HasPrefix(mimeType, tt.expectedMime) {
				t.Errorf("Expected mime type to start with '%s', got '%s'", tt.expectedMime, mimeType)
			}

			mimeType2, err := fv.GetMimeType()
			if err != nil {
				t.Fatalf("Second GetMimeType() call error = %v", err)
			}

			if mimeType != mimeType2 {
				t.Errorf("MIME type should be cached. First: %s, Second: %s", mimeType, mimeType2)
			}

			content, err := fv.GetContent()
			if err != nil {
				t.Fatalf("GetContent after MIME detection error = %v", err)
			}

			if !bytes.Equal(content, tt.content) {
				t.Errorf("Content mismatch. Expected: %s, Got: %s", string(tt.content), string(content))
			}

			fv.Close()
		})
	}
}

func TestFileValue_GenerateContentString(t *testing.T) {
	content := []byte("hello world")
	file := io.NopCloser(bytes.NewReader(content))
	fv := NewFileValue(file, "test.txt")

	dataURI, err := fv.GenerateContentString()
	if err != nil {
		t.Fatalf("GenerateContentString() error = %v", err)
	}

	expectedPrefix := "data:text/plain"
	if !strings.HasPrefix(dataURI, expectedPrefix) {
		t.Errorf("Expected data URI to start with '%s', got '%s'", expectedPrefix, dataURI)
	}

	if !strings.Contains(dataURI, ";base64,") {
		t.Errorf("Expected data URI to contain ';base64,', got '%s'", dataURI)
	}

	parts := strings.Split(dataURI, ";base64,")
	if len(parts) != 2 {
		t.Errorf("Expected data URI to have exactly one ';base64,' separator, got '%s'", dataURI)
	}

	decodedContent, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode base64 content: %v", err)
	}

	if !bytes.Equal(decodedContent, content) {
		t.Errorf("Decoded content mismatch. Expected: %s, Got: %s", string(content), string(decodedContent))
	}

	fv.Close()
}

func TestFileValue_GetContent(t *testing.T) {
	t.Run("content is cached", func(t *testing.T) {
		content := []byte("test content for caching")
		file := io.NopCloser(bytes.NewReader(content))
		fv := NewFileValue(file, "test.txt")

		content1, err := fv.GetContent()
		require.NoError(t, err)

		content2, err := fv.GetContent()
		require.NoError(t, err)

		assert.Equal(t, content1, content2)
		assert.Equal(t, content, content1)

		fv.Close()
	})

	t.Run("content consistent after multiple method calls", func(t *testing.T) {
		content := []byte("consistent content test")
		file := io.NopCloser(bytes.NewReader(content))
		fv := NewFileValue(file, "test.txt")

		_, err := fv.GetMimeType()
		require.NoError(t, err)

		_, err = fv.GenerateContentString()
		require.NoError(t, err)

		gotContent, err := fv.GetContent()
		require.NoError(t, err)
		assert.Equal(t, content, gotContent)

		fv.Close()
	})
}

func TestFileValue_NewReader(t *testing.T) {
	t.Run("returns fresh reader each time", func(t *testing.T) {
		content := []byte("test content for reader")
		file := io.NopCloser(bytes.NewReader(content))
		fv := NewFileValue(file, "test.txt")

		reader1, err := fv.NewReader()
		require.NoError(t, err)

		read1, err := io.ReadAll(reader1)
		require.NoError(t, err)
		assert.Equal(t, content, read1)

		reader2, err := fv.NewReader()
		require.NoError(t, err)

		read2, err := io.ReadAll(reader2)
		require.NoError(t, err)
		assert.Equal(t, content, read2)

		fv.Close()
	})
}

func TestFileValue_CompleteWorkflow(t *testing.T) {
	imageContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	file := io.NopCloser(bytes.NewReader(imageContent))
	fv := NewFileValue(file, "test.jpg")

	mimeType, err := fv.GetMimeType()
	if err != nil {
		t.Fatalf("GetMimeType() error = %v", err)
	}

	if !strings.HasPrefix(mimeType, "image/") {
		t.Errorf("Expected image MIME type, got '%s'", mimeType)
	}

	dataURI, err := fv.GenerateContentString()
	if err != nil {
		t.Fatalf("GenerateContentString() error = %v", err)
	}

	expectedPrefix := fmt.Sprintf("data:%s;base64,", mimeType)
	if !strings.HasPrefix(dataURI, expectedPrefix) {
		t.Errorf("Expected data URI to start with '%s', got '%s'", expectedPrefix, dataURI)
	}

	parts := strings.Split(dataURI, ";base64,")
	decodedContent, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode base64 content: %v", err)
	}

	if !bytes.Equal(decodedContent, imageContent) {
		t.Errorf("Decoded content mismatch")
	}

	reader, err := fv.NewReader()
	if err != nil {
		t.Fatalf("Failed to get NewReader(): %v", err)
	}

	readerContent, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read from NewReader(): %v", err)
	}

	if !bytes.Equal(readerContent, imageContent) {
		t.Errorf("NewReader() content mismatch")
	}

	fv.Close()
}

func TestFileValue_CloseBehavior(t *testing.T) {
	t.Run("idempotent close", func(t *testing.T) {
		content := []byte("test content")
		file := io.NopCloser(bytes.NewReader(content))
		fv := NewFileValue(file, "test.txt")

		err := fv.Close()
		if err != nil {
			t.Errorf("Expected no error on close, got: %v", err)
		}

		err = fv.Close()
		if err != nil {
			t.Errorf("Expected no error on second close, got: %v", err)
		}
	})

	t.Run("auto close on content caching", func(t *testing.T) {
		content := []byte("test content for auto close")
		file := io.NopCloser(bytes.NewReader(content))
		fv := NewFileValue(file, "test.txt")

		_, err := fv.GenerateContentString()
		if err != nil {
			t.Fatalf("GenerateContentString() error = %v", err)
		}

		err = fv.Close()
		if err != nil {
			t.Errorf("Expected no error on close after auto-close, got: %v", err)
		}

		readContent, err := fv.GetContent()
		if err != nil {
			t.Fatalf("Getting cached content failed: %v", err)
		}

		if !bytes.Equal(readContent, content) {
			t.Errorf("Cached content mismatch. Expected: %s, Got: %s", string(content), string(readContent))
		}
	})
}
