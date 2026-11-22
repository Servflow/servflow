package fetch

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFetch_Execute(t *testing.T) {
	rows := []map[string]interface{}{{"id": "1", "name": "test1"}}
	idFilter := filters.Filter{Field: "id", Operation: filters.Equals, Comparator: "1"}

	tests := []struct {
		name        string
		cfg         Config
		wantFilter  filters.Filter
		fetchRows   []map[string]interface{}
		fetchErr    error
		want        any
		wantErr     bool
		wantFailure bool
	}{
		{
			name:       "successful run",
			cfg:        Config{Filters: []filters.Filter{idFilter}},
			wantFilter: idFilter,
			fetchRows:  rows,
			want:       rows,
		},
		{
			name:       "single returns the first row",
			cfg:        Config{Single: true, Filters: []filters.Filter{idFilter}},
			wantFilter: idFilter,
			fetchRows:  rows,
			want:       rows[0],
		},
		{
			name:       "templated filter resolves to its text unchanged",
			cfg:        Config{Filters: []filters.Filter{{Field: "name", Operation: filters.Equals, Comparator: "{{ .variable_name }}"}}},
			wantFilter: filters.Filter{Field: "name", Operation: filters.Equals, Comparator: "say \"hi\"\nthen go"},
			fetchRows:  rows,
			want:       rows,
		},
		{
			name:       "fetch fails",
			cfg:        Config{Filters: []filters.Filter{idFilter}},
			wantFilter: idFilter,
			fetchErr:   errors.New("random error fetching"),
			wantErr:    true,
		},
		{
			name:       "empty result is an empty object",
			cfg:        Config{Filters: []filters.Filter{idFilter}},
			wantFilter: idFilter,
			fetchRows:  []map[string]interface{}{},
			want:       map[string]interface{}{},
		},
		{
			name:        "fail if empty with failure",
			cfg:         Config{FailIfEmpty: true, Filters: []filters.Filter{idFilter}},
			wantFilter:  idFilter,
			fetchRows:   []map[string]interface{}{},
			wantErr:     true,
			wantFailure: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockIntegration := NewMockfetchImplementation(gomock.NewController(t))
			mockIntegration.EXPECT().Fetch(gomock.Any(), map[string]string{"collection": "mock"}, tt.wantFilter).Return(tt.fetchRows, tt.fetchErr)
			rc := requestctxtest.New()
			rc.SetIntegration("mockds", mockIntegration)
			rc.AddVariables(map[string]any{"variable_name": "say \"hi\"\nthen go"}, "")
			ctx := requestctx.With(context.Background(), rc)

			tt.cfg.Table, tt.cfg.Integration = "mock", "mockds"
			fetch, err := New(tt.cfg)
			require.NoError(t, err)

			resp, _, err := fetch.Execute(ctx)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantFailure, errors.Is(err, actions.ErrFailure))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, resp)
		})
	}
}
