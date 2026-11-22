package requestctx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func TestResolveValues(t *testing.T) {
	rc := requestctxtest.New()
	rc.AddVariables(map[string]any{"name": "Ada", "quote": "say \"hi\"\nthen go"}, "")
	ctx := requestctx.With(context.Background(), rc)

	tests := []struct {
		name string
		in   any
		want any
	}{
		{name: "string", in: "{{ .name }}", want: "Ada"},
		{name: "quotes and newlines arrive as written", in: "{{ .quote }}", want: "say \"hi\"\nthen go"},
		{
			name: "nested map and list",
			in: map[string]any{
				"{{ .name }}": "key kept",
				"to":          []any{"{{ .name }}", 3.0, true},
				"inner":       map[string]any{"text": "hello {{ .name }}"},
			},
			want: map[string]any{
				"{{ .name }}": "key kept",
				"to":          []any{"Ada", 3.0, true},
				"inner":       map[string]any{"text": "hello Ada"},
			},
		},
		{name: "no strings", in: []any{1.0, nil}, want: []any{1.0, nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requestctx.ResolveValues(ctx, tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveValuesNeedsRequestContext(t *testing.T) {
	_, err := requestctx.ResolveValues(context.Background(), "x")
	assert.ErrorIs(t, err, requestctx.ErrNoContext)
}
