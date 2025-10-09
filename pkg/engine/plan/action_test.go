package plan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testStep struct {
	id string
}

func (t *testStep) ID() string {
	//TODO implement me
	panic("implement me")
}

func (t *testStep) Execute(ctx context.Context) (Step, error) {
	return nil, nil
}

func TestAction_Execute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Run("success with simple variable", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			conf := fmt.Sprintf("test {{ .%sname }}", requestctx2.BareVariablesPrefixStripped)

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil)

			nextStep := testStep{id: "next"}

			ctx := requestctx2.NewTestContext()
			err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx2.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				configStr: conf,
				exec:      mockExec,
				id:        "test",
				next:      &nextStep,
				out:       fmt.Sprintf("%stest", requestctx2.VariableActionPrefix),
			}

			next, err := act.Execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &nextStep, next)

			field, err := requestctx2.ReplaceVariableValuesInContext(ctx, fmt.Sprintf("{{ .%stest }}", requestctx2.VariableActionPrefix))
			require.NoError(t, err)
			assert.Equal(t, "response string", field)
		})

		t.Run("success with complex variables", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			config := fmt.Sprintf("test {{ .%sname.actualname }}", requestctx2.BareVariablesPrefixStripped)
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil)

			nextStep := testStep{id: "next"}

			ctx := requestctx2.NewTestContext()
			err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{
				fmt.Sprintf("%sname", requestctx2.BareVariablesPrefixStripped): map[string]interface{}{
					"actualname": "actual name",
				},
			}, "")
			require.NoError(t, err)

			act := Action{
				configStr: config,
				exec:      mockExec,
				id:        "test",
				next:      &nextStep,
				out:       fmt.Sprintf("%stest", requestctx2.VariableActionPrefix),
			}

			next, err := act.Execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &nextStep, next)

			field, err := requestctx2.ReplaceVariableValuesInContext(ctx, fmt.Sprintf("{{ .%stest }}", requestctx2.VariableActionPrefix))
			require.NoError(t, err)
			assert.Equal(t, "response string", field)
		})

		t.Run("success with custom out variable", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("custom response", nil)

			nextStep := testStep{id: "next"}

			ctx := requestctx2.NewTestContext()

			act := Action{
				exec: mockExec,
				id:   "test",
				next: &nextStep,
				out:  "custom_out",
			}

			next, err := act.Execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &nextStep, next)

			field, err := requestctx2.ReplaceVariableValuesInContext(ctx, "{{ .custom_out }}")
			require.NoError(t, err)
			assert.Equal(t, "custom response", field)
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Run("error with no fail step", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Type().Return("mock").AnyTimes()
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", errors.New("dummy error"))

			mockStep := testStep{id: "next"}

			ctx := requestctx2.NewTestContext()
			err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx2.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				out:  "field1",
				id:   "test",
				next: &mockStep,
			}

			next, err := act.Execute(ctx)
			assert.Error(t, err)
			assert.Equal(t, next, nil)

		})

		t.Run("error with a fail step", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", errors.New("dummy error"))

			nextStep := testStep{id: "next"}
			failStep := testStep{id: "fail"}

			ctx := requestctx2.NewTestContext()
			err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx2.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				exec: mockExec,
				out:  "field1",
				id:   "test",
				next: &nextStep,
				fail: &failStep,
			}

			next, err := act.Execute(ctx)
			assert.NoError(t, err)
			assert.Equal(t, &failStep, next)
			val, err := requestctx2.GetRequestVariable(ctx, requestctx2.ErrorTagStripped)
			assert.NoError(t, err)
			assert.Equal(t, "dummy error", val)
		})

	})
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
			tmpl, err := requestctx2.CreateTextTemplate(context.Background(), testCase.template, nil)
			require.NoError(t, err)
			var buff bytes.Buffer
			err = tmpl.Execute(&buff, variables)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, buff.String())
		})
	}

}
