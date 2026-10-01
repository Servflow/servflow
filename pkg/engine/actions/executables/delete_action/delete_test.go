package delete_action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewDeleteAction(t *testing.T) {
	del, err := New(Config{
		Integration:       "testID",
		Table:             "mock_table",
		DatasourceOptions: map[string]string{"optiontest": "test"},
		Filters: []filters.Filter{
			{
				Field:      "id",
				Comparator: "1",
			},
		},
	})
	require.NoError(t, err)

	jsonFilters, err := json.Marshal([]filters.Filter{
		{
			Field:      "id",
			Comparator: "1",
		},
	})
	require.NoError(t, err)
	assert.JSONEq(t, string(jsonFilters), del.Config())
}

func TestDelete_Execute(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockdeleteImplementation(ctr)
		mockIntegration.EXPECT().Delete(
			gomock.Any(),
			map[string]string{"collection": "mock_table"},
			filters.Filter{Field: "id", Comparator: "1"},
		).Return(nil)

		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		d, err := New(Config{
			Integration:       "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		// Create JSON string for filters
		modifiedConfig := `[{"field":"id","comparator":"1"}]`

		resp, _, err := d.Execute(ctx, modifiedConfig)
		require.NoError(t, err)
		assert.Nil(t, resp) // Delete operation should return nil
	})

	t.Run("delete fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockdeleteImplementation(ctr)
		mockIntegration.EXPECT().Delete(
			gomock.Any(),
			map[string]string{"collection": "mock_table"},
			filters.Filter{Field: "id", Comparator: "1"},
		).Return(errors.New("random error deleting"))

		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		d, err := New(Config{
			Integration:       "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		// Create JSON string for filters
		modifiedConfig := `[{"field":"id","comparator":"1"}]`

		_, _, err = d.Execute(ctx, modifiedConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete with filters")
	})

	t.Run("invalid config JSON", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockdeleteImplementation(ctr)
		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		d, err := New(Config{
			Integration:       "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		// Invalid JSON string for filters
		modifiedConfig := `{"invalid":"json"`

		_, _, err = d.Execute(ctx, modifiedConfig)
		require.Error(t, err)
	})

	t.Run("missing datasource id", func(t *testing.T) {
		_, err := New(Config{
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "datasource is required")
	})

	t.Run("missing table", func(t *testing.T) {
		_, err := New(Config{
			Integration:       "mockds",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})
}
