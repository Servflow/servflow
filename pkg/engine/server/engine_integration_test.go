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

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectConfigEngine_Integration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

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

	directConfigs := &DirectConfigs{
		APIConfigs:   apiConfigs,
		EngineConfig: &EngineConfig{},
	}

	engine, err := New(fmt.Sprintf("%d", port), "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	errChan := make(chan error, 1)
	go func() {
		if err := engine.Start(); err != nil {
			errChan <- err
		}
	}()

	time.Sleep(100 * time.Millisecond)

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

	select {
	case err := <-errChan:
		t.Fatalf("Engine startup error: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	err = engine.Stop()
	require.NoError(t, err)
}

func TestDirectConfigEngine_ValidationError(t *testing.T) {
	apiConfigs := []*apiconfig.APIConfig{
		{
			ID: "",
			HttpConfig: apiconfig.HttpConfig{
				ListenPath: "/test",
				Method:     "GET",
			},
		},
	}

	directConfigs := &DirectConfigs{
		APIConfigs:   apiConfigs,
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8081", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	assert.NotNil(t, engine)
	assert.Equal(t, directConfigs, engine.directConfigs)
}

func TestDirectConfigEngine_EmptyConfigs(t *testing.T) {
	directConfigs := &DirectConfigs{
		APIConfigs:   []*apiconfig.APIConfig{},
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8082", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	err = engine.Start()
	assert.NoError(t, err)

	err = engine.Stop()
	assert.NoError(t, err)
}

func TestDirectConfigEngine_ContextCancellation(t *testing.T) {
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
		APIConfigs:   apiConfigs,
		EngineConfig: &EngineConfig{},
	}

	engine, err := New("8083", "test", WithDirectConfigs(directConfigs))
	require.NoError(t, err)

	doneChan := engine.DoneChan()

	engine.cancel()

	select {
	case <-doneChan:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DoneChan should be closed after context cancellation")
	}
}

func TestDirectConfigEngine_IdleTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	apiConfigs := []*apiconfig.APIConfig{
		{
			ID: "test-idle",
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
		APIConfigs:   apiConfigs,
		EngineConfig: &EngineConfig{},
	}

	engine, err := New(fmt.Sprintf("%d", port), "test", WithDirectConfigs(directConfigs), WithIdleTimeout(200*time.Millisecond))
	require.NoError(t, err)

	errChan := make(chan error, 1)
	go func() {
		if err := engine.Start(); err != nil {
			errChan <- err
		}
	}()

	time.Sleep(50 * time.Millisecond)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	resp, err := http.Get(baseURL + "/test")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	time.Sleep(100 * time.Millisecond)
	select {
	case <-engine.DoneChan():
		t.Fatal("Engine should not have timed out yet")
	default:
	}

	time.Sleep(250 * time.Millisecond)
	select {
	case <-engine.DoneChan():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Engine should have shut down due to idle timeout")
	}

	engine.Stop()
}
