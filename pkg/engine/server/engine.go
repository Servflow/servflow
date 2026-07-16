package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Servflow/servflow/pkg/apiconfig"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/agent"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/authenticate"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/delete_action"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/email"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/fetch"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/fetchvector"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/firestore"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/get_key"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/hash"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/http"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/javascript"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/jwt"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/mongoquery"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/parallel"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/save"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/static"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/store_key"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/storevector"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/stub"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/update"
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/write"
	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/engine/integration"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/claude"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/mongo"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/openai"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/qdrant"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/sql"
	"github.com/Servflow/servflow/pkg/engine/plan"
	_ "github.com/Servflow/servflow/pkg/engine/responses/http"
	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"github.com/Servflow/servflow/pkg/tracing"
	"github.com/gorilla/mux"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type EngineConfig struct {
	Cors CorsConfig `yaml:"cors"`
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

func WithFileConfig(configFolder, engineConfigFile string) Option {
	return func(e *Engine) {
		apiConfigs, err := LoadAPIConfigsFromYAML(configFolder, false, e.logger)
		if err != nil {
			e.logger.Error("failed to load API configs from YAML", zap.Error(err))
			return
		}
		engineConfig, integrations, err := LoadEngineConfigFromYAML(engineConfigFile, e.logger)
		if err != nil {
			e.logger.Error("failed to load engine config from YAML", zap.Error(err))
			return
		}
		e.directConfigs = &DirectConfigs{
			APIConfigs:   apiConfigs,
			EngineConfig: engineConfig,
		}
		// Register the file-loaded integrations up front so they are available
		// before any plan is compiled from these configs.
		if err := e.RegisterIntegrations(integrations); err != nil {
			e.initErr = fmt.Errorf("failed to register integrations from file config: %w", err)
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
	Headers           map[string]string
	SpanAttributes    func() map[string]string
}

func WithOTELTracing(cfg TracingConfig) Option {
	return func(e *Engine) {
		shutdown, err := tracing.InitTracer(e.ctx, tracing.Config{
			ServiceName:       cfg.ServiceName,
			OrgID:             cfg.OrgID,
			CollectorEndpoint: cfg.CollectorEndpoint,
			Headers:           cfg.Headers,
			SpanAttributes:    cfg.SpanAttributes,
		})
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

// WorkspaceProvider resolves the file capability for a given API config, from
// the workspace assigned to the agent that owns the config. Returning (nil, nil)
// means the config has no workspace and its file actions will fail with
// requestctx.ErrNoWorkspace.
type WorkspaceProvider func(config *apiconfig.APIConfig) (requestctx.Workspace, error)

// WithWorkspaceProvider installs a resolver that supplies each config's workspace
// capability. The resolved workspace is baked into the config's plan and applied
// to every request's context before execution.
func WithWorkspaceProvider(provider WorkspaceProvider) Option {
	return func(e *Engine) {
		e.workspaceProvider = provider
	}
}

// resolveWorkspace resolves the workspace capability for a config via the
// configured provider, returning nil when no provider is installed.
func (e *Engine) resolveWorkspace(config *apiconfig.APIConfig) (requestctx.Workspace, error) {
	if e.workspaceProvider == nil {
		return nil, nil
	}
	return e.workspaceProvider(config)
}

type DirectConfigs struct {
	APIConfigs   []*apiconfig.APIConfig
	EngineConfig *EngineConfig
}

// Engine routes workflow requests. It is an http.Handler and never binds a
// listener itself: callers own their http.Server (the standalone binary serves
// the engine directly; servflow-pro composes it with the dashboard first).
type Engine struct {
	env           string
	directConfigs *DirectConfigs
	// routes holds the current routing table. Reload builds a complete
	// replacement router and swaps this pointer, so every request — no matter
	// who serves the engine — routes through the latest table, race-free.
	// In-flight requests finish on the router they started with.
	routes            atomic.Pointer[mux.Router]
	mcpServer         *server.MCPServer
	logger            *zap.Logger
	ctx               context.Context
	cancel            func()
	idleTimeout       time.Duration
	idleTimer         *time.Timer
	timerMutex        sync.Mutex
	tracerShutdown    func(context.Context) error
	requestHook       RequestHook
	backgroundManager *plan.BackgroundManager
	workspaceProvider WorkspaceProvider
	initErr           error
}

func New(env string, opts ...Option) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
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

	if e.initErr != nil {
		return nil, e.initErr
	}

	if e.directConfigs == nil {
		e.directConfigs = &DirectConfigs{
			APIConfigs:   []*apiconfig.APIConfig{},
			EngineConfig: &EngineConfig{},
		}
	}

	return e, nil
}

// ServeHTTP dispatches to the current routing table. The table is looked up
// per request, so a ReloadConfigs takes effect on the very next request with
// no re-wiring by the caller.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	routes := e.routes.Load()
	if routes == nil {
		http.Error(w, "engine not started", http.StatusServiceUnavailable)
		return
	}
	routes.ServeHTTP(w, r)
}

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

// RegisterIntegrations registers (or replaces) the given integrations in the
// integration manager. It is decoupled from Start so callers can control when
// integrations become available - notably before compiling any plans that
// resolve integrations eagerly (e.g. a trigger scheduler). Safe to call more
// than once; each call re-initializes the supplied integrations by id.
func (e *Engine) RegisterIntegrations(configs []apiconfig.IntegrationConfig) error {
	logging.DebugContext(e.ctx, "Registering integrations...")
	return integration.RegisterIntegrationsFromConfig(configs)
}

// Start prepares the engine to serve: it compiles the initial routing table and
// starts lifecycle helpers (background manager, idle timer). It does not bind a
// listener — serve the engine by passing it to an http.Server as its Handler.
func (e *Engine) Start() error {
	e.ctx = logging.WithLogger(e.ctx, e.logger)

	e.backgroundManager = plan.NewBackgroundManager(e.ctx)

	e.routes.Store(e.createMuxHandler(e.directConfigs.APIConfigs))

	e.initIdleTimer()

	logging.InfoContext(e.ctx, "engine started")
	return nil
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
		logging.WarnContext(e.ctx, "Reloading with no API configurations - engine will run with no API endpoints")
	}

	logging.DebugContext(e.ctx, "Reloading API configurations...")

	// Integrations must already be registered (via RegisterIntegrations) before
	// this call: planning resolves each action's integration eagerly, so a newly
	// added integration must be in the manager before its config is planned.
	e.directConfigs = newDirectConfigs
	e.routes.Store(e.createMuxHandler(newDirectConfigs.APIConfigs))

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

	if e.backgroundManager != nil {
		e.backgroundManager.Shutdown()
	}

	if e.tracerShutdown != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.tracerShutdown(shutdownCtx); err != nil {
			logging.ErrorContext(e.ctx, "failed to shutdown tracer", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer cancel()
	if err := integration.GetManager().Shutdown(shutdownCtx); err != nil {
		logging.ErrorContext(e.ctx, "failed to shutdown integrations", err)
	}

	cl, err := storage.GetClient()
	if err != nil {
		return err
	}

	if err := cl.Close(); err != nil {
		return err
	}
	e.cancel()
	return nil
}

// ShutdownServer stops the engine's request-side lifecycle (idle timer,
// background work) without the full teardown Stop performs — Stop also closes
// shared resources like the storage client, which a caller running several
// engines in one process must not do per engine. Shutting down the caller's
// http.Server is the caller's job.
func (e *Engine) ShutdownServer() error {
	e.timerMutex.Lock()
	if e.idleTimer != nil {
		e.idleTimer.Stop()
		e.idleTimer = nil
	}
	e.timerMutex.Unlock()

	if e.backgroundManager != nil {
		e.backgroundManager.Shutdown()
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
