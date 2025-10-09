package update

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdate_Execute(t *testing.T) {
	t.Run("successful run", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		fields := map[string]interface{}{"name": "test"}
		filtersList := []filters.Filter{
			{
				Field:      "test",
				Comparator: "test",
				Operation:  "==",
			},
		}

		mockIntegration := NewMockupdateIntegration(ctr)
		mockIntegration.EXPECT().Update(gomock.Any(), fields, map[string]string{"collection": "mock_table"}, filtersList[0]).Return(nil)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		update, err := New(Config{
			IntegrationID:     "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"option": "test"},
			Filters:           filtersList,
			Fields:            fields,
		})
		require.NoError(t, err)

		resp, err := update.Execute(context.Background(), update.Config())
		require.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("update fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		fields := map[string]interface{}{"name": "test"}
		filtersList := []filters.Filter{
			{
				Field:      "test",
				Comparator: "test",
				Operation:  "==",
			},
		}

		mockIntegration := NewMockupdateIntegration(ctr)
		mockIntegration.EXPECT().Update(gomock.Any(), fields, map[string]string{"collection": "mock_table"}, filtersList[0]).Return(errors.New("test error"))
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil)

		update, err := New(Config{
			IntegrationID:     "mockds",
			Table:             "mock_table",
			DatasourceOptions: map[string]string{"option": "test"},
			Filters:           filtersList,
			Fields:            fields,
		})
		require.NoError(t, err)

		_, err = update.Execute(context.Background(), update.Config())
		assert.Error(t, err)
	})

	t.Run("missing table", func(t *testing.T) {
		_, err := New(Config{
			IntegrationID:     "mockds",
			DatasourceOptions: map[string]string{"option": "test"},
			Filters: []filters.Filter{
				{
					Field:      "test",
					Comparator: "test",
					Operation:  "==",
				},
			},
			Fields: map[string]interface{}{"name": "test"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})
}
