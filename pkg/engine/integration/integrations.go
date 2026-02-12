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

type IntegrationRegistrationInfo struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fields      map[string]FieldInfo `json:"fields"`
	Constructor factoryFunc          `json:"-"`
}

type Manager struct {
	integrations          sync.Map
	availableConstructors map[string]IntegrationRegistrationInfo
}

var integrationManager = &Manager{
	availableConstructors: make(map[string]IntegrationRegistrationInfo),
	integrations:          sync.Map{},
}

type Integration interface {
	Type() string
}

type factoryFunc func(map[string]any) (Integration, error)

func RegisterIntegration(integrationType string, info IntegrationRegistrationInfo) error {
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
		integrationManager.availableConstructors[integrationType] = IntegrationRegistrationInfo{
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

func GetInfoForIntegration(integrationType string) (IntegrationRegistrationInfo, error) {
	info, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return IntegrationRegistrationInfo{}, fmt.Errorf("integration type %s not registered", integrationType)
	}
	return info, nil
}

func InitializeIntegration(integrationType, id string, config map[string]any) error {
	info, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return fmt.Errorf("integration type %s not registered", integrationType)
	}

	integration, err := info.Constructor(config)
	if err != nil {
		return err
	}

	integrationManager.integrations.Store(id, integration)
	return nil
}

func GetIntegration(id string) (Integration, error) {
	integration, ok := integrationManager.integrations.Load(id)
	if !ok {
		return nil, fmt.Errorf("integration %s not found", id)
	}
	return integration.(Integration), nil
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
			var conf map[string]any

			if dsConfig.Config != nil {
				if err := json.Unmarshal(dsConfig.Config, &conf); err != nil {
					errChan <- &errorReport{
						integrationID: config.ID,
						error:         fmt.Errorf("error parsing database config: %w", err),
					}
					return
				}
			} else {
				conf = dsConfig.NewConfig
			}

			for k, r := range conf {
				switch v := r.(type) {
				case string:
					tmpl, err := template.New("config").Funcs(template.FuncMap{
						"secret": func(key string) string {
							return secrets.FetchSecret(key)
						},
					}).Parse(v)
					if err != nil {
						errChan <- &errorReport{
							integrationID: config.ID,
							error:         fmt.Errorf("error parsing database config: %w", err),
						}
						return
					}

					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, k); err != nil {
						errChan <- &errorReport{
							integrationID: config.ID,
							error:         fmt.Errorf("error executing database config: %w", err),
						}
						return
					}
					conf[k] = buf.String()
				default:

				}
			}

			if err := InitializeIntegration(dsConfig.Type, dsConfig.ID, conf); err != nil {
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
