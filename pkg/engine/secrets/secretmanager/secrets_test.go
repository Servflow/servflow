package secretmanager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretsManager(t *testing.T) {
	t.Run("Singleton Test", func(t *testing.T) {
		require.Equal(t, GetSecretsManager(), GetSecretsManager())
	})
}
