package fetch

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
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
		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		fetch, err := New(Config{
			Table:       "mock",
			Integration: "mockds",
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		resp, _, err := fetch.Execute(ctx, fetch.Config())
		require.NoError(t, err)
		assert.Equal(t, fetchReturn, resp)
	})

	t.Run("fetch fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(gomock.Any(), map[string]string{"collection": "mock"}, filters.Filter{Field: "id", Comparator: "1"}).
			Return(nil, errors.New("random error fetching"))
		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		fetch, err := New(Config{
			Table:       "mock",
			Integration: "mockds",
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		_, _, err = fetch.Execute(ctx, fetch.Config())
		require.Error(t, err)
	})

	t.Run("fail if empty with failure", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(gomock.Any(), map[string]string{"collection": "mock"}, filters.Filter{Field: "id", Comparator: "1"}).
			Return([]map[string]interface{}{}, nil)
		rc := requestctxtest.New()
		rc.SetIntegration("mockds", mockIntegration)
		ctx := requestctx.With(context.Background(), rc)

		fetch, err := New(Config{
			Table:       "mock",
			Integration: "mockds",
			FailIfEmpty: true,
			Filters: []filters.Filter{
				{
					Field:      "id",
					Comparator: "1",
				},
			},
		})
		require.NoError(t, err)

		_, _, err = fetch.Execute(ctx, fetch.Config())
		require.Error(t, err)
		assert.True(t, errors.Is(err, actions.ErrFailure), "Expected failure error to be wrapped with actions.ErrFailure")
	})
}
