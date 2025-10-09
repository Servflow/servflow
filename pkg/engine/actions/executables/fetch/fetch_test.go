package fetch

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

func TestFetch_Execute(t *testing.T) {
	t.Run("successful run", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		fetchReturn := []map[string]interface{}{
			{"id": "1", "name": "test1"},
		}

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(gomock.Any(), map[string]string{"collection": "mock"}, filters.Filter{Field: "id", Comparator: "1"}).Return(fetchReturn, nil)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		fetch, err := New(Config{
			Table:             "mock",
			IntegrationID:     "mockds",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		resp, err := fetch.Execute(context.Background(), fetch.Config())
		require.NoError(t, err)
		assert.Equal(t, fetchReturn, resp)
	})

	t.Run("fetch fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(gomock.Any(), map[string]string{"collection": "mock"}, filters.Filter{Field: "id", Comparator: "1"}).
			Return(nil, errors.New("random error fetching"))
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil)

		fetch, err := New(Config{
			Table:             "mock",
			IntegrationID:     "mockds",
			DatasourceOptions: map[string]string{"optiontest": "test"},
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		_, err = fetch.Execute(context.Background(), fetch.Config())
		require.Error(t, err)
	})
}
