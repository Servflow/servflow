package logging

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type fakeScrubber struct {
	active bool
	secret string
}

func (f *fakeScrubber) HasSecrets() bool { return f.active }
func (f *fakeScrubber) Scrub(s string) string {
	return strings.ReplaceAll(s, f.secret, "«sf:token»")
}

func TestScrubCoreMasksRevealedValues(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	s := &fakeScrubber{active: true, secret: "supersecret"}
	logger := WrapWithScrubber(zap.New(core), s)

	logger.Debug("token is supersecret",
		zap.String("url", "https://x?t=supersecret"),
		zap.ByteString("body", []byte("body supersecret body")),
		zap.Error(errors.New("dial fail: supersecret")),
	)

	entry := logs.All()[0]
	if strings.Contains(entry.Message, "supersecret") {
		t.Errorf("message not scrubbed: %q", entry.Message)
	}
	for _, f := range entry.Context {
		var v string
		switch f.Type {
		case zapcore.StringType:
			v = f.String
		case zapcore.ByteStringType:
			v = string(f.Interface.([]byte))
		}
		if strings.Contains(v, "supersecret") {
			t.Errorf("field %q not scrubbed: %q", f.Key, v)
		}
	}
}

func TestScrubCorePassthroughWhenInactive(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	s := &fakeScrubber{active: false, secret: "supersecret"}
	logger := WrapWithScrubber(zap.New(core), s)

	logger.Info("value supersecret untouched")
	if got := logs.All()[0].Message; got != "value supersecret untouched" {
		t.Errorf("inactive scrubber must pass through, got %q", got)
	}
}

func TestScrubCoreSurvivesWith(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	s := &fakeScrubber{active: true, secret: "supersecret"}
	logger := WrapWithScrubber(zap.New(core), s).With(zap.String("component", "x"))

	logger.Warn("leak supersecret here")
	if strings.Contains(logs.All()[0].Message, "supersecret") {
		t.Error("derived logger (With) lost scrubbing core")
	}
}
