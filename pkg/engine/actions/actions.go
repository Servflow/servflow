package actions

import (
	"context"
	"errors"
)

// TODO think of how to handle fatal errors

var ErrorFatal = errors.New("fatal error")

const (
	FileTypeRequest = "request"
	FileTypeAction  = "action"
)

type FileInput struct {
	Type       string `json:"type" yaml:"type"`
	Identifier string `json:"identifier" yaml:"identifier"`
}

type ActionExecutable interface {
	Config() string
	Execute(ctx context.Context, modifiedConfig string) (interface{}, error)
	Type() string
}
