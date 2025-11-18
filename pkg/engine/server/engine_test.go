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

	directConfigs := &DirectConfigs{
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

	engine, err := New(cfg, WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, cfg, engine.cfg)
	assert.Equal(t, directConfigs, engine.directConfigs)
	assert.NotNil(t, engine.logger)
	assert.NotNil(t, engine.ctx)
}

func TestNew_WithFileConfig(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	engine, err := New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, cfg, engine.cfg)
	assert.Nil(t, engine.directConfigs)
	assert.NotNil(t, engine.logger)
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

	directConfigs := &DirectConfigs{
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

	directConfigs := &DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(cfg, WithDirectConfigs(directConfigs))
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

	directConfigs := &DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	// Test with custom logger option
	engine, err := New(cfg, WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine.logger)
}

func TestNew_EmptyDirectConfigs(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := &DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(cfg, WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Len(t, engine.directConfigs.APIConfigs, 0)
	assert.Len(t, engine.directConfigs.IntegrationConfigs, 0)
}

func TestNew_MultipleOptions(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
		Env:  "test",
	}

	directConfigs := &DirectConfigs{
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

	engine, err := New(cfg, WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.directConfigs)
	assert.NotNil(t, engine.logger)
}
