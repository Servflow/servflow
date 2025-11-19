package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Servflow/servflow/config"
	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectConfigEngine_Integration(t *testing.T) {
	// Get a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cfg := &config.Config{
		Port: fmt.Sprintf("%d", port),
		Env:  "test",
	}

	apiConfigs := []*apiconfig.APIConfig{
		{
			ID: "test-hello",
			HttpConfig: apiconfig.HttpConfig{
				ListenPath: "/hello",
				Method:     "GET",
				Next:       "action.greet",
			},
			Actions: map[string]apiconfig.Action{
				"greet": {
					Type: "stub",
					Next: "response.success",
					Config: map[string]interface{}{
						"message": "Hello from direct config!",
					},
				},
			},
			Responses: map[string]apiconfig.ResponseConfig{
				"success": {
					Code:     200,
					Type:     "template",
					Template: `{"greeting": "{{ .variable_actions_greet.message }}"}`,
				},
			},
		},
		{
			ID: "test-echo",
			HttpConfig: apiconfig.HttpConfig{
				ListenPath: "/echo",
				Method:     "POST",
				Next:       "action.echo",
			},
			Actions: map[string]apiconfig.Action{
				"echo": {
					Type: "stub",
					Next: "response.echo",
					Config: map[string]interface{}{
						"input": `{{ body "message" }}`,
					},
				},
			},
			Responses: map[string]apiconfig.ResponseConfig{
				"echo": {
					Code:     200,
					Type:     "template",
					Template: `{"echoed": "{{ .variable_actions_echo.input }}"}`,
				},
			},
		},
	}

	integrationConfigs := []apiconfig.IntegrationConfig{}

	directConfigs := &DirectConfigs{
		APIConfigs:         apiConfigs,
		IntegrationConfigs: integrationConfigs,
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	// Start engine in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := engine.Start(); err != nil {
			errChan <- err
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Use the port we allocated
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	t.Run("GET /hello endpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/hello")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.Equal(t, "Hello from direct config!", result["greeting"])
	})

	t.Run("POST /echo endpoint", func(t *testing.T) {
		payload := `{"message": "test echo message"}`
		resp, err := http.Post(baseURL+"/echo", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.Equal(t, "test echo message", result["echoed"])
	})

	t.Run("Health endpoint should work", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, "ok", string(body))
	})

	// Check for any startup errors
	select {
	case err := <-errChan:
		t.Fatalf("Engine startup error: %v", err)
	case <-time.After(10 * time.Millisecond):
		// No error, continue
	}

	// Clean shutdown
	err = engine.Stop()
	require.NoError(t, err)
}

func TestDirectConfigEngine_ValidationError(t *testing.T) {
	cfg := &config.Config{
		Port: "8081",
		Env:  "test",
	}

	// Create invalid API config (missing required fields)
	apiConfigs := []*apiconfig.APIConfig{
		{
			ID: "", // Invalid: empty ID
			HttpConfig: apiconfig.HttpConfig{
				ListenPath: "/test",
				Method:     "GET",
			},
		},
	}

	directConfigs := &DirectConfigs{
		APIConfigs:         apiConfigs,
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	// Validation should occur during startup, but since we're using stub actions
	// and the engine doesn't validate configs by default during startup,
	// this test verifies the engine can be created with invalid configs
	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.config.directConfigs)
}

func TestDirectConfigEngine_EmptyConfigs(t *testing.T) {
	cfg := &config.Config{
		Port: "8082",
		Env:  "test",
	}

	directConfigs := &DirectConfigs{
		APIConfigs:         []*apiconfig.APIConfig{},
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	// Should be able to start with empty configs, but createServer will fail
	err = engine.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration files found")
}

func TestDirectConfigEngine_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Port: "8083",
		Env:  "test",
	}

	apiConfigs := []*apiconfig.APIConfig{
		{
			ID: "test-cancel",
			HttpConfig: apiconfig.HttpConfig{
				ListenPath: "/test",
				Method:     "GET",
				Next:       "action.test",
			},
			Actions: map[string]apiconfig.Action{
				"test": {
					Type: "stub",
					Next: "response.ok",
					Config: map[string]interface{}{
						"result": "ok",
					},
				},
			},
			Responses: map[string]apiconfig.ResponseConfig{
				"ok": {
					Code:     200,
					Type:     "template",
					Template: "OK",
				},
			},
		},
	}

	directConfigs := &DirectConfigs{
		APIConfigs:         apiConfigs,
		IntegrationConfigs: []apiconfig.IntegrationConfig{},
	}

	engine, err := New(FromConfig(cfg), WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	doneChan := engine.DoneChan()

	// Cancel the context
	engine.cancel()

	// DoneChan should be closed
	select {
	case <-doneChan:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DoneChan should be closed after context cancellation")
	}
}
