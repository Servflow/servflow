package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Servflow/servflow/config"
	apiconfig "github.com/Servflow/servflow/pkg/definitions"
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
	"github.com/Servflow/servflow/pkg/engine/yamlloader"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Option func(*Engine)

func WithLogger(core zapcore.Core) Option {
	return func(e *Engine) {
		e.logger = zap.New(core)
	}
}

type DirectConfigs struct {
	APIConfigs         []*apiconfig.APIConfig
	IntegrationConfigs []apiconfig.IntegrationConfig
}

type Engine struct {
	server        *http.Server
	cfg           *config.Config
	yamlLoader    *yamlloader.YAMLLoader
	directConfigs *DirectConfigs
	mcpServer     *server.MCPServer
	logger        *zap.Logger
	ctx           context.Context
	cancel        func()
}

func NewWithConfig(cfg *config.Config, opts ...Option) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		opt(e)
	}

	if e.logger == nil {
		e.logger = e.createLogger(cfg.Env)
	}

	return e, nil
}

func NewWithDirectConfigs(cfg *config.Config, directConfigs *DirectConfigs, opts ...Option) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:           cfg,
		directConfigs: directConfigs,
		ctx:           ctx,
		cancel:        cancel,
	}

	for _, opt := range opts {
		opt(e)
	}

	if e.logger == nil {
		e.logger = e.createLogger(cfg.Env)
	}

	return e, nil
}

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

func (e *Engine) Start() error {
	// Add logger to root context
	e.ctx = logging.WithLogger(e.ctx, e.logger)

	yamlLoader := yamlloader.NewYAMLLoader(
		e.cfg.ConfigFolder,
		e.cfg.IntegrationsFile,
		logging.FromContext(e.ctx),
	)
	e.yamlLoader = yamlLoader

	apiConfigs, err := yamlLoader.FetchAPIConfigs(false)
	if err != nil {
		return fmt.Errorf("error fetching actions: %w", err)
	}

	integrationsConfig, err := yamlLoader.FetchIntegrationsConfig()
	if err != nil {
		return fmt.Errorf("error fetching database configs: %w", err)
	}

	return e.startWithConfigs(apiConfigs, integrationsConfig)
}

func (e *Engine) StartWithDirectConfigs() error {
	e.ctx = logging.WithLogger(e.ctx, e.logger)

	if e.directConfigs == nil {
		return fmt.Errorf("direct configs not provided")
	}

	return e.startWithConfigs(e.directConfigs.APIConfigs, e.directConfigs.IntegrationConfigs)
}

func (e *Engine) startWithConfigs(apiConfigs []*apiconfig.APIConfig, integrationsConfig []apiconfig.IntegrationConfig) error {
	logging.DebugContext(e.ctx, "Starting integrations...")
	if err := integration.RegisterIntegrationsFromConfig(integrationsConfig); err != nil {
		return err
	}

	srv, err := e.createServer(apiConfigs, e.cfg.Port)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}
	e.server = srv

	logging.InfoContext(e.ctx, "starting engine...")
	e.startServer()
	logging.InfoContext(e.ctx, "engine started")
	return nil
}

func (e *Engine) startServer() {
	go func() {
		err := e.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.ErrorContext(e.ctx, "error starting server", err)
			e.cancel()
		}
	}()
}

func (e *Engine) createLogger(env string) *zap.Logger {
	var c zap.Config
	if env == "production" {
		c = zap.NewProductionConfig()
	} else {
		c = zap.NewDevelopmentConfig()
	}
	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := c.Build()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	return logger
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
