package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"text/template"

	"github.com/Servflow/servflow/config"
	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/internal/storage"
	"github.com/Servflow/servflow/pkg/definitions"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/agent"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/authenticate"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/delete_action"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/email"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/fetch"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/fetchvector"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/firestore"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/hash"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/http"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/jwt"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/mongoquery"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/static"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/store"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/storevector"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/stub"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/update"
	"github.com/Servflow/servflow/pkg/engine/integration"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/mongo"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/openai"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/qdrant"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/sql"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/yamlloader"
	"github.com/mark3labs/mcp-go/server"
)

type Engine struct {
	server     *http.Server
	cfg        *config.Config
	yamlLoader *yamlloader.YAMLLoader
	mcpServer  *server.MCPServer

	ctx    context.Context
	cancel func()
}

func NewWithConfig(cfg *config.Config) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	return e, nil
}

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

func (e *Engine) Start() error {
	yamlLoader := yamlloader.NewYAMLLoader(
		e.cfg.ConfigFolder,
		e.cfg.IntegrationsFile,
		logging.GetLogger(),
	)
	e.yamlLoader = yamlLoader
	requestctx.SetSecretStore(yamlLoader)

	apiConfigs, err := yamlLoader.FetchAPIConfigs()
	if err != nil {
		return fmt.Errorf("error fetching actions: %w", err)
	}

	datasourcesConfig, err := yamlLoader.FetchIntegrationsConfig()
	if err != nil {
		return fmt.Errorf("error fetching database configs: %w", err)
	}

	logging.Debug(e.ctx, "Starting integrations...")
	if err := registerIntegrations(datasourcesConfig); err != nil {
		return err
	}

	server, err := e.createServer(apiConfigs, e.cfg.Port)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}
	e.server = server

	logging.Info(e.ctx, "starting engine...")
	e.startServer()
	logging.Info(e.ctx, "engine started")
	return nil
}

// TODO depreciate config completely and use map[string]interface
// NOTE: Datasources is being transitioned to integrations
func registerIntegrations(datasourcesConfig []apiconfig.DatasourceConfig) error {
	wg := &sync.WaitGroup{}
	wg.Add(len(datasourcesConfig))
	errChan := make(chan error, len(datasourcesConfig))
	doneChan := make(chan struct{})

	go func() {
		wg.Wait()
		close(doneChan)
		close(errChan)
	}()
	for _, dsConfig := range datasourcesConfig {
		go func(config *apiconfig.DatasourceConfig) {
			defer wg.Done()
			var conf map[string]any

			if dsConfig.Config != nil {
				if err := json.Unmarshal(dsConfig.Config, &conf); err != nil {
					errChan <- fmt.Errorf("error parsing database config: %w", err)
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
						errChan <- fmt.Errorf("error parsing database config: %w", err)
					}

					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, k); err != nil {
						errChan <- fmt.Errorf("error executing database config: %w", err)
					}
					conf[k] = buf.String()
				default:

				}
			}

			if err := integration.InitializeIntegration(dsConfig.Type, dsConfig.ID, conf); err != nil {
				errChan <- fmt.Errorf("error initializing integration with ID %s and type %s: %w", dsConfig.ID, dsConfig.Type, err)
				return
			}
		}(&dsConfig)
	}

	select {
	case <-doneChan:
		return nil
	case err := <-errChan:
		if err != nil {
			return err
		}
		return nil
	}
}

func (e *Engine) startServer() {
	go func() {
		err := e.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error(e.ctx, "error starting server", err)
			e.cancel()
		}

	}()
}

func (e *Engine) Stop() error {
	cl, err := storage.GetClient()
	if err != nil {
		return err
	}

	if err := cl.Close(); err != nil {
		return err
	}
	return e.server.Shutdown(e.ctx)
}
