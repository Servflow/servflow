package actions

import (
	"context"
	"errors"
)

// TODO think of how to handle fatal errors

var ErrorFatal = errors.New("fatal error")

type ActionExecutable interface {
	Config() string
	Execute(ctx context.Context, modifiedConfig string) (resp interface{}, fields map[string]string, err error)
	Type() string
	SupportsReplica() bool
}
