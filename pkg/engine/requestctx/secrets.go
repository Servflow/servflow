package requestctx

import (
	"sort"
	"strings"
	"sync"
)

// Secret protection (scrub-gateway model): `{{ secret "name" }}` resolves to
// the secret's REAL value, so template transforms (escape, eq, hash, ...)
// behave exactly as they would on any string. At resolution time the value is
// recorded in the request's secretTable; every observability exit — loggers,
// span attributes, stored action outputs — is derived from the request
// context and scrubs tracked values on the way out (see pkg/logging.scrubCore
// and pkg/tracing's span wrapper). Protection therefore depends on exits being
// reachable only through the context; never log or trace through globals.
const (
	scrubMarkerPrefix = "«sf:"
	scrubMarkerSuffix = "»"

	// minTrackedValueLen guards scrubbing against degenerate secret values:
	// replacing very short strings (e.g. "a") would corrupt unrelated output
	// far more than it protects, so such values are not tracked.
	minTrackedValueLen = 4

	// minTrackedLineLen is the threshold for tracking individual lines of a
	// multi-line secret. Higher than minTrackedValueLen because a single line
	// is likelier to collide with unrelated output.
	minTrackedLineLen = 16
)

// secretTable records the secret values resolved during one request AND every
// child workflow it invokes — child RequestContexts share the table by pointer
// (see ShareSecretsWith), making the root request the single source of truth
// for its whole call tree.
type secretTable struct {
	mu     sync.RWMutex
	values map[string]string // real value → secret name
}

func newSecretTable() *secretTable {
	return &secretTable{values: make(map[string]string)}
}

// track records a resolved secret value so scrubbers can mask it wherever it
// surfaces for the rest of the request. Transformed copies produced by
// template functions (escaped, url-encoded, ...) are tracked automatically by
// the taint wrapper (see taint.go), which calls track with the transform's
// real output — no transform prediction here.
//
// For multi-line secrets (e.g. PEM keys) each substantial line is tracked too:
// lines are verbatim substrings of the value, so a copy whose newlines are
// re-encoded or reflowed by an external system is still masked.
func (t *secretTable) track(name, value string) {
	if len(value) < minTrackedValueLen {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.values[value] = name
	if strings.Contains(value, "\n") {
		for _, line := range strings.Split(value, "\n") {
			line = strings.TrimSpace(line)
			if len(line) >= minTrackedLineLen {
				t.values[line] = name
			}
		}
	}
}

// matchName reports the name of a tracked secret whose value appears in s.
// Used by the taint wrapper to decide whether a transform's output is derived
// from a secret.
func (t *secretTable) matchName(s string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.values) == 0 || s == "" {
		return "", false
	}
	for v, name := range t.values {
		if strings.Contains(s, v) {
			return name, true
		}
	}
	return "", false
}

// scrub replaces any tracked secret value found in s with «sf:name». Exact
// match against the (tiny) tracked set; a no-op for requests that resolved no
// secrets. Longer entries replace first so a full multi-line value is masked
// as one marker rather than line by line.
func (t *secretTable) scrub(s string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.values) == 0 || s == "" {
		return s
	}
	keys := make([]string, 0, len(t.values))
	for v := range t.values {
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, v := range keys {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, scrubMarkerPrefix+t.values[v]+scrubMarkerSuffix)
		}
	}
	return s
}

func (t *secretTable) hasSecrets() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.values) > 0
}

// Scrub replaces any secret value resolved during this request's call tree
// with its «sf:name» marker. Cheap when the request resolved no secrets. Safe
// on a nil receiver so observability paths can call it unconditionally.
func (rc *RequestContext) Scrub(s string) string {
	if rc == nil {
		return s
	}
	return rc.secrets.scrub(s)
}

// HasSecrets reports whether any secret value has been resolved in this
// request's call tree — the fast path check for scrubbers.
func (rc *RequestContext) HasSecrets() bool {
	if rc == nil {
		return false
	}
	return rc.secrets.hasSecrets()
}

// ShareSecretsWith makes child use THIS context's secret table, so secrets
// resolved by a parent workflow are scrubbed from a child's (e.g. one invoked
// via callworkflow) logs and spans, and vice versa. Call before the child
// context is used concurrently.
func (rc *RequestContext) ShareSecretsWith(child *RequestContext) {
	if rc == nil || child == nil {
		return
	}
	child.secrets = rc.secrets
}

// ScrubValue walks an action output (strings, maps, slices as produced by
// JSON decoding) and scrubs tracked secret values from every string in it.
// Values are mutated in place where possible; non-container, non-string types
// pass through untouched.
func (rc *RequestContext) ScrubValue(v interface{}) interface{} {
	if rc == nil || !rc.secrets.hasSecrets() {
		return v
	}
	return scrubWalk(v, rc.secrets)
}

func scrubWalk(v interface{}, t *secretTable) interface{} {
	switch x := v.(type) {
	case string:
		return t.scrub(x)
	case []byte:
		return []byte(t.scrub(string(x)))
	case map[string]interface{}:
		for k, val := range x {
			x[k] = scrubWalk(val, t)
		}
		return x
	case map[string]string:
		for k, val := range x {
			x[k] = t.scrub(val)
		}
		return x
	case []interface{}:
		for i := range x {
			x[i] = scrubWalk(x[i], t)
		}
		return x
	case []map[string]interface{}:
		for i := range x {
			scrubWalk(x[i], t)
		}
		return x
	case []string:
		for i := range x {
			x[i] = t.scrub(x[i])
		}
		return x
	default:
		return v
	}
}
