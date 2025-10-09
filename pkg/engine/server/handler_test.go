package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler_TemplateFunc(t *testing.T) {
	tests := []struct {
		name          string
		setupRequest  func() *http.Request
		testFunction  string
		argument      string
		expectedValue string
	}{
		{
			name: "header function returns correct header value",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Test-Header", "test-value")
				return req
			},
			testFunction:  "header",
			argument:      "X-Test-Header",
			expectedValue: "test-value",
		},
		{
			name: "param function returns correct form value",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/?testParam=paramValue", nil)
				return req
			},
			testFunction:  "param",
			argument:      "testParam",
			expectedValue: "paramValue",
		},
		{
			name: "body function returns JSON value for valid JSON body",
			setupRequest: func() *http.Request {
				jsonBody := `{"test": "value"}`
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			testFunction:  "body",
			argument:      "test",
			expectedValue: "value",
		},
		{
			name: "body function returns empty string for non-JSON content type",
			setupRequest: func() *http.Request {
				jsonBody := `{"test": "value"}`
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(jsonBody))
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			testFunction:  "body",
			argument:      "test",
			expectedValue: "",
		},
		{
			name: "body function returns empty string for invalid JSON path",
			setupRequest: func() *http.Request {
				jsonBody := `{"test": "value"}`
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			testFunction:  "body",
			argument:      "nonexistent",
			expectedValue: "",
		},
		{
			name: "body function returns nested JSON value",
			setupRequest: func() *http.Request {
				jsonBody := `{"nested": {"value": "nestedValue"}}`
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			testFunction:  "body",
			argument:      "nested.value",
			expectedValue: "nestedValue",
		},
		{
			name: "body function correctly handles special characters",
			setupRequest: func() *http.Request {
				jsonBody := `{"test": "line1\nline2\t\"quoted\"\r\n\\escaped"}`
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			testFunction:  "body",
			argument:      "test",
			expectedValue: `line1\nline2\t\"quoted\"\r\n\\escaped`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			templateFunctions := requestTemplateFunctions(req)

			fn, ok := templateFunctions[tt.testFunction]
			if !ok {
				t.Errorf("Template function %s not found in request", tt.testFunction)
			}

			// Call the function based on its type
			var result string
			switch f := fn.(type) {
			case func(string) string:
				result = f(tt.argument)
			default:
				t.Fatalf("unexpected function type for %s", tt.testFunction)
			}

			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestRequestContext_BodyFunctionCaching(t *testing.T) {
	jsonBody := `{"test": "value"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	funcs := requestTemplateFunctions(req)

	bodyFunc := funcs["body"].(func(string) string)

	// First call should read the body
	result1 := bodyFunc("test")
	if result1 != "value" {
		t.Errorf("expected 'value', got %q", result1)
	}

	// Second call should use cached body
	result2 := bodyFunc("test")
	if result2 != "value" {
		t.Errorf("expected 'value', got %q", result2)
	}
}

func TestRequestContext_BodySizeLimit(t *testing.T) {
	// Create a body that's larger than the 1MB limit
	largeBody := make([]byte, 2*1024*1024) // 2MB
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")

	funcs := requestTemplateFunctions(req)

	bodyFunc := funcs["body"].(func(string) string)

	// The body function should return empty string for oversized body
	result := bodyFunc("test")
	if result != "" {
		t.Errorf("expected empty string for oversized body, got %q", result)
	}
}
