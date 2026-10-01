package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/secrets"

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
			url := c.serverSetup(t)
			config := c.Config
			config.URL = url

			// V2: the action reads its parsed config and resolves fields against
			// the request context, so no config string is passed to Execute.
			h := New(config)
			ctx := requestctx.NewTestContext()

			resp, _, err := h.Execute(ctx)
			if c.ShouldError {
				require.Error(t, err)
				if c.Name == "Expected Response Code Failure" || c.Name == "Empty Response Failure" || c.Name == "invalid response path" {
					assert.True(t, errors.Is(err, actions.ErrFailure), "Expected failure error to be wrapped with actions.ErrFailure")
				}
				return
			}
			require.NoError(t, err)

			assert.Equal(t, c.Expected, resp)
		})
	}
}

func TestHeaderPairing(t *testing.T) {
	names := []string{"one", "two", "three", "four", "five", "six"}

	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	headers := map[string]string{}
	vars := map[string]interface{}{}
	want := map[string]string{}
	for _, n := range names {
		headers[`X-{{ .`+requestctx.BareVariablesPrefixStripped+`k_`+n+` }}`] = `{{ .` + requestctx.BareVariablesPrefixStripped + `v_` + n + ` }}`
		vars[requestctx.BareVariablesPrefixStripped+"k_"+n] = n
		vars[requestctx.BareVariablesPrefixStripped+"v_"+n] = n
		want["X-"+n] = n
	}

	h := New(Config{URL: srv.URL, Method: "GET", Headers: headers})
	ctx := requestctx.NewTestContext()
	require.NoError(t, requestctx.AddRequestVariables(ctx, vars, ""))

	_, _, err := h.Execute(ctx)
	require.NoError(t, err)

	for k, v := range want {
		assert.Equal(t, v, got.Get(k), "header %s mispaired", k)
	}
}

// TestHTTPActionSecretsOnWireTrackedForScrubbing is the end-to-end check for
// the scrub-gateway secret model: the outbound request (URL query, header,
// body) carries the REAL secret value, and from the moment of resolution the
// request context tracks it so every context-derived logger/span scrubs it.
func TestHTTPActionSecretsOnWireTrackedForScrubbing(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("HTTP_TEST_TOKEN", "realsecrettoken")

	var gotAuth, gotBody, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query().Get("token")
		bod, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(bod)
		w.Write([]byte(`{"echo": "realsecrettoken"}`))
	}))
	defer srv.Close()

	cfg := Config{
		Method:  http.MethodPost,
		URL:     srv.URL + `?token={{ secret "HTTP_TEST_TOKEN" }}`,
		Headers: map[string]string{"Authorization": `Bearer {{ secret "HTTP_TEST_TOKEN" }}`},
		Body:    json.RawMessage(`"body:{{ secret \"HTTP_TEST_TOKEN\" }}"`),
	}

	rc := requestctx.NewRequestContext("secret-egress-test")
	ctx := requestctx.WithAggregationContext(context.Background(), rc)

	resp, _, err := New(cfg).Execute(ctx)
	require.NoError(t, err)

	// The wire got the real value everywhere.
	assert.Equal(t, "Bearer realsecrettoken", gotAuth)
	assert.Equal(t, "realsecrettoken", gotQuery)
	assert.Equal(t, "body:realsecrettoken", gotBody)

	// The value was tracked at resolution time: scrubbers mask it wherever it
	// surfaces (logs, spans, stored outputs — the plan runner scrubs resp).
	assert.True(t, rc.HasSecrets())
	scrubbed := rc.Scrub("log line with realsecrettoken inside")
	assert.NotContains(t, scrubbed, "realsecrettoken")

	// The response echoing the token comes back to the caller un-scrubbed here
	// (the PLAN runner scrubs before storing); sanity-check shape only.
	require.IsType(t, map[string]interface{}{}, resp)
}
