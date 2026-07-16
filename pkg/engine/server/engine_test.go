package server

import (
	"net/http"
	"net/http/httptest"
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

	engine, err := New("test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, "test", engine.env)
	assert.Equal(t, directConfigs, engine.directConfigs)
	assert.NotNil(t, engine.logger)
	assert.NotNil(t, engine.ctx)
}

func TestNew_WithoutDirectConfigs(t *testing.T) {
	engine, err := New("test")
	require.NoError(t, err)
	assert.NotNil(t, engine)
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
				Name: "action1",
				Type: "stub",
				Config: map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	engineConfig := &EngineConfig{
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
	assert.Equal(t, "http://example.com", directConfigs.EngineConfig.Cors.AllowedOrigins[0])
}

func TestEngine_DoneChan(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("test", WithDirectConfigs(directConfigs))
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

	engine, err := New("test", WithDirectConfigs(directConfigs), WithLogger(nil))
	require.NoError(t, err)
	assert.NotNil(t, engine.logger)
}

func TestNew_EmptyDirectConfigs(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("test", WithDirectConfigs(directConfigs))
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

	engine, err := New("test", WithDirectConfigs(directConfigs), WithLogger(nil))
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
	engine, err := New("test", WithDirectConfigs(directConfigs), WithIdleTimeout(timeout))
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.Equal(t, timeout, engine.idleTimeout)

	engine2, err := New("test", WithDirectConfigs(directConfigs), WithIdleTimeout(0))
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

	engine, err := New("test", WithDirectConfigs(initialConfigs))
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

	engine, err := New("test", WithDirectConfigs(initialConfigs))
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

	engine, err := New("test", WithDirectConfigs(initialConfigs))
	require.NoError(t, err)

	emptyConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	err = engine.ReloadConfigs(emptyConfigs)
	require.NoError(t, err)
}

// stubConfig builds a minimal runnable config: GET listenPath -> stub action ->
// template response, so tests can exercise routes end to end.
func stubConfig(id, listenPath string) *apiconfig.APIConfig {
	return &apiconfig.APIConfig{
		ID: id,
		HttpConfig: apiconfig.HttpConfig{
			ListenPath: listenPath,
			Method:     "GET",
			Next:       "action.run",
		},
		Actions: map[string]apiconfig.Action{
			"run": {
				Name:   "run",
				Type:   "stub",
				Next:   "response.ok",
				Config: map[string]interface{}{"result": "ok"},
			},
		},
		Responses: map[string]apiconfig.ResponseConfig{
			"ok": {
				Name:     "ok",
				Code:     200,
				Type:     "template",
				Template: id,
			},
		},
	}
}

func TestEngine_ServeHTTP_BeforeStart(t *testing.T) {
	engine, err := New("test")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestEngine_ServeHTTP(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{stubConfig("hello", "/hello")},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	require.NoError(t, engine.Start())
	defer engine.Stop()

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	health := get("/health")
	assert.Equal(t, http.StatusOK, health.Code)
	assert.Equal(t, "ok", health.Body.String())

	hello := get("/hello")
	assert.Equal(t, http.StatusOK, hello.Code)
	assert.Equal(t, "hello", hello.Body.String())
}

// ReloadConfigs must take effect for anyone serving the engine, with no
// re-wiring: the engine's identity is the stable handler and reload swaps the
// routing table inside it.
func TestEngine_ReloadConfigs_SwapsRoutes(t *testing.T) {
	initialConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{stubConfig("initial", "/initial")},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("test", WithDirectConfigs(initialConfigs))
	require.NoError(t, err)

	require.NoError(t, engine.Start())
	defer engine.Stop()

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	require.Equal(t, http.StatusOK, get("/initial").Code)
	require.Equal(t, http.StatusNotFound, get("/reloaded").Code)

	newConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{stubConfig("reloaded", "/reloaded")},
		EngineConfig: &EngineConfig{},
	}
	require.NoError(t, engine.ReloadConfigs(newConfigs))

	assert.Equal(t, newConfigs, engine.directConfigs)
	reloaded := get("/reloaded")
	assert.Equal(t, http.StatusOK, reloaded.Code)
	assert.Equal(t, "reloaded", reloaded.Body.String())
	assert.Equal(t, http.StatusNotFound, get("/initial").Code)
}

// Requests and reloads run concurrently in production (a request can arrive
// mid-reload); the routes swap must be safe under the race detector.
func TestEngine_ReloadConfigs_ConcurrentWithRequests(t *testing.T) {
	engine, err := New("test", WithDirectConfigs(&DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{stubConfig("initial", "/initial")},
		EngineConfig: &EngineConfig{},
	}))
	require.NoError(t, err)
	require.NoError(t, engine.Start())
	defer engine.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		}
	}()

	for i := 0; i < 20; i++ {
		require.NoError(t, engine.ReloadConfigs(&DirectConfigs{
			APIConfigs:   []*apiconfig.APIConfig{stubConfig("swap", "/swap")},
			EngineConfig: &EngineConfig{},
		}))
	}
	<-done
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

		engine, err := New("test", WithDirectConfigs(directConfigs))
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

		engine, err := New("test", WithDirectConfigs(directConfigs))
		require.NoError(t, err)

		corsConfig := engine.getCorsConfig()
		require.NotNil(t, corsConfig)
		assert.Empty(t, corsConfig.AllowedOrigins)
	})
}
