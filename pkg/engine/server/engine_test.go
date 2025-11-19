package server

import (
	"testing"
	"time"

	"github.com/Servflow/servflow/config"
	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_WithDirectConfigs(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "test-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/test",
					Method:     "GET",
				},
			},
		},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.config.directConfigs)
	assert.NotNil(t, engine.config.logger)
	assert.NotNil(t, engine.ctx)
}

func TestNew_WithFileConfig(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	engine, err := New(FromConfig(cfg))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Nil(t, engine.config.directConfigs.APIConfigs)
	assert.NotNil(t, engine.config.logger)
	assert.NotNil(t, engine.ctx)
}

func TestDirectConfigs_APIConfigsIntegrity(t *testing.T) {
	apiConfig := &apiconfig.APIConfig{
		ID: "test-api",
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: "/test",
			Method:     "GET",
		},
		Actions: map[string]apiconfig.Action{
			"action1": {
				Type: "stub",
				Config: map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	integrationConfig := apiconfig.IntegrationConfig{
		ID:   "test-integration",
		Type: "test-type",
	}

	directConfigs := DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{apiConfig},
		IntegrationConfigs: []apiconfig.IntegrationConfig{integrationConfig},
	}

	assert.Len(t, directConfigs.APIConfigs, 1)
	assert.Equal(t, "test-api", directConfigs.APIConfigs[0].ID)
	assert.Len(t, directConfigs.IntegrationConfigs, 1)
	assert.Equal(t, "test-integration", directConfigs.IntegrationConfigs[0].ID)
}

func TestEngine_DoneChan(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	doneChan := engine.DoneChan()
	assert.NotNil(t, doneChan)

	select {
	case <-doneChan:
		t.Fatal("DoneChan should not be closed initially")
	case <-time.After(10 * time.Millisecond):
		// Expected behavior
	}

	// Test that canceling the context closes the done channel
	engine.cancel()

	select {
	case <-doneChan:
		// Expected behavior
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DoneChan should be closed after cancel")
	}
}

func TestNew_WithLogger(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	// Test with custom logger option
	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine.config.logger)
}

func TestNew_EmptyDirectConfigs(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Len(t, engine.config.directConfigs.APIConfigs, 0)
	assert.Len(t, engine.config.directConfigs.IntegrationConfigs, 0)
}

func TestNew_MultipleOptions(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "test-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/test",
					Method:     "GET",
				},
			},
		},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.config.directConfigs)
	assert.NotNil(t, engine.config.logger)
}

func TestNew_WithOptions(t *testing.T) {
	engine, err := New(
		WithPort("9000"),
		WithEnvironment("production"),
		WithConfigFolder("./test-configs"),
		WithIntegrationsFile("./test-integrations.yaml"),
	)
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, "9000", engine.config.port)
	assert.Equal(t, "production", engine.config.env)
	assert.Equal(t, "./test-configs", engine.config.configFolder)
	assert.Equal(t, "./test-integrations.yaml", engine.config.integrationsFile)
	assert.NotNil(t, engine.config.logger)
}

func TestNew_WithDefaults(t *testing.T) {
	engine, err := New(WithDefaults())
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, "8080", engine.config.port)
	assert.Equal(t, "development", engine.config.env)
	assert.NotNil(t, engine.config.logger)
}

func TestNew_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name: "valid config with explicit values",
			options: []Option{
				WithPort("9000"),
				WithEnvironment("production"),
				WithConfigFolder("./configs"),
			},
			expectError: false,
		},
		{
			name: "valid config with defaults only",
			options: []Option{
				WithDefaults(),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := New(tt.options...)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, engine)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, engine)
			}
		})
	}
}

func TestNewWithConfig_BackwardCompatibility(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	engine, err := NewWithConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, cfg.Port, engine.config.port)
	assert.Equal(t, cfg.Env, engine.config.env)
}
