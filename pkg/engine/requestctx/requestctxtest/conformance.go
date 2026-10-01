package requestctxtest

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Conformance checks the behavior actions rely on from a RequestContext.
// newContext must return a context carrying a fresh RequestContext with no
// variables, files, workspace, request, or secrets.
func Conformance(t *testing.T, newContext func() context.Context) {
	t.Helper()

	t.Run("variables are stored under their prefix", func(t *testing.T) {
		ctx := newContext()
		require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]any{"name": "Ada"}, requestctx.BareVariablesPrefixStripped))

		got, err := requestctx.GetRequestVariable(ctx, "variable_name")
		require.NoError(t, err)
		assert.Equal(t, "Ada", got)

		all, err := requestctx.GetAllRequestVariables(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Ada", all["variable_name"])
	})

	t.Run("resolve renders variables", func(t *testing.T) {
		ctx := newContext()
		require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]any{"name": "Ada"}, requestctx.BareVariablesPrefixStripped))
		rc := mustRC(t, ctx)

		got, err := rc.Resolve(ctx, "hello {{ .variable_name }}")
		require.NoError(t, err)
		assert.Equal(t, "hello Ada", got)
	})

	t.Run("a missing variable renders empty", func(t *testing.T) {
		ctx := newContext()
		got, err := mustRC(t, ctx).Resolve(ctx, "[{{ .variable_missing }}]")
		require.NoError(t, err)
		assert.Equal(t, "[]", got)
	})

	t.Run("an action variable can drop its prefix", func(t *testing.T) {
		ctx := newContext()
		require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]any{"fetch": "result"}, ""))
		got, err := mustRC(t, ctx).Resolve(ctx, "{{ .variable_actions_fetch }}")
		require.NoError(t, err)
		assert.Equal(t, "result", got)
	})

	t.Run("escaped quotes inside an action are unescaped", func(t *testing.T) {
		ctx := newContext()
		got, err := mustRC(t, ctx).Resolve(ctx, `{{ printf \"%s\" \"quoted\" }}`)
		require.NoError(t, err)
		assert.Equal(t, "quoted", got)
	})

	t.Run("resolve batch keeps order and count", func(t *testing.T) {
		ctx := newContext()
		require.NoError(t, requestctx.AddRequestVariables(ctx, map[string]any{"a": "1", "b": "2"}, requestctx.BareVariablesPrefixStripped))
		got, err := mustRC(t, ctx).ResolveBatch(ctx, "{{ .variable_b }}", "plain", "{{ .variable_a }}", "")
		require.NoError(t, err)
		assert.Equal(t, []string{"2", "plain", "1", ""}, got)
	})

	t.Run("an invalid template is an error", func(t *testing.T) {
		ctx := newContext()
		_, err := mustRC(t, ctx).Resolve(ctx, "{{ .variable_a ")
		assert.Error(t, err)
	})

	t.Run("files are found by their file input", func(t *testing.T) {
		ctx := newContext()
		rc := mustRC(t, ctx)
		requestctx.AddRequestFile(rc, "upload", requestctx.NewFileValue(io.NopCloser(strings.NewReader("req")), "a.txt"))
		requestctx.AddActionFile(rc, "render", requestctx.NewFileValue(io.NopCloser(strings.NewReader("act")), "b.txt"))

		f, err := requestctx.GetFileFromContext(ctx, apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "upload"})
		require.NoError(t, err)
		assert.Equal(t, "a.txt", f.Name)

		f, err = requestctx.GetFileFromContext(ctx, apiconfig.FileInput{Type: apiconfig.FileInputTypeAction, Identifier: "action.render"})
		require.NoError(t, err)
		assert.Equal(t, "b.txt", f.Name)

		_, err = requestctx.GetFileFromContext(ctx, apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "missing"})
		assert.ErrorIs(t, err, requestctx.ErrFileNotFound)
	})

	t.Run("a fresh request has no workspace, request, or secrets", func(t *testing.T) {
		ctx := newContext()
		rc := mustRC(t, ctx)

		_, err := requestctx.WorkspaceFromContext(ctx)
		assert.ErrorIs(t, err, requestctx.ErrNoWorkspace)
		assert.Nil(t, rc.Request())
		assert.False(t, rc.HasSecrets())
		assert.Equal(t, "nothing secret", rc.Scrub("nothing secret"))
	})
}

func mustRC(t *testing.T, ctx context.Context) requestctx.RequestContext {
	t.Helper()
	rc, err := requestctx.FromContextOrError(ctx)
	require.NoError(t, err)
	return rc
}
