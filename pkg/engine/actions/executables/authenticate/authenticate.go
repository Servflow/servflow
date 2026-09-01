package authenticate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	Integration     string `json:"integration" yaml:"integration"`
	DatabaseField   string `json:"databaseField" yaml:"databaseField"`
	JWTKey          string `json:"jwtKey" yaml:"jwtKey"`
	Token           string `json:"token" yaml:"token"`
	Collection      string `json:"collection" yaml:"collection"`
	FailOnAuthError bool   `json:"failOnAuthError" yaml:"failOnAuthError"`
}

type fetchImplementation interface {
	integration.Integration
	Fetch(ctx context.Context, options map[string]string, filters ...filters.Filter) ([]map[string]interface{}, error)
}

type Action struct {
	fetchImplementation fetchImplementation
	cfg                 Config
}

func New(config Config) (*Action, error) {
	integrationRef := config.Integration
	databaseField := config.DatabaseField

	if integrationRef == "" {
		return nil, errors.New("integration is required")
	}
	if databaseField == "" {
		return nil, errors.New("database field required")
	}

	i, err := integration.GetIntegration(context.Background(), config.Integration)
	if err != nil {
		return nil, err
	}
	config.Integration = ""

	u, ok := i.(fetchImplementation)
	if !ok {
		return nil, errors.New("integration is not a fetch implementation")
	}

	return &Action{
		cfg:                 config,
		fetchImplementation: u,
	}, nil
}

func (a *Action) Config() string {
	jsonString, _ := json.Marshal(a.cfg)
	return string(jsonString)
}

func (a *Action) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var cfg Config

	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, err
	}

	token, err := jwt.Parse(cfg.Token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTKey), nil
	})
	if err != nil {
		return nil, nil, err
	}

	subject := ""
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			subject = sub
		}
	}
	if subject == "" {
		if cfg.FailOnAuthError {
			return nil, nil, fmt.Errorf("%w: invalid token subject", plan.ErrFailure)
		}
		return nil, nil, errors.New("token subject is invalid")
	}

	resp, err := a.fetchImplementation.Fetch(ctx, map[string]string{"collection": cfg.Collection}, filters.Filter{
		Field:      cfg.DatabaseField,
		Operation:  filters.Equals,
		Comparator: subject,
	})
	if err != nil {
		return nil, nil, err
	}
	if len(resp) < 1 {
		if cfg.FailOnAuthError {
			return nil, nil, fmt.Errorf("%w: authentication failed - user not found", plan.ErrFailure)
		}
		return nil, nil, errors.New("token subject is invalid")
	}

	return subject, nil, nil
}

func (a *Action) Type() string {
	return "authenticate"
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "User Database",
			Description: "The SQL or MongoDB integration holding the user records",
			Required:    true,
		},
		"databaseField": {
			Type:        actions.FieldTypeString,
			Label:       "Database Field",
			Description: "Field name in database",
			Required:    true,
		},
		"jwtKey": {
			Type:        actions.FieldTypeString,
			Label:       "JWT Key",
			Description: "JWT signing key",
			Required:    true,
		},
		"token": {
			Type:        actions.FieldTypeString,
			Label:       "Token",
			Description: "Authentication token",
			Required:    true,
		},
		"collection": {
			Type:        actions.FieldTypeString,
			Label:       "Collection",
			Description: "Database collection name",
			Required:    true,
		},
		"failOnAuthError": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Fail on Auth Error",
			Description: "Treat authentication failures as workflow failures",
			Required:    false,
			Default:     true,
		},
	}

	if err := actions.RegisterAction("authenticate", actions.ActionRegistrationInfo{
		Name:        "Authenticate",
		Description: "Validates JWT tokens and authenticates users against database records",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputValue,
			Description: "The authenticated subject, taken from the token's sub claim.",
		},
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating authenticate action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
