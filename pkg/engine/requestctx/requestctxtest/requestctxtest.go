// Package requestctxtest provides a RequestContext for testing actions, and a
// conformance suite every RequestContext implementation must pass.
package requestctxtest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"text/template"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Context is a RequestContext for tests. It renders templates with
// text/template and no template functions, and it has no secrets, so Scrub
// returns its input. A test that needs template functions or secret handling
// belongs with the host's implementation.
type Context struct {
	mu        sync.Mutex
	id        string
	variables map[string]any
	files     map[string]*requestctx.FileValue
	workspace requestctx.Workspace
	request   *http.Request
}

var _ requestctx.RequestContext = (*Context)(nil)

// New returns an empty Context with the id "test".
func New() *Context {
	return &Context{
		id:        "test",
		variables: make(map[string]any),
		files:     make(map[string]*requestctx.FileValue),
	}
}

// NewContext returns a context carrying a new Context.
func NewContext() context.Context {
	return requestctx.With(context.Background(), New())
}

// SetWorkspace gives the request a workspace.
func (c *Context) SetWorkspace(ws requestctx.Workspace) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspace = ws
}

// SetRequest gives the request an HTTP request.
func (c *Context) SetRequest(req *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.request = req
}

func (c *Context) ID() string { return c.id }

func (c *Context) Variables() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]any, len(c.variables))
	for k, v := range c.variables {
		out[k] = v
	}
	return out
}

func (c *Context) AddVariables(variables map[string]any, prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range variables {
		c.variables[prefix+k] = v
	}
}

func (c *Context) Resolve(_ context.Context, tmpl string) (string, error) {
	t, err := template.New("input").Option("missingkey=zero").Parse(requestctx.PrepareTemplate(tmpl))
	if err != nil {
		return "", fmt.Errorf("creating template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, c.Variables()); err != nil {
		return "", fmt.Errorf("error processing template: %w", err)
	}
	return requestctx.StripNoValue(buf.String()), nil
}

func (c *Context) ResolveBatch(ctx context.Context, templates ...string) ([]string, error) {
	out := make([]string, len(templates))
	for i, tmpl := range templates {
		resolved, err := c.Resolve(ctx, tmpl)
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

func (c *Context) Scrub(s string) string { return s }

func (c *Context) HasSecrets() bool { return false }

func (c *Context) File(key string) (*requestctx.FileValue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.files[key]
	return f, ok
}

func (c *Context) AddFile(key string, file *requestctx.FileValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[key] = file
}

func (c *Context) Workspace() requestctx.Workspace {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workspace
}

func (c *Context) Request() *http.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.request
}
