package requestctx

import (
	"context"
	"text/template"
)

// templateFuncsKey carries a template function overlay on a context.
type templateFuncsKey struct{}

// WithTemplateFuncs returns a context whose template resolutions see funcs on
// top of the base and request-scoped functions.
//
// The overlay belongs to the context, not to the request: two callers holding
// the same RequestContext but different contexts resolve independently. That
// is what a per-call function needs — a tool argument reader is only correct
// for the one call it was created for, so registering it on the shared
// RequestContext (AddRequestTemplateFunctions) would let concurrent calls
// overwrite each other.
//
// Overlays compose: applying this to a context that already carries one keeps
// the outer functions and lets the inner ones win a name collision. An
// explicit funcMap passed to CreateTextTemplate still beats both.
func WithTemplateFuncs(ctx context.Context, funcs template.FuncMap) context.Context {
	if len(funcs) == 0 {
		return ctx
	}
	existing := templateFuncsFromContext(ctx)
	merged := make(template.FuncMap, len(existing)+len(funcs))
	for name, fn := range existing {
		merged[name] = fn
	}
	for name, fn := range funcs {
		merged[name] = fn
	}
	return context.WithValue(ctx, templateFuncsKey{}, merged)
}

// templateFuncsFromContext returns the overlay ctx carries, or nil.
func templateFuncsFromContext(ctx context.Context) template.FuncMap {
	if ctx == nil {
		return nil
	}
	funcs, _ := ctx.Value(templateFuncsKey{}).(template.FuncMap)
	return funcs
}
