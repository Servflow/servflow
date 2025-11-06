package jwt

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	Mode   string                 `json:"mode" yaml:"mode"`
	Field  string                 `json:"field" yaml:"field"`
	Key    string                 `json:"key" yaml:"key"`
	Claims map[string]interface{} `json:"claims" yaml:"claims"`
}

type JWT struct {
	config Config
}

func (a *JWT) Type() string {
	return "jwt"
}

func New(config Config) *JWT {
	return &JWT{
		config: config,
	}
}

func (a *JWT) Config() string {
	return a.config.Field
}

func (a *JWT) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	field := modifiedConfig
	if a.config.Mode == "" {
		a.config.Mode = "encode"
	}
	isEncodeMode := a.config.Mode == "encode"

	if isEncodeMode {
		return a.encode(context.Background(), field)
	} else {
		return a.decode(context.Background(), field)
	}
}

func (a *JWT) encode(ctx context.Context, payload string) (interface{}, error) {
	var signingMethod jwt.SigningMethod
	var key interface{}

	secret := []byte(a.config.Key)

	if block, _ := pem.Decode(secret); block != nil {
		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %s", err)
		}

		signingMethod = jwt.SigningMethodRS256
		key = privateKey
	} else {
		signingMethod = jwt.SigningMethodHS256
		key = secret
	}

	claims := jwt.MapClaims{
		"sub": payload,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	}

	for key, value := range a.config.Claims {
		if key != "sub" && key != "exp" {
			claims[key] = value
		}
	}

	token := jwt.NewWithClaims(signingMethod, claims)

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (a *JWT) decode(ctx context.Context, tokenString string) (interface{}, error) {
	secret := []byte(a.config.Key)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		alg := token.Header["alg"]

		switch alg {
		case "HS256", "HS384", "HS512":
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", alg)
			}
			return secret, nil

		case "RS256", "RS384", "RS512":
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", alg)
			}

			block, _ := pem.Decode(secret)
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
		default:
			return nil, fmt.Errorf("unsupported signing method: %v", alg)
		}
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
		return nil, fmt.Errorf("sub claim not found")
	}
	return nil, fmt.Errorf("invalid token")
}

func init() {
	fields := map[string]actions.FieldInfo{
		"mode": {
			Type:        "string",
			Label:       "Mode",
			Placeholder: "sign or verify",
			Required:    true,
		},
		"field": {
			Type:        "string",
			Label:       "Field",
			Placeholder: "Field name for token data",
			Required:    true,
		},
		"key": {
			Type:        "string",
			Label:       "Key",
			Placeholder: "JWT signing/verification key",
			Required:    true,
		},
		"claims": {
			Type:        "object",
			Label:       "Claims",
			Placeholder: "JWT claims as key-value pairs",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("jwt", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var cfg Config
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("error creating jwt action: %v", err)
		}
		return New(cfg), nil
	}, fields); err != nil {
		panic(err)
	}
}
