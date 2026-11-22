package fetchvector

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

func TestFetchVector_Execute(t *testing.T) {
	vectors := []float32{1.1, 2.2, 3.3}
	results := []map[string]any{{"result": "success"}}

	tests := []struct {
		name        string
		cfg         Config
		wantOptions map[string]any
		fetchErr    error
		want        any
		wantErr     bool
	}{
		{
			name:        "successful run",
			cfg:         Config{Vector: "[1.1,2.2,3.3]", Options: map[string]any{"optionTest": "test"}},
			wantOptions: map[string]any{"optionTest": "test"},
			want:        results,
		},
		{
			name: "templated vector and options resolve to their text unchanged",
			cfg: Config{
				Vector:  "{{ .variable_vector }}",
				Options: map[string]any{"filter": map[string]any{"note": "{{ .variable_note }}"}},
			},
			wantOptions: map[string]any{"filter": map[string]any{"note": "say \"hi\"\nthen go"}},
			want:        results,
		},
		{
			name:        "fetch vector fails",
			cfg:         Config{Vector: "[1.1,2.2,3.3]", Options: map[string]any{"optionTest": "test"}},
			wantOptions: map[string]any{"optionTest": "test"},
			fetchErr:    errors.New("dummy error"),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockIntegration := NewMockfetchVectorIntegration(gomock.NewController(t))
			var ret []map[string]any
			if tt.fetchErr == nil {
				ret = results
			}
			mockIntegration.EXPECT().FetchVector(vectors, tt.wantOptions).Return(ret, tt.fetchErr)
			rc := requestctxtest.New()
			rc.SetIntegration("mockid", mockIntegration)
			rc.AddVariables(map[string]any{"variable_vector": "[1.1,2.2,3.3]", "variable_note": "say \"hi\"\nthen go"}, "")
			ctx := requestctx.With(context.Background(), rc)

			tt.cfg.Integration = "mockid"
			fetchVector, err := New(tt.cfg)
			require.NoError(t, err)

			result, _, err := fetchVector.Execute(ctx)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("invalid vector is an error", func(t *testing.T) {
		rc := requestctxtest.New()
		ctx := requestctx.With(context.Background(), rc)
		fetchVector, err := New(Config{Integration: "mockid", Vector: "not a vector"})
		require.NoError(t, err)

		_, _, err = fetchVector.Execute(ctx)
		assert.ErrorContains(t, err, "invalid value for vectors")
	})
}
