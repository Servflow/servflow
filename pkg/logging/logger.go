package logging

import (
	"context"
	"os"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func GetNewLogger() *zap.Logger {
	env := os.Getenv("SERVFLOW_ENV")
	var cfg zap.Config
	switch env {
	case "production":
		cfg = zap.NewProductionConfig()
	default:
		cfg = zap.NewDevelopmentConfig()
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

// loggerKey is the context key for storing logger instances
type loggerKey struct{}

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext retrieves a logger from the context.
// If no logger is found, it falls back to the singleton logger
func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
			return logger
		}
	}
	// Fallback to singleton during migration
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
