package requestctx

import (
	"context"
	"fmt"
	"sync"
	"text/template"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Span attribute keys owned by the request layer. pkg/tracing aliases them
// (it imports this package; the reverse would cycle).
const (
	// AttrRequestID is stamped on the request's root span via SpanAttributes.
	AttrRequestID = "sf.request_id"
)

// flowDrainTimeout caps how long completion waits for child flows after
// Done(). An un-ended span is never exported, so a hung flow must never leak
// the root span — better a capped duration than a vanished trace.
// Var, not const, so tests can shrink it.
var flowDrainTimeout = 10 * time.Minute

// lifecycle is the request's completion latch: the request completes when the
// main flow is done AND every child flow has ended (or the drain cap fires).
type lifecycle struct {
	mu         sync.Mutex
	rootSpan   trace.Span         // nil when tracing is disabled
	beforeEnd  []func(trace.Span) // run right before rootSpan.End()
	onComplete []func()           // run after completion (span ended or absent)
	flows      int
	mainDone   bool
	completed  bool
	timer      *time.Timer
}

// Options configures a request opened with Start.
type Options struct {
	// ID is the request id; "request_<unixnano>" is generated when empty.
	ID string
	// Logger is the base logger. Start attaches the request_id field, wraps it
	// with the secret scrubber, and installs it in the returned ctx — the
	// logger every step (logging.FromContext) sees. Nil skips logger setup.
	Logger *zap.Logger
	// SpanAttributes are appended to EVERY span created within this request
	// (root and children) by pkg/tracing. sf.request_id is always added.
	SpanAttributes []attribute.KeyValue
	// TemplateFuncs seeds the request template function table.
	TemplateFuncs template.FuncMap
	// TemplateFuncsExclusive is the overwrite flag passed to
	// AddRequestTemplateFunctions for the seeded funcs.
	TemplateFuncsExclusive bool
	// Parent links a sub-workflow to its caller: secrets are shared, and this
	// request registers as a child flow of the parent, so the parent's total
	// time transitively covers this request's entire lifetime.
	Parent *RequestContext
}

// Start opens a request and its main flow. The caller MUST call Done() when
// the main flow completes (response written / Execute returned) — typically
// `defer rc.Done()` immediately after Start.
func Start(ctx context.Context, opts Options) (context.Context, *RequestContext) {
	id := opts.ID
	if id == "" {
		id = fmt.Sprintf("request_%d", time.Now().UnixNano())
	}
	rc := NewRequestContext(id)
	rc.spanAttrs = append(append([]attribute.KeyValue{}, opts.SpanAttributes...),
		attribute.String(AttrRequestID, id))
	if len(opts.TemplateFuncs) > 0 {
		rc.AddRequestTemplateFunctions(opts.TemplateFuncs, opts.TemplateFuncsExclusive)
	}
	if opts.Parent != nil {
		opts.Parent.ShareSecretsWith(rc)
		end := opts.Parent.BeginFlow("workflow:" + id)
		rc.OnComplete(end)
	}
	ctx = WithAggregationContext(ctx, rc)
	if opts.Logger != nil {
		l := WrapWithScrubber(opts.Logger.With(zap.String("request_id", id)), rc)
		ctx = WithLogger(ctx, l)
	}
	return ctx, rc
}

// SpanAttributes returns the request-wide span attributes. Set once in Start
// before the ctx is shared; read-only afterwards.
func (rc *RequestContext) SpanAttributes() []attribute.KeyValue {
	return rc.spanAttrs
}

// BindRootSpan registers the request's root span and hooks to run immediately
// before it ends. Called by pkg/tracing's root constructors. One root per
// request: a second call overwrites (see pitfalls).
func (rc *RequestContext) BindRootSpan(span trace.Span, beforeEnd ...func(trace.Span)) {
	rc.lc.mu.Lock()
	defer rc.lc.mu.Unlock()
	rc.lc.rootSpan = span
	rc.lc.beforeEnd = append(rc.lc.beforeEnd, beforeEnd...)
}

// BeginFlow registers a child flow (dispatch chain, sub-workflow). The
// returned end func is idempotent and safe from any goroutine. name is
// documentation/debugging only.
//
// Flow-count safety: first-level BeginFlow calls happen during plan execution
// (before Done), and nested ones from within a still-open flow, so `flows`
// cannot touch zero prematurely; an end() after a timeout-forced completion is
// a no-op.
func (rc *RequestContext) BeginFlow(name string) (end func()) {
	_ = name
	rc.lc.mu.Lock()
	rc.lc.flows++
	rc.lc.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			rc.lc.mu.Lock()
			rc.lc.flows--
			rc.lc.mu.Unlock()
			rc.maybeComplete(false)
		})
	}
}

// OnComplete registers fn to run at full completion; fires immediately if the
// request already completed. Span-agnostic: works with tracing disabled.
func (rc *RequestContext) OnComplete(fn func()) {
	rc.lc.mu.Lock()
	if rc.lc.completed {
		rc.lc.mu.Unlock()
		fn()
		return
	}
	rc.lc.onComplete = append(rc.lc.onComplete, fn)
	rc.lc.mu.Unlock()
}

// Done marks the main flow complete. Non-blocking. The request fully completes
// — root span End (Duration = TOTAL time), request files closed, OnComplete
// callbacks — once Done has been called AND every BeginFlow has ended, or at
// flowDrainTimeout.
func (rc *RequestContext) Done() {
	rc.lc.mu.Lock()
	if rc.lc.mainDone {
		rc.lc.mu.Unlock()
		return
	}
	rc.lc.mainDone = true
	if rc.lc.flows > 0 {
		rc.lc.timer = time.AfterFunc(flowDrainTimeout, func() { rc.maybeComplete(true) })
	}
	rc.lc.mu.Unlock()
	rc.maybeComplete(false)
}

// maybeComplete finishes the request exactly once, when the main flow is done
// and child flows have drained (or force, on drain timeout). Effects run
// outside the lock, in order: beforeEnd hooks → root span End → close request
// files → OnComplete callbacks.
func (rc *RequestContext) maybeComplete(force bool) {
	rc.lc.mu.Lock()
	if rc.lc.completed || !rc.lc.mainDone || (rc.lc.flows > 0 && !force) {
		rc.lc.mu.Unlock()
		return
	}
	rc.lc.completed = true
	if rc.lc.timer != nil {
		rc.lc.timer.Stop()
	}
	span := rc.lc.rootSpan
	beforeEnd := rc.lc.beforeEnd
	onComplete := rc.lc.onComplete
	rc.lc.onComplete = nil
	rc.lc.mu.Unlock()

	if span != nil {
		for _, fn := range beforeEnd {
			fn(span)
		}
		span.End()
	}
	rc.closeFiles()
	for _, fn := range onComplete {
		fn()
	}
}

// closeFiles closes every request file. Runs once at completion (not at Done:
// child flows may still read request files after the response is written).
// Close errors (incl. double-close) are ignored — cleanup is best-effort.
func (rc *RequestContext) closeFiles() {
	rc.Lock()
	files := make([]*FileValue, 0, len(rc.availableFiles))
	for _, f := range rc.availableFiles {
		files = append(files, f)
	}
	rc.Unlock()
	for _, f := range files {
		_ = f.Close()
	}
}
