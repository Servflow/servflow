package delete_action

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDelete_Execute(t *testing.T) {
	tests := []struct {
		name       string
		filters    []filters.Filter
		wantFilter filters.Filter
		deleteErr  error
		wantErr    string
	}{
		{
			name:       "successful delete",
			filters:    []filters.Filter{{Field: "id", Operation: filters.Equals, Comparator: "1"}},
			wantFilter: filters.Filter{Field: "id", Operation: filters.Equals, Comparator: "1"},
		},
		{
			name:       "templated filter resolves to its text unchanged",
			filters:    []filters.Filter{{Field: "{{ .variable_field }}", Operation: filters.Equals, Comparator: "{{ .variable_name }}"}},
			wantFilter: filters.Filter{Field: "name", Operation: filters.Equals, Comparator: "say \"hi\"\nthen go"},
		},
		{
			name:       "delete fails",
			filters:    []filters.Filter{{Field: "id", Operation: filters.Equals, Comparator: "1"}},
			wantFilter: filters.Filter{Field: "id", Operation: filters.Equals, Comparator: "1"},
			deleteErr:  errors.New("random error deleting"),
			wantErr:    "delete with filters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockIntegration := NewMockdeleteImplementation(gomock.NewController(t))
			mockIntegration.EXPECT().Delete(
				gomock.Any(),
				map[string]string{"collection": "mock_table"},
				tt.wantFilter,
			).Return(tt.deleteErr)

			rc := requestctxtest.New()
			rc.SetIntegration("mockds", mockIntegration)
			rc.AddVariables(map[string]any{"variable_field": "name", "variable_name": "say \"hi\"\nthen go"}, "")
			ctx := requestctx.With(context.Background(), rc)

			d, err := New(Config{Integration: "mockds", Table: "mock_table", Filters: tt.filters})
			require.NoError(t, err)

			resp, _, err := d.Execute(ctx)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"missing datasource id", Config{Table: "mock_table"}, "datasource is required"},
		{"missing table", Config{Integration: "mockds"}, "table is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
