package fetchvector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
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
		rc := requestctxtest.New()
		rc.SetIntegration("mockid", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		fetchVectorObj := FetchVector{
			cfg: &Config{
				Integration: "mockid",
				Vector:      string(jsonVectors),
			},
		}

		result, _, err := fetchVectorObj.Execute(ctx, fetchVectorObj.Config())
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
		rc := requestctxtest.New()
		rc.SetIntegration("mockid", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		fetchVectorObj := FetchVector{
			cfg: &Config{
				Integration: "mockid",
				Vector:      string(jsonVectors),
				Options:     options,
			},
		}

		_, _, err = fetchVectorObj.Execute(ctx, fetchVectorObj.Config())
		assert.Error(t, err)
	})
}
