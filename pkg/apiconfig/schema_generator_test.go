package apiconfig

import (
	"encoding/json"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIConfigSchema(t *testing.T) {
	// Register a test action for schema generation
	testFields := map[string]actions.FieldInfo{
		"url": {
			Type:     actions.FieldTypeString,
			Required: true,
		},
		"method": {
			Type:     actions.FieldTypeString,
			Required: false,
			Default:  "GET",
			Values:   []string{"GET", "POST", "PUT", "DELETE"},
		},
		"headers": {
			Type:     actions.FieldTypeMap,
			Required: false,
		},
		"enabled": {
			Type:     actions.FieldTypeBoolean,
			Required: false,
			Default:  true,
		},
	}

	err := actions.RegisterAction("test-schema-action", actions.ActionRegistrationInfo{
		Name:        "Test Schema Action",
		Description: "Test action for schema generation",
		Fields:      testFields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil // Not needed for schema tests
		},
	})
	require.NoError(t, err)

	schema, err := GenerateAPIConfigSchema()
	require.NoError(t, err)
	assert.NotNil(t, schema)

	// Verify schema structure
	assert.Equal(t, "http://json-schema.org/draft-07/schema#", schema["$schema"])
	assert.Equal(t, "apiconfig.json", schema["$id"])

	// Verify definitions exist
	definitions, ok := schema["definitions"].(map[string]interface{})
	require.True(t, ok, "definitions should be a map")

	// Verify Action definition exists and has oneOf structure
	actionDef, ok := definitions["Action"].(map[string]interface{})
	require.True(t, ok, "Action definition should exist")

	oneOf, ok := actionDef["oneOf"].([]interface{})
	require.True(t, ok, "Action should have oneOf structure")
	assert.Greater(t, len(oneOf), 0, "Should have at least one action schema")

	// Find our test action schema
	var testActionSchema map[string]interface{}
	for _, actionSchemaInterface := range oneOf {
		actionSchema := actionSchemaInterface.(map[string]interface{})
		props := actionSchema["properties"].(map[string]interface{})
		typeField := props["type"].(map[string]interface{})
		if typeField["const"] == "test-schema-action" {
			testActionSchema = actionSchema
			break
		}
	}
	require.NotNil(t, testActionSchema, "Test action schema should exist")

	// Verify test action schema structure
	props := testActionSchema["properties"].(map[string]interface{})
	configSchema := props["config"].(map[string]interface{})
	configProps := configSchema["properties"].(map[string]interface{})

	// Verify required string field
	urlField := configProps["url"].(map[string]interface{})
	assert.Equal(t, "string", urlField["type"])

	// Verify optional enum field with default
	methodField := configProps["method"].(map[string]interface{})
	assert.Equal(t, "string", methodField["type"])
	assert.Equal(t, "GET", methodField["default"])

	enumVals, ok := methodField["enum"].([]interface{})
	require.True(t, ok, "enum should be []interface{}")
	expectedEnums := []string{"GET", "POST", "PUT", "DELETE"}
	for i, expected := range expectedEnums {
		assert.Equal(t, expected, enumVals[i].(string))
	}

	// Verify map field
	headersField := configProps["headers"].(map[string]interface{})
	assert.Equal(t, "object", headersField["type"])

	// Verify boolean field with default
	enabledField := configProps["enabled"].(map[string]interface{})
	assert.Equal(t, "boolean", enabledField["type"])
	assert.Equal(t, true, enabledField["default"])

	// Verify required fields
	required, ok := configSchema["required"].([]interface{})
	require.True(t, ok, "required should be []interface{}")

	requiredStrs := make([]string, len(required))
	for i, r := range required {
		requiredStrs[i] = r.(string)
	}
	assert.Contains(t, requiredStrs, "url")
	assert.NotContains(t, requiredStrs, "method")
}

func TestSchemaValidation(t *testing.T) {
	// Register test action
	err := actions.RegisterAction("test-validation-action", actions.ActionRegistrationInfo{
		Name:        "Test Validation Action",
		Description: "Test action for validation",
		Fields: map[string]actions.FieldInfo{
			"required_field": {
				Type:     actions.FieldTypeString,
				Required: true,
			},
			"optional_field": {
				Type:     actions.FieldTypeBoolean,
				Required: false,
				Default:  false,
			},
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	schema, err := GenerateAPIConfigSchema()
	require.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	err = compiler.AddResource("test-schema.json", schema)
	require.NoError(t, err)

	compiledSchema, err := compiler.Compile("test-schema.json")
	require.NoError(t, err)

	t.Run("valid config passes validation", func(t *testing.T) {
		validConfig := map[string]interface{}{
			"id": "test-api",
			"actions": map[string]interface{}{
				"action1": map[string]interface{}{
					"type": "test-validation-action",
					"config": map[string]interface{}{
						"required_field": "test value",
						"optional_field": true,
					},
				},
			},
			"http": map[string]interface{}{
				"listenPath": "/test",
				"method":     "POST",
				"next":       "action1",
			},
			"mcpTool": map[string]interface{}{
				"name":  "test",
				"start": "action1",
			},
		}

		err := compiledSchema.Validate(validConfig)
		assert.NoError(t, err)
	})

	t.Run("missing required field fails validation", func(t *testing.T) {
		invalidConfig := map[string]interface{}{
			"id": "test-api",
			"actions": map[string]interface{}{
				"action1": map[string]interface{}{
					"type": "test-validation-action",
					"config": map[string]interface{}{
						"optional_field": true,
						// missing required_field
					},
				},
			},
			"http": map[string]interface{}{
				"listenPath": "/test",
				"method":     "POST",
				"next":       "action1",
			},
			"mcpTool": map[string]interface{}{
				"name":  "test",
				"start": "action1",
			},
		}

		err := compiledSchema.Validate(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required_field")
	})

	t.Run("wrong action type fails validation", func(t *testing.T) {
		invalidConfig := map[string]interface{}{
			"id": "test-api",
			"actions": map[string]interface{}{
				"action1": map[string]interface{}{
					"type": "non-existent-action",
					"config": map[string]interface{}{
						"some_field": "value",
					},
				},
			},
			"http": map[string]interface{}{
				"listenPath": "/test",
				"method":     "POST",
				"next":       "action1",
			},
			"mcpTool": map[string]interface{}{
				"name":  "test",
				"start": "action1",
			},
		}

		err := compiledSchema.Validate(invalidConfig)
		assert.Error(t, err)
	})

	t.Run("APIConfig.Validate with valid config", func(t *testing.T) {
		config := &APIConfig{
			ID: "test-integration",
			Actions: map[string]Action{
				"step1": {
					Type: "test-validation-action",
					Config: map[string]interface{}{
						"required_field": "test",
						"optional_field": false,
					},
				},
			},
			HttpConfig: HttpConfig{
				ListenPath: "/test",
				Method:     "GET",
				Next:       "step1",
			},
			McpTool: MCPToolConfig{
				Name:  "test-tool",
				Start: "step1",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("APIConfig.Validate with invalid config", func(t *testing.T) {
		config := &APIConfig{
			ID: "test-integration",
			Actions: map[string]Action{
				"step1": {
					Type: "test-validation-action",
					Config: map[string]interface{}{
						// missing required "required_field"
						"optional_field": true,
					},
				},
			},
			HttpConfig: HttpConfig{
				ListenPath: "/test",
				Method:     "GET",
				Next:       "step1",
			},
			McpTool: MCPToolConfig{
				Name:  "test-tool",
				Start: "step1",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestFieldTypeConversion(t *testing.T) {
	tests := []struct {
		name      string
		fieldInfo actions.FieldInfo
		expected  map[string]interface{}
	}{
		{
			name: "string field",
			fieldInfo: actions.FieldInfo{
				Type: actions.FieldTypeString,
			},
			expected: map[string]interface{}{
				"type": "string",
			},
		},
		{
			name: "boolean field with default",
			fieldInfo: actions.FieldInfo{
				Type:    actions.FieldTypeBoolean,
				Default: false,
			},
			expected: map[string]interface{}{
				"type":    "boolean",
				"default": false,
			},
		},
		{
			name: "string field with enum values",
			fieldInfo: actions.FieldInfo{
				Type:   actions.FieldTypeString,
				Values: []string{"option1", "option2", "option3"},
			},
			expected: map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"option1", "option2", "option3"},
			},
		},
		{
			name: "map field",
			fieldInfo: actions.FieldInfo{
				Type: actions.FieldTypeMap,
			},
			expected: map[string]interface{}{
				"type": "object",
			},
		},
		{
			name: "integration field",
			fieldInfo: actions.FieldInfo{
				Type: actions.FieldTypeIntegration,
			},
			expected: map[string]interface{}{
				"type": "string",
			},
		},
		{
			name: "array field",
			fieldInfo: actions.FieldInfo{
				Type: "array",
			},
			expected: map[string]interface{}{
				"type": "array",
			},
		},
		{
			name: "unknown field type defaults to object",
			fieldInfo: actions.FieldInfo{
				Type: "unknown",
			},
			expected: map[string]interface{}{
				"type": "object",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertFieldInfoToSchema(tt.fieldInfo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchemaGeneration(t *testing.T) {
	t.Run("generateActionSchemas with registered actions", func(t *testing.T) {
		actionSchemas, err := generateActionSchemas()
		require.NoError(t, err)

		require.Contains(t, actionSchemas, "definitions", "Should have definitions")
		definitions, ok := actionSchemas["definitions"].(map[string]interface{})
		require.True(t, ok, "definitions should be a map")

		require.Contains(t, definitions, "Action", "Should have Action definition")
		actionDef, ok := definitions["Action"].(map[string]interface{})
		require.True(t, ok, "Action definition should be a map")

		// Should have proper action schema structure
		assert.Equal(t, "object", actionDef["type"])

		// Since we have registered actions in the test environment, should have oneOf
		oneOf, hasOneOf := actionDef["oneOf"]
		require.True(t, hasOneOf, "Should have oneOf structure when actions are registered")
		assert.NotNil(t, oneOf, "Should have oneOf array")
	})

	t.Run("buildConfigSchema", func(t *testing.T) {
		fields := map[string]actions.FieldInfo{
			"required_string": {
				Type:     actions.FieldTypeString,
				Required: true,
			},
			"optional_bool": {
				Type:     actions.FieldTypeBoolean,
				Required: false,
				Default:  true,
			},
			"enum_field": {
				Type:     actions.FieldTypeString,
				Required: true,
				Values:   []string{"val1", "val2"},
			},
		}

		schema := buildConfigSchema(fields)

		assert.Equal(t, "object", schema["type"])
		assert.Equal(t, false, schema["additionalProperties"])

		required, ok := schema["required"].([]interface{})
		require.True(t, ok, "required should be []interface{}")

		requiredStrs := make([]string, len(required))
		for i, r := range required {
			requiredStrs[i] = r.(string)
		}
		assert.Contains(t, requiredStrs, "required_string")
		assert.Contains(t, requiredStrs, "enum_field")
		assert.NotContains(t, requiredStrs, "optional_bool")

		props := schema["properties"].(map[string]interface{})

		// Check required string field
		requiredString := props["required_string"].(map[string]interface{})
		assert.Equal(t, "string", requiredString["type"])

		// Check optional boolean field with default
		optionalBool := props["optional_bool"].(map[string]interface{})
		assert.Equal(t, "boolean", optionalBool["type"])
		assert.Equal(t, true, optionalBool["default"])

		// Check enum field
		enumField := props["enum_field"].(map[string]interface{})
		assert.Equal(t, "string", enumField["type"])

		enumVals, ok := enumField["enum"].([]interface{})
		require.True(t, ok, "enum should be []interface{}")
		assert.Equal(t, "val1", enumVals[0].(string))
		assert.Equal(t, "val2", enumVals[1].(string))
	})
}
