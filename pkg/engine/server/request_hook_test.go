package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRequestHook(t *testing.T) {
	tests := []struct {
		name           string
		hook           RequestHook
		wantStatus     int
		wantBody       string
		wantHandlerHit bool
	}{
		{
			name: "hook passes and handler is called",
			hook: func(w http.ResponseWriter, r *http.Request) bool {
				return true
			},
			wantStatus:     http.StatusOK,
			wantBody:       "handler reached",
			wantHandlerHit: true,
		},
		{
			name: "hook aborts request",
			hook: func(w http.ResponseWriter, r *http.Request) bool {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return false
			},
			wantStatus:     http.StatusUnauthorized,
			wantBody:       "unauthorized",
			wantHandlerHit: false,
		},
		{
			name:           "nil hook is handled gracefully",
			hook:           nil,
			wantStatus:     http.StatusOK,
			wantBody:       "handler reached",
			wantHandlerHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerHit := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerHit = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("handler reached"))
			})

			logger := zap.NewNop()
			engine := &Engine{
				logger:      logger,
				requestHook: tt.hook,
			}

			wrappedHandler := engine.wrapMiddlewareWithReqIDLogger(logger, testHandler)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
			assert.Equal(t, tt.wantHandlerHit, handlerHit)
		})
	}
}

func TestRequestHookHasAccessToEnrichedContext(t *testing.T) {
	hookCalled := false
	hook := func(w http.ResponseWriter, r *http.Request) bool {
		hookCalled = true
		return true
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := zap.NewNop()
	engine := &Engine{
		logger:      logger,
		requestHook: hook,
	}

	wrappedHandler := engine.wrapMiddlewareWithReqIDLogger(logger, testHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hookCalled)
}

func TestRequestHookWithEngineOption(t *testing.T) {
	hookCalled := false
	hook := func(w http.ResponseWriter, r *http.Request) bool {
		hookCalled = true
		return true
	}

	engine, err := New("0", "test", WithRequestHook(hook))
	assert.NoError(t, err)
	assert.NotNil(t, engine.requestHook)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := engine.wrapMiddlewareWithReqIDLogger(engine.logger, testHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.True(t, hookCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestHookCanModifyResponse(t *testing.T) {
	hook := func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("X-Custom-Header", "hook-added")
		return true
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	logger := zap.NewNop()
	engine := &Engine{
		logger:      logger,
		requestHook: hook,
	}

	wrappedHandler := engine.wrapMiddlewareWithReqIDLogger(logger, testHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hook-added", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "ok", w.Body.String())
}
