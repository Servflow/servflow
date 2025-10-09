package secrets

import (
	"os"
	"sync"
)

type SecretStorage interface {
	Init()
	FetchSecret(key string) string
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
	return &envStorage{}
}

type envStorage struct {
}

func (e *envStorage) Init() {
	// init not needed
}

func (e *envStorage) FetchSecret(key string) string {
	return os.Getenv(key)
}
