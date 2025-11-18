//go:generate mockgen -source secrets.go -destination secrets_mock.go -package secrets
package secrets

import (
	"os"
	"sync"
)

type SecretStorage interface {
	Init()
	FetchSecret(key string) string
	AddSecret(key string, value string)
}

var (
	storage SecretStorage
	once    sync.Once
)

func SetStorage(s SecretStorage) {
	once.Do(func() {
		s.Init()
		storage = s
	})
}

func GetStorage() SecretStorage {
	if storage == nil {
		SetStorage(NewEnvStorage())
	}
	return storage
}

func FetchSecret(key string) string {
	return GetStorage().FetchSecret(key)
}

func NewEnvStorage() SecretStorage {
	return &envStorage{
		localSecrets: make(map[string]string),
	}
}

type envStorage struct {
	localSecrets map[string]string
}

func (e *envStorage) AddSecret(key string, value string) {
	e.localSecrets[key] = value
}

func (e *envStorage) Init() {
	// init not needed
}

func (e *envStorage) FetchSecret(key string) string {
	if value, ok := e.localSecrets[key]; ok {
		return value
	}
	return os.Getenv(key)
}
