package storevector

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

func TestStoreVectors_Execute(t *testing.T) {

	t.Run("successful run", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		vectors := []float32{1.1, 2.2, 3.3}
		fields := map[string]interface{}{"id": "1", "name": "test1"}

		jsonVectors, err := json.Marshal(vectors)
		require.NoError(t, err)

		mockIntegration := NewMockstoreVectorIntegration(ctr)
		mockIntegration.EXPECT().StoreVectors(vectors, fields, map[string]string{"optiontest": "test"}).Return(nil)
		rc := requestctxtest.New()
		rc.SetIntegration("mockid", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		storeVectors, err := New(Config{
			Integration: "mockid",
			Fields:      fields,
			Options:     map[string]string{"optiontest": "test"},
			Vectors:     string(jsonVectors),
		})
		require.NoError(t, err)

		_, _, err = storeVectors.Execute(ctx, storeVectors.Config())
		require.NoError(t, err)
	})

	t.Run("store vectors fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		vectors := []float32{1.1, 2.2, 3.3}
		fields := map[string]interface{}{"id": "1", "name": "test1"}

		jsonVectors, err := json.Marshal(vectors)
		require.NoError(t, err)

		mockIntegration := NewMockstoreVectorIntegration(ctr)
		mockIntegration.EXPECT().StoreVectors(vectors, fields, map[string]string{"optiontest": "test"}).Return(errors.New("dummy error"))
		rc := requestctxtest.New()
		rc.SetIntegration("mockid", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		storeVectors, err := New(Config{
			Integration: "mockid",
			Fields:      fields,
			Options:     map[string]string{"optiontest": "test"},
			Vectors:     string(jsonVectors),
		})
		require.NoError(t, err)

		_, _, err = storeVectors.Execute(ctx, storeVectors.Config())
		assert.Error(t, err)
	})
}
