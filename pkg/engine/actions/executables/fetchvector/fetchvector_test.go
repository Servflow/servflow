package fetchvector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFetchVector_Execute(t *testing.T) {

	t.Run("successful run", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		vectors := []float32{1.1, 2.2, 3.3}

		jsonVectors, err := json.Marshal(vectors)
		require.NoError(t, err)

		mockIntegration := NewMockfetchVectorIntegration(ctr)
		mockIntegration.EXPECT().FetchVector(vectors, gomock.Any()).Return([]map[string]any{
			{
				"result": "success",
			},
		}, nil)
		integration.ReplaceIntegrationType("mock", func(m map[string]interface{}) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err = integration.InitializeIntegration("mock", "mockid", nil)
		require.NoError(t, err)

		fetchVectorObj := FetchVector{
			cfg: &Config{
				IntegrationID: "mockid",
				Vector:        string(jsonVectors),
			},
			fetchIntegration: mockIntegration,
		}

		result, err := fetchVectorObj.Execute(context.Background(), fetchVectorObj.Config())
		require.NoError(t, err)
		assert.Equal(t, []map[string]any{
			{
				"result": "success",
			},
		}, result)
	})

	t.Run("fetch vector fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		vectors := []float32{1.1, 2.2, 3.3}
		options := map[string]any{"optionTest": "test"}

		jsonVectors, err := json.Marshal(vectors)
		require.NoError(t, err)

		mockIntegration := NewMockfetchVectorIntegration(ctr)
		mockIntegration.EXPECT().FetchVector(vectors, options).Return(nil, errors.New("dummy error"))
		integration.ReplaceIntegrationType("mock", func(m map[string]interface{}) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err = integration.InitializeIntegration("mock", "mockid", nil)
		require.NoError(t, err)

		fetchVectorObj := FetchVector{
			cfg: &Config{
				IntegrationID: "mockid",
				Vector:        string(jsonVectors),
				Options:       options,
			},
			fetchIntegration: mockIntegration,
		}

		_, err = fetchVectorObj.Execute(context.Background(), fetchVectorObj.Config())
		assert.Error(t, err)
	})
}
