package store_key

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreKey_Execute(t *testing.T) {
	t.Run("basic store and verify", func(t *testing.T) {
		cfg := Config{
			Key:   "test-store-key",
			Value: "test-value",
		}
		exec := NewExecutable(cfg)

		result, fields, err := exec.Execute(context.Background(), "processed-value")
		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, "processed-value", result)

		stored, found, err := storage.Get("test-store-key")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "processed-value", stored)
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		cfg := Config{
			Key:   "test-overwrite-key",
			Value: "original",
		}
		exec := NewExecutable(cfg)

		_, _, err := exec.Execute(context.Background(), "first-value")
		require.NoError(t, err)

		_, _, err = exec.Execute(context.Background(), "second-value")
		require.NoError(t, err)

		stored, found, err := storage.Get("test-overwrite-key")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "second-value", stored)
	})

	t.Run("error on empty key", func(t *testing.T) {
		cfg := Config{
			Key:   "",
			Value: "some-value",
		}
		exec := NewExecutable(cfg)

		_, _, err := exec.Execute(context.Background(), "processed-value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})
}

func TestStoreKey_Type(t *testing.T) {
	exec := NewExecutable(Config{})
	assert.Equal(t, "store_key", exec.Type())
}

func TestStoreKey_SupportsReplica(t *testing.T) {
	exec := NewExecutable(Config{})
	assert.True(t, exec.SupportsReplica())
}

func TestStoreKey_Config(t *testing.T) {
	cfg := Config{
		Key:   "my-key",
		Value: "my-value",
	}
	exec := NewExecutable(cfg)
	assert.Equal(t, "my-value", exec.Config())
}
