package plan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

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

func TestBackgroundManager_Dispatch(t *testing.T) {
	ctx := context.Background()
	bgMgr := NewBackgroundManager(ctx)
	require.NotNil(t, bgMgr)

	executed := make(chan bool, 1)

	bgMgr.Dispatch(func(ctx context.Context) {
		executed <- true
	})

	select {
	case <-executed:
		// Success
	case <-time.After(time.Second):
		t.Fatal("dispatch function was not executed")
	}
}

func TestBackgroundManager_Shutdown(t *testing.T) {
	ctx := context.Background()
	bgMgr := NewBackgroundManager(ctx)
	require.NotNil(t, bgMgr)

	contextCancelled := make(chan bool, 1)

	bgMgr.Dispatch(func(ctx context.Context) {
		<-ctx.Done()
		contextCancelled <- true
	})

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	bgMgr.Shutdown()

	select {
	case <-contextCancelled:
		// Success - context was cancelled
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled on shutdown")
	}
}

// fakePlanWorkspace is a minimal requestctx.Workspace used to assert the
// capability is threaded through the plan into the request context.
type fakePlanWorkspace struct {
	content []byte
}

func (f *fakePlanWorkspace) Read(ctx context.Context, path string) ([]byte, error) {
	return f.content, nil
}
func (f *fakePlanWorkspace) Write(ctx context.Context, path string, data []byte) error { return nil }
func (f *fakePlanWorkspace) Stat(ctx context.Context, path string) (requestctx2.WorkspaceEntry, error) {
	return requestctx2.WorkspaceEntry{}, nil
}

func TestPlan_WorkspacePassedToActions(t *testing.T) {
	fake := &fakePlanWorkspace{}

	testCases := []struct {
		name      string
		workspace requestctx2.Workspace
		expectNil bool
	}{
		{
			name:      "workspace capability is passed to request context",
			workspace: fake,
			expectNil: false,
		},
		{
			name:      "nil workspace yields ErrNoWorkspace",
			workspace: nil,
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var (
				captured    requestctx2.Workspace
				capturedErr error
			)

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Config().Return("").AnyTimes()
			mockExec.EXPECT().SupportsReplica().Return(false).AnyTimes()
			mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, _ string) (interface{}, map[string]string, error) {
					captured, capturedErr = requestctx2.WorkspaceFromContext(ctx)
					return "done", nil, nil
				})

			registry := actions.NewRegistry()
			registry.ReplaceActionType("workspace_test", func(config json.RawMessage) (actions.ActionExecutable, error) {
				return mockExec, nil
			})

			cfg := apiconfig.APIConfig{
				Actions: map[string]apiconfig.Action{
					"test_action": {
						Type: "workspace_test",
						Next: "response.success",
					},
				},
				Responses: map[string]apiconfig.ResponseConfig{
					"success": {
						Code: 200,
						Object: apiconfig.ResponseObject{
							Fields: map[string]apiconfig.ResponseObject{
								"status": {Value: "ok"},
							},
						},
					},
				},
			}

			planner := NewPlannerV2(PlannerConfig{
				Actions:        cfg.Actions,
				Responses:      cfg.Responses,
				CustomRegistry: registry,
				Workspace:      tc.workspace,
			}, logging.GetNewLogger())

			plan, err := planner.Plan()
			require.NoError(t, err)

			ctx := requestctx2.NewTestContext()
			_, err = plan.Execute(ctx, requestctx2.ActionConfigPrefix+"test_action", nil)
			require.NoError(t, err)

			if tc.expectNil {
				assert.ErrorIs(t, capturedErr, requestctx2.ErrNoWorkspace)
				assert.Nil(t, captured)
			} else {
				require.NoError(t, capturedErr)
				assert.Equal(t, requestctx2.Workspace(fake), captured)
			}
		})
	}
}

func TestPlan_WorkspaceTemplateFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fake := &fakePlanWorkspace{content: []byte("hello from workspace")}

	mockExec := NewMockActionExecutable(ctrl)
	mockExec.EXPECT().Config().Return(`{"content": "{{ file \"hello.txt\" }}"}`).AnyTimes()
	mockExec.EXPECT().SupportsReplica().Return(false).AnyTimes()
	mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, config string) (interface{}, map[string]string, error) {
			// The {{ file }} template function reads from the workspace capability.
			assert.Contains(t, config, "hello from workspace")
			return "done", nil, nil
		})

	registry := actions.NewRegistry()
	registry.ReplaceActionType("workspace_tmpl_test", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return mockExec, nil
	})

	cfg := apiconfig.APIConfig{
		Actions: map[string]apiconfig.Action{
			"test_action": {
				Type: "workspace_tmpl_test",
				Next: "response.success",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Code: 200,
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"status": {Value: "ok"},
					},
				},
			},
		},
	}

	planner := NewPlannerV2(PlannerConfig{
		Actions:        cfg.Actions,
		Responses:      cfg.Responses,
		CustomRegistry: registry,
		Workspace:      fake,
	}, logging.GetNewLogger())

	plan, err := planner.Plan()
	require.NoError(t, err)

	ctx := requestctx2.NewTestContext()
	_, err = plan.Execute(ctx, requestctx2.ActionConfigPrefix+"test_action", nil)
	require.NoError(t, err)
}
