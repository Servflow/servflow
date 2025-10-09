package actions

import "context"

type ActionExecutable interface {
	Config() string
	Execute(ctx context.Context, modifiedConfig string) (interface{}, error)
	Type() string
}
