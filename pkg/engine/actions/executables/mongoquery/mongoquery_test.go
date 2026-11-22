package mongoquery

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func withIntegration(t *testing.T, impl mongoDBIntegration) context.Context {
	rc := requestctxtest.New()
	rc.SetIntegration("mongo", impl)
	return requestctx.With(context.Background(), rc)
}

func execute(t *testing.T, ctx context.Context, cfg Config) (any, error) {
	cfg.Integration = "mongo"
	q, err := New(cfg)
	require.NoError(t, err)
	res, _, err := q.Execute(ctx)
	return res, err
}

func TestMGOQuery_Operations(t *testing.T) {
	docs := []map[string]interface{}{{"name": "ada"}}

	t.Run("find is the default", func(t *testing.T) {
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		mock.EXPECT().ExecuteQuery(gomock.Any(), "users", `{"name":"ada"}`, `{"name":1}`).Return(docs, nil)

		res, err := execute(t, withIntegration(t, mock), Config{Collection: "users", FilterQuery: `{"name":"ada"}`, Projection: `{"name":1}`})
		require.NoError(t, err)
		assert.Equal(t, docs, res)
	})

	t.Run("aggregate runs the pipeline", func(t *testing.T) {
		pipeline := `[{"$lookup":{"from":"orders","localField":"_id","foreignField":"userId","as":"orders"}}]`
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		mock.EXPECT().Aggregate(gomock.Any(), "users", pipeline).Return(docs, nil)

		res, err := execute(t, withIntegration(t, mock), Config{Operation: OperationAggregate, Collection: "users", Pipeline: pipeline})
		require.NoError(t, err)
		assert.Equal(t, docs, res)
	})

	t.Run("listCollections needs no collection", func(t *testing.T) {
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		mock.EXPECT().ListCollections(gomock.Any()).Return([]string{"orders", "users"}, nil)

		res, err := execute(t, withIntegration(t, mock), Config{Operation: OperationListCollections})
		require.NoError(t, err)
		assert.Equal(t, []string{"orders", "users"}, res)
	})

	t.Run("failIfEmpty applies to listCollections", func(t *testing.T) {
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		mock.EXPECT().ListCollections(gomock.Any()).Return(nil, nil)

		_, err := execute(t, withIntegration(t, mock), Config{Operation: OperationListCollections, FailIfEmpty: true})
		assert.ErrorIs(t, err, actions.ErrFailure)
	})
}

func TestMGOQuery_ReadOnly(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		key  string
	}{
		{"$out stage", Config{Operation: OperationAggregate, Collection: "users", Pipeline: `[{"$out":"backup"}]`}, "$out"},
		{"$merge inside $facet", Config{Operation: OperationAggregate, Collection: "users", Pipeline: `[{"$facet":{"a":[{"$merge":"x"}]}}]`}, "$merge"},
		{"$out inside a $lookup pipeline", Config{Operation: OperationAggregate, Collection: "users", Pipeline: `[{"$lookup":{"from":"orders","as":"o","pipeline":[{"$out":"x"}]}}]`}, "$out"},
		{"$where in a filter", Config{Collection: "users", FilterQuery: `{"$where":"this.a > 1"}`}, "$where"},
		{"$function in a projection", Config{Collection: "users", Projection: `{"x":{"$function":{"body":"function(){}","args":[],"lang":"js"}}}`}, "$function"},
		{"$accumulator in a $group", Config{Operation: OperationAggregate, Collection: "users", Pipeline: `[{"$group":{"_id":null,"x":{"$accumulator":{}}}}]`}, "$accumulator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The mock expects no calls: a refused query never reaches MongoDB.
			mock := NewMockmongoDBIntegration(gomock.NewController(t))

			_, err := execute(t, withIntegration(t, mock), tt.cfg)
			require.ErrorIs(t, err, actions.ErrFailure)
			assert.Contains(t, err.Error(), tt.key)
		})
	}

	t.Run("invalid JSON is refused", func(t *testing.T) {
		mock := NewMockmongoDBIntegration(gomock.NewController(t))

		_, err := execute(t, withIntegration(t, mock), Config{Collection: "users", FilterQuery: `{name: "ada"}`})
		require.ErrorIs(t, err, actions.ErrFailure)
		assert.Contains(t, err.Error(), "filterQuery")
	})
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"find without a collection", Config{}, "collection is required"},
		{"aggregate without a collection", Config{Operation: OperationAggregate, Pipeline: `[]`}, "collection is required"},
		{"aggregate without a pipeline", Config{Operation: OperationAggregate, Collection: "users"}, "pipeline is required"},
		{"unknown operation", Config{Operation: "insert", Collection: "users"}, "unknown operation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Integration = "mongo"
			_, err := New(tt.cfg)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}

	t.Run("integration is required", func(t *testing.T) {
		_, err := New(Config{Collection: "users"})
		assert.ErrorContains(t, err, "integration is required")
	})

	t.Run("a templated operation is checked once resolved", func(t *testing.T) {
		q, err := New(Config{Integration: "mongo", Operation: "{{ .variable_operation }}", Collection: "{{ .variable_collection }}"})
		require.NoError(t, err)

		rc := requestctxtest.New()
		rc.SetIntegration("mongo", NewMockmongoDBIntegration(gomock.NewController(t)))
		rc.AddVariables(map[string]any{"variable_operation": "insert", "variable_collection": "users"}, "")
		_, _, err = q.Execute(requestctx.With(context.Background(), rc))
		require.ErrorIs(t, err, actions.ErrFailure)
		assert.ErrorContains(t, err, "unknown operation")
	})
}

func TestMGOQuery_ResolvesTemplates(t *testing.T) {
	t.Run("each call resolves the stored templates again", func(t *testing.T) {
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		gomock.InOrder(
			mock.EXPECT().ListCollections(gomock.Any()).Return([]string{"users"}, nil),
			mock.EXPECT().ExecuteQuery(gomock.Any(), "users", "{}", "").Return([]map[string]interface{}{{"name": "ada"}}, nil),
		)
		rc := requestctxtest.New()
		rc.SetIntegration("mongo", mock)
		ctx := requestctx.With(context.Background(), rc)

		q, err := New(Config{Integration: "mongo", Operation: "{{ .variable_operation }}", Collection: "{{ .variable_collection }}", FilterQuery: "{{ .variable_filter }}"})
		require.NoError(t, err)

		rc.AddVariables(map[string]any{"variable_operation": OperationListCollections}, "")
		_, _, err = q.Execute(ctx)
		require.NoError(t, err)

		rc.AddVariables(map[string]any{"variable_operation": OperationFind, "variable_collection": "users", "variable_filter": "{}"}, "")
		res, _, err := q.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, []map[string]interface{}{{"name": "ada"}}, res)
	})

	t.Run("a resolved value reaches the query unchanged", func(t *testing.T) {
		filter := "{\"note\":\"say \\\"hi\\\"\\nthen go\"}"
		mock := NewMockmongoDBIntegration(gomock.NewController(t))
		mock.EXPECT().ExecuteQuery(gomock.Any(), "users", filter, "").Return([]map[string]interface{}{{"name": "ada"}}, nil)
		rc := requestctxtest.New()
		rc.SetIntegration("mongo", mock)
		rc.AddVariables(map[string]any{"variable_filter": filter}, "")

		_, err := execute(t, requestctx.With(context.Background(), rc), Config{Collection: "users", FilterQuery: "{{ .variable_filter }}"})
		require.NoError(t, err)
	})
}
