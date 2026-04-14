package requestctx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadAndRestoreBody(t *testing.T) {
	tests := []struct {
		name     string
		request  func() *http.Request
		expected string
	}{
		{
			name: "reads body successfully",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
			},
			expected: `{"key":"value"}`,
		},
		{
			name: "nil request returns empty string",
			request: func() *http.Request {
				return nil
			},
			expected: "",
		},
		{
			name: "nil body returns empty string",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Body = nil
				return req
			},
			expected: "",
		},
		{
			name: "empty body returns empty string",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.request()
			result := ReadAndRestoreBody(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadAndRestoreBody_CanReadAgain(t *testing.T) {
	body := `{"test":"data","number":123}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))

	result1 := ReadAndRestoreBody(req)
	assert.Equal(t, body, result1)

	result2 := ReadAndRestoreBody(req)
	assert.Equal(t, body, result2)

	bodyBytes, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, body, string(bodyBytes))
}
