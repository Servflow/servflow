package save

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupTestContext(t *testing.T) context.Context {
	ctx := requestctx.NewTestContext()
	return ctx
}

func TestSave_Insert(t *testing.T) {
	t.Run("successful insert with provided id", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		ctx := setupTestContext(t)

		mockIntegration := NewMocksaveIntegration(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), map[string]interface{}{"id": "test-id", "name": "test"}, map[string]string{"collection": "mock_table"}).Return(nil)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil, false)
		require.NoError(t, err)

		save, err := New(Config{
			IntegrationID: "mockds",
			Table:         "mock_table",
			Fields:        map[string]interface{}{"id": "test-id", "name": "test"},
		})
		require.NoError(t, err)

		resp, _, err := save.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"id": "test-id"}, resp)
	})

	t.Run("successful insert with generated id", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		ctx := setupTestContext(t)

		mockIntegration := NewMocksaveIntegration(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), gomock.Any(), map[string]string{"collection": "mock_table"}).Return(nil)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil, false)
		require.NoError(t, err)

		save, err := New(Config{
			IntegrationID: "mockds",
			Table:         "mock_table",
			Fields:        map[string]interface{}{"name": "test"},
		})
		require.NoError(t, err)

		resp, _, err := save.Execute(ctx)
		require.NoError(t, err)

		respMap, ok := resp.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, respMap, "id")
		assert.NotEmpty(t, respMap["id"])
	})

	t.Run("insert fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		ctx := setupTestContext(t)

		mockIntegration := NewMocksaveIntegration(ctr)
		mockIntegration.EXPECT().Store(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("store error"))

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil, false)

		save, err := New(Config{
			IntegrationID: "mockds",
			Table:         "mock_table",
			Fields:        map[string]interface{}{"name": "test"},
		})
		require.NoError(t, err)

		_, _, err = save.Execute(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error storing")
	})
}

func TestSave_Update(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		ctx := setupTestContext(t)

		filtersList := []filters.Filter{
			{Field: "id", Operation: "==", Comparator: "123"},
		}

		mockIntegration := NewMocksaveIntegration(ctr)
		mockIntegration.EXPECT().Update(
			gomock.Any(),
			map[string]interface{}{"name": "updated"},
			map[string]string{"collection": "mock_table"},
			filtersList[0],
		).Return("123", nil)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil, false)
		require.NoError(t, err)

		save, err := New(Config{
			IntegrationID: "mockds",
			Table:         "mock_table",
			Fields:        map[string]interface{}{"name": "updated"},
			Filters:       filtersList,
		})
		require.NoError(t, err)

		resp, _, err := save.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"id": "123"}, resp)
	})

	t.Run("update fails", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		ctx := setupTestContext(t)

		filtersList := []filters.Filter{
			{Field: "id", Operation: "==", Comparator: "123"},
		}

		mockIntegration := NewMocksaveIntegration(ctr)
		mockIntegration.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("update error"))

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})
		integration.InitializeIntegration("mock", "mockds", nil, false)

		save, err := New(Config{
			IntegrationID: "mockds",
			Table:         "mock_table",
			Fields:        map[string]interface{}{"name": "updated"},
			Filters:       filtersList,
		})
		require.NoError(t, err)

		_, _, err = save.Execute(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error updating")
	})
}

func TestSave_Validation(t *testing.T) {
	t.Run("missing integrationID", func(t *testing.T) {
		_, err := New(Config{
			Table:  "mock_table",
			Fields: map[string]interface{}{"name": "test"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "integrationID is required")
	})

	t.Run("missing table", func(t *testing.T) {
		_, err := New(Config{
			IntegrationID: "mockds",
			Fields:        map[string]interface{}{"name": "test"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table is required")
	})
}

func TestSave_Type(t *testing.T) {
	ctr := gomock.NewController(t)
	defer ctr.Finish()

	mockIntegration := NewMocksaveIntegration(ctr)
	integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
		return mockIntegration, nil
	})
	integration.InitializeIntegration("mock", "mockds", nil, false)

	save, err := New(Config{
		IntegrationID: "mockds",
		Table:         "mock_table",
		Fields:        map[string]interface{}{"name": "test"},
	})
	require.NoError(t, err)

	assert.Equal(t, "save", save.Type())
	assert.True(t, save.SupportsReplica())
}
