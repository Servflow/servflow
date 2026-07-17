package logging

import (
	"context"
	"os"
	"sync"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Build constructs a zap logger for the given environment: "production" gets
// the JSON production config, anything else the console development config,
// both with ISO8601 timestamps. SERVFLOW_LOG_LEVEL, when set to a valid zap
// level, overrides the config's default level. This is the single logger
// construction path for engine and embedders alike.
func Build(env string) *zap.Logger {
	var cfg zap.Config
	switch env {
	case "production":
		cfg = zap.NewProductionConfig()
	default:
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if lvl := os.Getenv("SERVFLOW_LOG_LEVEL"); lvl != "" {
		if parsed, err := zapcore.ParseLevel(lvl); err == nil {
			cfg.Level = zap.NewAtomicLevelAt(parsed)
		}
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

var (
	rootOnce sync.Once
	root     *zap.Logger
)

// GetNewLogger returns the process-wide root logger, built once from
// SERVFLOW_ENV. Despite the historical name it is a singleton: every
// context-less FromContext call and bootstrap shares this instance.
func GetNewLogger() *zap.Logger {
	rootOnce.Do(func() {
		root = Build(os.Getenv("SERVFLOW_ENV"))
	})
	return root
}

// loggerKey is the context key for storing logger instances
type loggerKey struct{}

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext retrieves a logger from the context.
// If no logger is found, it falls back to a fresh logger — but never an
// unscrubbed one when the context carries a RequestContext: a request that
// resolved secrets must not be able to log them through the fallback path.
func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
			return logger
		}
		if rc, ok := requestctx.FromContext(ctx); ok {
			return WrapWithScrubber(GetNewLogger(), rc)
		}
	}
	// Context-less fallback: the shared root.
	return GetNewLogger()
}

// WithContextEnriched returns a logger from context with request context enrichment
func WithContextEnriched(ctx context.Context) *zap.Logger {
	return WithContext(ctx)
}

func WithContext(ctx context.Context) *zap.Logger {
	logger := FromContext(ctx)

	// Extract request context
	reqCtx, ok := requestctx.FromContext(ctx)
	if !ok {
		return logger
	}

	// Always add request ID if available
	return logger.With(zap.String("request_id", reqCtx.ID()))
}

// InfoContext logs an info message using logger from context
func InfoContext(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContextEnriched(ctx).Info(msg, fields...)
}

// DebugContext logs a debug message using logger from context
func DebugContext(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContextEnriched(ctx).Debug(msg, fields...)
}

// WarnContext logs a warning message using logger from context
func WarnContext(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContextEnriched(ctx).Warn(msg, fields...)
}

// ErrorContext logs an error message using logger from context
func ErrorContext(ctx context.Context, msg string, err error, fields ...zapcore.Field) {
	logger := WithContextEnriched(ctx)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Error(msg, fields...)
}

// FatalContext logs a fatal message using logger from context
func FatalContext(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContextEnriched(ctx).Fatal(msg, fields...)
}
