package apiconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
)

//go:embed apiconfig_schema.json
var baseSchemaData string

// GenerateAPIConfigSchema returns a complete JSON schema for APIConfig
// that includes validation for all currently registered actions
func GenerateAPIConfigSchema() (map[string]interface{}, error) {
	baseSchema, err := loadBaseSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load base schema: %w", err)
	}

	actionSchemas, err := generateActionSchemas()
	if err != nil {
		return nil, fmt.Errorf("failed to generate action schemas: %w", err)
	}

	return mergeActionSchemas(baseSchema, actionSchemas), nil
}

func loadBaseSchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(baseSchemaData), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse base schema: %w", err)
	}
	return schema, nil
}

func generateActionSchemas() (map[string]interface{}, error) {
	actionTypes := actions.GetRegisteredActionTypes()
	if len(actionTypes) == 0 {
		// Return minimal action schema if no actions registered
		return map[string]interface{}{
			"definitions": map[string]interface{}{
				"Action": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":   map[string]interface{}{"type": "string", "minLength": 1},
						"config": map[string]interface{}{"type": []interface{}{"object", "null"}},
						"next":   map[string]interface{}{"type": "string"},
						"fail":   map[string]interface{}{"type": "string"},
					},
					"required":             []interface{}{"type"},
					"additionalProperties": false,
				},
			},
		}, nil
	}

	var oneOfSchemas []map[string]interface{}

	for _, actionType := range actionTypes {
		fields, err := actions.GetFieldsForAction(actionType)
		if err != nil {
			return nil, fmt.Errorf("failed to get fields for action %s: %w", actionType, err)
		}

		configSchema := buildConfigSchema(fields)

		actionSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type":   map[string]interface{}{"const": actionType},
				"config": configSchema,
				"next":   map[string]interface{}{"type": "string"},
				"fail":   map[string]interface{}{"type": "string"},
			},
			"required":             []interface{}{"type"},
			"additionalProperties": false,
		}

		oneOfSchemas = append(oneOfSchemas, actionSchema)
	}

	// Convert oneOf schemas to proper interface{} slice for JSON schema
	oneOfInterface := make([]interface{}, len(oneOfSchemas))
	for i, schema := range oneOfSchemas {
		oneOfInterface[i] = schema
	}

	actionDefinition := map[string]interface{}{
		"type":     "object",
		"oneOf":    oneOfInterface,
		"required": []interface{}{"type"},
	}

	return map[string]interface{}{
		"definitions": map[string]interface{}{
			"Action": actionDefinition,
		},
	}, nil
}

func buildConfigSchema(fields map[string]actions.FieldInfo) map[string]interface{} {
	properties := make(map[string]interface{})
	var required []interface{}

	for fieldName, fieldInfo := range fields {
		properties[fieldName] = convertFieldInfoToSchema(fieldInfo)

		if fieldInfo.Required {
			required = append(required, fieldName)
		}
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func convertFieldInfoToSchema(fieldInfo actions.FieldInfo) map[string]interface{} {
	schema := make(map[string]interface{})

	switch fieldInfo.Type {
	case actions.FieldTypeString:
		schema["type"] = "string"
		// Add minLength for required fields to prevent empty strings
		if fieldInfo.Required {
			schema["minLength"] = 1
		}
	case actions.FieldTypeBoolean:
		schema["type"] = "boolean"
	case actions.FieldTypeMap:
		schema["type"] = "object"
	case actions.FieldTypeIntegration:
		schema["type"] = "string"
	case actions.FieldTypeFile:
		schema["type"] = "string"
	case actions.FieldTypeTextArea:
		schema["type"] = "string"
		// Add minLength for required fields to prevent empty strings
		if fieldInfo.Required {
			schema["minLength"] = 1
		}
	case "array":
		schema["type"] = "array"
	default:
		// Default to object for unknown types
		schema["type"] = "object"
	}

	if fieldInfo.Default != nil {
		schema["default"] = fieldInfo.Default
	}

	if len(fieldInfo.Values) > 0 {
		enumValues := make([]interface{}, len(fieldInfo.Values))
		for i, v := range fieldInfo.Values {
			enumValues[i] = v
		}
		schema["enum"] = enumValues
	}

	return schema
}

func mergeActionSchemas(baseSchema, actionSchemas map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base schema
	for k, v := range baseSchema {
		result[k] = v
	}

	// Merge action definitions
	if actionDefs, ok := actionSchemas["definitions"]; ok {
		if baseDefs, ok := result["definitions"]; ok {
			if baseDefsMap, ok := baseDefs.(map[string]interface{}); ok {
				if actionDefsMap, ok := actionDefs.(map[string]interface{}); ok {
					for k, v := range actionDefsMap {
						baseDefsMap[k] = v
					}
				}
			}
		} else {
			result["definitions"] = actionDefs
		}
	}

	return result
}
