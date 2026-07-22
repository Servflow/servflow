package requestctx

import (
	"fmt"
	"reflect"
	"text/template"
)

// Taint tracking propagates secret tracking through template transforms.
//
// A secret resolves to its real value (so `escape`, `urlquery`, `hash`, etc.
// behave normally), and the scrubber masks tracked values by exact match. The
// gap that leaves: a transform produces a NEW string the scrubber has never
// seen — `escape (secret "x")` yields an escaped copy that no longer matches
// the raw value, so it slips into logs/traces.
//
// Rather than predict what each transform produces, we observe it: every
// template function is wrapped so that whenever it runs with an argument
// containing a tracked secret, its string result is tracked too. This covers
// every transform — built-in, request-scoped, or added by an entry handler —
// and any chain of them, because each tainted output re-taints the next call's
// input. Only transforms performed OUTSIDE the engine (e.g. an API base64-ing
// a token before echoing it) remain invisible, which is unavoidable.

// taintWrap returns fn wrapped to track its output when fed a tracked secret.
// Functions that cannot return a string as their first result are returned
// unchanged (they can't leak a string), as are non-functions.
func (rc *RequestContext) taintWrap(fn interface{}) interface{} {
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return fn
	}
	t := v.Type()
	if t.NumOut() == 0 || t.Out(0).Kind() != reflect.String {
		return fn
	}
	variadic := t.IsVariadic()
	return reflect.MakeFunc(t, func(args []reflect.Value) []reflect.Value {
		var out []reflect.Value
		if variadic {
			// MakeFunc presents a variadic function's trailing arguments as a
			// single slice in the final position, which is exactly the form
			// CallSlice forwards.
			out = v.CallSlice(args)
		} else {
			out = v.Call(args)
		}
		// Fast path: no secrets resolved yet → nothing to propagate.
		if rc.secrets.hasSecrets() {
			if name, ok := taintName(rc.secrets, args); ok {
				if s := out[0].String(); len(s) >= minTrackedValueLen {
					rc.secrets.track(name, s)
				}
			}
		}
		return out
	}).Interface()
}

// taintName reports the secret name if any argument's string form contains a
// tracked secret value. Matching on substring (not data-flow provenance) is
// what makes chains and composites work: a formatted or concatenated argument
// that embeds a secret still matches.
func taintName(t *secretTable, args []reflect.Value) (string, bool) {
	for _, a := range args {
		if !a.IsValid() || !a.CanInterface() {
			continue
		}
		var s string
		switch x := a.Interface().(type) {
		case string:
			s = x
		default:
			s = fmt.Sprintf("%v", x)
		}
		if name, ok := t.matchName(s); ok {
			return name, true
		}
	}
	return "", false
}

// taintableBuiltins returns thin stand-ins for the string-producing
// text/template built-ins. Built-ins are not registered via FuncMap, so they
// would bypass taint wrapping; a FuncMap entry of the same name overrides the
// built-in. Each delegates to the exact stdlib function the built-in uses, so
// behaviour is identical — only the taint wrapper is added. Comparison
// built-ins (eq, len, index, ...) return non-strings and need no shadow.
func taintableBuiltins() template.FuncMap {
	return template.FuncMap{
		"printf":   fmt.Sprintf,
		"print":    fmt.Sprint,
		"println":  fmt.Sprintln,
		"html":     template.HTMLEscaper,
		"js":       template.JSEscaper,
		"urlquery": template.URLQueryEscaper,
	}
}
