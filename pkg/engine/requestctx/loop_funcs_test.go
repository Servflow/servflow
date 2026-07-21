package requestctx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoopTemplateFuncs(t *testing.T) {
	t.Run("outside a loop returns empty item and zero index", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)

		item, err := rc.Resolve(ctx, "{{ loop_item }}")
		require.NoError(t, err)
		assert.Equal(t, "", item)

		idx, err := rc.Resolve(ctx, "{{ loop_index }}")
		require.NoError(t, err)
		assert.Equal(t, "0", idx)
	})

	t.Run("scalar element and index", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)

		rc.PushLoop("hello", 3)
		defer rc.PopLoop()

		item, err := rc.Resolve(ctx, "{{ loop_item }}")
		require.NoError(t, err)
		assert.Equal(t, "hello", item)

		idx, err := rc.Resolve(ctx, "{{ loop_index }}")
		require.NoError(t, err)
		assert.Equal(t, "3", idx)
	})

	t.Run("object field access", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)

		rc.PushLoop(map[string]interface{}{"id": "42", "name": "bot"}, 0)
		defer rc.PopLoop()

		id, err := rc.Resolve(ctx, `{{ loop_item "id" }}`)
		require.NoError(t, err)
		assert.Equal(t, "42", id)

		name, err := rc.Resolve(ctx, `{{ loop_item "name" }}`)
		require.NoError(t, err)
		assert.Equal(t, "bot", name)
	})

	t.Run("field access on a non-object element yields empty", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)

		rc.PushLoop("scalar", 0)
		defer rc.PopLoop()

		got, err := rc.Resolve(ctx, `{{ loop_item "id" }}`)
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("nested loops shadow and restore", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)

		rc.PushLoop("outer", 0)
		rc.PushLoop("inner", 5)

		item, err := rc.Resolve(ctx, "{{ loop_item }}/{{ loop_index }}")
		require.NoError(t, err)
		assert.Equal(t, "inner/5", item)

		rc.PopLoop()

		item, err = rc.Resolve(ctx, "{{ loop_item }}/{{ loop_index }}")
		require.NoError(t, err)
		assert.Equal(t, "outer/0", item)

		rc.PopLoop()

		item, err = rc.Resolve(ctx, "{{ loop_item }}/{{ loop_index }}")
		require.NoError(t, err)
		assert.Equal(t, "/0", item)
	})

	t.Run("PopLoop with no active loop is a no-op", func(t *testing.T) {
		ctx := NewTestContext()
		rc, ok := FromContext(ctx)
		require.True(t, ok)
		assert.NotPanics(t, func() { rc.PopLoop() })
	})
}
