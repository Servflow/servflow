package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	plan2 "github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var sessionIDHeader = "Mcp-Session-Id"

// TestRequest represents a single HTTP request to be made during testing
type TestRequest struct {
	Name        string                                       // Name of this request for logging/debugging
	Path        string                                       // URL path for the request
	Method      string                                       // HTTP method (GET, POST, etc)
	Body        string                                       // Request body as a string
	Headers     map[string]string                            // Custom headers for the request
	WantStatus  int                                          // Expected HTTP status code
	WantBody    string                                       // Expected response body (for exact matches)
	WantJSON    interface{}                                  // Expected response body as JSON (for structured comparison)
	AssertExtra func(*testing.T, *httptest.ResponseRecorder) // Additional custom assertions
}

// TestRunner encapsulates the test environment and configuration
type TestRunner struct {
	t         *testing.T
	ctrl      *gomock.Controller
	apiConfig *apiconfig.APIConfig
	handler   http.Handler
}

// NewTestRunner creates a new TestRunner with the given configuration
func NewTestRunner(t *testing.T, config *apiconfig.APIConfig) *TestRunner {
	t.Helper()
	ctrl := gomock.NewController(t)

	runner := &TestRunner{
		t:         t,
		ctrl:      ctrl,
		apiConfig: config,
	}

	return runner
}

// WithMocks sets up mock expectations using a provided setup function
func (r *TestRunner) WithMocks(setup func(*gomock.Controller)) *TestRunner {
	setup(r.ctrl)
	return r
}

// WithDefaultMocks sets up default mock behavior
func (r *TestRunner) WithDefaultMocks() *TestRunner {
	mockProvider := plan2.NewMockActionProvider(r.ctrl)
	mockExecutable := plan2.NewMockActionExecutable(r.ctrl)
	mockExecutable.EXPECT().Config().Return("").AnyTimes()
	mockExecutable.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("default response", nil).AnyTimes()
	mockProvider.EXPECT().GetActionExecutable(gomock.Any(), gomock.Any()).Return(mockExecutable, nil).AnyTimes()
	return r
}

// Init initializes the test runner, creating the HTTP handler
func (r *TestRunner) Init() *TestRunner {
	eng := Engine{}
	r.handler = eng.createCustomMuxHandler([]*apiconfig.APIConfig{r.apiConfig})
	return r
}

// RunRequests executes a series of test requests
func (r *TestRunner) RunRequests(requests ...TestRequest) {
	for _, req := range requests {
		r.t.Run(req.Name, func(t *testing.T) {

			httpReq := httptest.NewRequestWithContext(context.Background(), req.Method, req.Path, bytes.NewBufferString(req.Body))
			httpReq.Header.Add("Content-Type", "application/json")
			httpReq.Header.Add("Accept", "application/json")

			// Add headers
			for key, value := range req.Headers {
				httpReq.Header.Add(key, value)
			}

			w := httptest.NewRecorder()
			r.handler.ServeHTTP(w, httpReq)

			// Status code assertion
			if req.WantStatus != 0 {
				assert.Equal(t, req.WantStatus, w.Code, "unexpected status code")
			}

			// Body assertions
			if req.WantBody != "" {
				assert.Equal(t, req.WantBody, w.Body.String(), "unexpected response body")
			}

			if req.WantJSON != nil {
				var got interface{}
				err := json.Unmarshal(w.Body.Bytes(), &got)
				assert.NoError(t, err, "failed to parse response JSON")
				assert.Equal(t, req.WantJSON, got, "unexpected JSON response")
			}

			// Run additional assertions if provided
			if req.AssertExtra != nil {
				req.AssertExtra(t, w)
			}
		})
	}
}

// Example test data
var mcpInit = `{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "roots": {
        "listChanged": true
      },
      "sampling": {},
      "elicitation": {}
    },
    "clientInfo": {
      "name": "ExampleClient",
      "title": "Example Client Display Name",
      "version": "1.0.0"
    }
  }
}`

var mcpInitResponse = `{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "prompts": {},
      "resources": {},
      "tools": {
        "listChanged": true
      }
    },
    "serverInfo": {
      "name": "Servflow MCP",
      "version": "0.1.0"
    }
  }
}`

func TestCreateCustomMuxHandler(t *testing.T) {
	config := &apiconfig.APIConfig{
		McpTool: apiconfig.MCPToolConfig{
			Name:   "mcptool",
			Start:  "action.action1",
			Result: fmt.Sprintf(`{{ .%saction2.key }}`, requestctx.VariableActionPrefix),
			//Result:      dpl.VariableActionPrefix + "action2",
			Description: "Test Endpoint",
			Args: map[string]apiconfig.ArgType{
				"parameter1": {
					Name: "parameter1",
					Type: "string",
				},
			},
		},
		Actions: map[string]apiconfig.Action{
			"action2": {
				Type: "stub",
				Config: map[string]interface{}{
					"key": "value",
				},
			},
			"action1": {
				Type: "stub",
				Next: "action.action2",
				Config: map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	// Create and initialize test runner
	runner := NewTestRunner(t, config).
		Init()

	// Run all test requests
	var sessionID string
	runner.RunRequests(TestRequest{
		Name:       "successful MCP initialization",
		Path:       "/mcp",
		Method:     http.MethodPost,
		Body:       mcpInit,
		WantStatus: http.StatusOK,
		AssertExtra: func(t *testing.T, w *httptest.ResponseRecorder) {
			sessionID = w.Header().Get(sessionIDHeader)
			assert.JSONEq(t, mcpInitResponse, w.Body.String())
		},
	})

	runner.RunRequests(TestRequest{
		Name:   "MCP List tools",
		Path:   "/mcp",
		Method: http.MethodPost,
		Headers: map[string]string{
			sessionIDHeader: sessionID,
		},
		Body: `{
	  "jsonrpc": "2.0",
	  "id": 1,
	  "method": "tools/list",
	  "params": {
	  }
}`,
		WantStatus: http.StatusOK,
		AssertExtra: func(t *testing.T, w *httptest.ResponseRecorder) {
			assert.JSONEq(t, `
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "annotations": {
          "readOnlyHint": false,
          "destructiveHint": true,
          "idempotentHint": false,
          "openWorldHint": true
        },
		"description" : "Test Endpoint",
        "inputSchema": {
          "properties": {
            "parameter1": {
              "type": "string"
            }
          },
          "required": [
            "parameter1"
          ],
          "type": "object"
        },
        "name": "mcptool"
      }
    ]
  }
}`, w.Body.String())
		},
	})
	runner.RunRequests(TestRequest{
		Name: "Mcp call tool",
		Path: "/mcp",
		Headers: map[string]string{
			sessionIDHeader: sessionID,
		},
		Body: `
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "mcptool",
    "arguments": {
      "parameter1": "parameter value"
    }
  }
}`,
		Method:     http.MethodPost,
		WantStatus: http.StatusOK,
		AssertExtra: func(t *testing.T, w *httptest.ResponseRecorder) {
			assert.JSONEq(t, `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"value"}]}}`, w.Body.String())
		},
	})

	runner.RunRequests([]TestRequest{
		{
			Name:       "invalid JSON request",
			Path:       "/mcp",
			Method:     http.MethodPost,
			Body:       `{"invalid": json`,
			WantStatus: http.StatusBadRequest,
		},
		{
			Name:   "custom headers test",
			Path:   "/mcp",
			Method: http.MethodPost,
			Body:   mcpInit,
			Headers: map[string]string{
				"Content-Type":    "application/json",
				"Accept":          "application/json",
				"X-Custom-Header": "test-value",
			},
			WantStatus: http.StatusOK,
		},
	}...)
}

func TestExtractURLParam(t *testing.T) {
	config := &apiconfig.APIConfig{
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/test/{id}",
			Method:     "POST",
			Next:       "action.action1",
		},
		Actions: map[string]apiconfig.Action{
			"action1": {
				Type: "stub",
				Next: "response.finish",
				Config: map[string]interface{}{
					"key": "value",
				},
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"finish": {
				Type:     "template",
				Code:     200,
				Template: `{{ urlparam "id" }}`,
			},
		},
	}

	runner := NewTestRunner(t, config).WithDefaultMocks().Init()

	runner.RunRequests(TestRequest{
		Name:     "extract url param",
		Method:   "POST",
		Path:     "/test/hello",
		WantBody: "hello",
	})
}
