package plan

import (
	"testing"

	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndStep_Execute(t *testing.T) {
	t.Run("write successfully", func(t *testing.T) {
		endStep := &EndStep{
			destinationKey: "groupid",
			lookupKey:      "testaction",
		}

		ctx := requestctx2.NewTestContext()
		err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testaction": "testvalue"}, "")
		require.NoError(t, err)

		v, err := endStep.execute(ctx)
		assert.Nil(t, v)
		assert.NoError(t, err)
		val, err := requestctx2.GetRequestVariable(ctx, "groupid")
		assert.NoError(t, err)
		assert.Equal(t, "testvalue", val)
	})

	t.Run("no end destinationKey and endvar", func(t *testing.T) {
		endStep := &EndStep{}
		v, err := endStep.execute(requestctx2.NewTestContext())
		assert.Nil(t, v)
		assert.NoError(t, err)
	})

	t.Run("end with end template", func(t *testing.T) {
		endStep := &EndStep{
			destinationKey: "groupid",
			endTemplate:    `{{ index .items 1 }}`,
		}

		ctx := requestctx2.NewTestContext()
		err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{
			"items": []string{"testvalue", "secondValue"},
		}, "")
		require.NoError(t, err)

		v, err := endStep.execute(ctx)
		assert.Nil(t, v)
		assert.NoError(t, err)
		val, err := requestctx2.GetRequestVariable(ctx, "groupid")
		assert.NoError(t, err)
		assert.Equal(t, "secondValue", val)
	})
}
