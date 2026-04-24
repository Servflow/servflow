package get_key

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Servflow/servflow/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	client, err := storage.GetClient()
	if err != nil {
		panic(err)
	}

	code := m.Run()

	client.Close()

	os.Exit(code)
}

func TestGetKey_Execute(t *testing.T) {
	t.Run("retrieve existing key", func(t *testing.T) {
		key := "get-key-test-existing"
		value := "test-value"

		err := storage.Set(key, value)
		require.NoError(t, err)

		executable := NewExecutable(Config{Key: key})
		modifiedConfig, _ := json.Marshal(Config{Key: key})
		result, fields, err := executable.Execute(context.Background(), string(modifiedConfig))

		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, value, result)
	})

	t.Run("retrieve non-existent key returns empty string", func(t *testing.T) {
		key := "get-key-test-non-existent"

		executable := NewExecutable(Config{Key: key})
		modifiedConfig, _ := json.Marshal(Config{Key: key})
		result, fields, err := executable.Execute(context.Background(), string(modifiedConfig))

		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, "", result)
	})

	t.Run("empty key returns no error", func(t *testing.T) {
		executable := NewExecutable(Config{Key: ""})
		modifiedConfig, _ := json.Marshal(Config{Key: ""})
		_, _, err := executable.Execute(context.Background(), string(modifiedConfig))

		require.NoError(t, err)
	})

	t.Run("failIfEmpty returns error when key not found", func(t *testing.T) {
		key := "get-key-test-fail-if-empty"

		executable := NewExecutable(Config{Key: key, FailIfEmpty: true})
		modifiedConfig, _ := json.Marshal(Config{Key: key, FailIfEmpty: true})
		_, _, err := executable.Execute(context.Background(), string(modifiedConfig))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("failIfEmpty false returns empty string when key not found", func(t *testing.T) {
		key := "get-key-test-no-fail-if-empty"

		executable := NewExecutable(Config{Key: key, FailIfEmpty: false})
		modifiedConfig, _ := json.Marshal(Config{Key: key, FailIfEmpty: false})
		result, fields, err := executable.Execute(context.Background(), string(modifiedConfig))

		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, "", result)
	})
}

func TestGetKey_Type(t *testing.T) {
	executable := NewExecutable(Config{Key: "test"})
	assert.Equal(t, "get_key", executable.Type())
}

func TestGetKey_SupportsReplica(t *testing.T) {
	executable := NewExecutable(Config{Key: "test"})
	assert.True(t, executable.SupportsReplica())
}

func TestGetKey_Config(t *testing.T) {
	key := "test-key"
	executable := NewExecutable(Config{Key: key, FailIfEmpty: true})

	var resultCfg Config
	err := json.Unmarshal([]byte(executable.Config()), &resultCfg)
	require.NoError(t, err)
	assert.Equal(t, key, resultCfg.Key)
	assert.True(t, resultCfg.FailIfEmpty)
}
