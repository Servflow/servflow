package plan

import (
	"context"
	"fmt"

	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

// EndStep writes values from the lookupKey in the context to the destinationKey
// this is important in making sure groups can write a value to their endtag
type EndStep struct {
	destinationKey string
	lookupKey      string
	endTemplate    string
}

func (e *EndStep) ID() string {
	return "end_" + e.destinationKey
}

// Execute copies the value in the context from the lookupKey to the destinationKey
func (e *EndStep) execute(ctx context.Context) (*stepWrapper, error) {
	if e.destinationKey == "" {
		return nil, nil
	}

	logger := logging.FromContext(ctx)
	if e.lookupKey != "" {
		val, err := requestctx2.GetRequestVariable(ctx, e.lookupKey)
		if err != nil {
			return nil, fmt.Errorf("error getting request variable %q: %v", e.destinationKey, err)
		}

		if err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{e.destinationKey: val}, ""); err != nil {
			return nil, fmt.Errorf("error adding request variable %s to ctx: %v", e.destinationKey, err)
		}
	}
	if e.endTemplate != "" {
		tmpl, err := requestctx2.CreateTextTemplate(ctx, e.endTemplate, nil)
		if err != nil {
			return nil, err
		}
		val, err := requestctx2.ExecuteTemplateFromContext(ctx, tmpl)
		if err != nil {
			return nil, err
		}

		logger.Debug("template generated", zap.String("template", val))
		if err := requestctx2.AddRequestVariables(ctx, map[string]interface{}{e.destinationKey: val}, ""); err != nil {
			return nil, fmt.Errorf("error adding request variable %s to ctx: %v", e.destinationKey, err)
		}
	}

	return nil, nil
}
