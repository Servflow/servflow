//go:generate mockgen -source secrets.go -destination secrets_mock.go -package secrets
package secrets

import (
	"os"
	"sync"
)

type SecretStorage interface {
	FetchSecret(key string) string
	AddSecret(key string, value string)
}

type SecretManager struct {
	storages []SecretStorage
	mu       sync.RWMutex
}

var (
	manager *SecretManager
	once    sync.Once
)

// GetManager returns the singleton SecretManager instance
func GetManager() *SecretManager {
	once.Do(func() {
		manager = &SecretManager{
			storages: []SecretStorage{NewEnvStorage()}, // env storage as default
		}
	})
	return manager
}

// AddStorage adds a new secret storage to the manager
func (m *SecretManager) AddStorage(storage SecretStorage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storages = append(m.storages, storage)
}

// FetchSecret fetches a secret from the registered storages
// It iterates through all storages (starting with env) and returns the first non-empty value
func (m *SecretManager) FetchSecret(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, storage := range m.storages {
		if value := storage.FetchSecret(key); value != "" {
			return value
		}
	}
	return ""
}

// FetchSecret is a convenience function that uses the global manager
func FetchSecret(key string) string {
	return GetManager().FetchSecret(key)
}

// NewEnvStorage creates a new environment-based secret storage
func NewEnvStorage() SecretStorage {
	return &envStorage{
		localSecrets: make(map[string]string),
	}
}

type envStorage struct {
	localSecrets map[string]string
	mu           sync.RWMutex
}

func (e *envStorage) AddSecret(key string, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.localSecrets[key] = value
}

func (e *envStorage) FetchSecret(key string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if value, ok := e.localSecrets[key]; ok {
		return value
	}
	return os.Getenv(key)
}

// Reset resets the manager (useful for testing)
func Reset() {
	manager = nil
	once = sync.Once{}
}
