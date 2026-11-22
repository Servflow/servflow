package storevector

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStoreVectors_Execute(t *testing.T) {
	vectors := []float32{1.1, 2.2, 3.3}
	options := map[string]string{"optiontest": "{{ .variable_note }}"}

	tests := []struct {
		name       string
		cfg        Config
		wantFields map[string]any
		storeErr   error
	}{
		{
			name:       "successful run",
			cfg:        Config{Vectors: "[1.1,2.2,3.3]", Fields: map[string]any{"id": "1", "name": "test1"}},
			wantFields: map[string]any{"id": "1", "name": "test1"},
		},
		{
			name: "templated vectors and fields resolve to their text unchanged",
			cfg: Config{
				Vectors: "{{ .variable_vectors }}",
				Fields:  map[string]any{"id": "1", "tags": []any{"{{ .variable_note }}"}},
			},
			wantFields: map[string]any{"id": "1", "tags": []any{"say \"hi\"\nthen go"}},
		},
		{
			name:       "store vectors fails",
			cfg:        Config{Vectors: "[1.1,2.2,3.3]", Fields: map[string]any{"id": "1", "name": "test1"}},
			wantFields: map[string]any{"id": "1", "name": "test1"},
			storeErr:   errors.New("dummy error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockIntegration := NewMockstoreVectorIntegration(gomock.NewController(t))
			// Options are passed as configured.
			mockIntegration.EXPECT().StoreVectors(vectors, tt.wantFields, options).Return(tt.storeErr)
			rc := requestctxtest.New()
			rc.SetIntegration("mockid", mockIntegration)
			rc.AddVariables(map[string]any{"variable_vectors": "[1.1,2.2,3.3]", "variable_note": "say \"hi\"\nthen go"}, "")
			ctx := requestctx.With(context.Background(), rc)

			tt.cfg.Integration, tt.cfg.Options = "mockid", options
			storeVectors, err := New(tt.cfg)
			require.NoError(t, err)

			_, _, err = storeVectors.Execute(ctx)
			if tt.storeErr != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
