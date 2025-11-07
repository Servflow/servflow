package firestore

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/firestore"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

type Config struct {
	ServiceAccount string `json:"serviceAccount"`
	ProjectID      string `json:"projectID"`
	CollectionID   string `json:"collectionID"`

	DocumentTemplate string `json:"documentTemplate"`
}

type Firestore struct {
	client       *firestore.Client
	collectionID string

	config Config
}

func (f *Firestore) Type() string {
	return "firestore"
}

func NewFirestoreExecutable(cfg Config) (*Firestore, error) {
	if cfg.ServiceAccount == "" || cfg.ProjectID == "" || cfg.CollectionID == "" {
		return nil, fmt.Errorf("please make sure service account, projectID and collection id are valid")
	}

	credentialsFromJSON, err := google.CredentialsFromJSON(context.Background(), []byte(cfg.ServiceAccount), secretmanager.DefaultAuthScopes()...)
	if err != nil {
		return nil, fmt.Errorf("error creating credentials from JSON from service account key: %w", err)
	}

	fsClient, err := firestore.NewClient(context.Background(), cfg.ProjectID, option.WithCredentials(credentialsFromJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating secret client: %w", err)
	}

	return &Firestore{
		client:       fsClient,
		collectionID: cfg.CollectionID,
		config:       cfg,
	}, nil
}

func (f *Firestore) Config() string {
	return f.config.DocumentTemplate
}

func (f *Firestore) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	cfg := make(map[string]interface{})
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return "", err
	}

	_, _, err := f.client.Collection(f.collectionID).Add(ctx, cfg)
	if err != nil {
		return "", err
	}
	return modifiedConfig, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"serviceAccount": {
			Type:        "string",
			Label:       "Service Account",
			Placeholder: "Firebase service account JSON",
			Required:    true,
		},
		"projectID": {
			Type:        "string",
			Label:       "Project ID",
			Placeholder: "Firebase project ID",
			Required:    true,
		},
		"collectionID": {
			Type:        "string",
			Label:       "Collection ID",
			Placeholder: "Firestore collection name",
			Required:    true,
		},
		"documentTemplate": {
			Type:        "string",
			Label:       "Document Template",
			Placeholder: "Document template JSON",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("firestore", actions.ActionRegistration{
		Name:        "Firestore",
		Description: "Stores documents in Google Cloud Firestore database",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating firestore action: %v", err)
			}
			return NewFirestoreExecutable(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
