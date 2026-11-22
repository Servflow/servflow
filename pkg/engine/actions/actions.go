package actions

import (
	"context"
	"errors"
)

// ErrFailure marks an action error the run can recover from. An action wraps
// it ("%w: invalid token"); the host records the message and takes the
// step's failure branch instead of stopping the run. Any other error stops it.
var ErrFailure = errors.New("action failed")

// ActionExecutable is a built action. It resolves its own templated fields
// against the request context when it runs, with RequestContext.Resolve,
// RequestContext.ResolveBatch or requestctx.ResolveValues.
type ActionExecutable interface {
	Type() string
	Execute(ctx context.Context) (resp interface{}, fields map[string]string, err error)
}
