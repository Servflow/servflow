package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestHttp_Execute(t *testing.T) {
	cases := []struct {
		Name        string
		Config      Config
		Expected    interface{}
		ShouldError bool
		serverSetup func(t *testing.T) string
	}{
		{
			Name: "Successful Call",
			Config: Config{
				Method:  http.MethodGet,
				Headers: map[string]string{"Content-Type": "test"},
				Body:    json.RawMessage(`{"foo":"bar"}`),
			},
			Expected: map[string]interface{}{"hello": "world"},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.Header.Get("Content-Type"), "test")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, `{"foo": "bar"}`, string(bod))
					value := struct {
						Hello string `json:"hello"`
					}{
						Hello: "world",
					}

					resp, _ := json.Marshal(value)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "String Body with Special Characters",
			Config: Config{
				Method:  http.MethodPost,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`"string with \"quotes\", \\backslash, \n newline and \t tab"`),
			},
			Expected: map[string]interface{}{"received": true},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					// String body should be unwrapped with special characters preserved
					assert.Equal(t, "string with \"quotes\", \\backslash, \n newline and \t tab", string(bod))

					w.Write([]byte(`{"received": true}`))
				}))
				return srv.URL
			},
		},
		{
			Name: "Integer Body",
			Config: Config{
				Method:  http.MethodPost,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`123`),
			},
			Expected: map[string]interface{}{"received": true},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.Equal(t, "123", string(bod))

					w.Write([]byte(`{"received": true}`))
				}))
				return srv.URL
			},
		},
		{
			Name: "Plain String Body Without Quotes",
			Config: Config{
				Method:  http.MethodPost,
				Headers: map[string]string{"Content-Type": "text/plain"},
				Body:    json.RawMessage(`"hello world"`),
			},
			Expected: map[string]interface{}{"received": true},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					// The body should be "hello world" without the surrounding quotes
					assert.Equal(t, "hello world", string(bod))

					w.Write([]byte(`{"received": true}`))
				}))
				return srv.URL
			},
		},
		{
			Name: "JSON Object As String Body",
			Config: Config{
				Method:  http.MethodPost,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`"{\n  \"body\": \"There is a typo in the struct tag for CollectorType: 'envconfig:\\\"collectorype\\\"' should be 'envconfig:\\\"collectortype\\\"'. This typo could prevent reading the value from environment variables as intended.\",\n  \"path\": \"config/config.go\",\n  \"commit_id\": \"c23131b154c538c44a2f196e1b2e02a1ab621ca1\",\n  \"line\": 10,\n  \"side\": \"RIGHT\"\n}"`),
			},
			Expected: map[string]interface{}{"received": true},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					// The body should be valid JSON that the server can parse
					var parsed map[string]interface{}
					err = json.Unmarshal(bod, &parsed)
					require.NoError(t, err, "Server should receive valid JSON, got: %s", string(bod))

					assert.Equal(t, "There is a typo in the struct tag for CollectorType: 'envconfig:\"collectorype\"' should be 'envconfig:\"collectortype\"'. This typo could prevent reading the value from environment variables as intended.", parsed["body"])
					assert.Equal(t, "config/config.go", parsed["path"])
					assert.Equal(t, "c23131b154c538c44a2f196e1b2e02a1ab621ca1", parsed["commit_id"])
					assert.Equal(t, float64(10), parsed["line"])
					assert.Equal(t, "RIGHT", parsed["side"])

					w.Write([]byte(`{"received": true}`))
				}))
				return srv.URL
			},
		},
		{
			Name: "Double Encoded JSON Body",
			Config: Config{
				Method:  http.MethodPost,
				Headers: map[string]string{"Content-Type": "application/json"},
				// This is a JSON object wrapped as a JSON string (double-encoded)
				Body: json.RawMessage(`"{\n  \"body\": \"Test comment\",\n  \"path\": \"config/config.go\",\n  \"line\": 12\n}"`),
			},
			Expected: map[string]interface{}{"received": true},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					// The body should be valid JSON that the server can parse
					var parsed map[string]interface{}
					err = json.Unmarshal(bod, &parsed)
					require.NoError(t, err, "Server should receive valid JSON, got: %s", string(bod))

					assert.Equal(t, "Test comment", parsed["body"])
					assert.Equal(t, "config/config.go", parsed["path"])
					assert.Equal(t, float64(12), parsed["line"])

					w.Write([]byte(`{"received": true}`))
				}))
				return srv.URL
			},
		},

		{
			Name: "has response path",
			Config: Config{
				Method:       http.MethodGet,
				Headers:      map[string]string{"Content-Type": "test"},
				ResponsePath: "hello",
			},
			Expected: "world",
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.Header.Get("Content-Type"), "test")
					value := struct {
						Hello string `json:"hello"`
					}{
						Hello: "world",
					}
					resp, _ := json.Marshal(value)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "invalid response path",
			Config: Config{
				Method:       http.MethodGet,
				ResponsePath: "hello",
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")

					v := struct {
						Hi string `json:"hi"`
					}{
						Hi: "world",
					}

					resp, _ := json.Marshal(v)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "Error Call",
			Config: Config{
				Method:               http.MethodPost,
				Headers:              nil,
				ExpectedResponseCode: "200",
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return srv.URL
			},
		},
		{
			Name: "Expected Response Code Failure",
			Config: Config{
				Method:               http.MethodGet,
				ExpectedResponseCode: "200",
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte(`{"error": "not found"}`))
				}))
				return srv.URL
			},
		},
		{
			Name: "Empty Response Failure",
			Config: Config{
				Method:              http.MethodGet,
				FailIfResponseEmpty: true,
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				return srv.URL
			},
		},
		{
			Name: "Expected Response Code Success",
			Config: Config{
				Method:               http.MethodGet,
				ExpectedResponseCode: "201",
			},
			Expected: map[string]interface{}{"status": "created"},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte(`{"status": "created"}`))
				}))
				return srv.URL
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			//
			h := New(c.Config)
			//configBytes, err := json.Marshal(c.Config)
			//require.NoError(t, err)
			url := c.serverSetup(t)
			config := c.Config
			config.URL = url

			conf, err := json.Marshal(config)
			require.NoError(t, err)

			resp, _, err := h.Execute(context.Background(), string(conf))
			if c.ShouldError {
				require.Error(t, err)
				if c.Name == "Expected Response Code Failure" || c.Name == "Empty Response Failure" || c.Name == "invalid response path" {
					assert.True(t, errors.Is(err, plan.ErrFailure), "Expected failure error to be wrapped with plan.ErrFailure")
				}
				return
			}
			require.NoError(t, err)

			assert.Equal(t, c.Expected, resp)
		})
	}
}

func TestHttp_Config(t *testing.T) {
	cases := []struct {
		Name     string
		Config   Config
		Expected string
	}{
		{
			Name: "Basic Config",
			Config: Config{
				URL:     "https://test.com",
				Method:  http.MethodGet,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`{"test": "value"}`),
			},
			Expected: `{"url":"https://test.com","method":"GET","headers":{"Content-Type":"application/json"},"body":{"test":"value"}, "responsePath": "", "expectedResponseCode": "", "failIfResponseEmpty": false}`,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			h := New(c.Config)
			result := h.Config()
			require.JSONEq(t, c.Expected, result)
		})
	}
}

func TestHTTPActionWithEscapeTemplateViaPlanExecute(t *testing.T) {
	var receivedBody map[string]interface{}
	serverCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true

		bod, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(bod, &receivedBody)
		require.NoError(t, err, "Server should receive valid JSON, got: %s", string(bod))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer srv.Close()

	// Build the config body template with escape function
	configBody := fmt.Sprintf(`{
  "message": "{{ escape .%scontent | js }}",
  "path": "{{ .%sfilepath }}",
	"array": [
]
}`, requestctx.BareVariablesPrefixStripped, requestctx.BareVariablesPrefixStripped)

	// Create API config with HTTP action
	apiCfg := apiconfig.APIConfig{
		Actions: map[string]apiconfig.Action{
			"test_http": {
				Name: "test_http",
				Type: "http",
				Config: map[string]interface{}{
					"url":     srv.URL,
					"method":  "POST",
					"headers": map[string]string{"Content-Type": "application/json"},
					"body":    configBody,
				},
				Next: "response.success",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Name: "success",
				Code: 200,
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"status": {Value: "ok"},
					},
				},
			},
		},
	}

	// Create planner and generate plan
	planner := plan.NewPlannerV2(plan.PlannerConfig{
		Actions:   apiCfg.Actions,
		Responses: apiCfg.Responses,
	}, logging.GetNewLogger())

	p, err := planner.Plan()
	require.NoError(t, err)

	// Set up request context with variables containing double quotes
	ctx := requestctx.NewTestContext()
	err = requestctx.AddRequestVariables(ctx, map[string]interface{}{
		fmt.Sprintf("%scontent", requestctx.BareVariablesPrefixStripped):  `This has "quoted" text`,
		fmt.Sprintf("%sfilepath", requestctx.BareVariablesPrefixStripped): "some/path",
	}, "")
	require.NoError(t, err)

	// Execute the plan
	_, err = p.Execute(ctx, requestctx.ActionConfigPrefix+"test_http", nil)
	require.NoError(t, err)

	// Verify the server was called
	assert.True(t, serverCalled, "Test server should have been called")

	// Verify the escaped content was received correctly
	assert.Equal(t, `This has "quoted" text`, receivedBody["message"], "Message should have properly escaped quotes")
	assert.Equal(t, "some/path", receivedBody["path"], "Path should be set correctly")
}
