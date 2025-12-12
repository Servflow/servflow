package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Servflow/servflow/config"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
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

func WithDirectConfigs(directConfigs *DirectConfigs) Option {
	return func(e *Engine) {
		e.directConfigs = directConfigs
	}
}

func WithIdleTimeout(timeout time.Duration) Option {
	return func(e *Engine) {
		e.idleTimeout = timeout
	}
}

type DirectConfigs struct {
	APIConfigs         []*apiconfig.APIConfig
	IntegrationConfigs []apiconfig.IntegrationConfig
}

type Engine struct {
	server        *http.Server
	cfg           *config.Config
	directConfigs *DirectConfigs
	mcpServer     *server.MCPServer
	logger        *zap.Logger
	ctx           context.Context
	cancel        func()
	idleTimeout   time.Duration
	idleTimer     *time.Timer
	timerMutex    sync.Mutex
}

func New(cfg *config.Config, opts ...Option) (*Engine, error) {
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

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

func (e *Engine) Start() error {
	e.ctx = logging.WithLogger(e.ctx, e.logger)

	var apiConfigs []*apiconfig.APIConfig
	var integrationConfigs []apiconfig.IntegrationConfig
	var err error

	if e.directConfigs != nil {
		apiConfigs = e.directConfigs.APIConfigs
		integrationConfigs = e.directConfigs.IntegrationConfigs
	} else {
		apiConfigs, err = LoadAPIConfigsFromYAML(e.cfg.ConfigFolder, false, logging.FromContext(e.ctx))
		if err != nil {
			return fmt.Errorf("error fetching actions: %w", err)
		}

		integrationConfigs, err = LoadIntegrationsConfigFromYAML(e.cfg.IntegrationsFile, logging.FromContext(e.ctx))
		if err != nil {
			return fmt.Errorf("error fetching database configs: %w", err)
		}
	}

	return e.startWithConfigs(apiConfigs, integrationConfigs)
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

	e.initIdleTimer()

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

func (e *Engine) ReloadConfigs(newDirectConfigs *DirectConfigs) error {
	if newDirectConfigs == nil {
		return fmt.Errorf("new configs cannot be nil")
	}

	if len(newDirectConfigs.APIConfigs) == 0 {
		return fmt.Errorf("at least one API config is required")
	}

	logging.DebugContext(e.ctx, "Reloading API configurations...")

	newHandler := e.createCustomMuxHandler(newDirectConfigs.APIConfigs)

	e.server.Handler = newHandler

	logging.InfoContext(e.ctx, "API configurations reloaded successfully")
	return nil
}

func (e *Engine) Stop() error {
	e.timerMutex.Lock()
	if e.idleTimer != nil {
		e.idleTimer.Stop()
		e.idleTimer = nil
	}
	e.timerMutex.Unlock()

	cl, err := storage.GetClient()
	if err != nil {
		return err
	}

	if err := cl.Close(); err != nil {
		return err
	}
	return e.server.Shutdown(e.ctx)
}

func (e *Engine) initIdleTimer() {
	if e.idleTimeout <= 0 {
		return
	}

	e.timerMutex.Lock()
	defer e.timerMutex.Unlock()

	e.idleTimer = time.AfterFunc(e.idleTimeout, func() {
		logging.InfoContext(e.ctx, "Idle timeout reached, shutting down engine")
		e.cancel()
	})
}

func (e *Engine) resetIdleTimer() {
	if e.idleTimeout <= 0 {
		return
	}

	e.timerMutex.Lock()
	defer e.timerMutex.Unlock()

	if e.idleTimer != nil {
		e.idleTimer.Reset(e.idleTimeout)
	}
}
