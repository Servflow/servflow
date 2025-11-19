package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Option func(*Engine)

func WithLogger(logger *zap.Logger) Option {
	return func(e *Engine) {
		e.config.logger = logger
	}
}

func WithLoggerCore(core zapcore.Core) Option {
	return func(e *Engine) {
		e.config.logger = zap.New(core)
	}
}

func WithDirectConfigs(directConfigs *DirectConfigs) Option {
	return func(e *Engine) {
		e.config.directConfigs = directConfigs
	}
}

func WithPort(port string) Option {
	return func(e *Engine) {
		e.config.port = port
	}
}

func WithEnvironment(env string) Option {
	return func(e *Engine) {
		e.config.env = env
	}
}

func WithConfigFolder(folder string) Option {
	return func(e *Engine) {
		e.config.configFolder = folder
	}
}

func WithIntegrationsFile(file string) Option {
	return func(e *Engine) {
		e.config.integrationsFile = file
	}
}

func FromConfig(cfg *config.Config) Option {
	return func(e *Engine) {
		e.config.port = cfg.Port
		e.config.env = cfg.Env
		e.config.configFolder = cfg.ConfigFolder
		e.config.integrationsFile = cfg.IntegrationsFile
	}
}

func WithDefaults() Option {
	return func(e *Engine) {
		e.config.setDefaults()
	}
}

type DirectConfigs struct {
	APIConfigs         []*apiconfig.APIConfig
	IntegrationConfigs []apiconfig.IntegrationConfig
}

type Engine struct {
	server    *http.Server
	mcpServer *server.MCPServer
	ctx       context.Context
	cancel    func()
	config    engineConfig
}

type engineConfig struct {
	port             string
	env              string
	configFolder     string
	integrationsFile string
	directConfigs    *DirectConfigs
	logger           *zap.Logger
	shutdownTimeout  time.Duration
	readTimeout      time.Duration
	writeTimeout     time.Duration
}

func (c *engineConfig) setDefaults() {
	if c.port == "" {
		c.port = "8080"
	}
	if c.env == "" {
		c.env = "development"
	}
	if c.shutdownTimeout == 0 {
		c.shutdownTimeout = 30 * time.Second
	}
	if c.readTimeout == 0 {
		c.readTimeout = 10 * time.Second
	}
	if c.writeTimeout == 0 {
		c.writeTimeout = 10 * time.Second
	}
}

func (c *engineConfig) validate() error {
	if c.port == "" {
		return errors.New("port is required")
	}
	if c.env == "" {
		return errors.New("environment is required")
	}
	return nil
}

func New(opts ...Option) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		ctx:    ctx,
		cancel: cancel,
		config: engineConfig{},
	}

	for _, opt := range opts {
		opt(e)
	}

	e.config.setDefaults()

	if err := e.config.validate(); err != nil {
		cancel()
		return nil, fmt.Errorf("invalid engine configuration: %w", err)
	}

	if e.config.logger == nil {
		e.config.logger = e.createLogger(e.config.env)
	}

	return e, nil
}

// NewWithConfig creates a new engine with the legacy config.Config for backward compatibility
// Deprecated: Use New with FromConfig option instead
func NewWithConfig(cfg *config.Config, opts ...Option) (*Engine, error) {
	allOpts := make([]Option, 0, len(opts)+1)
	allOpts = append(allOpts, FromConfig(cfg))
	allOpts = append(allOpts, opts...)
	return New(allOpts...)
}

func (e *Engine) DoneChan() <-chan struct{} {
	return e.ctx.Done()
}

func (e *Engine) Start() error {
	e.ctx = logging.WithLogger(e.ctx, e.config.logger)

	var apiConfigs []*apiconfig.APIConfig
	var integrationConfigs []apiconfig.IntegrationConfig
	var err error

	if e.config.directConfigs != nil {
		apiConfigs = e.config.directConfigs.APIConfigs
		integrationConfigs = e.config.directConfigs.IntegrationConfigs
	} else {
		apiConfigs, err = LoadAPIConfigsFromYAML(e.config.configFolder, false, logging.FromContext(e.ctx))
		if err != nil {
			return fmt.Errorf("error fetching actions: %w", err)
		}

		integrationConfigs, err = LoadIntegrationsConfigFromYAML(e.config.integrationsFile, logging.FromContext(e.ctx))
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

	srv, err := e.createServer(apiConfigs, e.config.port)
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
