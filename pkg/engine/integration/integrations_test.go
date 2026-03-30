package integration

import (
	"encoding/json"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
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
		err := InitializeIntegration("mock", "mock-1", map[string]any{"key": "value"}, false)
		require.NoError(t, err)

		_, ok := integrationManager.integrations.Load("mock-1")
		require.True(t, ok, "Integration was not stored in the manager")

		err = InitializeIntegration("unknown", "unknown-1", nil, false)
		require.Error(t, err, "Expected error when initializing unregistered integration")
	})

	t.Run("InitializeIntegration lazy load", func(t *testing.T) {
		err := InitializeIntegration("mock", "mock-2", map[string]any{"key": "value"}, true)
		require.NoError(t, err)

		_, ok := integrationManager.integrations.Load("mock-2")
		assert.False(t, ok, "Integration should not be preloaded to manager")

		v, ok := integrationManager.lazyIntegrations.Load("mock-2")
		assert.True(t, ok, "Integration be lazy loaded to manager")

		conf, ok := v.(LazyIntegration)
		assert.True(t, ok, "Integration should be lazy loaded to manager")

		confBytes, err := json.Marshal(conf.Config)
		require.NoError(t, err)
		assert.Equal(t, conf.Type, "mock")
		assert.Equal(t, json.RawMessage(confBytes), conf.Config)
	})
}

func TestIntegrationManager_LazyLoad(t *testing.T) {
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

	err = InitializeIntegration("mock", "mock-1", map[string]any{"key": "value"}, true)
	require.NoError(t, err)

	integration, err := GetIntegration(requestctx.NewTestContext(), "mock-1")
	assert.NoError(t, err)

	mockIntegration, ok := integration.(*mockIntegration)
	assert.True(t, ok)
	assert.Equal(t, "MockIntegration", mockIntegration.Name())
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
