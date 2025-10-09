package logging

import (
	"context"
	"errors"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestGetLogger(t *testing.T) {
	logger := GetLogger()
	assert.NotNil(t, logger, "system logger should not be nil")
}

func TestWithContext(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.DebugLevel)
	SetLogger(zap.New(core))

	// Create a request context
	reqCtx := requestctx.NewRequestContext("test-request-123")
	ctx := requestctx.WithAggregationContext(context.Background(), reqCtx)

	// Get logger with context and log a message
	logger := WithContext(ctx)
	logger.Info("test message")

	// Verify the logged entry contains the request ID
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")

	fields := logs[0].ContextMap()
	assert.Equal(t, "test-request-123", fields["request_id"], "request_id should match")
}

func TestHelperFunctions(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.DebugLevel)
	SetLogger(zap.New(core))

	// Create a request context
	reqCtx := requestctx.NewRequestContext("test-request-123")
	ctx := requestctx.WithAggregationContext(context.Background(), reqCtx)

	// Test all helper functions
	testErr := errors.New("test error")

	Info(ctx, "info message", zap.String("test", "value"))
	Debug(ctx, "debug message")
	Warn(ctx, "warn message")
	Error(ctx, "error message", testErr)

	// Verify logs
	logs := recorded.All()
	assert.Equal(t, 4, len(logs), "expected four log entries")

	// Verify log levels and messages
	assert.Equal(t, zap.InfoLevel, logs[0].Level)
	assert.Equal(t, "info message", logs[0].Message)
	assert.Equal(t, "value", logs[0].ContextMap()["test"])

	assert.Equal(t, zap.DebugLevel, logs[1].Level)
	assert.Equal(t, "debug message", logs[1].Message)

	assert.Equal(t, zap.WarnLevel, logs[2].Level)
	assert.Equal(t, "warn message", logs[2].Message)

	assert.Equal(t, zap.ErrorLevel, logs[3].Level)
	assert.Equal(t, "error message", logs[3].Message)
	assert.Equal(t, testErr.Error(), logs[3].ContextMap()["error"])
}

func TestWithError(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	testErr := errors.New("test error")
	loggerWithError := WithError(logger, testErr)
	loggerWithError.Info("message with error")

	logs := recorded.All()
	assert.Equal(t, 1, len(logs))
	assert.Equal(t, testErr.Error(), logs[0].ContextMap()["error"])

	// Test with nil error
	loggerWithNilError := WithError(logger, nil)
	assert.Equal(t, logger, loggerWithNilError, "logger should be unchanged when error is nil")
}

func TestSetLoggerAndSync(t *testing.T) {
	// Test SetLogger
	originalLogger := GetLogger()

	// Test Sync
	SetLogger(nil)
	assert.NoError(t, Sync(), "Sync should not return error when logger is nil")

	// Restore original logger
	SetLogger(originalLogger)
}
