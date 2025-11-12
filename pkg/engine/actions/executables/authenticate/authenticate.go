package authenticate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	IntegrationID string `json:"integration_id"`
	DatabaseField string `json:"database_field"`
	JWTKey        string `json:"jwt_key"`
	Token         string `json:"token"`
	Collection    string `json:"collection"`
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
	integrationID := config.IntegrationID
	databaseField := config.DatabaseField

	if integrationID == "" {
		return nil, errors.New("integration ID required")
	}
	if databaseField == "" {
		return nil, errors.New("database field required")
	}

	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}
	config.IntegrationID = ""

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

func (a *Action) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var cfg Config

	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, err
	}

	token, err := jwt.Parse(cfg.Token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTKey), nil
	})
	if err != nil {
		return nil, err
	}

	subject := ""
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			subject = sub
		}
	}
	if subject == "" {
		return nil, errors.New("token subject is invalid")
	}

	resp, err := a.fetchImplementation.Fetch(ctx, map[string]string{"collection": cfg.Collection}, filters.Filter{
		Field:      cfg.DatabaseField,
		Operation:  filters.Equals,
		Comparator: subject,
	})
	if err != nil {
		return nil, err
	}
	if len(resp) < 1 {
		return nil, errors.New("token subject is invalid")
	}

	return subject, nil
}

func (a *Action) Type() string {
	return "authenticate"
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integration_id": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Integration ID",
			Placeholder: "Database integration identifier",
			Required:    true,
		},
		"database_field": {
			Type:        actions.FieldTypeString,
			Label:       "Database Field",
			Placeholder: "Field name in database",
			Required:    true,
		},
		"jwt_key": {
			Type:        actions.FieldTypeString,
			Label:       "JWT Key",
			Placeholder: "JWT signing key",
			Required:    true,
		},
		"token": {
			Type:        actions.FieldTypeString,
			Label:       "Token",
			Placeholder: "Authentication token",
			Required:    true,
		},
		"collection": {
			Type:        actions.FieldTypeString,
			Label:       "Collection",
			Placeholder: "Database collection name",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("authenticate", actions.ActionRegistrationInfo{
		Name:        "Authenticate",
		Description: "Validates JWT tokens and authenticates users against database records",
		Fields:      fields,
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
