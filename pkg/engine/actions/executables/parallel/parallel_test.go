package parallel

import (
	"context"
	"encoding/json"
	"errors"

	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	requestctx "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestParallelExec_Execute(t *testing.T) {
	// Helper function to create a test plan with mock actions
	createTestPlanWithMocks := func(ctrl *gomock.Controller, mockActions map[string]*plan.MockActionExecutable) *plan.Plan {
		cfg := apiconfig.APIConfig{
			Actions: make(map[string]apiconfig.Action),
		}

		customRegistry := actions.NewRegistry()

		// Register mock actions
		for actionID, mockExec := range mockActions {
			cfg.Actions[actionID] = apiconfig.Action{
				Type: actionID + "_type",
			}

			customRegistry.ReplaceActionType(actionID+"_type", func(config json.RawMessage) (actions.ActionExecutable, error) {
				return mockExec, nil
			})

			mockExec.EXPECT().Config().Return("").AnyTimes()
		}

		planner := plan.NewPlannerV2(plan.PlannerConfig{
			Actions:        cfg.Actions,
			Responses:      cfg.Responses,
			CustomRegistry: customRegistry,
		}, logging.GetNewLogger())

		testPlan, err := planner.Plan()
		require.NoError(t, err)
		return testPlan
	}

	t.Run("successful parallel execution of multiple steps", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock actions
		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockAction2 := plan.NewMockActionExecutable(ctrl)
		mockAction3 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
			"action2": mockAction2,
			"action3": mockAction3,
		}

		// Set up expectations - all should succeed
		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result1", nil)
		mockAction2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result2", nil)
		mockAction3.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result3", nil)

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		// Create parallel action
		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
					requestctx.ActionConfigPrefix + "action2",
					requestctx.ActionConfigPrefix + "action3",
				},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.NoError(t, err)
		assert.Nil(t, result) // Parallel action returns nil on success
	})

	t.Run("stopOnFailure=true - stops on first failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockAction2 := plan.NewMockActionExecutable(ctrl)
		mockAction3 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
			"action2": mockAction2,
			"action3": mockAction3,
		}

		expectedError := errors.New("action1 failed")
		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, expectedError)

		// Other actions might or might not be called due to cancellation
		mockAction2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result2", nil).AnyTimes()
		mockAction3.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result3", nil).AnyTimes()

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
					requestctx.ActionConfigPrefix + "action2",
					requestctx.ActionConfigPrefix + "action3",
				},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.Error(t, err)
		assert.ErrorContains(t, err, expectedError.Error()) // Should return the first error
		assert.Nil(t, result)
	})

	t.Run("stopOnFailure=false - collects all errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockAction2 := plan.NewMockActionExecutable(ctrl)
		mockAction3 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
			"action2": mockAction2,
			"action3": mockAction3,
		}

		// Set up expectations - multiple actions fail
		error1 := errors.New("action1 failed")
		error3 := errors.New("action3 failed")

		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error1)
		mockAction2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result2", nil)
		mockAction3.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error3)

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
					requestctx.ActionConfigPrefix + "action2",
					requestctx.ActionConfigPrefix + "action3",
				},
				StopOnFailure: false,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.Error(t, err)

		// Should be a groupError containing both failures
		var groupErr *groupError
		ok := errors.As(err, &groupErr)
		require.True(t, ok, "Expected groupError, got %T", err)

		// Verify error message contains information about both failures
		errMsg := groupErr.Error()
		assert.Contains(t, errMsg, "action1 failed")
		assert.Contains(t, errMsg, "action3 failed")
		assert.NotContains(t, errMsg, "action2") // action2 succeeded

		assert.Nil(t, result)
	})

	t.Run("stopOnFailure=true - all actions fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockAction2 := plan.NewMockActionExecutable(ctrl)
		mockAction3 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
			"action2": mockAction2,
			"action3": mockAction3,
		}

		// Set up expectations - all actions fail but with stopOnFailure=true,
		// only the first error should be returned
		error1 := errors.New("action1 failed")
		error2 := errors.New("action2 failed")
		error3 := errors.New("action3 failed")

		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error1)
		mockAction2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error2).AnyTimes()
		mockAction3.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error3).AnyTimes()

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
					requestctx.ActionConfigPrefix + "action2",
					requestctx.ActionConfigPrefix + "action3",
				},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.Error(t, err)
		// Should return just the first error, not a group error
		// The error will be wrapped, so check if it contains the original error message
		assert.Contains(t, err.Error(), "action1 failed")
		// Ensure it's not a groupError
		var groupErr *groupError
		assert.False(t, errors.As(err, &groupErr), "Should not be a groupError when stopOnFailure=true")
		assert.Nil(t, result)
	})

	t.Run("stopOnFailure=false - all actions fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockAction2 := plan.NewMockActionExecutable(ctrl)
		mockAction3 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
			"action2": mockAction2,
			"action3": mockAction3,
		}

		// Set up expectations - all actions fail
		error1 := errors.New("action1 failed")
		error2 := errors.New("action2 failed")
		error3 := errors.New("action3 failed")

		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error1)
		mockAction2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error2)
		mockAction3.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, error3)

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
					requestctx.ActionConfigPrefix + "action2",
					requestctx.ActionConfigPrefix + "action3",
				},
				StopOnFailure: false,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.Error(t, err)

		// Should be a groupError containing all failures
		groupErr, ok := err.(*groupError)
		require.True(t, ok, "Expected groupError, got %T", err)

		// Verify error message contains information about all failures
		errMsg := groupErr.Error()
		assert.Contains(t, errMsg, "action1 failed")
		assert.Contains(t, errMsg, "action2 failed")
		assert.Contains(t, errMsg, "action3 failed")

		assert.Nil(t, result)
	})

	t.Run("context cancellation handling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)

		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
		}

		// Mock action returns context canceled error
		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, plan.ErrContextCanceled)

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
				},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions - context canceled errors should be ignored
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty steps array", func(t *testing.T) {
		ctx := requestctx.NewTestContext()

		parallelExec := &Exec{
			config: Config{
				Steps:         []string{},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions - should succeed with no work to do
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("single step execution", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockAction1 := plan.NewMockActionExecutable(ctrl)
		mockActions := map[string]*plan.MockActionExecutable{
			"action1": mockAction1,
		}

		mockAction1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return("result1", nil)

		testPlan := createTestPlanWithMocks(ctrl, mockActions)
		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, plan.ContextKey, testPlan)

		parallelExec := &Exec{
			config: Config{
				Steps: []string{
					requestctx.ActionConfigPrefix + "action1",
				},
				StopOnFailure: true,
			},
		}

		// Execute
		result, err := parallelExec.Execute(ctx, "")

		// Assertions
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}
