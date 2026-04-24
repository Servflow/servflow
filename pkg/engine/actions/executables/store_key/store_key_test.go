package store_key

import (
	"context"
	"encoding/json"
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

		modifiedConfig, _ := json.Marshal(Config{Key: "test-store-key", Value: "processed-value"})
		result, fields, err := exec.Execute(context.Background(), string(modifiedConfig))
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

		firstConfig, _ := json.Marshal(Config{Key: "test-overwrite-key", Value: "first-value"})
		_, _, err := exec.Execute(context.Background(), string(firstConfig))
		require.NoError(t, err)

		secondConfig, _ := json.Marshal(Config{Key: "test-overwrite-key", Value: "second-value"})
		_, _, err = exec.Execute(context.Background(), string(secondConfig))
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

		modifiedConfig, _ := json.Marshal(Config{Key: "", Value: "processed-value"})
		_, _, err := exec.Execute(context.Background(), string(modifiedConfig))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})

	t.Run("key is templatable", func(t *testing.T) {
		cfg := Config{
			Key:   "{{.response}}",
			Value: "test-value",
		}
		exec := NewExecutable(cfg)

		modifiedConfig, _ := json.Marshal(Config{Key: "dynamic-key", Value: "test-value"})
		result, fields, err := exec.Execute(context.Background(), string(modifiedConfig))
		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, "test-value", result)

		stored, found, err := storage.Get("dynamic-key")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "test-value", stored)
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

	var resultCfg Config
	err := json.Unmarshal([]byte(exec.Config()), &resultCfg)
	require.NoError(t, err)
	assert.Equal(t, "my-key", resultCfg.Key)
	assert.Equal(t, "my-value", resultCfg.Value)
}
