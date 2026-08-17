package integration

import (
	"context"
	"sync"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
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
		err := InitializeIntegration("mock", "mock-1", map[string]any{"key": "value"})
		require.NoError(t, err)

		_, ok := integrationManager.integrations.Load("mock-1")
		require.True(t, ok, "Integration was not stored in the manager")

		err = InitializeIntegration("unknown", "unknown-1", nil)
		require.Error(t, err, "Expected error when initializing unregistered integration")
	})

	t.Run("GetIntegration returns the constructed instance", func(t *testing.T) {
		require.NoError(t, InitializeIntegration("mock", "mock-2", map[string]any{"key": "value"}))

		got, err := GetIntegration(context.Background(), "mock-2")
		require.NoError(t, err)
		mock, ok := got.(*mockIntegration)
		require.True(t, ok)
		assert.Equal(t, "MockIntegration", mock.Name())
	})

	t.Run("GetIntegration on an unknown id errors", func(t *testing.T) {
		_, err := GetIntegration(context.Background(), "nope")
		require.Error(t, err)
	})
}

const testPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEabc123
-----END RSA PRIVATE KEY-----`

// registerCapturingMock registers a "mock" integration type whose constructor
// records the config it was handed, so a test can assert on what the resolver
// produced.
func registerCapturingMock(t *testing.T) func() map[string]any {
	t.Helper()

	integrationManager = &Manager{
		availableConstructors: make(map[string]RegistrationInfo),
	}

	var (
		mu     sync.Mutex
		gotCfg map[string]any
	)
	err := RegisterIntegration("mock", RegistrationInfo{
		Name: "Mock",
		Constructor: func(config map[string]any) (Integration, error) {
			mu.Lock()
			defer mu.Unlock()
			gotCfg = config
			return &mockIntegration{name: "MockIntegration"}, nil
		},
	})
	require.NoError(t, err, "registering mock integration")

	return func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return gotCfg
	}
}

func literal(v any) apiconfig.ConfigValue { return apiconfig.ConfigValue{Value: v} }
func secretRef(k string) apiconfig.ConfigValue {
	return apiconfig.ConfigValue{Secret: k}
}

func TestRegisterIntegrationsFromConfig(t *testing.T) {
	t.Run("literals pass through with their types", func(t *testing.T) {
		captured := registerCapturingMock(t)

		err := RegisterIntegrationsFromConfig(context.Background(), []apiconfig.IntegrationConfig{{
			ID:   "typed",
			Type: "mock",
			Config: map[string]apiconfig.ConfigValue{
				"host": literal("db.example.com"),
				"port": literal(float64(5432)),
				"tls":  literal(true),
			},
		}})
		require.NoError(t, err)

		cfg := captured()
		require.NotNil(t, cfg, "constructor should have been called")
		assert.Equal(t, "db.example.com", cfg["host"])
		assert.Equal(t, float64(5432), cfg["port"])
		assert.Equal(t, true, cfg["tls"])
	})

	t.Run("a multi-line secret reaches the constructor intact", func(t *testing.T) {
		captured := registerCapturingMock(t)
		t.Setenv("github_app_pem", testPEM)

		err := RegisterIntegrationsFromConfig(context.Background(), []apiconfig.IntegrationConfig{{
			ID:   "gh",
			Type: "mock",
			Config: map[string]apiconfig.ConfigValue{
				"client_id": literal("Iv1.abc"),
				"pem":       secretRef("github_app_pem"),
			},
		}})
		require.NoError(t, err)

		cfg := captured()
		require.NotNil(t, cfg)
		assert.Equal(t, testPEM, cfg["pem"])
		assert.Equal(t, "Iv1.abc", cfg["client_id"])
	})

	t.Run("a missing secret errors and does not initialize", func(t *testing.T) {
		captured := registerCapturingMock(t)

		err := RegisterIntegrationsFromConfig(context.Background(), []apiconfig.IntegrationConfig{{
			ID:     "missing",
			Type:   "mock",
			Config: map[string]apiconfig.ConfigValue{"token": secretRef("nope_not_set")},
		}})
		require.Error(t, err)

		assert.Nil(t, captured(), "constructor must not run when a secret is missing")
		_, ok := integrationManager.integrations.Load("missing")
		assert.False(t, ok, "a failed integration must not be registered")
	})
}

func TestResolveConfig(t *testing.T) {
	t.Run("secret wins when both are set", func(t *testing.T) {
		t.Setenv("s", "from-secret")
		got, err := resolveConfig("id", map[string]apiconfig.ConfigValue{
			"field": {Value: "from-literal", Secret: "s"},
		})
		require.NoError(t, err)
		assert.Equal(t, "from-secret", got["field"])
	})

	t.Run("literal is used when no secret is named", func(t *testing.T) {
		got, err := resolveConfig("id", map[string]apiconfig.ConfigValue{
			"field": {Value: "plain"},
		})
		require.NoError(t, err)
		assert.Equal(t, "plain", got["field"])
	})

	t.Run("a missing secret names the integration, field and key", func(t *testing.T) {
		_, err := resolveConfig("gh", map[string]apiconfig.ConfigValue{
			"pem": secretRef("absent_key"),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `integration "gh"`)
		assert.Contains(t, err.Error(), `field "pem"`)
		assert.Contains(t, err.Error(), `secret "absent_key"`)
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
