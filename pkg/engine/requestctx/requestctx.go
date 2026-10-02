// Package requestctx is the request state an action reads and writes while it
// runs. The host implements RequestContext and installs it on the context with
// With; actions reach it with FromContext and the helpers in this package.
package requestctx

import (
	"context"
	"errors"
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/integration"
)

// ErrNoContext is returned when a context carries no RequestContext.
var ErrNoContext = errors.New("no context provided in request")

// RequestContext is one request's state, as an action sees it.
type RequestContext interface {
	// ID is the request id.
	ID() string

	// Variables returns the request variables templates render against.
	Variables() map[string]any
	// AddVariables stores variables, each key prefixed with prefix.
	AddVariables(variables map[string]any, prefix string)

	// Resolve renders a template against the request variables.
	Resolve(ctx context.Context, template string) (string, error)
	// ResolveBatch renders several templates and returns the results in the
	// same order.
	ResolveBatch(ctx context.Context, templates ...string) ([]string, error)

	// Scrub replaces every secret value the request has resolved in s.
	Scrub(s string) string
	// HasSecrets reports whether the request has resolved any secret, so a
	// caller can skip scrubbing when it has not.
	HasSecrets() bool

	// File returns the file stored under key.
	File(key string) (*FileValue, bool)
	// AddFile stores a file under key.
	AddFile(key string, file *FileValue)

	// Workspace returns the request's workspace, or nil if it has none.
	Workspace() Workspace
	// Request returns the HTTP request that opened this request, or nil when
	// it did not arrive over HTTP.
	Request() *http.Request

	// Integration returns the live integration configured under id. ctx is
	// the context the lookup is made for, so the host can decide what it may
	// reach.
	Integration(ctx context.Context, id string) (integration.Integration, error)
}

type contextKey struct{}

// With returns a context that carries rc.
func With(ctx context.Context, rc RequestContext) context.Context {
	return context.WithValue(ctx, contextKey{}, rc)
}

// FromContext returns the RequestContext ctx carries.
func FromContext(ctx context.Context) (RequestContext, bool) {
	rc, ok := ctx.Value(contextKey{}).(RequestContext)
	return rc, ok
}

// FromContextOrError returns the RequestContext ctx carries, or ErrNoContext.
func FromContextOrError(ctx context.Context) (RequestContext, error) {
	rc, ok := FromContext(ctx)
	if !ok {
		return nil, ErrNoContext
	}
	return rc, nil
}

// GetIntegration returns the integration configured under id for the request
// ctx carries.
func GetIntegration(ctx context.Context, id string) (integration.Integration, error) {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	return rc.Integration(ctx, id)
}
