package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sfhttp "github.com/Servflow/servflow/internal/http"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/entryhandlers"
	"github.com/Servflow/servflow/pkg/engine/outputs"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// outputConfig builds an HTTP workflow whose chain ends WITHOUT a response step,
// so its response can only come from the output handler.
func outputConfig(id string, output apiconfig.OutputConfig, entryHandler string) *apiconfig.APIConfig {
	return &apiconfig.APIConfig{
		ID: id,
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/run",
			Method:     "POST",
			Next:       "action.greet",
			Handler:    entryHandler,
		},
		Actions: map[string]apiconfig.Action{
			"greet": {
				Name:   "greet",
				Type:   "stub",
				Config: map[string]interface{}{"message": "hello"},
			},
		},
		Output: output,
	}
}

// An HTTP workflow with an output handler needs no response step at all: this is
// the shape a studio agent workflow lowers to.
func TestHTTPOutputHandler_TemplateWithoutResponseStep(t *testing.T) {
	config := outputConfig("output-template", apiconfig.OutputConfig{
		Handler: outputs.HandlerTemplate,
		Config:  map[string]interface{}{"template": "{{ .greet.message }}"},
	}, "")
	runner := NewTestRunner(t, config).Init()

	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/run", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

// The studio default end-to-end: the run's output is the agent's last message,
// read off the shared conversation thread, with no response step and no result
// expression anywhere in the config.
func TestHTTPOutputHandler_ConversationWithoutResponseStep(t *testing.T) {
	entryhandlers.Register("test_output_speaks", func(_ map[string]interface{}, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc, ok := requestctx.FromContext(r.Context())
			require.True(t, ok)
			rc.Conversation().Append(requestctx.MessageTypeContent{
				Message: requestctx.Message{Type: requestctx.MessageTypeText},
				Role:    requestctx.RoleTypeAssistant,
				Content: "the agent's answer",
			})
			next.ServeHTTP(w, r)
		})
	})

	config := outputConfig("output-conversation", apiconfig.OutputConfig{
		Handler: outputs.HandlerConversation,
	}, "test_output_speaks")
	runner := NewTestRunner(t, config).Init()

	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/run", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "the agent's answer", w.Body.String())
}

// A response step is an explicit terminus and still wins: configuring both is
// not a behaviour change for existing workflows.
func TestHTTPOutputHandler_ResponseStepWins(t *testing.T) {
	config := outputConfig("output-response-wins", apiconfig.OutputConfig{
		Handler: outputs.HandlerTemplate,
		Config:  map[string]interface{}{"template": "from the output handler"},
	}, "")
	config.Actions["greet"] = apiconfig.Action{
		Name:   "greet",
		Type:   "stub",
		Next:   "response.ok",
		Config: map[string]interface{}{"message": "hello"},
	}
	config.Responses = map[string]apiconfig.ResponseConfig{
		"ok": {Name: "ok", Code: 201, Type: "template", Template: "from the response step"},
	}
	runner := NewTestRunner(t, config).Init()

	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/run", nil))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "from the response step", w.Body.String())
}

// Without either a response step or an output handler the request is still a
// 500 — unchanged from before output handlers existed.
func TestHTTPOutputHandler_NoOutputIsStillAnError(t *testing.T) {
	runner := NewTestRunner(t, outputConfig("output-none", apiconfig.OutputConfig{}, "")).Init()

	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/run", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHTTPOutputHandler_UnknownHandlerFailsAtLoad(t *testing.T) {
	eng := Engine{}
	_, err := eng.createBasicHandler(outputConfig("output-bad", apiconfig.OutputConfig{Handler: "nope"}, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestResolveOutput(t *testing.T) {
	t.Run("explicit output handler wins over the deprecated mcp result", func(t *testing.T) {
		ex, err := resolveOutput(&apiconfig.APIConfig{
			McpTool: apiconfig.MCPToolConfig{Result: "legacy"},
			Output: apiconfig.OutputConfig{
				Handler: outputs.HandlerConversation,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, ex)

		ctx, rc := requestctx.Start(context.Background(), requestctx.Options{})
		defer rc.Done()
		rc.Conversation().Append(requestctx.MessageTypeContent{
			Message: requestctx.Message{Type: requestctx.MessageTypeText},
			Role:    requestctx.RoleTypeAssistant,
			Content: "from the thread",
		})
		result, err := ex.Extract(ctx)
		require.NoError(t, err)
		assert.Equal(t, outputs.TextResult{Text: "from the thread"}, result)
	})

	// The deprecated field keeps working untouched: no stored config has to
	// change for this refactor.
	t.Run("deprecated mcp result becomes a template handler", func(t *testing.T) {
		ex, err := resolveOutput(&apiconfig.APIConfig{
			McpTool: apiconfig.MCPToolConfig{Result: "legacy result"},
		})
		require.NoError(t, err)
		require.NotNil(t, ex)

		ctx, rc := requestctx.Start(context.Background(), requestctx.Options{})
		defer rc.Done()
		result, err := ex.Extract(ctx)
		require.NoError(t, err)
		assert.Equal(t, outputs.TextResult{Text: "legacy result"}, result)
	})

	t.Run("no output configured yields no extractor", func(t *testing.T) {
		ex, err := resolveOutput(&apiconfig.APIConfig{})
		require.NoError(t, err)
		assert.Nil(t, ex)
	})
}

func TestHTTPResponseFor(t *testing.T) {
	t.Run("an http response passes through", func(t *testing.T) {
		want := &sfhttp.SfResponse{Code: 204}
		got, err := httpResponseFor(want)
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("no result is a missing-response error", func(t *testing.T) {
		_, err := httpResponseFor(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response missing")
	})

	// A typed nil must not slip past the type switch and reach the writer.
	t.Run("a nil http response is a missing-response error", func(t *testing.T) {
		var typedNil *sfhttp.SfResponse
		_, err := httpResponseFor(typedNil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response missing")
	})

	t.Run("a non-http result names the type", func(t *testing.T) {
		_, err := httpResponseFor(outputs.FileResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outputs.FileResult")
	})
}
