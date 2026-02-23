package plan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
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
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): "actual name"}, "")
			require.NoError(t, err)

			act := Action{
				configStr: conf,
				exec:      mockExec,
				id:        "test",
				next:      &stepWrapper{id: "next", step: &nextStep},
				out:       "test",
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
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil)

			nextStep := testStep{id: "next"}

			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, map[string]interface{}{
				fmt.Sprintf("%sname", requestctx.BareVariablesPrefixStripped): map[string]interface{}{
					"actualname": "actual name",
				},
			}, "")
			require.NoError(t, err)

			act := Action{
				configStr: config,
				exec:      mockExec,
				id:        "test",
				next:      &stepWrapper{id: "next", step: &nextStep},
				out:       "test",
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
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("custom response", nil)

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
			mockExec.EXPECT().Execute(gomock.Any(), "").Return(reader, nil)

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
			var buf bytes.Buffer
			_, err = io.Copy(&buf, fileValue.GetReader())
			require.NoError(t, err)
			assert.Equal(t, fileContent, buf.String())
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Run("has error", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExec := NewMockActionExecutable(ctrl)
			mockExec.EXPECT().Type().Return("mock").AnyTimes()
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", errors.New("dummy error"))

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
			mockExec.EXPECT().Execute(gomock.Any(), "").Return("response string", fmt.Errorf("%w: dummy error", ErrFailure)).AnyTimes()

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
			tmpl, err := requestctx.CreateTextTemplate(context.Background(), testCase.template, nil)
			require.NoError(t, err)
			var buff bytes.Buffer
			err = tmpl.Execute(&buff, variables)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, buff.String())
		})
	}

}
