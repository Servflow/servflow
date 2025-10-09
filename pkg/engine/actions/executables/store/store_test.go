package store

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStore_Execute(t *testing.T) {
	t.Run("successful run", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		item := map[string]interface{}{"id": "1", "name": "test1"}

		mockIntegration := NewMockstorageIntegrations(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), item, map[string]string{"collection": "mock_table"}).Return(nil)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		store, err := New(Config{
			IntegrationID:     "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Fields:            item,
		})
		require.NoError(t, err)

		resp, err := store.Execute(context.Background(), store.Config())
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"id":   "1",
			"name": "test1",
		}, resp)
	})

	t.Run("successful run without id", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		item := map[string]interface{}{"name": "test1"}

		mockIntegration := NewMockstorageIntegrations(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), gomock.Any(), map[string]string{"collection": "mock_table"}).Return(nil)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		store, err := New(Config{
			IntegrationID:     "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Fields:            item,
		})
		require.NoError(t, err)

		resp, err := store.Execute(context.Background(), store.Config())
		require.NoError(t, err)
		assert.Contains(t, resp, "id")
	})

	t.Run("fetch fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		item := map[string]interface{}{"id": "1", "name": "test1"}

		mockIntegration := NewMockstorageIntegrations(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), item, map[string]string{"collection": "mock_table"}).Return(errors.New("dummy error"))
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil)

		store, err := New(Config{
			IntegrationID:     "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Fields:            item,
		})
		require.NoError(t, err)

		_, err = store.Execute(context.Background(), store.Config())
		assert.Error(t, err)
	})

	t.Run("missing table", func(t *testing.T) {
		_, err := New(Config{
			IntegrationID:     "mockds",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Fields:            map[string]interface{}{"name": "test"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})
}
