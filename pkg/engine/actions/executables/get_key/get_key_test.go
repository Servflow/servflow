package get_key

import (
	"context"
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
		result, fields, err := executable.Execute(context.Background(), key)

		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, value, result)
	})

	t.Run("retrieve non-existent key returns empty string", func(t *testing.T) {
		key := "get-key-test-non-existent"

		executable := NewExecutable(Config{Key: key})
		result, fields, err := executable.Execute(context.Background(), key)

		require.NoError(t, err)
		assert.Nil(t, fields)
		assert.Equal(t, "", result)
	})

	t.Run("empty key returns error", func(t *testing.T) {
		executable := NewExecutable(Config{Key: ""})
		_, _, err := executable.Execute(context.Background(), "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
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
	executable := NewExecutable(Config{Key: key})
	assert.Equal(t, key, executable.Config())
}
