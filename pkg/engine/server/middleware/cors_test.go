package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCors_Handle_AllowedOrigin(t *testing.T) {
	cors := &Cors{AllowedOrigins: []string{"http://example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	err := cors.Handle(w, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Errorf("expected origin %v, got %v", "http://example.com", got)
	}
}

func TestCors_Handle_DisallowedOrigin(t *testing.T) {
	cors := &Cors{AllowedOrigins: []string{"http://example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://notallowed.com")
	w := httptest.NewRecorder()

	err := cors.Handle(w, req)
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %v, got %v", http.StatusForbidden, resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no origin, got %v", got)
	}
}

func TestCors_Handle_Preflight(t *testing.T) {
	t.Run("normal preflight request", func(t *testing.T) {
		cors := &Cors{AllowedOrigins: []string{"http://example.com"}}

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		err := cors.Handle(w, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resp := w.Result()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status %v, got %v", http.StatusNoContent, resp.StatusCode)
		}

		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://example.com" {
			t.Errorf("expected origin %v, got %v", "http://example.com", got)
		}
	})

	t.Run("preflight request empty allowed", func(t *testing.T) {
		cors := &Cors{AllowedOrigins: []string{}}

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()

		err := cors.Handle(w, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resp := w.Result()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status %v, got %v", http.StatusNoContent, resp.StatusCode)
		}

		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://example.com" {
			t.Errorf("expected origin %v, got %v", "http://example.com", got)
		}
	})

}

func TestCors_Handle_PreflightNotAllowed(t *testing.T) {
	cors := &Cors{AllowedOrigins: []string{"http://example.com"}}

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://examples.com")
	w := httptest.NewRecorder()

	err := cors.Handle(w, req)
	assert.ErrorIs(t, err, ErrMiddlewareFailed)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %v, got %v", http.StatusForbidden, resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no origin, got %v", got)
	}
}

func TestCors_Handle_NoOriginHeader(t *testing.T) {
	cors := &Cors{AllowedOrigins: []string{"http://example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	err := cors.Handle(w, req)
	assert.ErrorIs(t, err, ErrMiddlewareFailed)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %v, got %v", http.StatusOK, resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no origin, got %v", got)
	}
}

func TestCors_Handle_EmptyAllowedOrigins(t *testing.T) {
	cors := &Cors{AllowedOrigins: []string{}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://anyorigin.com")
	w := httptest.NewRecorder()

	err := cors.Handle(w, req)
	assert.NoError(t, err)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, resp.StatusCode)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://anyorigin.com" {
		t.Errorf("expected origin %v, got %v", "http://anyorigin.com", got)
	}
}
