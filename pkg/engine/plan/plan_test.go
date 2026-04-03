package plan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPlan_Execute(t *testing.T) {
	cfg := apiconfig.APIConfig{
		Actions: map[string]apiconfig.Action{
			"action1": {
				Next: "response.success",
				Type: "action1",
			},
			"action2": {
				Type: "action2",
			},
			"action3": {
				Type: "action3",
				Next: "response.second",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Code: 200,
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"status": {
							Value: "success",
						},
					},
				},
			},
			"second": {
				Code: 200,
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"data": {
							Value: "{{ .variable_actions_action3 }}",
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name            string
		startAction     string
		endValue        *EndValueSpec
		contextSetup    func(context.Context)
		mockAssertions  func(*MockActionExecutable, *MockActionExecutable, *MockActionExecutable)
		expectedBody    string
		expectedErr     bool
		expectedJSON    bool
		expectedNilResp bool
	}{
		{
			name:         "success from response template",
			startAction:  requestctx2.ActionConfigPrefix + "action1",
			endValue:     nil,
			contextSetup: func(ctx context.Context) {},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				exec1.EXPECT().SupportsReplica().Return(false).AnyTimes()
			},
			expectedBody: `{"status": "success"}`,
			expectedJSON: true,
		},
		{
			name:        "success from end value",
			startAction: requestctx2.ActionConfigPrefix + "action2",
			endValue:    &EndValueSpec{ValType: StringEndValue, StringVal: "{{ .testValue }}"},
			contextSetup: func(ctx context.Context) {
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "hello"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				exec2.EXPECT().SupportsReplica().Return(false).AnyTimes()
			},
			expectedBody: "hello",
		},
		{
			name:         "invalid step",
			startAction:  "invalidID",
			endValue:     nil,
			contextSetup: func(ctx context.Context) {},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				// No mock expectations for invalid action
			},
			expectedErr:     true,
			expectedNilResp: true,
		},
		{
			name:        "execute in action",
			startAction: requestctx2.ActionConfigPrefix + "action3",
			endValue:    nil,
			contextSetup: func(ctx context.Context) {
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "test value"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec3.EXPECT().Execute(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, _ string) (interface{}, map[string]string, error) {
						resp, err := ExecuteFromContext(ctx, requestctx2.ActionConfigPrefix+"action2", &EndValueSpec{
							StringVal: "{{ .testValue }}",
						})
						require.NoError(t, err)
						assert.Equal(t, "test value", string(resp.Body))
						return string(resp.Body), nil, nil
					})
				exec3.EXPECT().SupportsReplica().Return(false).AnyTimes()
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				exec2.EXPECT().SupportsReplica().Return(false).AnyTimes()
			},
			expectedBody: `{"data": "test value"}`,
			expectedJSON: true,
		},
		{
			name:        "get from secret",
			startAction: requestctx2.ActionConfigPrefix + "action2",
			endValue:    &EndValueSpec{ValType: StringEndValue, StringVal: `{{ secret "MONGO_PASS" }}`},
			contextSetup: func(ctx context.Context) {
				os.Setenv("MONGO_PASS", "secret")
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "hello"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				exec2.EXPECT().SupportsReplica().Return(false).AnyTimes()
			},
			expectedBody: "secret",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec1 := NewMockActionExecutable(ctrl)
			mockExec2 := NewMockActionExecutable(ctrl)
			mockExec3 := NewMockActionExecutable(ctrl)

			registry := actions.NewRegistry()
			registry.ReplaceActionType("action1", func(config json.RawMessage) (actions.ActionExecutable, error) {
				return mockExec1, nil
			})
			registry.ReplaceActionType("action2", func(config json.RawMessage) (actions.ActionExecutable, error) {
				return mockExec2, nil
			})
			registry.ReplaceActionType("action3", func(config json.RawMessage) (actions.ActionExecutable, error) {
				return mockExec3, nil
			})
			mockExec1.EXPECT().Config().Return("").AnyTimes()
			mockExec2.EXPECT().Config().Return("").AnyTimes()
			mockExec3.EXPECT().Config().Return("").AnyTimes()

			tc.mockAssertions(mockExec1, mockExec2, mockExec3)

			planner := NewPlannerV2(PlannerConfig{
				Actions:        cfg.Actions,
				Responses:      cfg.Responses,
				CustomRegistry: registry,
			}, logging.GetNewLogger())

			plan, err := planner.Plan()
			require.NoError(t, err)

			ctx := requestctx2.NewTestContext()
			tc.contextSetup(ctx)

			resp, err := plan.Execute(ctx, tc.startAction, tc.endValue)

			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.expectedNilResp {
				require.Nil(t, resp)
			} else {
				if tc.expectedJSON {
					assert.JSONEq(t, tc.expectedBody, string(resp.Body))
				} else {
					assert.Equal(t, tc.expectedBody, string(resp.Body))
				}
			}
		})
	}
}

func TestExecuteSingleAction(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Execute(gomock.Any(), `{"key":"value"}`).Return("test response", nil, nil)

		actions.ReplaceActionType("test-single-action", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		result, _, err := ExecuteSingleAction("test-single-action", json.RawMessage(`{"key":"value"}`))
		require.NoError(t, err)
		assert.Equal(t, "test response", result)
	})

	t.Run("unregistered action type", func(t *testing.T) {
		result, _, err := ExecuteSingleAction("unregistered-action-type", json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Execute(gomock.Any(), `{}`).Return(nil, nil, errors.New("execution failed"))

		actions.ReplaceActionType("test-single-action-error", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		result, _, err := ExecuteSingleAction("test-single-action-error", json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "execution failed")
	})
}
