package plan

import (
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestActionV2_OutputPublishing covers the request-variable contract: an action
// that returns a value publishes it under its id, and an action that returns nil
// publishes nothing at all. The nil case is what lets an action route its result
// elsewhere — the agent action contributes to the request conversation instead of
// handing a value to later steps.
func TestActionV2_OutputPublishing(t *testing.T) {
	t.Run("nil output writes no variable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := requestctx.NewTestContext()
		exec := NewMockActionExecutableV2(ctrl)
		exec.EXPECT().Type().Return("agent").AnyTimes()
		exec.EXPECT().SupportsReplica().Return(false).AnyTimes()
		exec.EXPECT().Execute(gomock.Any()).Return(nil, nil, nil)

		action := &ActionV2{id: "silentaction", exec: exec}
		_, err := action.execute(ctx)
		require.NoError(t, err)

		vars, err := requestctx.GetAllRequestVariables(ctx)
		require.NoError(t, err)
		_, present := vars["silentaction"]
		assert.False(t, present, "a nil-returning action must not publish a request variable")
	})

	t.Run("non-nil output is published under the action id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := requestctx.NewTestContext()
		exec := NewMockActionExecutableV2(ctrl)
		exec.EXPECT().Type().Return("http").AnyTimes()
		exec.EXPECT().SupportsReplica().Return(false).AnyTimes()
		exec.EXPECT().Execute(gomock.Any()).Return("a result", nil, nil)

		action := &ActionV2{id: "loudaction", exec: exec}
		_, err := action.execute(ctx)
		require.NoError(t, err)

		vars, err := requestctx.GetAllRequestVariables(ctx)
		require.NoError(t, err)
		assert.Equal(t, "a result", vars["loudaction"])
	})
}
