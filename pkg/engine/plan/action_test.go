package plan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func resetReplicaManager() {
	replicaManager = &ReplicaManager{replicas: []Replica{}}
}

type testStep struct {
	id string
}

func (t *testStep) ID() string {
	//TODO implement me
	panic("implement me")
}

func (t *testStep) execute(ctx context.Context) (*stepWrapper, error) {
	return nil, nil
}

func TestAction_Execute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Run("success with simple variable", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			conf := fmt.Sprintf("test {{ .%sname }}", requestctx.BareVariablesPrefixStripped)

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Config().Return(conf)
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil, nil)
			mockExec.EXPECT().SupportsReplica().Return(false)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
				out:  "test",
			}

			next, err := act.execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

			field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .test }}")
			require.NoError(t, err)
			assert.Equal(t, "response string", field)
		})

		t.Run("success with complex variables", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			config := fmt.Sprintf("test {{ .%sname.actualname }}", requestctx.BareVariablesPrefixStripped)
			mockExec.EXPECT().Config().Return(config)
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil, nil)
			mockExec.EXPECT().SupportsReplica().Return(false)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{
				fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): map[string]interface{}{
					"actualname": "actual name",
				},
			}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
				out:  "test",
			}

			next, err := act.execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

			field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .test }}")
			require.NoError(t, err)
			assert.Equal(t, "response string", field)
		})

		t.Run("success with custom out variable", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Config().Return("")
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("custom response", nil, nil)
			mockExec.EXPECT().SupportsReplica().Return(false)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()

			act := Action{
				exec: mockExec,
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
				out:  "custom_out",
			}

			next, err := act.execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

			field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .custom_out }}")
			require.NoError(t, err)
			assert.Equal(t, "custom response", field)
		})

		t.Run("success with reader response stored as file", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			fileContent := "test file content"
			reader := io.NopCloser(strings.NewReader(fileContent))
			mockExec.EXPECT().Config().Return("")
			mockExec.EXPECT().Execute(gomock.Any(), "").Return(reader, nil, nil)
			mockExec.EXPECT().SupportsReplica().Return(false)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()

			act := Action{
				exec: mockExec,
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
				out:  "file_output",
			}

			next, err := act.execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

			require.NoError(t, err)

			fileValue, err := requestctx.GetFileFromContext(ctx, apiconfig.FileInput{
				Type:       apiconfig.FileInputTypeAction,
				Identifier: "file_output",
			})
			require.NoError(t, err)
			require.NotNil(t, fileValue)
			assert.Equal(t, "file_output", fileValue.Name)

			// Read the file content to verify it was stored correctly
			content, err := fileValue.GetContent()
			require.NoError(t, err)
			assert.Equal(t, fileContent, string(content))
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Run("has error", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Config().Return("")
			mockExec.EXPECT().Type().Return("mock").AnyTimes()
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", nil, errors.New("dummy error"))
			mockExec.EXPECT().SupportsReplica().Return(false)

			mockStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				out:  "field1",
				id:   "test",
				next: &stepWrapper{id: "next", step: &mockStep},
			}

			next, err := act.execute(ctx)
			assert.Error(t, err)
			assert.Nil(t, next)
		})

		t.Run("failure error with a fail step", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Config().Return("").AnyTimes()
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", nil, fmt.Errorf("%w: dummy error", ErrFailure)).AnyTimes()
			mockExec.EXPECT().SupportsReplica().Return(false).AnyTimes()

			nextStep := testStep{id: "next"}
			failStep := testStep{id: "fail"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				out:  "field1",
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
				fail: &stepWrapper{id: "fail", step: &failStep},
			}

			next, err := act.execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &stepWrapper{id: "fail", step: &failStep}, next)

			// Verify error variable is stored
			errorVal, err := requestctx.GetRequestVariable(ctx, requestctx.ErrorTagStripped)
			assert.NoError(t, err)
			assert.Contains(t, errorVal, "dummy error")

			// Verify output variable contains error message
			outVal, err := requestctx.GetRequestVariable(ctx, "field1")
			assert.NoError(t, err)
			assert.Contains(t, outVal, "dummy error")

			//check if it returns nil too
			act2 := Action{
				exec: mockExec,
				out:  "field1",
				id:   "test",
				next: &stepWrapper{id: "next", step: &nextStep},
			}

			next, err = act2.execute(ctx)
			assert.NoError(t, err)
			assert.Nil(t, next)
		})

	})
}

func TestAction_ExecuteWithReplica(t *testing.T) {
	t.Run("replica manager is called when useReplica=true and SupportsReplica=true", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetReplicaManager()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().SupportsReplica().Return(true).AnyTimes()
		mockExec.EXPECT().Type().Return("mock")

		mockReplica := NewMockReplica(ctrl)
		mockReplica.EXPECT().ExecuteAction("mock", "").Return("replica response", nil, nil)
		GetReplicaManager().AddReplica(mockReplica)

		ctx := requestctx.NewTestContext()
		nextStep := testStep{id: "next"}

		act := Action{
			exec:       mockExec,
			id:         "test",
			next:       &stepWrapper{id: "next", step: &nextStep},
			out:        "test",
			useReplica: true,
		}

		next, err := act.execute(ctx)
		assert.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

		field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .test }}")
		require.NoError(t, err)
		assert.Equal(t, "replica response", field)
	})

	t.Run("replica manager is NOT called when SupportsReplica=false", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetReplicaManager()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().SupportsReplica().Return(false).AnyTimes()
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("direct response", nil, nil)

		mockReplica := NewMockReplica(ctrl)
		GetReplicaManager().AddReplica(mockReplica)

		ctx := requestctx.NewTestContext()
		nextStep := testStep{id: "next"}

		act := Action{
			exec:       mockExec,
			id:         "test",
			next:       &stepWrapper{id: "next", step: &nextStep},
			out:        "test",
			useReplica: true,
		}

		next, err := act.execute(ctx)
		assert.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

		field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .test }}")
		require.NoError(t, err)
		assert.Equal(t, "direct response", field)
	})

	t.Run("falls back to direct execution when replica manager fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetReplicaManager()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().SupportsReplica().Return(true).AnyTimes()
		mockExec.EXPECT().Type().Return("mock")
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("fallback response", nil, nil)

		mockReplica := NewMockReplica(ctrl)
		mockReplica.EXPECT().ExecuteAction("mock", "").Return(nil, nil, errors.New("replica error"))
		GetReplicaManager().AddReplica(mockReplica)

		ctx := requestctx.NewTestContext()
		nextStep := testStep{id: "next"}

		act := Action{
			exec:       mockExec,
			id:         "test",
			next:       &stepWrapper{id: "next", step: &nextStep},
			out:        "test",
			useReplica: true,
		}

		next, err := act.execute(ctx)
		assert.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "next", step: &nextStep}, next)

		field, err := requestctx.ReplaceVariableValuesInContext(ctx, "{{ .test }}")
		require.NoError(t, err)
		assert.Equal(t, "fallback response", field)
	})

	t.Run("returns error when both replica and fallback fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetReplicaManager()

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().SupportsReplica().Return(true).AnyTimes()
		mockExec.EXPECT().Type().Return("mock").AnyTimes()
		mockExec.EXPECT().Execute(gomock.Any(), "").Return(nil, nil, errors.New("direct execution error"))

		mockReplica := NewMockReplica(ctrl)
		mockReplica.EXPECT().ExecuteAction("mock", "").Return(nil, nil, errors.New("replica error"))
		GetReplicaManager().AddReplica(mockReplica)

		ctx := requestctx.NewTestContext()
		nextStep := testStep{id: "next"}

		act := Action{
			exec:       mockExec,
			id:         "test",
			next:       &stepWrapper{id: "next", step: &nextStep},
			out:        "test",
			useReplica: true,
		}

		next, err := act.execute(ctx)
		assert.Error(t, err)
		assert.Nil(t, next)
		assert.Contains(t, err.Error(), "direct execution error")
	})
}

func resetBackgroundManagerForActionTest() {
	backgroundMgr = nil
	backgroundMgrOnce = sync.Once{}
}

func TestAction_ExecuteWithDispatch(t *testing.T) {
	t.Run("triggers background chains", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetBackgroundManagerForActionTest()

		// Initialize background manager
		bgMgr := InitBackgroundManager(context.Background())
		require.NotNil(t, bgMgr)

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("response", nil, nil)
		mockExec.EXPECT().SupportsReplica().Return(false)

		dispatchExecuted := make(chan bool, 1)

		// Create a mock dispatch step
		dispatchStep := &testStep{id: "dispatch_action"}

		// Create a plan with the dispatch step
		p := &Plan{
			steps: map[string]stepWrapper{
				"action.dispatch_action": {
					id:   "action.dispatch_action",
					step: dispatchStep,
				},
			},
		}

		ctx := requestctx.NewTestContext()
		ctx = context.WithValue(ctx, ContextKey, p)

		// Override the dispatch step to signal when executed
		p.steps["action.dispatch_action"] = stepWrapper{
			id: "action.dispatch_action",
			step: &executableStep{
				fn: func(ctx context.Context) (*stepWrapper, error) {
					dispatchExecuted <- true
					return nil, nil
				},
			},
		}

		act := Action{
			exec:     mockExec,
			id:       "test",
			out:      "test",
			dispatch: []string{"action.dispatch_action"},
		}

		next, err := act.execute(ctx)
		assert.NoError(t, err)
		assert.Nil(t, next)

		select {
		case <-dispatchExecuted:
			// Success - dispatch was executed
		case <-time.After(time.Second):
			t.Fatal("dispatch chain was not executed")
		}
	})

	t.Run("shares same RequestContext", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetBackgroundManagerForActionTest()

		bgMgr := InitBackgroundManager(context.Background())
		require.NotNil(t, bgMgr)

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("response", nil, nil)
		mockExec.EXPECT().SupportsReplica().Return(false)

		var capturedReqCtx *requestctx.RequestContext
		dispatchDone := make(chan bool, 1)

		ctx := requestctx.NewTestContext()
		originalReqCtx, ok := requestctx.FromContext(ctx)
		require.True(t, ok)

		p := &Plan{
			steps: map[string]stepWrapper{
				"action.dispatch_action": {
					id: "action.dispatch_action",
					step: &executableStep{
						fn: func(ctx context.Context) (*stepWrapper, error) {
							capturedReqCtx, _ = requestctx.FromContext(ctx)
							dispatchDone <- true
							return nil, nil
						},
					},
				},
			},
		}

		ctx = context.WithValue(ctx, ContextKey, p)

		act := Action{
			exec:     mockExec,
			id:       "test",
			out:      "test",
			dispatch: []string{"action.dispatch_action"},
		}

		_, err := act.execute(ctx)
		assert.NoError(t, err)

		select {
		case <-dispatchDone:
			assert.Same(t, originalReqCtx, capturedReqCtx, "dispatch should use same RequestContext")
		case <-time.After(time.Second):
			t.Fatal("dispatch chain was not executed")
		}
	})

	t.Run("continues main flow without blocking", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetBackgroundManagerForActionTest()

		bgMgr := InitBackgroundManager(context.Background())
		require.NotNil(t, bgMgr)

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("response", nil, nil)
		mockExec.EXPECT().SupportsReplica().Return(false)

		dispatchStarted := make(chan bool, 1)
		dispatchContinue := make(chan bool, 1)

		ctx := requestctx.NewTestContext()

		p := &Plan{
			steps: map[string]stepWrapper{
				"action.slow_dispatch": {
					id: "action.slow_dispatch",
					step: &executableStep{
						fn: func(ctx context.Context) (*stepWrapper, error) {
							dispatchStarted <- true
							<-dispatchContinue // Block until signaled
							return nil, nil
						},
					},
				},
			},
		}

		ctx = context.WithValue(ctx, ContextKey, p)

		nextStep := &testStep{id: "next"}
		act := Action{
			exec:     mockExec,
			id:       "test",
			out:      "test",
			next:     &stepWrapper{id: "next", step: nextStep},
			dispatch: []string{"action.slow_dispatch"},
		}

		// Execute should return immediately without waiting for dispatch
		next, err := act.execute(ctx)
		assert.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "next", step: nextStep}, next)

		// Wait for dispatch to start
		select {
		case <-dispatchStarted:
			// Good - dispatch started in background
		case <-time.After(time.Second):
			t.Fatal("dispatch did not start")
		}

		// Signal dispatch to continue so test can clean up
		close(dispatchContinue)
	})

	t.Run("respects dispatch timeout from plan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		defer resetBackgroundManagerForActionTest()

		bgMgr := InitBackgroundManager(context.Background())
		require.NotNil(t, bgMgr)

		mockExec := NewMockActionExecutable(ctrl)
		mockExec.EXPECT().Config().Return("")
		mockExec.EXPECT().Execute(gomock.Any(), "").Return("response", nil, nil)
		mockExec.EXPECT().SupportsReplica().Return(false)

		contextTimedOut := make(chan bool, 1)

		ctx := requestctx.NewTestContext()

		p := &Plan{
			dispatchTimeout: 50 * time.Millisecond,
			steps: map[string]stepWrapper{
				"action.slow_action": {
					id: "action.slow_action",
					step: &executableStep{
						fn: func(ctx context.Context) (*stepWrapper, error) {
							select {
							case <-ctx.Done():
								contextTimedOut <- true
							case <-time.After(time.Second):
								contextTimedOut <- false
							}
							return nil, nil
						},
					},
				},
			},
		}

		ctx = context.WithValue(ctx, ContextKey, p)

		act := Action{
			exec:     mockExec,
			id:       "test",
			out:      "test",
			dispatch: []string{"action.slow_action"},
		}

		_, err := act.execute(ctx)
		assert.NoError(t, err)

		select {
		case timedOut := <-contextTimedOut:
			assert.True(t, timedOut, "dispatch context should have timed out")
		case <-time.After(time.Second):
			t.Fatal("test timed out waiting for dispatch")
		}
	})
}

// executableStep is a test helper that wraps a function as a Step
type executableStep struct {
	fn func(ctx context.Context) (*stepWrapper, error)
}

func (e *executableStep) execute(ctx context.Context) (*stepWrapper, error) {
	return e.fn(ctx)
}

func TestActionTemplateFunctions(t *testing.T) {
	variables := map[string]interface{}{
		"header": "Bearer testttt",
	}
	testCases := []struct {
		name                  string
		template              string
		expected              string
		expectValidationError bool
	}{
		{
			name:     "simple email case",
			template: `{{ strip .header "Bearer"}}`,
			expected: "testttt",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := requestctx.NewTestContext()
			tmpl, err := requestctx.CreateTextTemplate(ctx, testCase.template, nil)
			require.NoError(t, err)
			var buff bytes.Buffer
			err = tmpl.Execute(&buff, variables)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, buff.String())
		})
	}

}
