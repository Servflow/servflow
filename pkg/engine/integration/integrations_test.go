package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegrationManager(t *testing.T) {
	integrationManager = &Manager{
		availableConstructors: make(map[string]RegistrationInfo),
	}
	mockConstructor := func(config map[string]any) (Integration, error) {
		return &mockIntegration{name: "MockIntegration"}, nil
	}

	err := RegisterIntegration("mock", RegistrationInfo{
		Name:        "Mock",
		Description: "Mock integration for testing",
		Constructor: mockConstructor,
	})
	require.NoError(t, err, "registering mock integration")

	err = RegisterIntegration("mock", RegistrationInfo{
		Name:        "Mock",
		Description: "Mock integration for testing",
		Constructor: mockConstructor,
	})
	require.Error(t, err, "expected error registering mock integration")

	t.Run("InitializeIntegration", func(t *testing.T) {
		err := InitializeIntegration("mock", "mock-1", map[string]any{"key": "value"})
		require.NoError(t, err)

		_, ok := integrationManager.integrations.Load("mock-1")
		require.True(t, ok, "Integration was not stored in the manager")

		err = InitializeIntegration("unknown", "unknown-1", nil)
		require.Error(t, err, "Expected error when initializing unregistered integration")
	})
}

// Mock integration for testing
type mockIntegration struct {
	name string
}

func (m *mockIntegration) Type() string {
	return "mock"
}

func (m *mockIntegration) Name() string {
	return m.name
}

func (m *mockIntegration) Init(config map[string]any) error {
	return nil
}
