package plan

import (
	"testing"

	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestConditionStep_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	validStep := &testStep{id: "valid"}
	invalidStep := &testStep{id: "invalid"}

	t.Run("fail", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    validStep,
			OnInvalid:  invalidStep,
			exprString: `{{ email .test "email" }}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value"}, "")
		next, err := condition.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, invalidStep, next)
	})
	t.Run("on empty condition", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    validStep,
			OnInvalid:  invalidStep,
			exprString: "",
		}

		ctx := requestctx2.NewTestContext()
		next, err := condition.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, validStep, next)
	})
	t.Run("on empty template", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    validStep,
			OnInvalid:  invalidStep,
			exprString: "{{ }}",
		}

		ctx := requestctx2.NewTestContext()
		_, err := condition.Execute(ctx)
		require.Error(t, err)
	})
	t.Run("pass", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    validStep,
			OnInvalid:  invalidStep,
			exprString: `{{ email .test "email"}}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value@addition.com"}, "")
		next, err := condition.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, validStep, next)
	})
	t.Run("fail with error", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    validStep,
			OnInvalid:  invalidStep,
			exprString: `{{ email .test "email"}}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value"}, "")
		next, err := condition.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, invalidStep, next)

		errVal, err := requestctx2.GetRequestVariable(ctx, requestctx2.ErrorTagStripped)
		require.NoError(t, err)
		assert.Equal(t, []string{"email is not a valid email address"}, errVal)
	})
}

func TestConditionTemplateFunctions(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	pass := string(hashed)
	variables := map[string]interface{}{
		"email":    "test@gmail.com",
		"emptymap": map[string]interface{}{},
		"map": map[string]interface{}{
			"test": "hello",
		},
		"pass": pass,
	}
	testCases := []struct {
		name                  string
		template              string
		expected              string
		expectValidationError bool
	}{
		{
			name:                  "simple email case",
			template:              `{{ email .email "email" }}`,
			expected:              "true",
			expectValidationError: false,
		},
		{
			name:                  "email not exist",
			template:              `{{ email .mails "email" }}`,
			expectValidationError: true,
		},
		{
			name:     "empty",
			template: `{{ empty .emptymap "field"}}`,
			expected: "true",
		},
		{
			name:     "not empty map",
			template: `{{ not (empty .map "field") }}`,
			expected: "true",
		},
		{
			name:                  "not empty function",
			template:              `{{ notempty .emptymap "map"  }}`,
			expected:              "",
			expectValidationError: true,
		},
		{
			name:     "not empty string",
			template: `{{ not (empty .email "field") }}`,
			expected: "true",
		},
		{
			name:     " fail empty string",
			template: `{{ empty .map.test "field" }}`,
			expected: "false",
		},
		{
			name:     "and combination",
			template: `{{ and (empty .emptymap "fields") (email "test@gmail.com" "email") }}`,
			expected: "true",
		},
		{
			name:     "failed and combination",
			template: `{{ and (empty .emptymap "field") (email "testgmail.com" "email") }}`,
			expected: "false",
		},
		{
			name:     "bcrypt pass",
			template: `{{ bcrypt "password" .pass "password" }}`,
			expected: "true",
		},
		{
			name:     "fail bcrypt pass",
			template: `{{  bcrypt "passworda" .pass "password"  }}`,
			expected: "false",
		},
		{
			name:                  "fail bcrypt pass throw err",
			template:              `{{  bcrypt "passworda" .pass "password" }}`,
			expected:              "",
			expectValidationError: true,
		},
		{
			name:     "or combination",
			template: `{{ or (empty .emptymap "field" ) (email "test" "email" ) }}`,
			expected: "true",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := requestctx2.NewTestContext()
			require.NoError(t, err)

			err = requestctx2.AddRequestVariables(ctx, variables, "")
			require.NoError(t, err)

			validNext := ConditionStep{
				id: "valid",
			}
			invalidNext := ConditionStep{
				id: "invalid",
			}

			cond := ConditionStep{
				id:         "test",
				exprString: testCase.template,
				OnValid:    &validNext,
				OnInvalid:  &invalidNext,
			}

			next, err := cond.Execute(ctx)
			require.NoError(t, err)

			if testCase.expected == "true" {
				assert.Equal(t, &validNext, next)
			} else if testCase.expected == "false" {
				assert.Equal(t, &invalidNext, next)
			}

			if testCase.expectValidationError {
				v, err := requestctx2.GetRequestVariable(ctx, "error")
				require.NoError(t, err)
				assert.NotEmpty(t, v)
			}
		})
	}

}
