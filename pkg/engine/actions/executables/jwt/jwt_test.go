package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateRSAKeyPair creates a new RSA key pair for testing
func generateRSAKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	if err := privateKey.Validate(); err != nil {
		return "", "", fmt.Errorf("generated key validation failed: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		},
	)
	if privateKeyPEM == nil {
		return "", "", fmt.Errorf("failed to encode private key to PEM")
	}

	publicKey := &privateKey.PublicKey

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		},
	)
	if publicKeyPEM == nil {
		return "", "", fmt.Errorf("failed to encode public key to PEM")
	}

	return string(privateKeyPEM), string(publicKeyPEM), nil
}

func TestJWT_Execute_Encode(t *testing.T) {
	// HMAC-based tests
	t.Run("EncodeWithPlainTextSecret", func(t *testing.T) {
		config := Config{
			Mode:  "encode",
			Field: "testSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte("testSecret")),
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), "testSubject")
		require.NoError(t, err)

		// Verify token can be decoded
		tokenString := result.(string)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("testSecret"), nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "testSubject", claims["sub"])
	})

	t.Run("EncodeWithCustomClaims", func(t *testing.T) {
		config := Config{
			Mode:  "encode",
			Field: "testSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte("testSecret")),
			Claims: map[string]interface{}{
				"role":       "admin",
				"department": "engineering",
				"userID":     123,
				"isActive":   true,
				"score":      98.6,
				"metadata":   map[string]interface{}{"team": "backend"},
				"sub":        "shouldNotOverride", // Should not override the actual subject
			},
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), "testSubject")
		require.NoError(t, err)

		// Verify token can be decoded with custom claims
		tokenString := result.(string)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("testSecret"), nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "testSubject", claims["sub"]) // Original subject should be preserved
		assert.Equal(t, "admin", claims["role"])
		assert.Equal(t, "engineering", claims["department"])
		assert.Equal(t, float64(123), claims["userID"])
		assert.Equal(t, true, claims["isActive"])
		assert.Equal(t, 98.6, claims["score"])
		metadataMap, ok := claims["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "backend", metadataMap["team"])
	})

	t.Run("Encode wih unencrypted text", func(t *testing.T) {
		config := Config{
			Mode:  "encode",
			Field: "testSubject",
			Key:   "testSecret",
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), "testSubject")
		require.NoError(t, err)

		// Verify token can be decoded
		tokenString := result.(string)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("testSecret"), nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "testSubject", claims["sub"])
	})

	t.Run("EncodeWithRSAPrivateKey", func(t *testing.T) {
		privateKeyPEM, publicKeyPEM, err := generateRSAKeyPair()
		require.NoError(t, err)

		config := Config{
			Mode:  "encode",
			Field: "testSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte(privateKeyPEM)),
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), "testSubject")
		require.NoError(t, err)

		// Verify token can be decoded with the public key
		tokenString := result.(string)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Ensure the correct signing method was used
			if token.Method != jwt.SigningMethodRS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			block, _ := pem.Decode([]byte(publicKeyPEM))
			if block == nil {
				return nil, fmt.Errorf("failed to parse PEM block containing public key")
			}

			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, err
			}

			rsaKey, ok := pub.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("not an RSA public key")
			}
			return rsaKey, nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "testSubject", claims["sub"])
	})
}

func TestJWT_Execute_Decode(t *testing.T) {
	// HMAC-based token for testing
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "decodedSubject",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("testSecret"))
	require.NoError(t, err)

	t.Run("DecodeWithPlainTextSecret", func(t *testing.T) {
		config := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte("testSecret")),
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), tokenString)
		require.NoError(t, err)
		assert.Equal(t, "decodedSubject", result)
	})

	t.Run("decode with unencrypted secret", func(t *testing.T) {
		config := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   "testSecret",
		}
		jwtAction := New(config)

		result, err := jwtAction.Execute(context.Background(), tokenString)
		require.NoError(t, err)
		assert.Equal(t, "decodedSubject", result)
	})

	t.Run("DecodeWithWrongSecret", func(t *testing.T) {
		config := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte("wrongSecret")),
		}
		jwtAction := New(config)

		_, err := jwtAction.Execute(context.Background(), tokenString)
		require.Error(t, err)
	})

	t.Run("DecodeExpiredToken", func(t *testing.T) {

		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "expiredSubject",
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
		expiredTokenString, err := expiredToken.SignedString([]byte("testSecret"))
		require.NoError(t, err)

		config := Config{
			Mode:  "decode",
			Field: expiredTokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte("testSecret")),
		}
		jwtAction := New(config)

		_, err = jwtAction.Execute(context.Background(), expiredTokenString)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	// Test with RSA keys
	t.Run("RSAKeyPair", func(t *testing.T) {
		privateKeyPEM, publicKeyPEM, err := generateRSAKeyPair()
		require.NoError(t, err)

		// Base64 encode keys

		// Test encode with private key
		encodeConfig := Config{
			Mode:  "encode",
			Field: "testSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte(privateKeyPEM)),
		}
		encodeAction := New(encodeConfig)

		tokenResult, err := encodeAction.Execute(context.Background(), "testSubject")
		require.NoError(t, err)
		tokenString := tokenResult.(string)

		// Test decode with public key
		decodeConfig := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte(publicKeyPEM)),
		}
		decodeAction := New(decodeConfig)

		result, err := decodeAction.Execute(context.Background(), tokenString)
		require.NoError(t, err)
		assert.Equal(t, "testSubject", result)
	})

	t.Run("EncodeAndDecodeWithRSAKeyPair", func(t *testing.T) {
		privateKeyPEM, publicKeyPEM, err := generateRSAKeyPair()
		require.NoError(t, err)

		// Configure encoder with the private key
		encodeConfig := Config{
			Mode:  "encode",
			Field: "rsaSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte(privateKeyPEM)),
		}
		encodeAction := New(encodeConfig)

		// Encode a subject
		tokenResult, err := encodeAction.Execute(context.Background(), "rsaSubject")
		require.NoError(t, err)
		tokenString := tokenResult.(string)

		// Configure decoder with the public key
		decodeConfig := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte(publicKeyPEM)),
		}
		decodeAction := New(decodeConfig)

		// Decode the token
		result, err := decodeAction.Execute(context.Background(), tokenString)
		require.NoError(t, err)
		assert.Equal(t, "rsaSubject", result)
	})

	t.Run("FailDecodeWithWrongPublicKey", func(t *testing.T) {
		// Generate first key pair
		privateKeyPEM1, _, err := generateRSAKeyPair()
		require.NoError(t, err)

		// Generate second key pair (different keys)
		_, publicKeyPEM2, err := generateRSAKeyPair()
		require.NoError(t, err)

		// Encode with first private key
		encodeConfig := Config{
			Mode:  "encode",
			Field: "rsaSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte(privateKeyPEM1)),
		}
		encodeAction := New(encodeConfig)

		tokenResult, err := encodeAction.Execute(context.Background(), "rsaSubject")
		require.NoError(t, err)
		tokenString := tokenResult.(string)

		// Try to decode with the second (wrong) public key
		decodeConfig := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte(publicKeyPEM2)),
		}
		decodeAction := New(decodeConfig)

		// This should fail since keys don't match
		_, err = decodeAction.Execute(context.Background(), tokenString)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "crypto/rsa: verification error")
	})

	t.Run("EncodeWithRSAAndCustomClaims", func(t *testing.T) {
		privateKeyPEM, publicKeyPEM, err := generateRSAKeyPair()
		require.NoError(t, err)

		// Configure encoder with the private key and custom claims
		encodeConfig := Config{
			Mode:  "encode",
			Field: "rsaSubject",
			Key:   base64.StdEncoding.EncodeToString([]byte(privateKeyPEM)),
			Claims: map[string]interface{}{
				"tenant_id":    "tenant-123",
				"user_type":    "service",
				"access_level": 5,
				"paid_account": true,
				"permissions":  []string{"read", "write", "admin"},
				"exp":          "shouldNotOverride", // Should not override the actual expiration
			},
		}
		encodeAction := New(encodeConfig)

		// Encode a subject
		tokenResult, err := encodeAction.Execute(context.Background(), "rsaSubject")
		require.NoError(t, err)
		tokenString := tokenResult.(string)

		// Configure decoder with the public key
		decodeConfig := Config{
			Mode:  "decode",
			Field: tokenString,
			Key:   base64.StdEncoding.EncodeToString([]byte(publicKeyPEM)),
		}
		decodeAction := New(decodeConfig)

		// Decode the token
		result, err := decodeAction.Execute(context.Background(), tokenString)
		require.NoError(t, err)
		assert.Equal(t, "rsaSubject", result)

		// Manually verify custom claims are present
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			block, _ := pem.Decode([]byte(publicKeyPEM))
			if block == nil {
				return nil, fmt.Errorf("failed to parse PEM block containing public key")
			}

			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, err
			}

			rsaKey, ok := pub.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("not an RSA public key")
			}
			return rsaKey, nil
		})
		require.NoError(t, err)
		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "rsaSubject", claims["sub"])
		assert.Equal(t, "tenant-123", claims["tenant_id"])
		assert.Equal(t, "service", claims["user_type"])
		assert.Equal(t, float64(5), claims["access_level"])
		assert.Equal(t, true, claims["paid_account"])
		permissions, ok := claims["permissions"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, "read", permissions[0])
		assert.Equal(t, "write", permissions[1])
		assert.Equal(t, "admin", permissions[2])
		assert.NotEqual(t, "shouldNotOverride", claims["exp"])
	})
}
