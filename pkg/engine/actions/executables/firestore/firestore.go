package firestore

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
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

	fsClient, err := firestore.NewClient(context.Background(), cfg.ProjectID, option.WithCredentialsJSON([]byte(cfg.ServiceAccount)))
	if err != nil {
		return nil, fmt.Errorf("error creating secret client: %w", err)
	}

	return &Firestore{
		client:       fsClient,
		collectionID: cfg.CollectionID,
		config:       cfg,
	}, nil
}

// Execute resolves every text value in the document template and adds the
// document to the collection.
func (f *Firestore) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	var document map[string]any
	if err := json.Unmarshal([]byte(f.config.DocumentTemplate), &document); err != nil {
		return "", nil, err
	}
	resolved, err := requestctx.ResolveValues(ctx, document)
	if err != nil {
		return "", nil, err
	}

	_, _, err = f.client.Collection(f.collectionID).Add(ctx, resolved)
	if err != nil {
		return "", nil, err
	}
	written, err := json.Marshal(resolved)
	if err != nil {
		return "", nil, err
	}
	return string(written), nil, nil
}

// Definition describes the firestore action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"serviceAccount": {
			Type:        actions.FieldTypeString,
			Label:       "Service Account",
			Description: "Firebase service account JSON",
			Required:    true,
		},
		"projectID": {
			Type:        actions.FieldTypeString,
			Label:       "Project ID",
			Description: "Firebase project ID",
			Required:    true,
		},
		"collectionID": {
			Type:        actions.FieldTypeString,
			Label:       "Collection ID",
			Description: "Firestore collection name",
			Required:    true,
		},
		"documentTemplate": {
			Type:        actions.FieldTypeString,
			Label:       "Document Template",
			Description: "Document template JSON",
			Required:    true,
		},
	}

	return actions.Definition{
		Type:        "firestore",
		Name:        "Firestore",
		Description: "Stores documents in Google Cloud Firestore database",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputValue,
			Description: "The document that was written, as the JSON text it was sent as.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating firestore action: %v", err)
			}
			return NewFirestoreExecutable(cfg)
		},
	}
}
