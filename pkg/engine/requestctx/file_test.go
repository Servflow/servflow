package requestctx

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestRequestContext_AddAndRetrieveRequestFile(t *testing.T) {
	ctx := NewTestContext()
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		t.Fatalf("Failed to get request context: %v", err)
	}

	fileContent := "test file content"
	file := io.NopCloser(strings.NewReader(fileContent))

	reqCtx.AddRequestFile("testfile", &FileValue{
		File: file,
		Name: "test.txt",
	})

	if len(reqCtx.availableFiles) != 1 {
		t.Errorf("Expected 1 file in availableFiles, got %d", len(reqCtx.availableFiles))
	}

	expectedKey := fileKeyRequestPrefix + "testfile"
	if _, exists := reqCtx.availableFiles[expectedKey]; !exists {
		t.Errorf("Expected file with key '%s' not found in availableFiles", expectedKey)
	}

	reader, err := GetFileFromContext(ctx, FileInputTypeRequest, "testfile")
	if err != nil {
		t.Fatalf("Failed to retrieve file: %v", err)
	}

	content, err := io.ReadAll(reader.File)
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	if string(content) != fileContent {
		t.Errorf("Expected file content '%s', got '%s'", fileContent, string(content))
	}
}

func TestRequestContext_AddAndRetrieveActionFile(t *testing.T) {
	ctx := NewTestContext()
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		t.Fatalf("Failed to get request context: %v", err)
	}

	fileContent := "action file content"
	file := io.NopCloser(strings.NewReader(fileContent))

	reqCtx.AddActionFile("actionfile", &FileValue{
		File: file,
		Name: "action.txt",
	})

	expectedKey := fileKeyActionPrefix + "actionfile"
	if _, exists := reqCtx.availableFiles[expectedKey]; !exists {
		t.Errorf("Expected file with key '%s' not found in availableFiles", expectedKey)
	}

	reader, err := GetFileFromContext(ctx, FileInputTypeAction, "actionfile")
	if err != nil {
		t.Fatalf("Failed to retrieve action file: %v", err)
	}

	content, err := io.ReadAll(reader.File)
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	if string(content) != fileContent {
		t.Errorf("Expected file content '%s', got '%s'", fileContent, string(content))
	}
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

func TestGetReaderForFile_FileNotFound(t *testing.T) {
	ctx := NewTestContext()

	_, err := GetFileFromContext(ctx, FileInputTypeRequest, "nonexistent")
	if err == nil {
		t.Error("Expected error when retrieving non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestGetReaderForFile_InvalidInputType(t *testing.T) {
	ctx := NewTestContext()

	_, err := GetFileFromContext(ctx, FileInputType(999), "somefile")
	if err == nil {
		t.Error("Expected error for invalid input type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid file input type") {
		t.Errorf("Expected 'invalid file input type' error, got: %v", err)
	}
}

func TestRequestContext_MultipleFilesInMap(t *testing.T) {
	ctx := NewTestContext()
	reqCtx, _ := FromContextOrError(ctx)

	reqCtx.AddRequestFile("req1", &FileValue{
		File: io.NopCloser(strings.NewReader("request file 1")),
		Name: "req1.txt",
	})

	reqCtx.AddRequestFile("req2", &FileValue{
		File: io.NopCloser(strings.NewReader("request file 2")),
		Name: "req2.txt",
	})

	reqCtx.AddActionFile("act1", &FileValue{
		File: io.NopCloser(strings.NewReader("action file 1")),
		Name: "act1.txt",
	})

	if len(reqCtx.availableFiles) != 3 {
		t.Errorf("Expected 3 files in availableFiles, got %d", len(reqCtx.availableFiles))
	}

	testCases := []struct {
		inputType  FileInputType
		identifier string
		expected   string
	}{
		{FileInputTypeRequest, "req1", "request file 1"},
		{FileInputTypeRequest, "req2", "request file 2"},
		{FileInputTypeAction, "act1", "action file 1"},
	}

	for _, tc := range testCases {
		reader, err := GetFileFromContext(ctx, tc.inputType, tc.identifier)
		if err != nil {
			t.Errorf("Failed to retrieve file '%s': %v", tc.identifier, err)
			continue
		}

		content, _ := io.ReadAll(reader.File)
		if string(content) != tc.expected {
			t.Errorf("File '%s': expected content '%s', got '%s'", tc.identifier, tc.expected, string(content))
		}
	}
}

func TestRequestContext_FileOverwrite(t *testing.T) {
	ctx := NewTestContext()
	reqCtx, _ := FromContextOrError(ctx)

	reqCtx.AddRequestFile("file", &FileValue{
		File: io.NopCloser(strings.NewReader("original content")),
		Name: "original.txt",
	})

	reqCtx.AddRequestFile("file", &FileValue{
		File: io.NopCloser(strings.NewReader("new content")),
		Name: "new.txt",
	})

	if len(reqCtx.availableFiles) != 1 {
		t.Errorf("Expected 1 file after overwrite, got %d", len(reqCtx.availableFiles))
	}

	reader, _ := GetFileFromContext(ctx, FileInputTypeRequest, "file")
	content, _ := io.ReadAll(reader.File)

	if string(content) != "new content" {
		t.Errorf("Expected overwritten content 'new content', got '%s'", string(content))
	}
}
