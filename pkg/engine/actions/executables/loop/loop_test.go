package loop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newBodyPlan builds a plan whose single action "body" runs bodyFn on each
// execution, so a test can observe the loop_item/loop_index visible to the body.
func newBodyPlan(t *testing.T, ctrl *gomock.Controller, bodyFn func(ctx context.Context) error) *plan.Plan {
	t.Helper()

	cfg := apiconfig.APIConfig{Actions: map[string]apiconfig.Action{
		"body": {Name: "body", Type: "body_type"},
	}}

	mockExec := plan.NewMockActionExecutable(ctrl)
	mockExec.EXPECT().Config().Return("").AnyTimes()
	mockExec.EXPECT().SupportsReplica().Return(false).AnyTimes()
	mockExec.EXPECT().Type().Return("body").AnyTimes()
	mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string) (interface{}, map[string]string, error) {
			return nil, nil, bodyFn(ctx)
		}).AnyTimes()

	registry := actions.NewRegistry()
	registry.ReplaceActionType("body_type", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return mockExec, nil
	})

	planner := plan.NewPlannerV2(plan.PlannerConfig{
		Actions:        cfg.Actions,
		Responses:      cfg.Responses,
		CustomRegistry: registry,
	}, logging.GetNewLogger())
	testPlan, err := planner.Plan()
	require.NoError(t, err)
	return testPlan
}

const bodyStart = requestctx.ActionConfigPrefix + "body"

func TestLoopExec_Execute(t *testing.T) {
	t.Run("iterates each element exposing item and index", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var seenItems, seenIdx []string
		testPlan := newBodyPlan(t, ctrl, func(ctx context.Context) error {
			rc, err := requestctx.FromContextOrError(ctx)
			require.NoError(t, err)
			item, err := rc.Resolve(ctx, "{{ loop_item }}")
			require.NoError(t, err)
			idx, err := rc.Resolve(ctx, "{{ loop_index }}")
			require.NoError(t, err)
			seenItems = append(seenItems, item)
			seenIdx = append(seenIdx, idx)
			return nil
		})

		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		exec := &Exec{config: Config{Items: `["a","b","c"]`, Start: bodyStart}}
		res, fields, err := exec.Execute(ctx)

		require.NoError(t, err)
		assert.Nil(t, res)
		assert.Equal(t, []string{"a", "b", "c"}, seenItems)
		assert.Equal(t, []string{"0", "1", "2"}, seenIdx)
		assert.Equal(t, "3", fields["count"])
	})

	t.Run("object elements support field access", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var ids []string
		testPlan := newBodyPlan(t, ctrl, func(ctx context.Context) error {
			rc, _ := requestctx.FromContextOrError(ctx)
			id, err := rc.Resolve(ctx, `{{ loop_item "id" }}`)
			require.NoError(t, err)
			ids = append(ids, id)
			return nil
		})

		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		exec := &Exec{config: Config{Items: `[{"id":"1"},{"id":"2"}]`, Start: bodyStart}}
		_, _, err := exec.Execute(ctx)

		require.NoError(t, err)
		assert.Equal(t, []string{"1", "2"}, ids)
	})

	t.Run("empty array is a clean no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		called := 0
		testPlan := newBodyPlan(t, ctrl, func(ctx context.Context) error {
			called++
			return nil
		})

		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		exec := &Exec{config: Config{Items: `[]`, Start: bodyStart}}
		res, fields, err := exec.Execute(ctx)

		require.NoError(t, err)
		assert.Nil(t, res)
		assert.Equal(t, 0, called)
		assert.Equal(t, "0", fields["count"])
	})

	t.Run("empty resolved items is a no-op", func(t *testing.T) {
		ctx := requestctx.NewTestContext()
		exec := &Exec{config: Config{Items: `{{ loop_item }}`, Start: bodyStart}} // resolves to ""
		res, _, err := exec.Execute(ctx)
		require.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		ctx := requestctx.NewTestContext()
		exec := &Exec{config: Config{Items: `not json`, Start: bodyStart}}
		_, _, err := exec.Execute(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "JSON array")
	})

	t.Run("non-array JSON errors", func(t *testing.T) {
		ctx := requestctx.NewTestContext()
		exec := &Exec{config: Config{Items: `{"a":1}`, Start: bodyStart}}
		_, _, err := exec.Execute(ctx)
		require.Error(t, err)
	})

	t.Run("body error stops the loop and unwinds loop state", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		boom := errors.New("boom")
		called := 0
		testPlan := newBodyPlan(t, ctrl, func(ctx context.Context) error {
			called++
			if called == 2 {
				return boom
			}
			return nil
		})

		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		exec := &Exec{config: Config{Items: `["a","b","c"]`, Start: bodyStart}}
		_, _, err := exec.Execute(ctx)

		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		assert.Equal(t, 2, called) // stopped at the failing element

		// The loop frame must be popped even on error: loop_item is empty again.
		rc, _ := requestctx.FromContextOrError(ctx)
		got, err := rc.Resolve(ctx, "{{ loop_item }}")
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})
}

func TestNew(t *testing.T) {
	_, err := New(Config{Items: `[]`, Start: ""})
	require.Error(t, err)

	e, err := New(Config{Items: `[]`, Start: "action.body"})
	require.NoError(t, err)
	assert.Equal(t, "loop", e.Type())
	assert.False(t, e.SupportsReplica())
}
