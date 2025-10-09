package delete_action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewDeleteAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIntegration := NewMockdeleteImplementation(ctrl)
	integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
		return mockIntegration, nil
	})
	err := integration.InitializeIntegration("mock", "testID", nil)
	require.NoError(t, err)

	del, err := New(Config{
		IntegrationID:     "testID",
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

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		d, err := New(Config{
			IntegrationID:     "mockds",
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

		resp, err := d.Execute(context.Background(), modifiedConfig)
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

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil)

		d, err := New(Config{
			IntegrationID:     "mockds",
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

		_, err = d.Execute(context.Background(), modifiedConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete with filters")
	})

	t.Run("invalid config JSON", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockdeleteImplementation(ctr)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil)

		d, err := New(Config{
			IntegrationID:     "mockds",
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

		_, err = d.Execute(context.Background(), modifiedConfig)
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
			IntegrationID:     "mockds",
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
