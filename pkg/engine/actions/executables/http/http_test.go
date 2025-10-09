package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestHttp_Execute(t *testing.T) {
	cases := []struct {
		Name        string
		Config      Config
		Expected    interface{}
		ShouldError bool
		serverSetup func(t *testing.T) string
	}{
		{
			Name: "Successful Call",
			Config: Config{
				Method:  http.MethodGet,
				Headers: map[string]string{"Content-Type": "test"},
				Body:    json.RawMessage(`{"foo":"bar"}`),
			},
			Expected: map[string]interface{}{"hello": "world"},
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.Header.Get("Content-Type"), "test")

					bod, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, `{"foo": "bar"}`, string(bod))
					value := struct {
						Hello string `json:"hello"`
					}{
						Hello: "world",
					}

					resp, _ := json.Marshal(value)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "has response path",
			Config: Config{
				Method:       http.MethodGet,
				Headers:      map[string]string{"Content-Type": "test"},
				ResponsePath: "hello",
			},
			Expected: "world",
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.Header.Get("Content-Type"), "test")
					value := struct {
						Hello string `json:"hello"`
					}{
						Hello: "world",
					}
					resp, _ := json.Marshal(value)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "invalid response path",
			Config: Config{
				Method:       http.MethodGet,
				ResponsePath: "hello",
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")

					v := struct {
						Hi string `json:"hi"`
					}{
						Hi: "world",
					}

					resp, _ := json.Marshal(v)
					w.Write(resp)
				}))
				return srv.URL
			},
		},
		{
			Name: "Error Call",
			Config: Config{
				Method:  http.MethodPost,
				Headers: nil,
			},
			ShouldError: true,
			serverSetup: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "POST")
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return srv.URL
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			//
			h := New(c.Config)
			//configBytes, err := json.Marshal(c.Config)
			//require.NoError(t, err)
			url := c.serverSetup(t)
			config := c.Config
			config.URL = url

			conf, err := json.Marshal(config)
			require.NoError(t, err)

			resp, err := h.Execute(context.Background(), string(conf))
			if c.ShouldError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, c.Expected, resp)
		})
	}
}

func TestHttp_Config(t *testing.T) {
	cases := []struct {
		Name     string
		Config   Config
		Expected string
	}{
		{
			Name: "Basic Config",
			Config: Config{
				URL:     "https://test.com",
				Method:  http.MethodGet,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`{"test": "value"}`),
			},
			Expected: `{"url":"https://test.com","method":"GET","headers":{"Content-Type":"application/json"},"body":{"test":"value"}, "response_path": ""}`,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			h := New(c.Config)
			result := h.Config()
			require.JSONEq(t, c.Expected, result)
		})
	}
}
