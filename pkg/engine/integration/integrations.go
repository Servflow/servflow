package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"text/template"

	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type Manager struct {
	integrations          sync.Map
	availableConstructors map[string]factoryFunc
}

var integrationManager = &Manager{
	availableConstructors: make(map[string]factoryFunc),
	integrations:          sync.Map{},
}

type Integration interface {
	Type() string
}

type factoryFunc func(map[string]any) (Integration, error)

func RegisterFactory(integrationType string, constructor factoryFunc) error {
	_, ok := integrationManager.availableConstructors[integrationType]
	if ok {
		return fmt.Errorf("integration type %s already registered", integrationType)
	}
	integrationManager.availableConstructors[integrationType] = constructor
	return nil
}

func ReplaceIntegrationType(integrationType string, constructor factoryFunc) {
	integrationManager.availableConstructors[integrationType] = constructor
}

func InitializeIntegration(integrationType, id string, config map[string]any) error {
	constructor, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return fmt.Errorf("integration type %s not registered", integrationType)
	}

	integration, err := constructor(config)
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

// TODO depreciate config completely and use map[string]interface

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
							return os.Getenv(key)
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

	logger := logging.GetLogger()

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
