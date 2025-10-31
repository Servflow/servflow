package plan

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/actions"
	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPlan_Execute(t *testing.T) {
	logging.SetLogger(silentLogger())
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
		endValue        string
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
			endValue:     "",
			contextSetup: func(ctx context.Context) {},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec1.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedBody: `{"status": "success"}`,
			expectedJSON: true,
		},
		{
			name:        "success from end value",
			startAction: requestctx2.ActionConfigPrefix + "action2",
			endValue:    "{{ .testValue }}",
			contextSetup: func(ctx context.Context) {
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "hello"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedBody: "hello",
		},
		{
			name:         "invalid step",
			startAction:  "invalidID",
			endValue:     "",
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
			endValue:    "",
			contextSetup: func(ctx context.Context) {
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "test value"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec3.EXPECT().Execute(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, _ string) (interface{}, error) {
						resp, err := ExecuteFromContext(ctx, requestctx2.ActionConfigPrefix+"action2", "{{ .testValue }}")
						require.NoError(t, err)
						assert.Equal(t, "test value", string(resp.Body))
						return string(resp.Body), nil
					})
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedBody: `{"data": "test value"}`,
			expectedJSON: true,
		},
		{
			name:        "get from secret",
			startAction: requestctx2.ActionConfigPrefix + "action2",
			endValue:    `{{ secret "MONGO_PASS" }}`,
			contextSetup: func(ctx context.Context) {
				os.Setenv("MONGO_PASS", "secret")
				requestctx2.AddRequestVariables(ctx, map[string]interface{}{"testValue": "hello"}, "")
			},
			mockAssertions: func(exec1, exec2, exec3 *MockActionExecutable) {
				exec2.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, nil)
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
			})

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
