//go:generate mockgen -source integrations.go -destination integrations_mock.go -package integration
package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

type BaseIntegration struct{}

func (b *BaseIntegration) Shutdown(ctx context.Context) error {
	return nil
}

type FieldType string

const (
	FieldTypeString   FieldType = "string"
	FieldTypeBoolean  FieldType = "boolean"
	FieldTypeNumber   FieldType = "number"
	FieldTypePassword FieldType = "password"
	FieldTypeSelect   FieldType = "select"
)

type FieldInfo struct {
	Type        FieldType `json:"type"`
	Label       string    `json:"label"`
	Placeholder string    `json:"placeholder"`
	Required    bool      `json:"required"`
	Default     any       `json:"default,omitempty"`
	Values      []string  `json:"values,omitempty"`
}

type RegistrationInfo struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fields      map[string]FieldInfo `json:"fields"`
	Constructor factoryFunc          `json:"-"`
	ImageURL    string               `json:"image_url"`
}

type Manager struct {
	integrations          sync.Map
	availableConstructors map[string]RegistrationInfo
}

var integrationManager = &Manager{
	availableConstructors: make(map[string]RegistrationInfo),
	integrations:          sync.Map{},
}

type Integration interface {
	Type() string
}

func GetManager() *Manager {
	return integrationManager
}

func (m *Manager) Shutdown(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	var shutdownErr error

	m.integrations.Range(func(key, value any) bool {
		id := key.(string)
		integration := value.(Integration)

		if shutdownable, ok := integration.(Shutdownable); ok {
			logger.Debug("shutting down integration", zap.String("id", id), zap.String("type", integration.Type()))
			if err := shutdownable.Shutdown(ctx); err != nil {
				logger.Error("failed to shutdown integration", zap.String("id", id), zap.Error(err))
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("integration %s: %w", id, err))
			}
		}

		return true
	})

	return shutdownErr
}

type factoryFunc func(map[string]any) (Integration, error)

func RegisterIntegration(integrationType string, info RegistrationInfo) error {
	_, ok := integrationManager.availableConstructors[integrationType]
	if ok {
		return fmt.Errorf("integration type %s already registered", integrationType)
	}
	integrationManager.availableConstructors[integrationType] = info
	return nil
}

func ReplaceIntegrationType(integrationType string, constructor factoryFunc) {
	existing, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		integrationManager.availableConstructors[integrationType] = RegistrationInfo{
			Constructor: constructor,
		}
		return
	}
	existing.Constructor = constructor
	integrationManager.availableConstructors[integrationType] = existing
}

func GetRegisteredIntegrationTypes() []string {
	types := make([]string, 0, len(integrationManager.availableConstructors))
	for integrationType := range integrationManager.availableConstructors {
		types = append(types, integrationType)
	}
	return types
}

func GetInfoForIntegration(integrationType string) (RegistrationInfo, error) {
	info, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return RegistrationInfo{}, fmt.Errorf("integration type %s not registered", integrationType)
	}
	return info, nil
}

// InitializeFromConfig resolves an integration config (fetching any secrets)
// and constructs the integration, replacing any prior instance under the same
// id.
func InitializeFromConfig(cfg apiconfig.IntegrationConfig) error {
	conf, err := resolveConfig(cfg.ID, cfg.Config)
	if err != nil {
		return fmt.Errorf("could not resolve integration config: %w", err)
	}
	return InitializeIntegration(cfg.Type, cfg.ID, conf)
}

// InitializeIntegration constructs the integration for integrationType under id
// and stores it, replacing any prior instance. The config is already resolved
// (secrets fetched) by the time it reaches here.
func InitializeIntegration(integrationType, id string, config map[string]any) error {
	info, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return fmt.Errorf("integration type %s not registered", integrationType)
	}

	integration, err := info.Constructor(config)
	if err != nil {
		return err
	}

	// Construct first; only once the new instance is ready do we shut down and
	// remove the old one, so a failed reload leaves the working integration intact.
	integrationManager.removeIntegration(id)
	integrationManager.integrations.Store(id, integration)

	return nil
}

// removeIntegration drops any existing instance registered under id. A live
// instance implementing Shutdownable is shut down (bounded by a short timeout)
// before removal so reloads don't leak resources.
func (m *Manager) removeIntegration(id string) {
	if existing, ok := m.integrations.Load(id); ok {
		if shutdownable, ok := existing.(Shutdownable); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := shutdownable.Shutdown(ctx); err != nil {
				logging.FromContext(ctx).Error("failed to shutdown integration on reload",
					zap.String("id", id), zap.Error(err))
			}
			cancel()
		}
		m.integrations.Delete(id)
	}
}

// GetIntegration returns the initialized integration registered under id.
func GetIntegration(ctx context.Context, id string) (Integration, error) {
	integration, ok := integrationManager.integrations.Load(id)
	if !ok {
		return nil, fmt.Errorf("integration %s not registered", id)
	}
	return integration.(Integration), nil
}

func RegisterIntegrationsFromConfig(ctx context.Context, integrationsConfig []apiconfig.IntegrationConfig) error {
	wg := &sync.WaitGroup{}
	wg.Add(len(integrationsConfig))
	type errorReport struct {
		integrationID string
		error         error
	}
	errChan := make(chan *errorReport, len(integrationsConfig))
	doneChan := make(chan struct{})

	go func() {
		wg.Wait()
		close(doneChan)
		close(errChan)
	}()
	for _, dsConfig := range integrationsConfig {
		go func(config *apiconfig.IntegrationConfig) {
			defer wg.Done()

			if err := InitializeFromConfig(*config); err != nil {
				errChan <- &errorReport{
					integrationID: config.ID,
					error:         fmt.Errorf("error initializing integration with ID %s and type %s: %w", config.ID, config.Type, err),
				}
				return
			}
		}(&dsConfig)
	}

	logger := logging.FromContext(ctx)

	var hasError bool
	for {
		select {
		case errRp := <-errChan:
			if errRp != nil {
				hasError = true
				logger.Error("error starting integration", zap.String("integration_id", errRp.integrationID), zap.Error(errRp.error))
			}
		case <-doneChan:
			if hasError {
				return errors.New("error starting integrations")
			}
			return nil
		}
	}
}

// resolveConfig materializes an integration's stored config for its
// constructor. A field that names a secret is fetched from the secret manager;
// anything else passes through as written. Secret wins when both are set.
//
// A named-but-missing secret is a hard error rather than a silent empty value:
// an integration credential is never usefully empty, so surfacing it here as a
// boot/reload failure that names the field beats a confusing auth error later.
func resolveConfig(id string, config map[string]apiconfig.ConfigValue) (map[string]any, error) {
	resolved := make(map[string]any, len(config))
	for field, v := range config {
		if v.Secret == "" {
			resolved[field] = v.Value
			continue
		}
		val := secrets.FetchSecret(v.Secret)
		if val == "" {
			return nil, fmt.Errorf("integration %q: field %q: secret %q not found", id, field, v.Secret)
		}
		resolved[field] = val
	}
	return resolved, nil
}
