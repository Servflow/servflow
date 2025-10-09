package authenticate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthenticate_New(t *testing.T) {
	t.Run("missing integration ID", func(t *testing.T) {
		_, err := New(Config{
			DatabaseField: "email",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "integration ID required")
	})

	t.Run("missing database field", func(t *testing.T) {
		_, err := New(Config{
			IntegrationID: "testintegration",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database field required")
	})

	t.Run("integration not found", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		_, err := New(Config{
			IntegrationID: "nonexistent",
			DatabaseField: "email",
		})
		require.Error(t, err)
	})

	t.Run("integration not a fetch implementation", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockInvalidIntegration := &mockInvalidIntegration{}

		integration.ReplaceIntegrationType("invalid", func(m map[string]any) (integration.Integration, error) {
			return mockInvalidIntegration, nil
		})

		err := integration.InitializeIntegration("invalid", "invalidds", nil)
		require.NoError(t, err)

		_, err = New(Config{
			IntegrationID: "invalidds",
			DatabaseField: "email",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "integration is not a fetch implementation")
	})

	t.Run("successful initialization", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: "email",
			JWTKey:        "secret",
			Token:         "",
			Collection:    "users",
		})
		require.NoError(t, err)
		assert.NotNil(t, auth)
	})
}

func TestAuthenticate_Execute(t *testing.T) {
	createToken := func(subject string, key string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": subject,
		})

		tokenString, _ := token.SignedString([]byte(key))
		return tokenString
	}

	const (
		jwtKey     = "test-secret-key"
		email      = "user@example.com"
		collection = "users"
		dbField    = "email"
	)

	t.Run("valid token and successful authentication", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		fetchReturn := []map[string]interface{}{
			{"id": "1", "email": email, "name": "Test User"},
		}

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(
			gomock.Any(),
			map[string]string{"collection": collection},
			filters.Filter{Field: dbField, Operation: filters.Equals, Comparator: email},
		).Return(fetchReturn, nil)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: dbField,
			JWTKey:        jwtKey,
			Collection:    collection,
		})
		require.NoError(t, err)

		token := createToken(email, jwtKey)

		config := Config{
			Token:         token,
			JWTKey:        jwtKey,
			DatabaseField: dbField,
			Collection:    collection,
		}
		configStr, err := json.Marshal(config)
		require.NoError(t, err)

		result, err := auth.Execute(context.Background(), string(configStr))
		require.NoError(t, err)
		assert.Equal(t, email, result)
	})

	t.Run("invalid token format", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: "email",
			JWTKey:        "secret",
			Collection:    "users",
		})
		require.NoError(t, err)

		// Create an invalid token
		config := Config{
			Token:         "not-a-valid-jwt-token",
			JWTKey:        "secret",
			DatabaseField: "email",
			Collection:    "users",
		}
		configStr, err := json.Marshal(config)
		require.NoError(t, err)

		_, err = auth.Execute(context.Background(), string(configStr))
		require.Error(t, err)
	})

	t.Run("token with wrong signature", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		mockIntegration := NewMockfetchImplementation(ctr)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: "email",
			JWTKey:        "correct-secret",
			Collection:    "users",
		})
		require.NoError(t, err)

		// Create a token with different key than what will be used to verify
		token := createToken("user@example.com", "wrong-secret")

		config := Config{
			Token:         token,
			JWTKey:        "correct-secret", // Different from the key used to sign
			DatabaseField: "email",
			Collection:    "users",
		}
		configStr, err := json.Marshal(config)
		require.NoError(t, err)

		_, err = auth.Execute(context.Background(), string(configStr))
		require.Error(t, err)
	})

	t.Run("token without subject claim", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		jwtKey := "secret-key"
		jwtKeyBytes := []byte(jwtKey)

		tokenWithoutSub := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"name": "Test User", // No "sub" claim
		})
		tokenString, err := tokenWithoutSub.SignedString([]byte(jwtKey))
		require.NoError(t, err)

		mockIntegration := NewMockfetchImplementation(ctr)
		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err = integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: "email",
			JWTKey:        string(jwtKeyBytes),
			Collection:    "users",
		})
		require.NoError(t, err)

		config := Config{
			Token:         tokenString,
			JWTKey:        string(jwtKeyBytes),
			DatabaseField: "email",
			Collection:    "users",
		}
		configStr, err := json.Marshal(config)
		require.NoError(t, err)

		_, err = auth.Execute(context.Background(), string(configStr))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token subject is invalid")
	})

	t.Run("fetch returns no results", func(t *testing.T) {
		ctr := gomock.NewController(t)
		defer ctr.Finish()

		nonexistentEmail := "nonexistent@example.com"

		fetchReturn := make([]map[string]interface{}, 0)

		mockIntegration := NewMockfetchImplementation(ctr)
		mockIntegration.EXPECT().Fetch(
			gomock.Any(),
			map[string]string{"collection": collection},
			filters.Filter{Field: dbField, Operation: filters.Equals, Comparator: nonexistentEmail},
		).Return(fetchReturn, nil)

		integration.ReplaceIntegrationType("mock", func(m map[string]any) (integration.Integration, error) {
			return mockIntegration, nil
		})

		err := integration.InitializeIntegration("mock", "mockds", nil)
		require.NoError(t, err)

		auth, err := New(Config{
			IntegrationID: "mockds",
			DatabaseField: dbField,
			JWTKey:        jwtKey,
			Collection:    collection,
		})
		require.NoError(t, err)

		token := createToken(nonexistentEmail, jwtKey)

		config := Config{
			Token:         token,
			JWTKey:        jwtKey,
			DatabaseField: dbField,
			Collection:    collection,
		}
		configStr, err := json.Marshal(config)
		require.NoError(t, err)

		resp, err := auth.Execute(context.Background(), string(configStr))
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "token subject is invalid")
	})
}

func TestAuthenticate_Type(t *testing.T) {
	auth := &Action{}
	assert.Equal(t, "authenticate", auth.Type())
}

func TestAuthenticate_Config(t *testing.T) {
	config := Config{
		IntegrationID: "testid",
		DatabaseField: "email",
		JWTKey:        "secret",
		Token:         "token",
		Collection:    "users",
	}

	auth := &Action{
		cfg: config,
	}

	configStr := auth.Config()

	// Parse the JSON string back to verify
	var parsedConfig Config
	err := json.Unmarshal([]byte(configStr), &parsedConfig)
	require.NoError(t, err)

	assert.Equal(t, config, parsedConfig)
}

// mockInvalidIntegration implements integration.Integration but not fetchImplementation
type mockInvalidIntegration struct{}

func (m *mockInvalidIntegration) Type() string {
	return "invalid"
}
