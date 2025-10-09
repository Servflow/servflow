package qdrant

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/assert"
)

func setUpTestContainer(t *testing.T) string {
	req := testcontainers.ContainerRequest{
		Image: "qdrant/qdrant:v1.13.6",
		ExposedPorts: []string{
			"6334/tcp",
		},
		WaitingFor: wait.ForLog("qdrant::tonic: Qdrant gRPC listening on 6334"),
	}

	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := testcontainers.TerminateContainer(container)
		require.NoError(t, err)
	})

	url, err := container.PortEndpoint(context.Background(), "6334/tcp", "")
	require.NoError(t, err)
	return url
}

func setupTestQdrant(t *testing.T) *Wrapper {
	qdrantURL := setUpTestContainer(t)

	var (
		host string
		port int
	)
	// Parse URL to extract host and port
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(qdrantURL, "http://"), "https://"), ":")
	host = parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Skip("QDRANT_URL environment variable not set")
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		t.Fatalf("Error setting up Qdrant client: %v", err)
	}

	w := &Wrapper{client: client}

	// Create test collection
	collectionName := "test_collection"
	err = client.CreateCollection(context.Background(), &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     4,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		t.Fatalf("Error creating test collection: %v", err)
	}

	t.Cleanup(func() {
		err = client.DeleteCollection(context.Background(), collectionName)
		assert.NoError(t, err)
	})
	return w
}

func TestWrapper_Embed(t *testing.T) {
	t.Parallel()
	w := setupTestQdrant(t)

	t.Run("embed vector into collection", func(t *testing.T) {
		vectors := []float32{0.1, 0.2, 0.3, 0.5}
		metadata := map[string]interface{}{
			"names": "Test Vector",
		}
		options := map[string]string{
			"collection": "test_collection",
		}

		err := w.StoreVectors(vectors, metadata, options)
		assert.NoError(t, err)
	})

	t.Run("fetch vectors from database", func(t *testing.T) {
		vectors := []float32{0.5, 1, 5, 2}
		metadata := map[string]interface{}{
			"names": "Test Vector",
		}
		options := map[string]string{
			"collection": "test_collection",
		}

		require.NoError(t, w.StoreVectors(vectors, metadata, options))
		require.NoError(t, w.StoreVectors(vectors, metadata, options))

		result, err := w.FetchVector(vectors, map[string]any{
			"collection": "test_collection",
			"limit":      2,
		})
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, result[0], map[string]any{"names": "Test Vector"})
	})
}
