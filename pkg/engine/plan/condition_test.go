package plan

import (
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
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
			OnValid:    &stepWrapper{id: "valid", step: validStep},
			OnInvalid:  &stepWrapper{id: "invalid", step: invalidStep},
			exprString: `{{ email .test "email" }}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value"}, "")
		next, err := condition.execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "invalid", step: invalidStep}, next)
	})
	t.Run("on empty condition", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    &stepWrapper{id: "valid", step: validStep},
			OnInvalid:  &stepWrapper{id: "invalid", step: invalidStep},
			exprString: "",
		}

		ctx := requestctx2.NewTestContext()
		next, err := condition.execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "valid", step: validStep}, next)
	})
	t.Run("on empty template", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    &stepWrapper{id: "valid", step: validStep},
			OnInvalid:  &stepWrapper{id: "invalid", step: invalidStep},
			exprString: "{{ }}",
		}

		ctx := requestctx2.NewTestContext()
		_, err := condition.execute(ctx)
		require.Error(t, err)
	})
	t.Run("pass", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    &stepWrapper{id: "valid", step: validStep},
			OnInvalid:  &stepWrapper{id: "invalid", step: invalidStep},
			exprString: `{{ email .test "email"}}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value@addition.com"}, "")
		next, err := condition.execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "valid", step: validStep}, next)
	})
	t.Run("fail with error", func(t *testing.T) {
		condition := ConditionStep{
			OnValid:    &stepWrapper{id: "valid", step: validStep},
			OnInvalid:  &stepWrapper{id: "invalid", step: invalidStep},
			exprString: `{{ email .test "email"}}`,
		}

		ctx := requestctx2.NewTestContext()
		requestctx2.AddRequestVariables(ctx, map[string]interface{}{"test": "value"}, "")
		next, err := condition.execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, &stepWrapper{id: "invalid", step: invalidStep}, next)

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
				OnValid:    &stepWrapper{id: "valid", step: &validNext},
				OnInvalid:  &stepWrapper{id: "invalid", step: &invalidNext},
			}

			next, err := cond.execute(ctx)
			require.NoError(t, err)

			if testCase.expected == "true" {
				assert.Equal(t, &stepWrapper{id: "valid", step: &validNext}, next)
			} else if testCase.expected == "false" {
				assert.Equal(t, &stepWrapper{id: "invalid", step: &invalidNext}, next)
			}

			if testCase.expectValidationError {
				v, err := requestctx2.GetRequestVariable(ctx, "error")
				require.NoError(t, err)
				assert.NotEmpty(t, v)
			}
		})
	}

}

func TestGenerateConditionItemTemplate(t *testing.T) {
	testCases := []struct {
		name     string
		item     apiconfig.ConditionItem
		expected string
		hasError bool
	}{
		{
			name: "email function",
			item: apiconfig.ConditionItem{
				Content:  ".email",
				Function: FunctionEmail,
				Title:    "Email Address",
			},
			expected: "email (.email) (\"Email Address\")",
		},
		{
			name: "notempty function",
			item: apiconfig.ConditionItem{
				Content:  ".username",
				Function: FunctionNotempty,
				Title:    "Username",
			},
			expected: "notempty (.username) (\"Username\")",
		},
		{
			name: "empty function",
			item: apiconfig.ConditionItem{
				Content:  ".field",
				Function: FunctionEmpty,
				Title:    "Field",
			},
			expected: "empty (.field) (\"Field\")",
		},
		{
			name: "bcrypt function",
			item: apiconfig.ConditionItem{
				Content:    ".password",
				Comparison: ".storedHash",
				Function:   FunctionBcrypt,
				Title:      "Password",
			},
			expected: "bcrypt (.password) (.storedHash) (\"Password\")",
		},
		{
			name: "bcrypt missing comparison",
			item: apiconfig.ConditionItem{
				Content:  ".password",
				Function: FunctionBcrypt,
				Title:    "Password",
			},
			hasError: true,
		},
		{
			name: "missing title",
			item: apiconfig.ConditionItem{
				Content:  ".email",
				Function: FunctionEmail,
			},
			expected: "email (.email) (\"field\")",
		},
		{
			name: "invalid function",
			item: apiconfig.ConditionItem{
				Content:  ".field",
				Function: "invalid",
				Title:    "Field",
			},
			hasError: true,
		},
		{
			name: "eq function",
			item: apiconfig.ConditionItem{
				Content:    ".status",
				Comparison: "\"active\"",
				Function:   FunctionEq,
			},
			expected: "eq (.status) (\"active\")",
		},
		{
			name: "ne function",
			item: apiconfig.ConditionItem{
				Content:    ".role",
				Comparison: "\"admin\"",
				Function:   FunctionNe,
			},
			expected: "ne (.role) (\"admin\")",
		},
		{
			name: "lt function",
			item: apiconfig.ConditionItem{
				Content:    ".age",
				Comparison: "18",
				Function:   FunctionLt,
			},
			expected: "lt (.age) (18)",
		},
		{
			name: "le function",
			item: apiconfig.ConditionItem{
				Content:    ".score",
				Comparison: "100",
				Function:   FunctionLe,
			},
			expected: "le (.score) (100)",
		},
		{
			name: "gt function",
			item: apiconfig.ConditionItem{
				Content:    ".price",
				Comparison: "50",
				Function:   FunctionGt,
			},
			expected: "gt (.price) (50)",
		},
		{
			name: "ge function",
			item: apiconfig.ConditionItem{
				Content:    ".quantity",
				Comparison: "1",
				Function:   FunctionGe,
			},
			expected: "ge (.quantity) (1)",
		},
		{
			name: "eq missing comparison",
			item: apiconfig.ConditionItem{
				Content:  ".status",
				Function: FunctionEq,
			},
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := generateConditionItemTemplate(tc.item)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestConvertStructureToTemplate(t *testing.T) {
	testCases := []struct {
		name      string
		structure [][]apiconfig.ConditionItem
		expected  string
		hasError  bool
	}{
		{
			name:      "empty structure",
			structure: [][]apiconfig.ConditionItem{},
			expected:  TemplateFalse,
		},
		{
			name: "single condition",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".email", Function: FunctionEmail, Title: "Email"},
				},
			},
			expected: "{{ email (.email) (\"Email\") }}",
		},
		{
			name: "single AND group",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".email", Function: FunctionEmail, Title: "Email"},
					{Content: ".username", Function: FunctionNotempty, Title: "Username"},
				},
			},
			expected: "{{ and (email (.email) (\"Email\")) (notempty (.username) (\"Username\")) }}",
		},
		{
			name: "multiple OR groups",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".email", Function: FunctionEmail, Title: "Email"},
				},
				{
					{Content: ".adminToken", Function: FunctionNotempty, Title: "Admin Token"},
				},
			},
			expected: "{{ or (email (.email) (\"Email\")) (notempty (.adminToken) (\"Admin Token\")) }}",
		},
		{
			name: "complex DNF",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".email", Function: FunctionEmail, Title: "Email"},
					{Content: ".password", Comparison: ".hash", Function: FunctionBcrypt, Title: "Password"},
				},
				{
					{Content: ".adminField", Function: FunctionEmpty, Title: "Admin Field"},
				},
			},
			expected: "{{ or (and (email (.email) (\"Email\")) (bcrypt (.password) (.hash) (\"Password\"))) (empty (.adminField) (\"Admin Field\")) }}",
		},
		{
			name: "invalid function in structure",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".field", Function: "invalid", Title: "Field"},
				},
			},
			hasError: true,
		},
		{
			name: "comparison functions",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".status", Comparison: "\"active\"", Function: FunctionEq},
					{Content: ".age", Comparison: "18", Function: FunctionGt},
				},
			},
			expected: "{{ and (eq (.status) (\"active\")) (gt (.age) (18)) }}",
		},
		{
			name: "mixed validation and comparison functions",
			structure: [][]apiconfig.ConditionItem{
				{
					{Content: ".email", Function: FunctionEmail, Title: "Email"},
					{Content: ".role", Comparison: "\"admin\"", Function: FunctionEq},
				},
				{
					{Content: ".balance", Comparison: "0", Function: FunctionGt},
				},
			},
			expected: "{{ or (and (email (.email) (\"Email\")) (eq (.role) (\"admin\"))) (gt (.balance) (0)) }}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ConvertStructureToTemplate(tc.structure)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
