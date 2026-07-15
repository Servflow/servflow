package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Scrubber removes tracked secret values from a string. Implemented by
// *requestctx.RequestContext; declared here as an interface so the logging
// package stays decoupled from the request layer.
type Scrubber interface {
	// HasSecrets is the fast path: when false (the request resolved no
	// secrets), scrubbing is skipped entirely and logging costs nothing extra.
	HasSecrets() bool
	Scrub(string) string
}

// WrapWithScrubber returns a logger whose output is scrubbed of any secret
// values resolved during the request owning s. Install once at request entry;
// every logger derived from it (via With/FromContext) inherits the scrubbing.
func WrapWithScrubber(l *zap.Logger, s Scrubber) *zap.Logger {
	if s == nil {
		return l
	}
	return l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return &scrubCore{Core: c, s: s}
	}))
}

// scrubCore is a zapcore.Core wrapper that scrubs entry messages and
// string-ish fields on write. Fields attached via With() are scrubbed at
// attach time (best effort — values tracked after attach but logged later
// through such fields bypass scrubbing; call-site fields never do).
type scrubCore struct {
	zapcore.Core
	s Scrubber
}

func (c *scrubCore) With(fields []zapcore.Field) zapcore.Core {
	return &scrubCore{Core: c.Core.With(c.scrubFields(fields)), s: c.s}
}

func (c *scrubCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *scrubCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if !c.s.HasSecrets() {
		return c.Core.Write(ent, fields)
	}
	ent.Message = c.s.Scrub(ent.Message)
	return c.Core.Write(ent, c.scrubFields(fields))
}

func (c *scrubCore) scrubFields(fields []zapcore.Field) []zapcore.Field {
	if !c.s.HasSecrets() {
		return fields
	}
	out := make([]zapcore.Field, len(fields))
	copy(out, fields)
	for i := range out {
		switch out[i].Type {
		case zapcore.StringType:
			out[i].String = c.s.Scrub(out[i].String)
		case zapcore.ByteStringType:
			if b, ok := out[i].Interface.([]byte); ok {
				out[i].Interface = []byte(c.s.Scrub(string(b)))
			}
		case zapcore.ErrorType:
			if err, ok := out[i].Interface.(error); ok {
				out[i] = zapcore.Field{Key: out[i].Key, Type: zapcore.StringType, String: c.s.Scrub(err.Error())}
			}
		}
	}
	return out
}
