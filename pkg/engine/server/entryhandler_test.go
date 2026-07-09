package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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
