package logging

import (
	"go.uber.org/zap"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Scrubber and WrapWithScrubber moved to requestctx (so requestctx.Start can
// wrap the request logger without importing this package); re-exported here
// to keep the logging API stable.
type Scrubber = requestctx.Scrubber

func WrapWithScrubber(l *zap.Logger, s Scrubber) *zap.Logger {
	return requestctx.WrapWithScrubber(l, s)
}
