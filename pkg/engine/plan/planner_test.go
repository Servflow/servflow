package plan

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var sampleConfig = &apiconfig.APIConfig{
	Actions: map[string]apiconfig.Action{
		"action1": {
			Type:   "action1",
			Next:   "$action.action2",
			Config: map[string]interface{}{"key": "value"},
		},
		"action2": {
			Next:   "$action.action3",
			Config: map[string]interface{}{"key": "value"},
		},
		"action3": {
			Next:   "$conditional.cond1",
			Config: map[string]interface{}{"key": "value2"},
		},
		"action4": {
			Type:   "mock",
			Next:   "end",
			Config: map[string]interface{}{"key": "value"},
		},
		"action5": {
			Next:   "$action.action4",
			Config: map[string]interface{}{"key": "value"},
		},
	},
	Conditionals: map[string]apiconfig.Conditional{
		"cond1": {
			Expression:  "true",
			ValidPath:   "$response.success",
			InvalidPath: "$response.failure",
		},
	},
	Responses: map[string]apiconfig.ResponseConfig{
		"success": {
			Code:     200,
			Template: `{"status": "success"}`,
			Type:     "json",
		},
		"failure": {
			Code:     400,
			Template: `{"status": "failure"}`,
			Type:     "json",
		},
	},
}

func silentLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	cfg.OutputPaths = []string{"/dev/null"}
	logger, _ := cfg.Build()
	return logger
}

func BenchmarkTestPlannerV2_Generate(b *testing.B) {
	logging.SetLogger(silentLogger())
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	mockExec := NewMockActionExecutable(ctrl)
	mockActionProvider := NewMockActionProvider(ctrl)

	mockRegistry := actions.NewRegistry()
	mockRegistry.ReplaceActionType("", func(config json.RawMessage) (actions.ActionExecutable, error) {
		return mockExec, nil
	})

	mockExec.EXPECT().Config().Return("").AnyTimes()
	mockActionProvider.EXPECT().GetActionExecutable(gomock.Any(), gomock.Any()).Return(mockExec, nil).AnyTimes()

	planner := NewPlannerV2(PlannerConfig{
		Actions:        sampleConfig.Actions,
		Conditions:     sampleConfig.Conditionals,
		Responses:      sampleConfig.Responses,
		TerminateTag:   "end",
		CustomRegistry: mockRegistry,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := planner.Plan()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPlannerV2_Generate(t *testing.T) {
	t.Run("normal config", func(t *testing.T) {
		config := &apiconfig.APIConfig{
			Actions: map[string]apiconfig.Action{
				"action1": {
					Next:   "$conditional.cond1",
					Config: map[string]interface{}{"key": "value"},
				},
				"action2": {
					Config: map[string]interface{}{"key": "value2"},
					Fail:   "$response.failure",
				},
			},
			Conditionals: map[string]apiconfig.Conditional{
				"cond1": {
					Expression:  "true",
					ValidPath:   "$response.success",
					InvalidPath: "$response.failure",
				},
			},
			Responses: map[string]apiconfig.ResponseConfig{
				"success": {
					Code:     200,
					Template: `{"status": "success"}`,
					Type:     "json",
				},
				"failure": {
					Code:     400,
					Template: `{"status": "failure"}`,
					Type:     "json",
				},
			},
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockExec := NewMockActionExecutable(ctrl)

		mockRegistry := actions.NewRegistry()
		mockRegistry.ReplaceActionType("", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		mockExec.EXPECT().Config().Return("").AnyTimes()

		planner := NewPlannerV2(PlannerConfig{
			Actions:        config.Actions,
			Conditions:     config.Conditionals,
			Responses:      config.Responses,
			CustomRegistry: mockRegistry,
		})

		plan, err := planner.Plan()
		require.NoError(t, err)

		t.Run("check all items exist", func(t *testing.T) {
			for id := range config.Actions {
				assert.NotNil(t, plan.steps[requestctx.ActionConfigPrefix+id])
				assert.IsType(t, &Action{}, plan.steps[requestctx.ActionConfigPrefix+id])
			}
			for id := range config.Conditionals {
				assert.NotNil(t, plan.steps[requestctx.ConditionalConfigPrefix+id])
				assert.IsType(t, &ConditionStep{}, plan.steps[requestctx.ConditionalConfigPrefix+id])
			}
			for id := range config.Responses {
				assert.NotNil(t, plan.steps[requestctx.ResponsesConfigPrefix+id])
				assert.IsType(t, &Response{}, plan.steps[requestctx.ResponsesConfigPrefix+id])
			}
		})

		t.Run("basic check of next", func(t *testing.T) {
			assert.NotNil(t, plan.steps[requestctx.ActionConfigPrefix+"action1"])

			act, ok := plan.steps[requestctx.ActionConfigPrefix+"action1"].(*Action)
			require.True(t, ok, "expected action to be a *Action")

			assert.IsType(t, &ConditionStep{}, act.next)
		})

		t.Run("Action Step with End", func(t *testing.T) {
			assert.NotNil(t, plan.steps[requestctx.ActionConfigPrefix+"action2"])

			act, ok := plan.steps[requestctx.ActionConfigPrefix+"action2"].(*Action)
			require.True(t, ok, "expected action to be a *Action")

			assert.Nil(t, act.next)
		})

		t.Run("Conditional Step", func(t *testing.T) {
			assert.NotNil(t, plan.steps[requestctx.ConditionalConfigPrefix+"cond1"])

			cond, ok := plan.steps[requestctx.ConditionalConfigPrefix+"cond1"].(*ConditionStep)
			require.True(t, ok, "expected conditional step to be a *ConditionStep")
			assert.IsType(t, &Response{}, cond.OnValid)
			assert.IsType(t, &Response{}, cond.OnInvalid)
		})
	})

	t.Run("config with only response", func(t *testing.T) {
		config := &apiconfig.APIConfig{
			Actions:      map[string]apiconfig.Action{},
			Conditionals: map[string]apiconfig.Conditional{},
			Responses: map[string]apiconfig.ResponseConfig{
				"success": {
					Code:     200,
					Template: `{"status": "success"}`,
					Type:     "json",
				},
				"failure": {
					Code:     400,
					Template: `{"status": "failure"}`,
					Type:     "json",
				},
			},
		}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockExec := NewMockActionExecutable(ctrl)
		mockActionProvider := NewMockActionProvider(ctrl)

		mockRegistry := actions.NewRegistry()
		mockRegistry.ReplaceActionType("", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		mockExec.EXPECT().Config().Return("").AnyTimes()
		mockActionProvider.EXPECT().GetActionExecutable(gomock.Any(), gomock.Any()).Return(mockExec, nil).AnyTimes()

		planner := NewPlannerV2(PlannerConfig{
			Actions:        config.Actions,
			Conditions:     config.Conditionals,
			Responses:      config.Responses,
			CustomRegistry: mockRegistry,
		})

		plan, err := planner.Plan()
		require.NoError(t, err)

		for id := range config.Responses {
			assert.NotNil(t, plan.steps[requestctx.ResponsesConfigPrefix+id])
			assert.IsType(t, &Response{}, plan.steps[requestctx.ResponsesConfigPrefix+id])
		}
	})
}

func TestPlannerV2_generateActionStep(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockExec := NewMockActionExecutable(ctrl)

		mockRegistry := actions.NewRegistry()
		mockRegistry.ReplaceActionType("action1", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})
		mockRegistry.ReplaceActionType("", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		mockExec.EXPECT().Config().Return("").AnyTimes()

		planner := NewPlannerV2(PlannerConfig{
			Actions:        sampleConfig.Actions,
			Conditions:     sampleConfig.Conditionals,
			Responses:      sampleConfig.Responses,
			CustomRegistry: mockRegistry,
		})
		_, err := planner.generateActionStep("action1")
		require.NoError(t, err)

	})

	t.Run("failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockActionProvider := NewMockActionProvider(ctrl)

		mockRegistry := actions.NewRegistry()

		mockActionProvider.EXPECT().GetActionExecutable(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("error generating action")).AnyTimes()

		planner := NewPlannerV2(PlannerConfig{
			Actions:        sampleConfig.Actions,
			Conditions:     sampleConfig.Conditionals,
			Responses:      sampleConfig.Responses,
			CustomRegistry: mockRegistry,
		})
		_, err := planner.generateActionStep("action2")
		assert.ErrorContains(t, err, "error creating actions")
	})

	t.Run("fail for next step", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockExec := NewMockActionExecutable(ctrl)

		mockRegistry := actions.NewRegistry()
		mockRegistry.ReplaceActionType("action1", func(config json.RawMessage) (actions.ActionExecutable, error) {
			return mockExec, nil
		})

		mockExec.EXPECT().Config().Return("").AnyTimes()

		planner := NewPlannerV2(PlannerConfig{
			Actions:        sampleConfig.Actions,
			Conditions:     sampleConfig.Conditionals,
			Responses:      sampleConfig.Responses,
			CustomRegistry: mockRegistry,
		})
		_, err := planner.generateActionStep("action1")
		assert.ErrorContains(t, err, "not registered")
	})
}

func TestPlannerV2_generateConditionalStep(t *testing.T) {
	config := &apiconfig.APIConfig{
		Conditionals: map[string]apiconfig.Conditional{
			"cond1": {
				Expression:  `{{  (and (email (printf "%s" .email) "email" false) (eq (printf "%s" .field1) (printf "hello" ))) }}`,
				ValidPath:   "$response.success",
				InvalidPath: "$response.failure",
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Code:     200,
				Template: `{"status": "success"}`,
				Type:     "json",
			},
			"failure": {
				Code:     400,
				Template: `{"status": "failure"}`,
				Type:     "json",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := actions.NewRegistry()

	planner := NewPlannerV2(PlannerConfig{
		Actions:        config.Actions,
		Conditions:     config.Conditionals,
		Responses:      config.Responses,
		TerminateTag:   "end",
		CustomRegistry: mockRegistry,
	})

	condition, err := planner.generateConditionalStep("cond1")
	require.NoError(t, err)
	assert.NotNil(t, condition)
	assert.Equal(t, "cond1", condition.id)
	assert.IsType(t, &Response{}, condition.OnValid)
	assert.IsType(t, &Response{}, condition.OnInvalid)

	_, err = planner.generateConditionalStep("nonexistent")
	assert.Error(t, err)
}

func TestPlannerV2_generateResponseStep(t *testing.T) {
	config := &apiconfig.APIConfig{
		Responses: map[string]apiconfig.ResponseConfig{
			"success": {
				Code:     200,
				Template: `{"status": "success"}`,
				Type:     "json",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := actions.NewRegistry()

	planner := NewPlannerV2(PlannerConfig{
		Actions:        config.Actions,
		Conditions:     config.Conditionals,
		Responses:      config.Responses,
		TerminateTag:   "end",
		CustomRegistry: mockRegistry,
	})

	response, err := planner.generateResponseStep("success")
	require.NoError(t, err)
	assert.NotNil(t, response)

	_, err = planner.generateResponseStep("nonexistent")
	assert.Error(t, err)
}
