package logging

import (
	"context"
	"os"
	"sync"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	systemLogger *zap.Logger
	once         sync.Once
)

// init initializes the system logger with production configuration
func init() {
	env := os.Getenv("SERVFLOW_ENV")

	var config zap.Config
	if env == "production" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	once.Do(func() {
		logger, err := config.Build()
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
		systemLogger = logger
	})
}

// GetLogger returns the global system logger instance
func GetLogger() *zap.Logger {
	return systemLogger
}

// WithContext returns a logger with request context fields
func WithContext(ctx context.Context) *zap.Logger {
	logger := GetLogger()

	// Extract request context
	reqCtx, ok := requestctx.FromContext(ctx)
	if !ok {
		return logger
	}

	// Always add request ID if available
	return logger.With(zap.String("request_id", reqCtx.ID()))
}

// GetRequestLogger is an alias for WithContext for more semantic naming
func GetRequestLogger(ctx context.Context) *zap.Logger {
	return WithContext(ctx)
}

// SetLogger allows setting a custom logger instance (mainly for testing)
func SetLogger(l *zap.Logger) {
	systemLogger = l
}

// Sync flushes any buffered log entries
func Sync() error {
	if systemLogger != nil {
		return systemLogger.Sync()
	}
	return nil
}

// Error is a helper function to create a standardized error log
func Error(ctx context.Context, msg string, err error, fields ...zapcore.Field) {
	logger := WithContext(ctx)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Error(msg, fields...)
}

// Info is a helper function to create a standardized info log
func Info(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContext(ctx).Info(msg, fields...)
}

// Debug is a helper function to create a standardized debug log
func Debug(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContext(ctx).Debug(msg, fields...)
}

// Warn is a helper function to create a standardized warning log
func Warn(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContext(ctx).Warn(msg, fields...)
}

// Fatal is a helper function to create a standardized fatal log
func Fatal(ctx context.Context, msg string, fields ...zapcore.Field) {
	WithContext(ctx).Fatal(msg, fields...)
}

// WithFields adds fields to a logger instance and returns a new logger
func WithFields(logger *zap.Logger, fields ...zapcore.Field) *zap.Logger {
	return logger.With(fields...)
}

// WithError adds an error field to a logger instance and returns a new logger
func WithError(logger *zap.Logger, err error) *zap.Logger {
	if err == nil {
		return logger
	}
	return logger.With(zap.Error(err))
}
