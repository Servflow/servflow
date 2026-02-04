package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Servflow/servflow/internal/tracing"
	"github.com/Servflow/servflow/pkg/apiconfig"
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
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/parallel"
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
	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type EngineConfig struct {
	Integrations map[string]apiconfig.IntegrationConfig `yaml:"integrations"`
	Cors         CorsConfig                             `yaml:"cors"`
}

type CorsConfig struct {
	AllowedOrigins []string `yaml:"allowedOrigins"`
	AllowedMethods []string `yaml:"allowedMethods"`
}

// RequestHook is a function that runs before each request.
// It receives the response writer and request, returning true to continue
// processing or false to halt (the hook is responsible for writing the response).
type RequestHook func(http.ResponseWriter, *http.Request) bool

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

// WithExternalMode prevents the engine from creating a server if set to true.
// The engines handler can be accessed via .Handler()
func WithExternalMode(externalMode bool) Option {
	return func(e *Engine) {
		e.externalMode = externalMode
	}
}

func WithFileConfig(configFolder, engineConfigFile string) Option {
	return func(e *Engine) {
		apiConfigs, err := LoadAPIConfigsFromYAML(configFolder, false, e.logger)
		if err != nil {
			e.logger.Error("failed to load API configs from YAML", zap.Error(err))
			return
		}
		engineConfig, err := LoadEngineConfigFromYAML(engineConfigFile, e.logger)
		if err != nil {
			e.logger.Error("failed to load engine config from YAML", zap.Error(err))
			return
		}
		e.directConfigs = &DirectConfigs{
			APIConfigs:   apiConfigs,
			EngineConfig: engineConfig,
		}
	}
}

func WithIdleTimeout(timeout time.Duration) Option {
	return func(e *Engine) {
		e.idleTimeout = timeout
	}
}

type TracingConfig struct {
	ServiceName       string
	OrgID             string
	CollectorEndpoint string
}

func WithOTELTracing(cfg TracingConfig) Option {
	return func(e *Engine) {
		shutdown, err := tracing.InitTracer(
			e.ctx,
			cfg.ServiceName,
			cfg.OrgID,
			cfg.CollectorEndpoint,
		)
		if err != nil {
			logging.ErrorContext(e.ctx, "failed to initialize tracer", err)
		} else {
			e.tracerShutdown = shutdown
			logging.InfoContext(e.ctx, "OTEL tracing initialized")
		}
	}
}

func WithSecretStorage(storage secrets.SecretStorage) Option {
	return func(e *Engine) {
		secrets.GetManager().AddStorage(storage)
	}
}

func WithRequestHook(hook RequestHook) Option {
	return func(e *Engine) {
		e.requestHook = hook
	}
}

type DirectConfigs struct {
	APIConfigs   []*apiconfig.APIConfig
	EngineConfig *EngineConfig
}

type Engine struct {
	server         *http.Server
	port           string
	env            string
	directConfigs  *DirectConfigs
	mcpServer      *server.MCPServer
	logger         *zap.Logger
	ctx            context.Context
	cancel         func()
	idleTimeout    time.Duration
	idleTimer      *time.Timer
	timerMutex     sync.Mutex
	tracerShutdown func(context.Context) error
	requestHook    RequestHook
	externalMode   bool
	handler        http.HandlerFunc
}

func New(port, env string, opts ...Option) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		port:   port,
		env:    env,
		ctx:    ctx,
		cancel: cancel,
	}

	if e.logger == nil {
		e.logger = e.createLogger(env)
	}

	for _, opt := range opts {
		opt(e)
	}

	if e.directConfigs == nil {
		e.directConfigs = &DirectConfigs{
			APIConfigs:   []*apiconfig.APIConfig{},
			EngineConfig: &EngineConfig{},
		}
	}

	return e, nil
}

func (e *Engine) Handler() http.HandlerFunc {
	return e.handler
}

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

func (e *Engine) Start() error {
	e.ctx = logging.WithLogger(e.ctx, e.logger)

	var integrationConfigs []apiconfig.IntegrationConfig
	if e.directConfigs.EngineConfig != nil {
		integrationConfigs = e.directConfigs.EngineConfig.GetIntegrationConfigs()
	}

	logging.DebugContext(e.ctx, "Starting integrations...")
	if err := integration.RegisterIntegrationsFromConfig(integrationConfigs); err != nil {
		return err
	}

	e.handler = e.createHandler()

	if !e.externalMode {
		logging.DebugContext(e.ctx, "Starting HTTP server...")
		srv, err := e.createServer(e.port)
		if err != nil {
			return fmt.Errorf("error creating server: %w", err)
		}
		e.server = srv

		e.initIdleTimer()

		logging.InfoContext(e.ctx, "starting engine...")
		e.startServer()
		logging.InfoContext(e.ctx, "engine started")
	}

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

	newHandler := e.createMuxHandler(newDirectConfigs.APIConfigs)

	e.handler = newHandler.ServeHTTP
	e.directConfigs = newDirectConfigs
	if !e.externalMode && e.server != nil {
		e.server.Handler = newHandler
	}

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

	if e.tracerShutdown != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.tracerShutdown(shutdownCtx); err != nil {
			logging.ErrorContext(e.ctx, "failed to shutdown tracer", err)
		}
	}

	cl, err := storage.GetClient()
	if err != nil {
		return err
	}

	if err := cl.Close(); err != nil {
		return err
	}
	if e.server != nil {
		if err := e.server.Shutdown(e.ctx); err != nil {
			return err
		}
	}
	return nil
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

func (e *Engine) getCorsConfig() *CorsConfig {
	if e.directConfigs != nil && e.directConfigs.EngineConfig != nil {
		return &e.directConfigs.EngineConfig.Cors
	}
	return nil
}
