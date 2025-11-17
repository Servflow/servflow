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

func TestWithLogger(t *testing.T) {
	// Create a test logger
	core, _ := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)

	// Test adding logger to context
	ctx := context.Background()
	ctxWithLogger := WithLogger(ctx, testLogger)

	// Verify logger can be retrieved
	retrievedLogger := FromContext(ctxWithLogger)
	assert.Equal(t, testLogger, retrievedLogger, "Retrieved logger should match the one added to context")
}

func TestWithContextEnriched(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)

	// Create a request context
	reqCtx := requestctx.NewRequestContext("test-request-123")
	ctx := requestctx.WithAggregationContext(context.Background(), reqCtx)
	ctxWithLogger := WithLogger(ctx, testLogger)

	// Get enriched logger and log a message
	enrichedLogger := WithContextEnriched(ctxWithLogger)
	enrichedLogger.Info("test message")

	// Verify the logged entry contains the request ID
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")

	fields := logs[0].ContextMap()
	assert.Equal(t, "test-request-123", fields["request_id"], "request_id should match")
}

func TestInfoContext(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.InfoLevel)
	testLogger := zap.New(core)

	// Create context with logger
	ctx := WithLogger(context.Background(), testLogger)

	// Test InfoContext
	InfoContext(ctx, "test info message", zap.String("key", "value"))

	// Verify log entry
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")
	assert.Equal(t, zap.InfoLevel, logs[0].Level)
	assert.Equal(t, "test info message", logs[0].Message)
	assert.Equal(t, "value", logs[0].ContextMap()["key"])
}

func TestDebugContext(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.DebugLevel)
	testLogger := zap.New(core)

	// Create context with logger
	ctx := WithLogger(context.Background(), testLogger)

	// Test DebugContext
	DebugContext(ctx, "test debug message", zap.String("debug", "info"))

	// Verify log entry
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")
	assert.Equal(t, zap.DebugLevel, logs[0].Level)
	assert.Equal(t, "test debug message", logs[0].Message)
	assert.Equal(t, "info", logs[0].ContextMap()["debug"])
}

func TestWarnContext(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.WarnLevel)
	testLogger := zap.New(core)

	// Create context with logger
	ctx := WithLogger(context.Background(), testLogger)

	// Test WarnContext
	WarnContext(ctx, "test warning message", zap.String("warning", "details"))

	// Verify log entry
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")
	assert.Equal(t, zap.WarnLevel, logs[0].Level)
	assert.Equal(t, "test warning message", logs[0].Message)
	assert.Equal(t, "details", logs[0].ContextMap()["warning"])
}

func TestErrorContext(t *testing.T) {
	// Create an observable logger for testing
	core, recorded := observer.New(zap.ErrorLevel)
	testLogger := zap.New(core)

	// Create context with logger
	ctx := WithLogger(context.Background(), testLogger)

	// Test ErrorContext with error
	testErr := errors.New("test error")
	ErrorContext(ctx, "test error message", testErr, zap.String("extra", "field"))

	// Verify log entry
	logs := recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")
	assert.Equal(t, zap.ErrorLevel, logs[0].Level)
	assert.Equal(t, "test error message", logs[0].Message)
	assert.Equal(t, testErr.Error(), logs[0].ContextMap()["error"])
	assert.Equal(t, "field", logs[0].ContextMap()["extra"])

	// Test ErrorContext without error
	recorded.TakeAll() // Clear previous logs
	ErrorContext(ctx, "test message without error", nil, zap.String("key", "value"))

	logs = recorded.All()
	assert.Equal(t, 1, len(logs), "expected one log entry")
	assert.Equal(t, zap.ErrorLevel, logs[0].Level)
	assert.Equal(t, "test message without error", logs[0].Message)
	assert.Equal(t, "value", logs[0].ContextMap()["key"])
	_, hasError := logs[0].ContextMap()["error"]
	assert.False(t, hasError, "should not have error field when error is nil")
}

func TestContextLoggingWithRequestContext(t *testing.T) {
	// Create an observable logger for testing with debug level
	core, recorded := observer.New(zap.DebugLevel)
	testLogger := zap.New(core)

	// Create a request context
	reqCtx := requestctx.NewRequestContext("test-request-456")
	ctx := requestctx.WithAggregationContext(context.Background(), reqCtx)
	ctxWithLogger := WithLogger(ctx, testLogger)

	// Test all context logging functions with request context
	InfoContext(ctxWithLogger, "info with request context")
	DebugContext(ctxWithLogger, "debug with request context")
	WarnContext(ctxWithLogger, "warn with request context")
	ErrorContext(ctxWithLogger, "error with request context", nil)

	// Verify all entries have request ID
	logs := recorded.All()
	assert.Equal(t, 4, len(logs), "expected four log entries")

	for i, log := range logs {
		fields := log.ContextMap()
		assert.Equal(t, "test-request-456", fields["request_id"], "request_id should be present in log %d", i)
	}
}
