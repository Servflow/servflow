package requestctx

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// newRecorder returns a span recorder and a tracer sourced from a provider
// wired to it, for observing End and attributes without touching global state.
func newRecorder() (*tracetest.SpanRecorder, trace.Tracer) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	return sr, tp.Tracer("lifecycle_test")
}

// startSpan opens a fresh root span from tr, returning it and the ctx it lives
// in.
func startSpan(tr trace.Tracer) (context.Context, trace.Span) {
	return tr.Start(context.Background(), "root")
}

// attrValue returns the value of key on the first ended span, or false.
func attrValue(spans []sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, s := range spans {
		for _, kv := range s.Attributes() {
			if string(kv.Key) == key {
				return kv.Value, true
			}
		}
	}
	return attribute.Value{}, false
}

// Case 1: Start → Done, no flows → span ended immediately.
func TestLifecycle_StartDoneNoFlows(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r1"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span)

	rc.Done()

	require.Len(t, sr.Ended(), 1, "root span should end immediately with no flows")
}

// Case 2: BeginFlow x2 → Done → NOT ended → end() x2 → ended exactly once.
func TestLifecycle_FlowsDrainBeforeEnd(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r2"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span)

	end1 := rc.BeginFlow("a")
	end2 := rc.BeginFlow("b")

	rc.Done()
	assert.Len(t, sr.Ended(), 0, "span must not end while flows are open")

	end1()
	assert.Len(t, sr.Ended(), 0, "span must not end with one flow still open")

	end2()
	assert.Len(t, sr.Ended(), 1, "span ends exactly once after all flows drain")
}

// Case 3: end() before Done → Done ends immediately.
func TestLifecycle_EndBeforeDone(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r3"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span)

	end := rc.BeginFlow("a")
	end()
	assert.Len(t, sr.Ended(), 0, "flow ended but main not done yet")

	rc.Done()
	assert.Len(t, sr.Ended(), 1)
}

// Case 4: the same end() called twice → only one decrement.
func TestLifecycle_EndIdempotent(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r4"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span)

	end1 := rc.BeginFlow("a")
	end2 := rc.BeginFlow("b")

	rc.Done()
	end1()
	end1() // duplicate — must NOT over-decrement and complete early
	assert.Len(t, sr.Ended(), 0, "duplicate end must not prematurely complete")

	end2()
	assert.Len(t, sr.Ended(), 1)
}

// Case 5: timeout — BeginFlow, Done, never end → span ends via drain cap;
// late end() is a no-op.
func TestLifecycle_DrainTimeout(t *testing.T) {
	old := flowDrainTimeout
	flowDrainTimeout = 50 * time.Millisecond
	t.Cleanup(func() { flowDrainTimeout = old })

	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r5"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span)

	end := rc.BeginFlow("stuck")
	rc.Done()
	assert.Len(t, sr.Ended(), 0)

	require.Eventually(t, func() bool { return len(sr.Ended()) == 1 }, time.Second, 5*time.Millisecond,
		"span should end at drain timeout")

	end() // late — no-op, no panic, no double end
	assert.Len(t, sr.Ended(), 1)
}

// Case 6: no bound span — Done + OnComplete fire fine (tracing-disabled path).
func TestLifecycle_NoSpan(t *testing.T) {
	_, rc := Start(context.Background(), Options{ID: "r6"})
	var called atomic.Bool
	rc.OnComplete(func() { called.Store(true) })
	rc.Done()
	assert.True(t, called.Load(), "OnComplete must fire even with no span")
}

// Case 7: Parent — child completes gate the parent completion, transitively.
func TestLifecycle_ParentChildNesting(t *testing.T) {
	sr, tr := newRecorder()

	_, parent := Start(context.Background(), Options{ID: "parent"})
	_, pspan := startSpan(tr)
	parent.BindRootSpan(pspan)

	// child registered as a flow of parent via Parent option.
	_, child := Start(context.Background(), Options{ID: "child", Parent: parent})
	_, cspan := startSpan(tr)
	child.BindRootSpan(cspan)

	// give the child its own flow, to prove transitivity.
	childFlow := child.BeginFlow("child-bg")

	parent.Done()
	assert.Len(t, sr.Ended(), 0, "parent has an open child flow")

	child.Done()
	assert.Len(t, sr.Ended(), 0, "child still has an open background flow")

	childFlow()
	// child completes → its OnComplete ends the parent's flow → parent completes.
	require.Len(t, sr.Ended(), 2)
	// parent must end no earlier than child.
	assert.GreaterOrEqual(t, pspan.(sdktrace.ReadOnlySpan).EndTime().UnixNano(),
		cspan.(sdktrace.ReadOnlySpan).EndTime().UnixNano(),
		"parent EndTime >= child EndTime")
}

// Case 8: beforeEnd hooks observe the span before End.
func TestLifecycle_BeforeEndHook(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r8"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span, func(s trace.Span) {
		s.SetAttributes(attribute.String("hooked", "yes"))
	})
	rc.Done()

	require.Len(t, sr.Ended(), 1)
	v, ok := attrValue(sr.Ended(), "hooked")
	require.True(t, ok)
	assert.Equal(t, "yes", v.AsString())
}

// Case 9: Start seeds template funcs and generates an id when opts.ID == "".
func TestLifecycle_StartSeedsFuncsAndID(t *testing.T) {
	_, rc := Start(context.Background(), Options{
		TemplateFuncs: template.FuncMap{"foo": func() string { return "bar" }},
	})
	assert.NotEmpty(t, rc.ID())
	assert.Contains(t, rc.ID(), "request_")
	_, ok := rc.TemplateFunctions()["foo"]
	assert.True(t, ok, "seeded template func should be present")

	// generated ids are stamped as the request-wide span attr too.
	found := false
	for _, kv := range rc.SpanAttributes() {
		if string(kv.Key) == AttrRequestID && kv.Value.AsString() == rc.ID() {
			found = true
		}
	}
	assert.True(t, found, "sf.request_id span attr should match id")
}

// Case 10: Start-installed logger scrubs tracked secrets.
func TestLifecycle_StartLoggerScrubs(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	base := zap.New(core)

	ctx, rc := Start(context.Background(), Options{ID: "r10", Logger: base})
	rc.secrets.track("MY_SECRET", "supersecretvalue")

	l := loggerFrom(t, ctx)
	l.Info("leaking supersecretvalue here")

	require.Equal(t, 1, logs.Len())
	assert.NotContains(t, logs.All()[0].Message, "supersecretvalue")
	assert.Contains(t, logs.All()[0].Message, "«sf:MY_SECRET»")
}

// Case 11: file cleanup timing — files close at completion, not Done; double
// close harmless.
func TestLifecycle_FileCleanupTiming(t *testing.T) {
	_, rc := Start(context.Background(), Options{ID: "r11"})
	rec := &recordCloser{}
	fv := NewFileValue(rec, "f")
	rc.Lock()
	rc.availableFiles["f"] = fv
	rc.Unlock()

	end := rc.BeginFlow("bg")
	rc.Done()
	assert.Equal(t, int32(0), rec.closes.Load(), "file must not close before flow drains")

	end()
	assert.Equal(t, int32(1), rec.closes.Load(), "file closes once at completion")

	// manual re-close tolerated.
	rc.closeFiles()
	assert.LessOrEqual(t, rec.closes.Load(), int32(1), "double close is a no-op (idempotent Close)")
}

// Case 13: late tokens — tokens added after Done, before drain, land in the
// exported root via the beforeEnd hook.
func TestLifecycle_LateTokensStamped(t *testing.T) {
	sr, tr := newRecorder()
	_, rc := Start(context.Background(), Options{ID: "r13"})
	_, span := startSpan(tr)
	rc.BindRootSpan(span, func(s trace.Span) {
		in, out := rc.TokenUsage()
		s.SetAttributes(attribute.Int64("usage.total", in+out))
	})

	end := rc.BeginFlow("bg")
	rc.Done()
	rc.AddTokenUsage(100, 50) // background tokens after response
	end()

	require.Len(t, sr.Ended(), 1)
	v, ok := attrValue(sr.Ended(), "usage.total")
	require.True(t, ok)
	assert.Equal(t, int64(150), v.AsInt64(), "late tokens included in exported total")
}

// Ordering shuffle: the six interleavings of {BeginFlow, Done, end} for a
// single flow all end the span exactly once.
func TestLifecycle_OrderingShuffle(t *testing.T) {
	type step int
	const (
		begin step = iota
		done
		end
	)
	// only orderings where begin precedes end are meaningful (end is the
	// return of begin); enumerate valid permutations.
	orders := [][]step{
		{begin, done, end},
		{begin, end, done},
		{done, begin, end}, // Done before any flow begins
	}
	for i, order := range orders {
		sr, tr := newRecorder()
		_, rc := Start(context.Background(), Options{ID: "ord"})
		_, span := startSpan(tr)
		rc.BindRootSpan(span)
		var endFn func()
		for _, s := range order {
			switch s {
			case begin:
				endFn = rc.BeginFlow("f")
			case done:
				rc.Done()
			case end:
				if endFn != nil {
					endFn()
				}
			}
		}
		assert.Len(t, sr.Ended(), 1, "ordering %d must end span exactly once", i)
	}
}

// Stress: many concurrent flows + a Done at a random-ish point; exactly one
// completion, one ended span. Run with -race.
func TestLifecycle_StressConcurrent(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		sr, tr := newRecorder()
		_, rc := Start(context.Background(), Options{ID: "stress"})
		_, span := startSpan(tr)
		rc.BindRootSpan(span)

		var completions atomic.Int32
		rc.OnComplete(func() { completions.Add(1) })

		const N = 100
		var wg sync.WaitGroup
		ends := make([]func(), N)
		for i := 0; i < N; i++ {
			ends[i] = rc.BeginFlow("f")
		}
		wg.Add(N)
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				// jitter without Math.random: derive from index+iter.
				time.Sleep(time.Duration((i*7+iter)%3) * time.Millisecond)
				ends[i]()
			}(i)
		}
		// Done fires concurrently with the ends.
		go rc.Done()

		wg.Wait()
		rc.Done() // ensure Done happened at least once
		require.Eventually(t, func() bool { return len(sr.Ended()) == 1 }, time.Second, time.Millisecond)
		assert.Equal(t, int32(1), completions.Load(), "exactly one completion")
	}
}

// --- helpers ---

type recordCloser struct{ closes atomic.Int32 }

func (r *recordCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (r *recordCloser) Close() error {
	r.closes.Add(1)
	return nil
}

func loggerFrom(t *testing.T, ctx context.Context) *zap.Logger {
	t.Helper()
	l, ok := LoggerFromContext(ctx)
	require.True(t, ok, "Start should install a logger")
	return l
}
