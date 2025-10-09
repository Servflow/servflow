package secretmanager

import (
	"sync"
)

// TODO remove singleton

var once sync.Once
var instance *SecretsManager

type SecretsManager struct {
	sampleSecrets map[string]string
}

func GetSecretsManager() *SecretsManager {
	once.Do(func() {
		instance = &SecretsManager{
			sampleSecrets: make(map[string]string),
		}

		instance.Init()
	})

	return instance
}

func (s *SecretsManager) Init() {
	// load secrets here
}

func (s *SecretsManager) FetchSecret(name string) string {
	return s.sampleSecrets[name]
}

func SecretForKey(name string) string {
	return GetSecretsManager().FetchSecret(name)
}
