package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/entryhandlers"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
)

// baseHTTPConfig returns a minimal HTTP workflow config: a stub action feeding a
// template response that echoes an injected request variable.
func baseHTTPConfig(id, handler string, handlerConfig map[string]interface{}) *apiconfig.APIConfig {
	return &apiconfig.APIConfig{
		ID: id,
		HttpConfig: apiconfig.HttpConfig{
			ListenPath:    "/hook",
			Method:        "POST",
			Next:          "action.greet",
			Handler:       handler,
			HandlerConfig: handlerConfig,
		},
		Actions: map[string]apiconfig.Action{
			"greet": {
				Name: "greet",
				Type: "stub",
				Next: "response.ok",
				Config: map[string]interface{}{
					"message": "hello",
				},
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"ok": {
				Name:     "ok",
				Code:     200,
				Type:     "template",
				Template: `{"injected":"{{ .injected }}"}`,
			},
		},
	}
}

func TestEntryHandler_RejectShortCircuitsPlan(t *testing.T) {
	entryhandlers.Register("test_reject", func(_ map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "denied", http.StatusUnauthorized)
			// intentionally does not call next
		})
	})

	config := baseHTTPConfig("reject-cfg", "test_reject", nil)
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	// The body must be exactly the handler's rejection. If the plan had run,
	// planRunner would have written its template response ({"injected":""}),
	// which would appear here — so an exact-match on "denied\n" proves the plan
	// was short-circuited, not merely that the status is 401.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "denied\n", w.Body.String())
}

func TestEntryHandler_PassThroughInjectsVariable(t *testing.T) {
	entryhandlers.Register("test_inject", func(_ map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = requestctx.AddRequestVariables(r.Context(), map[string]interface{}{
				"injected": "from-handler",
			}, "")
			next.ServeHTTP(w, r)
		})
	})

	config := baseHTTPConfig("inject-cfg", "test_inject", nil)
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"injected":"from-handler"}`, w.Body.String())
}

func TestEntryHandler_NoHandlerConfiguredIsUnchanged(t *testing.T) {
	config := baseHTTPConfig("plain-cfg", "", nil)
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// injected var absent -> empty
	assert.JSONEq(t, `{"injected":""}`, w.Body.String())
}

func TestEntryHandler_UnknownHandlerIsServerError(t *testing.T) {
	config := baseHTTPConfig("unknown-cfg", "does_not_exist", nil)
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEntryHandler_ConfigAvailableToMiddleware(t *testing.T) {
	var gotSecret string
	entryhandlers.Register("test_readconfig", func(config map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s, ok := config["secret"].(string); ok {
				gotSecret = s
			}
			next.ServeHTTP(w, r)
		})
	})

	config := baseHTTPConfig("readconfig-cfg", "test_readconfig", map[string]interface{}{"secret": "abc123"})
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, "abc123", gotSecret)
}

// TestEntryHandler_BodyReadableAfterHandlerConsumesIt is the regression guard
// for the aliasing bug where the `body` template function and the entry-handler
// middleware operated on different *http.Request copies. A handler that reads
// and restores the request body (as github_webhook does for HMAC verification)
// reassigns Body on the request it is served; if the `body` function was bound
// to a different copy, it read the drained original reader and every
// `body "..."` rendered empty. Here the handler drains+restores the body and
// the plan reads a field from it via `body "action"` — it must see the real
// value, not "".
func TestEntryHandler_BodyReadableAfterHandlerConsumesIt(t *testing.T) {
	entryhandlers.Register("test_drainbody", func(_ map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mirror github_webhook: fully read the raw body, then restore it.
			_ = requestctx.ReadAndRestoreBody(r)
			next.ServeHTTP(w, r)
		})
	})

	config := &apiconfig.APIConfig{
		ID: "drainbody-cfg",
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/hook",
			Method:     "POST",
			Next:       "action.greet",
			Handler:    "test_drainbody",
		},
		Actions: map[string]apiconfig.Action{
			"greet": {
				Name:   "greet",
				Type:   "stub",
				Next:   "response.ok",
				Config: map[string]interface{}{"message": "hello"},
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"ok": {
				Name:     "ok",
				Code:     200,
				Type:     "template",
				Template: `{"action":"{{ body "action" }}"}`,
			},
		},
	}
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook",
		strings.NewReader(`{"action":"synchronize"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Before the fix this rendered {"action":""} because the handler drained a
	// different request copy than the `body` function read.
	assert.JSONEq(t, `{"action":"synchronize"}`, w.Body.String())
}

func TestEntryHandler_ConfigTemplatesResolvedByEngine(t *testing.T) {
	var gotValue string
	entryhandlers.Register("test_resolveconfig", func(config map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The engine must have resolved the template before we see it.
			gotValue, _ = config["value"].(string)
			next.ServeHTTP(w, r)
		})
	})

	// The config value is a template referencing a request header; the engine
	// resolves it against the request context before handing it to the handler.
	config := baseHTTPConfig("resolveconfig-cfg", "test_resolveconfig", map[string]interface{}{
		"value": `{{ header "X-Test" }}`,
	})
	runner := NewTestRunner(t, config).Init()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	req.Header.Set("X-Test", "resolved-value")
	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, req)

	assert.Equal(t, "resolved-value", gotValue)
}
