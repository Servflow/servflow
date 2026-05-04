package claude

import (
	"encoding/json"

	"go.uber.org/zap"
)

func marshalArguments(logger *zap.Logger, args map[string]interface{}) string {
	if args == nil {
		return "{}"
	}
	data, err := json.Marshal(args)
	if err != nil {
		logger.Warn("Failed to marshal arguments", zap.Error(err))
		return "{}"
	}
	return string(data)
}

func unmarshalArguments(logger *zap.Logger, arguments string) map[string]interface{} {
	if arguments == "" {
		return make(map[string]interface{})
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		logger.Warn("Failed to unmarshal arguments", zap.Error(err))
		return make(map[string]interface{})
	}
	return args
}
