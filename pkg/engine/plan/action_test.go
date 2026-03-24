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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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
			mockExec.EXPECT().Config().Return(conf)
			mockExec.EXPECT().Execute(gomock.Any(), "test actual name").Return("response string", nil)

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
			mockExec.EXPECT().Config().Return("")
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
			mockExec.EXPECT().Config().Return("").AnyTimes()
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

func TestAddSpanAttribute(t *testing.T) {
	t.Run("adds attribute when span exists in context", func(t *testing.T) {
		tracer := noop.NewTracerProvider().Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		ctx = withActionSpan(ctx, span)

		ok := AddSpanAttribute(ctx, "test-key", attribute.StringValue("test-value"))
		assert.True(t, ok)
	})

	t.Run("returns false when no span in context", func(t *testing.T) {
		ctx := context.Background()

		ok := AddSpanAttribute(ctx, "test-key", attribute.StringValue("test-value"))
		assert.False(t, ok)
	})

	t.Run("works with different attribute types", func(t *testing.T) {
		tracer := noop.NewTracerProvider().Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		ctx = withActionSpan(ctx, span)

		testCases := []struct {
			name  string
			key   string
			value attribute.Value
		}{
			{"string", "string-key", attribute.StringValue("string-value")},
			{"int", "int-key", attribute.IntValue(42)},
			{"bool", "bool-key", attribute.BoolValue(true)},
			{"float", "float-key", attribute.Float64Value(3.14)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ok := AddSpanAttribute(ctx, tc.key, tc.value)
				assert.True(t, ok)
			})
		}
	})
}

func TestGetActionSpan(t *testing.T) {
	t.Run("returns span when present", func(t *testing.T) {
		tracer := noop.NewTracerProvider().Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		ctx = withActionSpan(ctx, span)

		retrieved, ok := getActionSpan(ctx)
		assert.True(t, ok)
		assert.Equal(t, span, retrieved)
	})

	t.Run("returns false when not present", func(t *testing.T) {
		ctx := context.Background()

		retrieved, ok := getActionSpan(ctx)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})
}

var _ trace.Span = (*noop.Span)(nil)

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
