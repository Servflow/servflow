package actions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"text/template"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runnerV1 records the config it was handed so a test can assert on what the
// runner resolved before calling it.
type runnerV1 struct {
	config     string
	gotConfig  string
	response   interface{}
	fields     map[string]string
	executeErr error
}

func (m *runnerV1) Type() string   { return "runner-v1" }
func (m *runnerV1) Config() string { return m.config }
func (m *runnerV1) Execute(_ context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	m.gotConfig = modifiedConfig
	return m.response, m.fields, m.executeErr
}

// runnerV2 resolves its own config, as a real v2 action does.
type runnerV2 struct {
	config     string
	resolved   string
	response   interface{}
	fields     map[string]string
	executeErr error
}

func (m *runnerV2) Type() string { return "runner-v2" }
func (m *runnerV2) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	if m.executeErr != nil {
		return nil, nil, m.executeErr
	}
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, err
	}
	m.resolved, err = rc.Resolve(ctx, m.config)
	if err != nil {
		return nil, nil, err
	}
	return m.response, m.fields, nil
}

// registerRunnerV1 and registerRunnerV2 register an action type for one test
// and return its name. Registration is global and permanent, so every name is
// unique, and the executable is fixed by the test rather than built from the
// config.
func registerRunnerV1(t *testing.T, name string, exec *runnerV1) string {
	t.Helper()
	require.NoError(t, RegisterAction(name, ActionRegistrationInfo{
		Constructor: func(json.RawMessage) (ActionExecutable, error) {
			return exec, nil
		},
		Fields: map[string]FieldInfo{},
	}))
	return name
}

func registerRunnerV2(t *testing.T, name string, exec *runnerV2) string {
	t.Helper()
	require.NoError(t, RegisterAction(name, ActionRegistrationInfo{
		UseV2: true,
		ConstructorV2: func(json.RawMessage) (ActionExecutableV2, error) {
			return exec, nil
		},
		Fields: map[string]FieldInfo{},
	}))
	return name
}

func TestNewRunner_UnregisteredType(t *testing.T) {
	_, err := NewRunner("runner-no-such-action", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner-no-such-action")
}

func TestNewRunner_ConstructorError(t *testing.T) {
	name := "runner-bad-constructor"
	require.NoError(t, RegisterAction(name, ActionRegistrationInfo{
		Constructor: func(json.RawMessage) (ActionExecutable, error) {
			return nil, errors.New("bad config")
		},
		Fields: map[string]FieldInfo{},
	}))

	// A config the action rejects fails while the runner is built, not at the
	// first call.
	_, err := NewRunner(name, json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad config")
}

func TestRunner_V1ReceivesResolvedConfig(t *testing.T) {
	exec := &runnerV1{
		config:   `{"symbol":"{{ .stored }}"}`,
		response: "ok",
	}
	name := registerRunnerV1(t, "runner-v1-resolves", exec)

	runner, err := NewRunner(name, json.RawMessage(`{"symbol":"{{ .stored }}"}`))
	require.NoError(t, err)
	assert.Equal(t, name, runner.Type())

	ctx := requestctx.NewTestContext()
	require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]interface{}{
		"stored": "BTCUSDT",
	}, ""))

	resp, _, err := runner.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, `{"symbol":"BTCUSDT"}`, exec.gotConfig,
		"v1 gets its config template rendered, as the plan executor does it")
}

func TestRunner_V1SeesContextOverlay(t *testing.T) {
	exec := &runnerV1{config: `{"symbol":"{{ tool_param \"symbol\" }}"}`}
	name := registerRunnerV1(t, "runner-v1-overlay", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	ctx := requestctx.WithTemplateFuncs(requestctx.NewTestContext(), template.FuncMap{
		"tool_param": func(string) string { return "ETHUSDT" },
	})

	_, _, err = runner.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, `{"symbol":"ETHUSDT"}`, exec.gotConfig)
}

func TestRunner_V1EmptyConfigIsNotResolved(t *testing.T) {
	exec := &runnerV1{config: ""}
	name := registerRunnerV1(t, "runner-v1-empty", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	_, _, err = runner.Execute(requestctx.NewTestContext())
	require.NoError(t, err)
	assert.Empty(t, exec.gotConfig)
}

func TestRunner_V1ResolutionErrorIsReported(t *testing.T) {
	exec := &runnerV1{config: `{{ notAFunction }}`}
	name := registerRunnerV1(t, "runner-v1-bad-template", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	_, _, err = runner.Execute(requestctx.NewTestContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving config")
	assert.Empty(t, exec.gotConfig, "a config that fails to resolve never reaches the action")
}

func TestRunner_V2ResolvesItsOwnConfig(t *testing.T) {
	exec := &runnerV2{
		config:   `{{ .stored }}`,
		response: map[string]any{"balance": 10},
	}
	name := registerRunnerV2(t, "runner-v2-resolves", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	ctx := requestctx.NewTestContext()
	require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]interface{}{
		"stored": "from-variables",
	}, ""))

	resp, _, err := runner.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"balance": 10}, resp)
	assert.Equal(t, "from-variables", exec.resolved)
}

func TestRunner_V2SeesContextOverlay(t *testing.T) {
	exec := &runnerV2{config: `{{ tool_param "symbol" }}`}
	name := registerRunnerV2(t, "runner-v2-overlay", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	// The v2 action resolves for itself, so the overlay has to survive being
	// passed through the runner untouched.
	ctx := requestctx.WithTemplateFuncs(requestctx.NewTestContext(), template.FuncMap{
		"tool_param": func(key string) string { return "arg-" + key },
	})

	_, _, err = runner.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "arg-symbol", exec.resolved)
}

func TestRunner_ExecutionErrorPassesThrough(t *testing.T) {
	sentinel := errors.New("action failed")

	v1 := &runnerV1{config: `{}`, executeErr: sentinel}
	v1Name := registerRunnerV1(t, "runner-v1-error", v1)
	v1Runner, err := NewRunner(v1Name, json.RawMessage(`{}`))
	require.NoError(t, err)
	_, _, err = v1Runner.Execute(requestctx.NewTestContext())
	assert.ErrorIs(t, err, sentinel)

	v2 := &runnerV2{config: `{}`, executeErr: sentinel}
	v2Name := registerRunnerV2(t, "runner-v2-error", v2)
	v2Runner, err := NewRunner(v2Name, json.RawMessage(`{}`))
	require.NoError(t, err)
	_, _, err = v2Runner.Execute(requestctx.NewTestContext())
	assert.ErrorIs(t, err, sentinel)
}

func TestRunner_ReturnsTraceFields(t *testing.T) {
	exec := &runnerV1{config: `{}`, fields: map[string]string{"url": "https://example.com"}}
	name := registerRunnerV1(t, "runner-v1-fields", exec)

	runner, err := NewRunner(name, json.RawMessage(`{}`))
	require.NoError(t, err)

	_, fields, err := runner.Execute(requestctx.NewTestContext())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"url": "https://example.com"}, fields)
}

func TestRunner_ExecutableIsTheConstructedAction(t *testing.T) {
	v1 := &runnerV1{}
	r1, err := NewRunner(registerRunnerV1(t, "runner-exec-v1", v1), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Same(t, v1, r1.Executable())

	v2 := &runnerV2{}
	r2, err := NewRunner(registerRunnerV2(t, "runner-exec-v2", v2), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Same(t, v2, r2.Executable())
}
