package server

import (
	"testing"
	"time"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_WithDirectConfigs(t *testing.T) {
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
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, "8080", engine.port)
	assert.Equal(t, "test", engine.env)
	assert.Equal(t, directConfigs, engine.directConfigs)
	assert.NotNil(t, engine.logger)
	assert.NotNil(t, engine.ctx)
}

func TestNew_WithoutDirectConfigs(t *testing.T) {
	engine, err := New("8080", "test")
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, "8080", engine.port)
	assert.Equal(t, "test", engine.env)
	assert.NotNil(t, engine.directConfigs)
	assert.NotNil(t, engine.directConfigs.EngineConfig)
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

	engineConfig := &EngineConfig{
		Integrations: map[string]apiconfig.IntegrationConfig{
			"test-integration": {
				Type: "test-type",
			},
		},
		Cors: CorsConfig{
			AllowedOrigins: []string{"http://example.com"},
		},
	}

	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{apiConfig},
		EngineConfig: engineConfig,
	}

	assert.Len(t, directConfigs.APIConfigs, 1)
	assert.Equal(t, "test-api", directConfigs.APIConfigs[0].ID)
	assert.NotNil(t, directConfigs.EngineConfig)
	assert.Len(t, directConfigs.EngineConfig.Integrations, 1)
	assert.Equal(t, "http://example.com", directConfigs.EngineConfig.Cors.AllowedOrigins[0])
}

func TestEngine_DoneChan(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	doneChan := engine.DoneChan()
	assert.NotNil(t, doneChan)

	select {
	case <-doneChan:
		t.Fatal("DoneChan should not be closed initially")
	case <-time.After(10 * time.Millisecond):
	}

	engine.cancel()

	select {
	case <-doneChan:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DoneChan should be closed after cancel")
	}
}

func TestNew_WithLogger(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine.logger)
}

func TestNew_EmptyDirectConfigs(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Len(t, engine.directConfigs.APIConfigs, 0)
}

func TestNew_MultipleOptions(t *testing.T) {
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
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.directConfigs)
	assert.NotNil(t, engine.logger)
}

func TestNew_WithIdleTimeout(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	timeout := 5 * time.Minute
	engine, err := New("8080", "test", WithDirectConfigs(directConfigs), WithIdleTimeout(timeout))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, timeout, engine.idleTimeout)

	engine2, err := New("8080", "test", WithDirectConfigs(directConfigs), WithIdleTimeout(0))
	require.NoError(t, err)
	assert.NotNil(t, engine2)
	assert.Equal(t, time.Duration(0), engine2.idleTimeout)
}

func TestEngine_ReloadConfigs(t *testing.T) {
	initialConfigs := &DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "initial-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/initial",
					Method:     "GET",
				},
			},
		},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(initialConfigs))
	require.NoError(t, err)

	err = engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	newConfigs := &DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "reloaded-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/reloaded",
					Method:     "POST",
				},
			},
		},
		EngineConfig: &EngineConfig{},
	}

	err = engine.ReloadConfigs(newConfigs)
	require.NoError(t, err)
	assert.Equal(t, newConfigs, engine.directConfigs)
}

func TestEngine_ReloadConfigs_NilConfigs(t *testing.T) {
	initialConfigs := &DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "initial-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/initial",
					Method:     "GET",
				},
			},
		},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(initialConfigs))
	require.NoError(t, err)

	err = engine.ReloadConfigs(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new configs cannot be nil")
}

func TestEngine_ReloadConfigs_EmptyAPIConfigs(t *testing.T) {
	initialConfigs := &DirectConfigs{
		APIConfigs: []*apiconfig.APIConfig{
			{
				ID: "initial-api",
				HttpConfig: apiconfig.HttpConfig{
					ListenPath: "/initial",
					Method:     "GET",
				},
			},
		},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8080", "test", WithDirectConfigs(initialConfigs))
	require.NoError(t, err)

	emptyConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	err = engine.ReloadConfigs(emptyConfigs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one API config is required")
}

func TestEngine_GetCorsConfig(t *testing.T) {
	t.Run("returns cors config when set", func(t *testing.T) {
		directConfigs := &DirectConfigs{
			APIConfigs: []*apiconfig.APIConfig{},
			EngineConfig: &EngineConfig{
				Cors: CorsConfig{
					AllowedOrigins: []string{"http://example.com"},
				},
			},
		}

		engine, err := New("8080", "test", WithDirectConfigs(directConfigs))
		require.NoError(t, err)

		corsConfig := engine.getCorsConfig()
		require.NotNil(t, corsConfig)
		assert.Equal(t, []string{"http://example.com"}, corsConfig.AllowedOrigins)
	})

	t.Run("returns empty cors config when not set", func(t *testing.T) {
		directConfigs := &DirectConfigs{
			APIConfigs:   []*apiconfig.APIConfig{},
			EngineConfig: &EngineConfig{},
		}

		engine, err := New("8080", "test", WithDirectConfigs(directConfigs))
		require.NoError(t, err)

		corsConfig := engine.getCorsConfig()
		require.NotNil(t, corsConfig)
		assert.Empty(t, corsConfig.AllowedOrigins)
	})
}
