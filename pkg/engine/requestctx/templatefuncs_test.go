package requestctx

import (
	"sync"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTemplateFuncs_ResolveSeesOverlay(t *testing.T) {
	ctx := NewTestContext()
	rc, ok := FromContext(ctx)
	require.True(t, ok)

	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"tool_param": func(key string) string { return "value-of-" + key },
	})

	result, err := rc.Resolve(ctx, `{{ tool_param "symbol" }}`)
	require.NoError(t, err)
	assert.Equal(t, "value-of-symbol", result)
}

func TestWithTemplateFuncs_ResolveBatchSeesOverlay(t *testing.T) {
	ctx := NewTestContext()
	rc, ok := FromContext(ctx)
	require.True(t, ok)

	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"tool_param": func(key string) string { return "arg:" + key },
	})

	// More than one template takes the concatenating batch path rather than
	// delegating to Resolve, so it needs its own coverage.
	results, err := rc.ResolveBatch(ctx, `{{ tool_param "a" }}`, `{{ tool_param "b" }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"arg:a", "arg:b"}, results)
}

func TestWithTemplateFuncs_CreateTextTemplateSeesOverlay(t *testing.T) {
	ctx := NewTestContext()
	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"tool_param": func(key string) string { return "arg:" + key },
	})

	// The v1 action config path: CreateTextTemplate, then execute.
	out, err := ExecuteTemplateString(ctx, `{{ tool_param "symbol" }}`)
	require.NoError(t, err)
	assert.Equal(t, "arg:symbol", out)
}

func TestWithTemplateFuncs_ResolvesAlongsideRequestFunctionsAndVariables(t *testing.T) {
	ctx := NewTestContext()
	require.NoError(t, AddRequestVariables(ctx, map[string]interface{}{
		"stored": "from-variables",
	}, ""))

	rc, ok := FromContext(ctx)
	require.True(t, ok)
	rc.AddRequestTemplateFunctions(template.FuncMap{
		"header": func(string) string { return "from-request-func" },
	}, false)

	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"tool_param": func(string) string { return "from-overlay" },
	})

	// An overlay must add to what the request already offers, never replace it.
	result, err := rc.Resolve(ctx,
		`{{ tool_param "x" }}|{{ header "y" }}|{{ .stored }}|{{ tostring "base" }}`)
	require.NoError(t, err)
	assert.Equal(t, "from-overlay|from-request-func|from-variables|base", result)
}

func TestWithTemplateFuncs_ExplicitFuncMapWinsOverOverlay(t *testing.T) {
	ctx := NewTestContext()
	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"pick": func() string { return "overlay" },
	})

	tmpl, err := CreateTextTemplate(ctx, `{{ pick }}`, template.FuncMap{
		"pick": func() string { return "caller" },
	})
	require.NoError(t, err)

	out, err := ExecuteTemplateFromContext(ctx, tmpl)
	require.NoError(t, err)
	assert.Equal(t, "caller", out)
}

func TestWithTemplateFuncs_OverlaysCompose(t *testing.T) {
	ctx := NewTestContext()
	rc, ok := FromContext(ctx)
	require.True(t, ok)

	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"outer": func() string { return "outer" },
		"both":  func() string { return "from-outer" },
	})
	ctx = WithTemplateFuncs(ctx, template.FuncMap{
		"inner": func() string { return "inner" },
		"both":  func() string { return "from-inner" },
	})

	result, err := rc.Resolve(ctx, `{{ outer }}|{{ inner }}|{{ both }}`)
	require.NoError(t, err)
	assert.Equal(t, "outer|inner|from-inner", result)
}

func TestWithTemplateFuncs_NoOverlayUnchanged(t *testing.T) {
	ctx := NewTestContext()
	require.NoError(t, AddRequestVariables(ctx, map[string]interface{}{
		"stored": "value",
	}, ""))
	rc, ok := FromContext(ctx)
	require.True(t, ok)

	result, err := rc.Resolve(ctx, `{{ .stored }}`)
	require.NoError(t, err)
	assert.Equal(t, "value", result)

	// An empty overlay must not allocate a new context either.
	assert.Equal(t, ctx, WithTemplateFuncs(ctx, nil))
}

func TestWithTemplateFuncs_UnknownFunctionStillFails(t *testing.T) {
	ctx := NewTestContext()
	rc, ok := FromContext(ctx)
	require.True(t, ok)

	// The overlay is scoped to the context that carries it: a sibling context
	// must not see it.
	_ = WithTemplateFuncs(ctx, template.FuncMap{"tool_param": func(string) string { return "x" }})

	_, err := rc.Resolve(ctx, `{{ tool_param "a" }}`)
	require.Error(t, err)
}

func TestWithTemplateFuncs_ConcurrentCallsAreIndependent(t *testing.T) {
	// The reason the overlay lives on the context: two calls sharing one
	// RequestContext must each see their own arguments. Registering per-call
	// functions on the RequestContext itself cannot do this.
	base := NewTestContext()
	rc, ok := FromContext(base)
	require.True(t, ok)

	var wg sync.WaitGroup
	results := make([]string, 50)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			want := string(rune('a' + i%26))
			ctx := WithTemplateFuncs(base, template.FuncMap{
				"tool_param": func(string) string { return want },
			})
			out, err := rc.Resolve(ctx, `{{ tool_param "k" }}`)
			if err != nil {
				results[i] = "error: " + err.Error()
				return
			}
			results[i] = out
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		assert.Equal(t, string(rune('a'+i%26)), got, "call %d saw another call's argument", i)
	}
}
