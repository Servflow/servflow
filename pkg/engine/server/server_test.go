package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	plan2 "github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

var sessionIDHeader = "Mcp-Session-Id"

type TestRequest struct {
	Name        string
	Request     *http.Request
	WantStatus  int
	WantBody    string
	WantJSON    interface{}
	AssertExtra func(*testing.T, *httptest.ResponseRecorder)
}

type TestRunner struct {
	t         *testing.T
	ctrl      *gomock.Controller
	apiConfig *apiconfig.APIConfig
	handler   http.Handler
}

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

func (r *TestRunner) WithMocks(setup func(*gomock.Controller)) *TestRunner {
	setup(r.ctrl)
	return r
}

func (r *TestRunner) WithDefaultMocks() *TestRunner {
	mockProvider := plan2.NewMockActionProvider(r.ctrl)
	mockExecutable := plan2.NewMockActionExecutable(r.ctrl)
	mockExecutable.EXPECT().Config().Return("").AnyTimes()
	mockExecutable.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("default response", nil).AnyTimes()
	mockProvider.EXPECT().GetActionExecutable(gomock.Any(), gomock.Any()).Return(mockExecutable, nil).AnyTimes()
	return r
}

func (r *TestRunner) Init() *TestRunner {
	devLogger, err := zap.NewDevelopment()
	if err != nil {
		r.t.Fatal(err)
	}
	eng := Engine{
		logger: devLogger,
	}
	r.handler = eng.createMuxHandler([]*apiconfig.APIConfig{r.apiConfig})
	return r
}

func (r *TestRunner) RunRequests(requests ...TestRequest) {
	for _, req := range requests {
		r.t.Run(req.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.handler.ServeHTTP(w, req.Request)

			if req.WantStatus != 0 {
				assert.Equal(t, req.WantStatus, w.Code, "unexpected status code")
			}

			if req.WantBody != "" {
				assert.Equal(t, req.WantBody, w.Body.String(), "unexpected response body")
			}

			if req.WantJSON != nil {
				var got interface{}
				err := json.Unmarshal(w.Body.Bytes(), &got)
				assert.NoError(t, err, "failed to parse response JSON")
				assert.Equal(t, req.WantJSON, got, "unexpected JSON response")
			}

			if req.AssertExtra != nil {
				req.AssertExtra(t, w)
			}
		})
	}
}

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

	runner := NewTestRunner(t, config).Init()

	var sessionID string
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", bytes.NewBufferString(mcpInit))
	req1.Header.Add("Content-Type", "application/json")
	req1.Header.Add("Accept", "application/json")

	runner.RunRequests(TestRequest{
		Name:       "successful MCP initialization",
		Request:    req1,
		WantStatus: http.StatusOK,
		AssertExtra: func(t *testing.T, w *httptest.ResponseRecorder) {
			sessionID = w.Header().Get(sessionIDHeader)
			assert.JSONEq(t, mcpInitResponse, w.Body.String())
		},
	})

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", bytes.NewBufferString(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/list",
			"params": {
			}
}`))
	req2.Header.Add("Content-Type", "application/json")
	req2.Header.Add("Accept", "application/json")
	req2.Header.Add(sessionIDHeader, sessionID)

	runner.RunRequests(TestRequest{
		Name:       "MCP List tools",
		Request:    req2,
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

	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", bytes.NewBufferString(`
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
}`))
	req3.Header.Add("Content-Type", "application/json")
	req3.Header.Add("Accept", "application/json")
	req3.Header.Add(sessionIDHeader, sessionID)

	runner.RunRequests(TestRequest{
		Name:       "Mcp call tool",
		Request:    req3,
		WantStatus: http.StatusOK,
		AssertExtra: func(t *testing.T, w *httptest.ResponseRecorder) {
			assert.JSONEq(t, `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"value"}]}}`, w.Body.String())
		},
	})

	req4 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", bytes.NewBufferString(`{"invalid": json`))
	req4.Header.Add("Content-Type", "application/json")
	req4.Header.Add("Accept", "application/json")

	req5 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", bytes.NewBufferString(mcpInit))
	req5.Header.Add("Content-Type", "application/json")
	req5.Header.Add("Accept", "application/json")
	req5.Header.Add("X-Custom-Header", "test-value")

	runner.RunRequests([]TestRequest{
		{
			Name:       "invalid JSON request",
			Request:    req4,
			WantStatus: http.StatusBadRequest,
		},
		{
			Name:       "custom headers test",
			Request:    req5,
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

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test/hello", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	runner.RunRequests(TestRequest{
		Name:     "extract url param",
		Request:  req,
		WantBody: "hello",
	})
}

func TestMultipartFormWithTemplatedAction(t *testing.T) {
	config := &apiconfig.APIConfig{
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/api/upload",
			Method:     "POST",
			Next:       "action.process_form",
		},
		Actions: map[string]apiconfig.Action{
			"process_form": {
				Type: "static",
				Config: map[string]interface{}{
					"return": `{{ param "testfield" }}`,
				},
				Next: "response.success",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Type:     "template",
				Code:     200,
				Template: `Field value: {{  .variable_actions_process_form }}`,
			},
		},
	}

	runner := NewTestRunner(t, config).WithDefaultMocks().Init()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	err := writer.WriteField("testfield", "hello_world")
	assert.NoError(t, err)
	fileWriter, err := writer.CreateFormFile("dummyfile", "test.txt")
	assert.NoError(t, err)
	_, err = fileWriter.Write([]byte("dummy file content"))
	assert.NoError(t, err)
	writer.Close()

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("Accept", "application/json")

	runner.RunRequests(TestRequest{
		Name:       "multipart form with templated field",
		Request:    req,
		WantStatus: 200,
		WantBody:   "Field value: hello_world",
	})
}
