package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveCORSOrigins(t *testing.T) {
	tests := []struct {
		name       string
		apiCors    []string
		engineCors *CorsConfig
		expected   []string
	}{
		{
			name:       "api config overrides engine config",
			apiCors:    []string{"http://api.example.com"},
			engineCors: &CorsConfig{AllowedOrigins: []string{"http://engine.example.com"}},
			expected:   []string{"http://api.example.com"},
		},
		{
			name:       "engine config used when api config is empty",
			apiCors:    []string{},
			engineCors: &CorsConfig{AllowedOrigins: []string{"http://engine.example.com"}},
			expected:   []string{"http://engine.example.com"},
		},
		{
			name:       "engine config used when api config is nil",
			apiCors:    nil,
			engineCors: &CorsConfig{AllowedOrigins: []string{"http://engine.example.com"}},
			expected:   []string{"http://engine.example.com"},
		},
		{
			name:       "returns nil when both are empty",
			apiCors:    []string{},
			engineCors: &CorsConfig{AllowedOrigins: []string{}},
			expected:   nil,
		},
		{
			name:       "returns nil when engine config is nil and api config is empty",
			apiCors:    []string{},
			engineCors: nil,
			expected:   nil,
		},
		{
			name:       "api config used when engine config is nil",
			apiCors:    []string{"http://api.example.com"},
			engineCors: nil,
			expected:   []string{"http://api.example.com"},
		},
		{
			name:       "returns nil when both are nil",
			apiCors:    nil,
			engineCors: nil,
			expected:   nil,
		},
		{
			name:       "api config with multiple origins overrides engine",
			apiCors:    []string{"http://one.example.com", "http://two.example.com"},
			engineCors: &CorsConfig{AllowedOrigins: []string{"http://engine.example.com"}},
			expected:   []string{"http://one.example.com", "http://two.example.com"},
		},
		{
			name:       "engine config with multiple origins used when api empty",
			apiCors:    []string{},
			engineCors: &CorsConfig{AllowedOrigins: []string{"http://one.example.com", "http://two.example.com"}},
			expected:   []string{"http://one.example.com", "http://two.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCORSOrigins(tt.apiCors, tt.engineCors)
			assert.Equal(t, tt.expected, result)
		})
	}
}
