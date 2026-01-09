package secrets_test

import (
	"os"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/stretchr/testify/assert"
)

// MockSecretStorage is a test implementation of SecretStorage
type MockSecretStorage struct {
	secrets map[string]string
}

func NewMockSecretStorage() *MockSecretStorage {
	return &MockSecretStorage{
		secrets: make(map[string]string),
	}
}

func (m *MockSecretStorage) FetchSecret(key string) string {
	return m.secrets[key]
}

func (m *MockSecretStorage) AddSecret(key string, value string) {
	m.secrets[key] = value
}

func TestSecretManager_FetchSecret(t *testing.T) {
	// Reset the manager before each test
	secrets.Reset()

	t.Run("fetch from environment", func(t *testing.T) {
		// Set an environment variable
		os.Setenv("TEST_SECRET", "test_value")
		defer os.Unsetenv("TEST_SECRET")

		// Should fetch from env storage by default
		value := secrets.FetchSecret("TEST_SECRET")
		assert.Equal(t, "test_value", value)
	})

	t.Run("fetch from multiple storages", func(t *testing.T) {
		secrets.Reset()

		// Create mock storages
		storage1 := NewMockSecretStorage()
		storage1.AddSecret("SECRET1", "value1")

		storage2 := NewMockSecretStorage()
		storage2.AddSecret("SECRET2", "value2")

		// Add storages to manager
		manager := secrets.GetManager()
		manager.AddStorage(storage1)
		manager.AddStorage(storage2)

		// Should find in first storage
		assert.Equal(t, "value1", secrets.FetchSecret("SECRET1"))

		// Should find in second storage
		assert.Equal(t, "value2", secrets.FetchSecret("SECRET2"))

		// Should return empty for non-existent
		assert.Equal(t, "", secrets.FetchSecret("NONEXISTENT"))
	})

	t.Run("first non-empty value wins", func(t *testing.T) {
		secrets.Reset()

		// Create storages with overlapping keys
		storage1 := NewMockSecretStorage()
		storage1.AddSecret("SECRET", "") // empty value

		storage2 := NewMockSecretStorage()
		storage2.AddSecret("SECRET", "value2")

		// Add storages
		manager := secrets.GetManager()
		manager.AddStorage(storage1)
		manager.AddStorage(storage2)

		// Should skip empty value and return from second storage
		assert.Equal(t, "value2", secrets.FetchSecret("SECRET"))
	})

	t.Run("env storage is default and first", func(t *testing.T) {
		secrets.Reset()

		// Set env var
		os.Setenv("PRIORITY_SECRET", "env_value")
		defer os.Unsetenv("PRIORITY_SECRET")

		// Add another storage with same key
		storage := NewMockSecretStorage()
		storage.AddSecret("PRIORITY_SECRET", "storage_value")

		manager := secrets.GetManager()
		manager.AddStorage(storage)

		// Env storage should win as it's first
		assert.Equal(t, "env_value", secrets.FetchSecret("PRIORITY_SECRET"))
	})
}

// TestBackwardCompatibility has been removed as the backward compatibility
// functions are no longer needed

func TestEnvStorage(t *testing.T) {
	storage := secrets.NewEnvStorage()

	t.Run("fetch from local secrets", func(t *testing.T) {
		storage.AddSecret("LOCAL_SECRET", "local_value")
		assert.Equal(t, "local_value", storage.FetchSecret("LOCAL_SECRET"))
	})

	t.Run("fetch from environment", func(t *testing.T) {
		os.Setenv("ENV_SECRET", "env_value")
		defer os.Unsetenv("ENV_SECRET")

		assert.Equal(t, "env_value", storage.FetchSecret("ENV_SECRET"))
	})

	t.Run("local secrets take precedence", func(t *testing.T) {
		os.Setenv("BOTH_SECRET", "env_value")
		defer os.Unsetenv("BOTH_SECRET")

		storage.AddSecret("BOTH_SECRET", "local_value")
		assert.Equal(t, "local_value", storage.FetchSecret("BOTH_SECRET"))
	})
}

func TestConcurrentAccess(t *testing.T) {
	secrets.Reset()
	manager := secrets.GetManager()

	// Add multiple storages concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			storage := NewMockSecretStorage()
			storage.AddSecret("KEY", "value")
			manager.AddStorage(storage)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_ = secrets.FetchSecret("KEY")
			done <- true
		}()
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
	assert.NotPanics(t, func() {
		secrets.FetchSecret("KEY")
	})
}
