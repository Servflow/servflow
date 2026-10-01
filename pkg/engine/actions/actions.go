package actions

import (
	"context"
	"errors"
)

// ErrFailure marks an action error the run can recover from. An action wraps
// it ("%w: invalid token"); the host records the message and takes the
// step's failure branch instead of stopping the run. Any other error stops it.
var ErrFailure = errors.New("action failed")

// ActionExecutable is the v1 action interface.
// Actions return their config as a string template, and receive
// the template-resolved config in Execute.
type ActionExecutable interface {
	Config() string
	Execute(ctx context.Context, modifiedConfig string) (resp interface{}, fields map[string]string, err error)
	Type() string
}

// ActionExecutableV2 is the v2 action interface.
// Actions handle their own template resolution using RequestContext.Resolve()
// or RequestContext.ResolveBatch() methods.
type ActionExecutableV2 interface {
	Type() string
	Execute(ctx context.Context) (resp interface{}, fields map[string]string, err error)
}
