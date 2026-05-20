package actions

import (
	"context"
	"errors"
)

var ErrorFatal = errors.New("fatal error")

// ActionExecutable is the v1 action interface.
// Actions return their config as a string template, and receive
// the template-resolved config in Execute.
type ActionExecutable interface {
	Config() string
	Execute(ctx context.Context, modifiedConfig string) (resp interface{}, fields map[string]string, err error)
	Type() string
	SupportsReplica() bool
}

// ActionExecutableV2 is the v2 action interface.
// Actions handle their own template resolution using RequestContext.Resolve()
// or RequestContext.ResolveBatch() methods.
type ActionExecutableV2 interface {
	Type() string
	Execute(ctx context.Context) (resp interface{}, fields map[string]string, err error)
	SupportsReplica() bool
}
