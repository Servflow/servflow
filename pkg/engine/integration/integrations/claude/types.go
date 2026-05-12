package claude

import (
	"encoding/json"

	"go.uber.org/zap"
)

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
