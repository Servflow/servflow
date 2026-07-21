package requestctx

import (
	"context"

	"go.uber.org/zap"
)

// loggerCtxKey stores the request logger in a context. It lives here (not in
// pkg/logging, which imports this package) so Start can install the
// fully-built request logger; pkg/logging delegates to these primitives.
type loggerCtxKey struct{}

// WithLogger stores the request logger in ctx.
func WithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext returns the logger installed by WithLogger, if any.
func LoggerFromContext(ctx context.Context) (*zap.Logger, bool) {
	l, ok := ctx.Value(loggerCtxKey{}).(*zap.Logger)
	return l, ok
}
