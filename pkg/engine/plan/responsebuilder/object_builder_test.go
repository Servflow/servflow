package responsebuilder

import (
	"net/http"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectBuilder_generateValue(t *testing.T) {
	testCases := []struct {
		name      string
		in        apiconfig.ResponseObject
		expected  interface{}
		variables map[string]interface{}
		expectErr bool
	}{
		{
			name: "basic case",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .name}}",
			},
			variables: map[string]interface{}{
				"name": "test",
			},
			expected:  "test",
			expectErr: false,
		},
		{
			name: "basic case without jsonraw",
			in: apiconfig.ResponseObject{
				Value: "{{ .name }}",
			},
			variables: map[string]interface{}{
				"name": "test",
			},
			expected: "test",
		},
		{
			name: "string value",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"status": {
						Value: "success",
					},
				},
			},
			expected:  map[string]interface{}{"status": "success"},
			expectErr: false,
		},
		{
			name: "invalid template",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .name}",
			},
			variables: map[string]interface{}{},
			expectErr: true,
		},
		{
			name: "nested case",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"data": {
						Fields: map[string]apiconfig.ResponseObject{
							"person": {
								Value: "{{ jsonraw .personobject }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"personobject": map[string]interface{}{
					"name": "test",
					"age":  14,
				},
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{
					"person": map[string]interface{}{
						"name": "test",
						"age":  float64(14),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "nested case with and without jsonraw",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"status": {
						Value: "{{ .status }}",
					},
					"data": {
						Fields: map[string]apiconfig.ResponseObject{
							"person": {
								Value: "{{ jsonraw .personobject }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"personobject": map[string]interface{}{
					"name": "test",
					"age":  14,
				},
				"status": "success",
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{
					"person": map[string]interface{}{
						"name": "test",
						"age":  float64(14),
					},
				},
				"status": "success",
			},
			expectErr: false,
		},
		{
			name: "root integer value",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .number }}",
			},
			variables: map[string]interface{}{
				"number": 42,
			},
			expected:  float64(42), // JSON unmarshaling converts numbers to float64
			expectErr: false,
		},
		{
			name: "root boolean value",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .boolean }}",
			},
			variables: map[string]interface{}{
				"boolean": true,
			},
			expected:  true,
			expectErr: false,
		},
		{
			name: "root float value",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .float }}",
			},
			variables: map[string]interface{}{
				"float": 3.14159,
			},
			expected:  3.14159,
			expectErr: false,
		},
		{
			name: "root null value",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .null }}",
			},
			variables: map[string]interface{}{
				"null": nil,
			},
			expected:  nil,
			expectErr: false,
		},
		{
			name: "root array value",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .array }}",
			},
			variables: map[string]interface{}{
				"array": []interface{}{1, "two", true},
			},
			expected:  []interface{}{float64(1), "two", true}, // JSON unmarshaling converts numbers to float64
			expectErr: false,
		},
		{
			name: "nested mixed types",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"string": {
						Value: "{{ jsonraw .string }}",
					},
					"number": {
						Value: "{{ jsonraw .number }}",
					},
					"boolean": {
						Value: "{{ jsonraw .boolean }}",
					},
					"object": {
						Fields: map[string]apiconfig.ResponseObject{
							"nested_string": {
								Value: "{{ jsonraw .nested_string }}",
							},
							"nested_number": {
								Value: "{{ jsonraw .nested_number }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"string":        "string value",
				"number":        42,
				"boolean":       true,
				"nested_string": "nested string value",
				"nested_number": 99,
			},
			expected: map[string]interface{}{
				"string":  "string value",
				"number":  float64(42),
				"boolean": true,
				"object": map[string]interface{}{
					"nested_string": "nested string value",
					"nested_number": float64(99),
				},
			},
			expectErr: false,
		},
		{
			name: "complex nested structure",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"data": {
						Fields: map[string]apiconfig.ResponseObject{
							"users": {
								Value: "{{ jsonraw .users }}",
							},
							"metadata": {
								Fields: map[string]apiconfig.ResponseObject{
									"count": {
										Value: "{{ jsonraw .count }}",
									},
									"pagination": {
										Fields: map[string]apiconfig.ResponseObject{
											"page": {
												Value: "{{ jsonraw .page }}",
											},
											"total_pages": {
												Value: "{{ jsonraw .total_pages }}",
											},
										},
									},
								},
							},
						},
					},
					"success": {
						Value: "{{ jsonraw .success }}",
					},
				},
			},
			variables: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{"id": 1, "name": "Alice"},
					map[string]interface{}{"id": 2, "name": "Bob"},
				},
				"count":       2,
				"page":        1,
				"total_pages": 1,
				"success":     true,
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{"id": float64(1), "name": "Alice"},
						map[string]interface{}{"id": float64(2), "name": "Bob"},
					},
					"metadata": map[string]interface{}{
						"count": float64(2),
						"pagination": map[string]interface{}{
							"page":        float64(1),
							"total_pages": float64(1),
						},
					},
				},
				"success": true,
			},
			expectErr: false,
		},
		{
			name: "empty object at root",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{},
			},
			variables: map[string]interface{}{},
			expected:  nil,
			expectErr: false,
		},
		{
			name: "mixed nested types with arrays",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"items": {
						Value: "{{ jsonraw .items }}",
					},
					"stats": {
						Fields: map[string]apiconfig.ResponseObject{
							"counts": {
								Value: "{{ jsonraw .counts }}",
							},
							"active": {
								Value: "{{ jsonraw .active }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id":    1,
						"valid": true,
						"tags":  []string{"important", "urgent"},
					},
					map[string]interface{}{
						"id":    2,
						"valid": false,
						"tags":  []string{"normal"},
					},
				},
				"counts": map[string]interface{}{
					"total":   2,
					"valid":   1,
					"invalid": 1,
				},
				"active": true,
			},
			expected: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id":    float64(1),
						"valid": true,
						"tags":  []interface{}{"important", "urgent"},
					},
					map[string]interface{}{
						"id":    float64(2),
						"valid": false,
						"tags":  []interface{}{"normal"},
					},
				},
				"stats": map[string]interface{}{
					"counts": map[string]interface{}{
						"total":   float64(2),
						"valid":   float64(1),
						"invalid": float64(1),
					},
					"active": true,
				},
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, tc.variables, "")
			require.NoError(t, err)

			gottenValue, err := generateValue(ctx, &tc.in)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, gottenValue)
		})
	}

}

func TestObjectBuilder(t *testing.T) {
	testCases := []struct {
		name        string
		in          apiconfig.ResponseObject
		expectErr   bool
		compareJson string
		variables   map[string]interface{}
		code        int
	}{
		{
			name: "simple",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"data": {
						Fields: map[string]apiconfig.ResponseObject{
							"name": {
								Value: "{{ jsonraw .name }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"name": "kofo okesola",
			},
			compareJson: `{"data":{"name":"kofo okesola"}}`,
			code:        http.StatusOK,
			expectErr:   false,
		},
		{
			name: "primitive_types",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"string": {
						Value: "{{ jsonraw .string_val }}",
					},
					"number": {
						Value: "{{ jsonraw .number_val }}",
					},
					"boolean": {
						Value: "{{ jsonraw .boolean_val }}",
					},
					"null": {
						Value: "{{ jsonraw .null_val }}",
					},
				},
			},
			variables: map[string]interface{}{
				"string_val":  "hello world",
				"number_val":  42,
				"boolean_val": true,
				"null_val":    nil,
			},
			compareJson: `{
				"string": "hello world",
				"number": 42,
				"boolean": true
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "complex_nested_structure",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"status": {
						Value: "{{ jsonraw .status }}",
					},
					"data": {
						Fields: map[string]apiconfig.ResponseObject{
							"users": {
								Value: "{{ jsonraw .users }}",
							},
							"metadata": {
								Fields: map[string]apiconfig.ResponseObject{
									"total": {
										Value: "{{ jsonraw .total }}",
									},
									"page": {
										Value: "{{ jsonraw .page }}",
									},
									"settings": {
										Fields: map[string]apiconfig.ResponseObject{
											"sort_by": {
												Value: "{{ jsonraw .sort_by }}",
											},
											"filters": {
												Value: "{{ jsonraw .filters }}",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"status": "success",
				"users": []interface{}{
					map[string]interface{}{
						"id":       1,
						"username": "alice",
						"active":   true,
						"tags":     []string{"admin", "verified"},
					},
					map[string]interface{}{
						"id":       2,
						"username": "bob",
						"active":   false,
						"tags":     []string{"verified"},
					},
				},
				"total":   2,
				"page":    1,
				"sort_by": "username",
				"filters": map[string]interface{}{
					"active": true,
					"role":   "admin",
				},
			},
			compareJson: `{
				"status": "success",
				"data": {
					"users": [
						{
							"id": 1,
							"username": "alice",
							"active": true,
							"tags": ["admin", "verified"]
						},
						{
							"id": 2,
							"username": "bob",
							"active": false,
							"tags": ["verified"]
						}
					],
					"metadata": {
						"total": 2,
						"page": 1,
						"settings": {
							"sort_by": "username",
							"filters": {
								"active": true,
								"role": "admin"
							}
						}
					}
				}
			}`,
			code:      http.StatusCreated,
			expectErr: false,
		},
		{
			name: "array_at_root",
			in: apiconfig.ResponseObject{
				Value: "{{ jsonraw .items }}",
			},
			variables: map[string]interface{}{
				"items": []interface{}{
					"first",
					"second",
					"third",
				},
			},
			compareJson: `["first", "second", "third"]`,
			code:        http.StatusOK,
			expectErr:   false,
		},
		{
			name: "error_status_with_message",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"error": {
						Value: "{{ jsonraw .error_message }}",
					},
					"code": {
						Value: "{{ jsonraw .error_code }}",
					},
					"details": {
						Value: "{{ jsonraw .details }}",
					},
				},
			},
			variables: map[string]interface{}{
				"error_message": "Resource not found",
				"error_code":    "NOT_FOUND",
				"details":       map[string]interface{}{"resource_id": "12345", "resource_type": "user"},
			},
			compareJson: `{
				"error": "Resource not found",
				"code": "NOT_FOUND",
				"details": {
					"resource_id": "12345",
					"resource_type": "user"
				}
			}`,
			code:      http.StatusNotFound,
			expectErr: false,
		},
		{
			name: "mixed_types_and_nesting",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"id": {
						Value: "{{ jsonraw .id }}",
					},
					"properties": {
						Fields: map[string]apiconfig.ResponseObject{
							"name": {
								Value: "{{ jsonraw .name }}",
							},
							"attributes": {
								Value: "{{ jsonraw .attributes }}",
							},
						},
					},
					"tags": {
						Value: "{{ jsonraw .tags }}",
					},
					"created_at": {
						Value: "{{ jsonraw .created_at }}",
					},
					"counts": {
						Fields: map[string]apiconfig.ResponseObject{
							"views": {
								Value: "{{ jsonraw .view_count }}",
							},
							"likes": {
								Value: "{{ jsonraw .like_count }}",
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"id":   "abcd-1234-5678",
				"name": "Example Item",
				"attributes": map[string]interface{}{
					"color": "blue",
					"size":  "medium",
					"features": []interface{}{
						"waterproof",
						"durable",
					},
				},
				"tags":       []string{"featured", "new", "seasonal"},
				"created_at": "2023-10-15T12:00:00Z",
				"view_count": 1250,
				"like_count": 43,
			},
			compareJson: `{
				"id": "abcd-1234-5678",
				"properties": {
					"name": "Example Item",
					"attributes": {
						"color": "blue",
						"size": "medium",
						"features": ["waterproof", "durable"]
					}
				},
				"tags": ["featured", "new", "seasonal"],
				"created_at": "2023-10-15T12:00:00Z",
				"counts": {
					"views": 1250,
					"likes": 43
				}
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "empty_fields",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"data": {
						Fields: map[string]apiconfig.ResponseObject{},
					},
					"status": {
						Value: "{{ jsonraw .status }}",
					},
				},
			},
			variables: map[string]interface{}{
				"status": "success",
			},
			compareJson: `{
				"status": "success"
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "template_error",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"result": {
						Value: "{{ jsonraws .missing_var }}",
					},
				},
			},
			variables:   map[string]interface{}{},
			expectErr:   true,
			code:        http.StatusInternalServerError,
			compareJson: "",
		},
		{
			name: "deeply_nested_structure",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"level1": {
						Fields: map[string]apiconfig.ResponseObject{
							"level2": {
								Fields: map[string]apiconfig.ResponseObject{
									"level3": {
										Fields: map[string]apiconfig.ResponseObject{
											"level4": {
												Fields: map[string]apiconfig.ResponseObject{
													"level5": {
														Value: "{{ jsonraw .deep_value }}",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			variables: map[string]interface{}{
				"deep_value": "found me!",
			},
			compareJson: `{
				"level1": {
					"level2": {
						"level3": {
							"level4": {
								"level5": "found me!"
							}
						}
					}
				}
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "empty_value_string",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"empty_string": {
						Value: "{{ jsonraw .empty_string }}",
					},
					"normal_string": {
						Value: "{{ jsonraw .normal_string }}",
					},
				},
			},
			variables: map[string]interface{}{
				"empty_string":  "",
				"normal_string": "not empty",
			},
			compareJson: `{
				"empty_string": "",
				"normal_string": "not empty"
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "numeric_values_and_formats",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"integer": {
						Value: "{{ jsonraw .integer }}",
					},
					"negative": {
						Value: "{{ jsonraw .negative }}",
					},
					"float": {
						Value: "{{ jsonraw .float }}",
					},
					"scientific": {
						Value: "{{ jsonraw .scientific }}",
					},
					"zero": {
						Value: "{{ jsonraw .zero }}",
					},
				},
			},
			variables: map[string]interface{}{
				"integer":    42,
				"negative":   -10,
				"float":      3.14159,
				"scientific": 1.23e-4,
				"zero":       0,
			},
			compareJson: `{
				"integer": 42,
				"negative": -10,
				"float": 3.14159,
				"scientific": 0.000123,
				"zero": 0
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "different_http_status_codes",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"message": {
						Value: "{{ jsonraw .message }}",
					},
				},
			},
			variables: map[string]interface{}{
				"message": "Created successfully",
			},
			compareJson: `{
				"message": "Created successfully"
			}`,
			code:      http.StatusCreated,
			expectErr: false,
		},
		{
			name: "unauthorized_status",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"error": {
						Value: "{{ jsonraw .error }}",
					},
				},
			},
			variables: map[string]interface{}{
				"error": "Unauthorized access",
			},
			compareJson: `{
				"error": "Unauthorized access"
			}`,
			code:      http.StatusUnauthorized,
			expectErr: false,
		},
		{
			name: "forbidden_status",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"error": {
						Value: "{{ jsonraw .error }}",
					},
					"details": {
						Value: "{{ jsonraw .details }}",
					},
				},
			},
			variables: map[string]interface{}{
				"error":   "Access forbidden",
				"details": "You do not have permission to access this resource",
			},
			compareJson: `{
				"error": "Access forbidden",
				"details": "You do not have permission to access this resource"
			}`,
			code:      http.StatusForbidden,
			expectErr: false,
		},
		{
			name: "bad_request_status",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"error": {
						Value: "{{ jsonraw .error }}",
					},
					"validation_errors": {
						Value: "{{ jsonraw .validation_errors }}",
					},
				},
			},
			variables: map[string]interface{}{
				"error": "Invalid request parameters",
				"validation_errors": []interface{}{
					map[string]interface{}{
						"field": "email",
						"error": "Invalid email format",
					},
					map[string]interface{}{
						"field": "age",
						"error": "Must be a positive number",
					},
				},
			},
			compareJson: `{
				"error": "Invalid request parameters",
				"validation_errors": [
					{
						"field": "email",
						"error": "Invalid email format"
					},
					{
						"field": "age",
						"error": "Must be a positive number"
					}
				]
			}`,
			code:      http.StatusBadRequest,
			expectErr: false,
		},
		{
			name: "server_error_status",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"error": {
						Value: "{{ jsonraw .error }}",
					},
					"request_id": {
						Value: "{{ jsonraw .request_id }}",
					},
				},
			},
			variables: map[string]interface{}{
				"error":      "Internal server error",
				"request_id": "req-12345-abcde",
			},
			compareJson: `{
				"error": "Internal server error",
				"request_id": "req-12345-abcde"
			}`,
			code:      http.StatusInternalServerError,
			expectErr: false,
		},
		{
			name: "special_json_characters",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"escaped_quotes": {
						Value: "{{ jsonraw .escaped_quotes }}",
					},
					"path": {
						Value: "{{ jsonraw .path }}",
					},
					"unicode": {
						Value: "{{ jsonraw .unicode }}",
					},
					"newlines": {
						Value: "{{ jsonraw .newlines }}",
					},
				},
			},
			variables: map[string]interface{}{
				"escaped_quotes": "Text with \"quoted\" content",
				"path":           "C:\\Program Files\\App\\file.txt",
				"unicode":        "Unicode: 你好, 世界! ñáéíóú",
				"newlines":       "Line 1\nLine 2\r\nLine 3",
			},
			compareJson: `{
				"escaped_quotes": "Text with \"quoted\" content",
				"path": "C:\\Program Files\\App\\file.txt",
				"unicode": "Unicode: 你好, 世界! ñáéíóú",
				"newlines": "Line 1\nLine 2\r\nLine 3"
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "empty_value_fields",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"empty_value": {
						Value: "",
					},
					"non_empty": {
						Value: "{{ jsonraw .non_empty }}",
					},
				},
			},
			variables: map[string]interface{}{
				"non_empty": "some value",
			},
			compareJson: `{
				"non_empty": "some value"
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
		{
			name: "large_number_values",
			in: apiconfig.ResponseObject{
				Fields: map[string]apiconfig.ResponseObject{
					"large_integer": {
						Value: "{{ jsonraw .large_integer }}",
					},
					"max_integer": {
						Value: "{{ jsonraw .max_integer }}",
					},
					"min_integer": {
						Value: "{{ jsonraw .min_integer }}",
					},
				},
			},
			variables: map[string]interface{}{
				"large_integer": 9223372036854775807,     // Max int64
				"max_integer":   1.7976931348623157e+308, // Max float64
				"min_integer":   -9223372036854775808,    // Min int64
			},
			compareJson: `{
				"large_integer": 9223372036854775807,
				"max_integer": 1.7976931348623157e+308,
				"min_integer": -9223372036854775808
			}`,
			code:      http.StatusOK,
			expectErr: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestctx.NewTestContext()
			err := requestctx.AddRequestVariables(ctx, tc.variables, "")

			require.NoError(t, err)
			builder := NewObjectBuilder(&tc.in, tc.code)

			sfResponse, err := builder.BuildResponse(ctx)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.JSONEq(t, tc.compareJson, string(sfResponse.Body))
			assert.Equal(t, tc.code, sfResponse.Code)
		})
	}
}
