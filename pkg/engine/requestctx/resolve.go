package requestctx

import (
	"context"
	"fmt"
)

// ResolveValues renders every string inside v against the request context of
// ctx, in one batch, and returns a copy of v with each string replaced by its
// rendering. v is a decoded JSON value: maps and slices are walked, map keys
// are kept as they are, and anything that is not a string is returned as it
// is.
func ResolveValues(ctx context.Context, v any) (any, error) {
	rc, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	var templates []string
	var sets []func(string)
	var walk func(v any, set func(any)) any
	walk = func(v any, set func(any)) any {
		switch t := v.(type) {
		case string:
			templates = append(templates, t)
			sets = append(sets, func(s string) { set(s) })
			return t
		case map[string]any:
			out := make(map[string]any, len(t))
			for k, item := range t {
				out[k] = walk(item, func(r any) { out[k] = r })
			}
			return out
		case []any:
			out := make([]any, len(t))
			for i, item := range t {
				out[i] = walk(item, func(r any) { out[i] = r })
			}
			return out
		default:
			return v
		}
	}
	var root any
	root = walk(v, func(r any) { root = r })
	if len(templates) == 0 {
		return root, nil
	}
	resolved, err := rc.ResolveBatch(ctx, templates...)
	if err != nil {
		return nil, fmt.Errorf("resolving values: %w", err)
	}
	for i, s := range resolved {
		sets[i](s)
	}
	return root, nil
}
