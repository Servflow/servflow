package outputs_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"text/template"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/outputs"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/responses"
)

// startRequest opens a request context the way a run entry point does, with the
// supplied template functions available to output templates.
func startRequest(t *testing.T, funcs template.FuncMap) (context.Context, *requestctx.RequestContext) {
	t.Helper()
	ctx, rc := requestctx.Start(context.Background(), requestctx.Options{
		TemplateFuncs: funcs,
	})
	t.Cleanup(rc.Done)
	return ctx, rc
}

func textMessage(role requestctx.RoleType, content string) requestctx.MessageTypeContent {
	return requestctx.MessageTypeContent{
		Message: requestctx.Message{Type: requestctx.MessageTypeText},
		Role:    role,
		Content: content,
	}
}

// extract resolves the handler config and runs it, failing the test on either
// step — the two are always paired in production (Resolve at config load,
// Extract at run end).
func extract(t *testing.T, ctx context.Context, cfg apiconfig.OutputConfig) responses.Result {
	t.Helper()
	ex, err := outputs.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve(%+v): %v", cfg, err)
	}
	if ex == nil {
		t.Fatalf("Resolve(%+v) returned no extractor", cfg)
	}
	result, err := ex.Extract(ctx)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return result
}

func wantText(t *testing.T, result responses.Result, want string) {
	t.Helper()
	text, ok := result.(outputs.TextResult)
	if !ok {
		t.Fatalf("result is %T, want outputs.TextResult", result)
	}
	if text.Text != want {
		t.Fatalf("text = %q, want %q", text.Text, want)
	}
	if text.Kind() != outputs.KindText {
		t.Fatalf("kind = %q, want %q", text.Kind(), outputs.KindText)
	}
}

func TestTemplateHandler_RendersAgainstRequestContext(t *testing.T) {
	ctx, _ := startRequest(t, template.FuncMap{
		"greeting": func() string { return "hello" },
	})

	result := extract(t, ctx, apiconfig.OutputConfig{
		Handler: outputs.HandlerTemplate,
		Config:  map[string]interface{}{"template": "{{ greeting }} world"},
	})
	wantText(t, result, "hello world")
}

// A template handler with no template configured is a config error, not a
// silently empty output: the whole point of naming the handler is to produce
// something.
func TestTemplateHandler_RequiresTemplate(t *testing.T) {
	for name, cfg := range map[string]map[string]interface{}{
		"missing":  {},
		"empty":    {"template": ""},
		"mistyped": {"template": 42},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := outputs.Resolve(apiconfig.OutputConfig{
				Handler: outputs.HandlerTemplate,
				Config:  cfg,
			}); err == nil {
				t.Fatal("expected an error resolving a template handler without a usable template")
			}
		})
	}
}

// The conversation handler returns the agent's answer — the last assistant
// message — not whatever happens to be last in the thread. An agent run
// typically ends with tool traffic appended after the reply.
func TestConversationHandler_LastAssistantMessageSkippingToolTraffic(t *testing.T) {
	ctx, rc := startRequest(t, nil)
	rc.Conversation().Append(
		textMessage(requestctx.RoleTypeUser, "what is the status?"),
		textMessage(requestctx.RoleTypeAssistant, "first pass"),
		requestctx.MessageToolCall{
			Message: requestctx.Message{Type: requestctx.MessageTypeToolCall},
			ID:      "call-1",
			Name:    "lookup",
		},
		textMessage(requestctx.RoleTypeAssistant, "all green"),
		requestctx.MessageToolCallResponse{
			Message:          requestctx.Message{Type: requestctx.MessageTypeToolResponse},
			ToolResponseType: requestctx.ToolResponseTypeText,
			ID:               "call-1",
			Text:             "raw tool output",
		},
	)

	result := extract(t, ctx, apiconfig.OutputConfig{Handler: outputs.HandlerConversation})
	wantText(t, result, "all green")
}

func TestConversationHandler_Role(t *testing.T) {
	ctx, rc := startRequest(t, nil)
	rc.Conversation().Append(
		textMessage(requestctx.RoleTypeUser, "the question"),
		textMessage(requestctx.RoleTypeAssistant, "the answer"),
	)

	t.Run("explicit user role", func(t *testing.T) {
		result := extract(t, ctx, apiconfig.OutputConfig{
			Handler: outputs.HandlerConversation,
			Config:  map[string]interface{}{"role": "user"},
		})
		wantText(t, result, "the question")
	})

	t.Run("any role takes the last message", func(t *testing.T) {
		result := extract(t, ctx, apiconfig.OutputConfig{
			Handler: outputs.HandlerConversation,
			Config:  map[string]interface{}{"role": "any"},
		})
		wantText(t, result, "the answer")
	})

	t.Run("unknown role is a config error", func(t *testing.T) {
		if _, err := outputs.Resolve(apiconfig.OutputConfig{
			Handler: outputs.HandlerConversation,
			Config:  map[string]interface{}{"role": "robot"},
		}); err == nil {
			t.Fatal("expected an error for an unknown role")
		}
	})
}

// A run whose agent never spoke has nothing to return. That is an empty output,
// not a failure — the caller decides what an absent result means.
func TestConversationHandler_NoMatchingMessageYieldsNoResult(t *testing.T) {
	ctx, rc := startRequest(t, nil)
	rc.Conversation().Append(textMessage(requestctx.RoleTypeUser, "only the user spoke"))

	ex, err := outputs.Resolve(apiconfig.OutputConfig{Handler: outputs.HandlerConversation})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	result, err := ex.Extract(ctx)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

// The thread carries model and tool output that no config author wrote, so a
// secret that leaked into it must not reach the caller verbatim. This is the
// scrub gateway at the exit boundary.
func TestConversationHandler_ScrubsResolvedSecrets(t *testing.T) {
	const secretValue = "sk-live-9f2b41cc"
	t.Setenv("OUTPUT_TEST_TOKEN", secretValue)

	ctx, rc := startRequest(t, nil)
	// Resolving the secret through a template is what tracks it for the request,
	// exactly as an action config referencing it would.
	if _, err := requestctx.ExecuteTemplateString(ctx, `{{ secret "OUTPUT_TEST_TOKEN" }}`); err != nil {
		t.Fatalf("resolving secret: %v", err)
	}
	if !rc.HasSecrets() {
		t.Fatal("secret was not tracked; the test cannot prove scrubbing")
	}
	rc.Conversation().Append(textMessage(requestctx.RoleTypeAssistant, "the token is "+secretValue))

	result := extract(t, ctx, apiconfig.OutputConfig{Handler: outputs.HandlerConversation})
	text, ok := result.(outputs.TextResult)
	if !ok {
		t.Fatalf("result is %T, want outputs.TextResult", result)
	}
	if strings.Contains(text.Text, secretValue) {
		t.Fatalf("extracted output leaked the secret value: %q", text.Text)
	}
	if !strings.Contains(text.Text, "OUTPUT_TEST_TOKEN") {
		t.Fatalf("expected the scrub marker naming the secret, got %q", text.Text)
	}
}

// An explicit template is author intent: a workflow that deliberately returns a
// credential to its caller keeps working. Contrast the conversation handler.
func TestTemplateHandler_DoesNotScrub(t *testing.T) {
	const secretValue = "sk-live-template-77b1"
	t.Setenv("OUTPUT_TEST_TEMPLATE_TOKEN", secretValue)

	ctx, _ := startRequest(t, nil)
	result := extract(t, ctx, apiconfig.OutputConfig{
		Handler: outputs.HandlerTemplate,
		Config:  map[string]interface{}{"template": `{{ secret "OUTPUT_TEST_TEMPLATE_TOKEN" }}`},
	})
	wantText(t, result, secretValue)
}

func TestFileHandler(t *testing.T) {
	ctx, rc := startRequest(t, nil)
	rc.AddActionFile("report", requestctx.NewFileValue(io.NopCloser(strings.NewReader("file body")), "report.txt"))

	result := extract(t, ctx, apiconfig.OutputConfig{
		Handler: outputs.HandlerFile,
		Config:  map[string]interface{}{"identifier": "report"},
	})
	file, ok := result.(outputs.FileResult)
	if !ok {
		t.Fatalf("result is %T, want outputs.FileResult", result)
	}
	if file.Kind() != outputs.KindFile {
		t.Fatalf("kind = %q, want %q", file.Kind(), outputs.KindFile)
	}
	content, err := file.File.GetContent()
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if string(content) != "file body" {
		t.Fatalf("content = %q, want %q", content, "file body")
	}
}

func TestFileHandler_RequiresIdentifier(t *testing.T) {
	if _, err := outputs.Resolve(apiconfig.OutputConfig{
		Handler: outputs.HandlerFile,
		Config:  map[string]interface{}{},
	}); err == nil {
		t.Fatal("expected an error resolving a file handler without an identifier")
	}
}

func TestNoneHandler(t *testing.T) {
	ctx, _ := startRequest(t, nil)
	ex, err := outputs.Resolve(apiconfig.OutputConfig{Handler: outputs.HandlerNone})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	result, err := ex.Extract(ctx)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestResolve(t *testing.T) {
	t.Run("no handler yields no extractor and no error", func(t *testing.T) {
		ex, err := outputs.Resolve(apiconfig.OutputConfig{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if ex != nil {
			t.Fatalf("extractor = %#v, want nil", ex)
		}
	})

	t.Run("unknown handler is an error", func(t *testing.T) {
		_, err := outputs.Resolve(apiconfig.OutputConfig{Handler: "nope"})
		if err == nil {
			t.Fatal("expected an error for an unregistered handler")
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Fatalf("error %q should name the unknown handler", err)
		}
	})
}

// Finalize is the rule every run entry point shares.
func TestFinalize(t *testing.T) {
	ctx, _ := startRequest(t, nil)
	extractorResult := outputs.TextResult{Text: "from handler"}
	ex := outputs.ExtractorFunc(func(context.Context) (responses.Result, error) {
		return extractorResult, nil
	})

	t.Run("a response step result wins over the handler", func(t *testing.T) {
		planResult := outputs.TextResult{Text: "from response step"}
		got, err := outputs.Finalize(ctx, planResult, ex)
		if err != nil {
			t.Fatalf("Finalize: %v", err)
		}
		wantText(t, got, "from response step")
	})

	t.Run("the handler runs when the plan produced nothing", func(t *testing.T) {
		got, err := outputs.Finalize(ctx, nil, ex)
		if err != nil {
			t.Fatalf("Finalize: %v", err)
		}
		wantText(t, got, "from handler")
	})

	t.Run("no plan result and no handler yields nothing", func(t *testing.T) {
		got, err := outputs.Finalize(ctx, nil, nil)
		if err != nil {
			t.Fatalf("Finalize: %v", err)
		}
		if got != nil {
			t.Fatalf("result = %#v, want nil", got)
		}
	})

	t.Run("a handler error propagates", func(t *testing.T) {
		sentinel := errors.New("boom")
		failing := outputs.ExtractorFunc(func(context.Context) (responses.Result, error) {
			return nil, sentinel
		})
		if _, err := outputs.Finalize(ctx, nil, failing); !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic registering a duplicate handler kind")
		}
	}()
	outputs.Register(outputs.HandlerTemplate, func(map[string]interface{}) (outputs.Extractor, error) {
		return nil, nil
	})
}

func TestRegisteredTypes_IncludesBuiltins(t *testing.T) {
	got := outputs.RegisteredTypes()
	for _, want := range []string{
		outputs.HandlerConversation, outputs.HandlerFile,
		outputs.HandlerNone, outputs.HandlerTemplate,
	} {
		if !outputs.Has(want) {
			t.Errorf("built-in handler %q is not registered (registered: %v)", want, got)
		}
	}
}
