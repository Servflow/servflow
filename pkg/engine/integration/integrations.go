//go:generate mockgen -source integrations.go -destination integrations_mock.go -package integration
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"text/template"

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
	lazyIntegrations      sync.Map
}

type LazyIntegration struct {
	Type   string
	Config json.RawMessage
}

var integrationManager = &Manager{
	availableConstructors: make(map[string]RegistrationInfo),
	integrations:          sync.Map{},
	lazyIntegrations:      sync.Map{},
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

func InitializeIntegration(integrationType, id string, config map[string]any, shouldLazyLoad bool) error {
	info, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return fmt.Errorf("integration type %s not registered", integrationType)
	}

	if shouldLazyLoad {
		jsonConfig, err := json.Marshal(config)
		if err != nil {
			return err
		}
		integrationManager.lazyIntegrations.Store(id, LazyIntegration{
			Type:   integrationType,
			Config: jsonConfig,
		})
	} else {
		integration, err := info.Constructor(config)
		if err != nil {
			return err
		}

		integrationManager.integrations.Store(id, integration)
	}

	return nil
}

// TODO the config is being converted to and from json multiple times, fix that

// GetIntegration gets an initialized registration from the list of integration
// as an interface
func GetIntegration(id string) (Integration, error) {
	//var (
	//	integration any
	//	ok          bool
	//)
	integration, ok := integrationManager.integrations.Load(id)
	if !ok {
		lazyIntegrationConf, ok := integrationManager.lazyIntegrations.Load(id)
		if !ok {
			return nil, fmt.Errorf("integration %s not registered", id)
		}

		lazyIntegration := lazyIntegrationConf.(LazyIntegration)
		info, err := GetInfoForIntegration(lazyIntegration.Type)
		if err != nil {
			return nil, fmt.Errorf("could not find info to lazy load integration for %s: %w", id, err)
		}

		config := map[string]any{}
		if err := json.Unmarshal(lazyIntegration.Config, &config); err != nil {
			return nil, err
		}
		integration, err := info.Constructor(config)
		if err != nil {
			return nil, fmt.Errorf("could not lazy load integration for %s: %w", id, err)
		}

		return integration, nil
	} else {
		return integration.(Integration), nil
	}
}

func RegisterIntegrationsFromConfig(integrationsConfig []apiconfig.IntegrationConfig) error {
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
			var (
				conf map[string]any
				buf  bytes.Buffer
			)

			tmpl, err := template.New("config").Funcs(template.FuncMap{
				"secret": func(key string) string {
					return secrets.FetchSecret(key)
				},
			}).Parse(string(config.Config))

			if err != nil {
				errChan <- &errorReport{
					integrationID: config.ID,
					error:         err,
				}
				return
			}

			if err := tmpl.Execute(&buf, map[string]string{}); err != nil {
				errChan <- &errorReport{
					integrationID: config.ID,
					error:         err,
				}
			}
			if err := json.Unmarshal(buf.Bytes(), &conf); err != nil {
				errChan <- &errorReport{
					integrationID: config.ID,
					error:         err,
				}
			}

			if err := InitializeIntegration(dsConfig.Type, dsConfig.ID, conf, false); err != nil {
				errChan <- &errorReport{
					integrationID: config.ID,
					error:         fmt.Errorf("error initializing integration with ID %s and type %s: %w", dsConfig.ID, dsConfig.Type, err),
				}
				return
			}
		}(&dsConfig)
	}

	logger := logging.FromContext(context.Background())

	var hasError bool
	for {
		select {
		case errRp := <-errChan:
			if errRp != nil {
				hasError = true
				logger.Error("error starting integration", zap.String("integrationID", errRp.integrationID), zap.Error(errRp.error))
			}
		case <-doneChan:
			if hasError {
				return errors.New("error starting integrations")
			}
			return nil
		}
	}
}
