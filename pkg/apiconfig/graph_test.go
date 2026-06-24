package apiconfig

import (
	"errors"
	"testing"
)

// runGraph runs only the graph pass so these tests are isolated from schema and
// action-registration validation.
func runGraph(cfg APIConfig, roots ...string) *ValidationErrors {
	ve := &ValidationErrors{}
	cfg.collectGraphErrors(ve, roots)
	return ve
}

func countErrs[T error](errs []error) int {
	n := 0
	for _, e := range errs {
		var target T
		if errors.As(e, &target) {
			n++
		}
	}
	return n
}

func TestGraph_ValidLinear(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions:    map[string]Action{"a": {Next: "response.ok"}},
		Responses:  map[string]ResponseConfig{"ok": {Code: 200}},
	}
	ve := runGraph(cfg)
	if ve.HasErrors() {
		t.Fatalf("unexpected errors: %v", ve.Error())
	}
	if len(ve.Warnings()) != 0 {
		t.Fatalf("unexpected warnings: %v", ve.Warnings())
	}
}

func TestGraph_ValidBranch(t *testing.T) {
	cfg := APIConfig{
		HttpConfig:   HttpConfig{Next: "conditional.c"},
		Conditionals: map[string]Conditional{"c": {OnTrue: "response.ok", OnFalse: "response.bad"}},
		Responses:    map[string]ResponseConfig{"ok": {Code: 200}, "bad": {Code: 400}},
	}
	ve := runGraph(cfg)
	if ve.HasErrors() || len(ve.Warnings()) != 0 {
		t.Fatalf("expected clean, got errors=%v warnings=%v", ve.errors, ve.warnings)
	}
}

func TestGraph_SelfCycle(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions:    map[string]Action{"a": {Next: "action.a"}},
	}
	ve := runGraph(cfg)
	if countErrs[*CycleError](ve.errors) != 1 {
		t.Fatalf("expected 1 fatal cycle, got errors=%v", ve.errors)
	}
}

func TestGraph_MultiNodeCycle(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions: map[string]Action{
			"a": {Next: "action.b"},
			"b": {Next: "action.a"},
		},
	}
	ve := runGraph(cfg)
	if countErrs[*CycleError](ve.errors) != 1 {
		t.Fatalf("expected 1 fatal cycle, got errors=%v", ve.errors)
	}
}

func TestGraph_DanglingRef(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions:    map[string]Action{"a": {Next: "action.missing"}},
	}
	ve := runGraph(cfg)
	if countErrs[*InvalidReferenceError](ve.errors) != 1 {
		t.Fatalf("expected 1 invalid reference, got errors=%v", ve.errors)
	}
}

func TestGraph_BadPrefix(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions:    map[string]Action{"a": {Next: "foo"}},
	}
	ve := runGraph(cfg)
	if countErrs[*InvalidReferenceError](ve.errors) != 1 {
		t.Fatalf("expected 1 invalid reference, got errors=%v", ve.errors)
	}
}

func TestGraph_MissingEntry(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.missing"},
		Actions:    map[string]Action{"a": {Next: "response.ok"}},
		Responses:  map[string]ResponseConfig{"ok": {Code: 200}},
	}
	ve := runGraph(cfg)
	if countErrs[*InvalidReferenceError](ve.errors) != 1 {
		t.Fatalf("expected entry to fail resolution, got errors=%v", ve.errors)
	}
}

func TestGraph_OrphanWarning(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions: map[string]Action{
			"a":      {Next: "response.ok"},
			"orphan": {Next: "response.ok"},
		},
		Responses: map[string]ResponseConfig{"ok": {Code: 200}},
	}
	ve := runGraph(cfg)
	if ve.HasErrors() {
		t.Fatalf("orphan must not be fatal, got errors=%v", ve.errors)
	}
	if countErrs[*UnreachableStepError](ve.warnings) != 1 {
		t.Fatalf("expected 1 orphan warning, got warnings=%v", ve.warnings)
	}
}

func TestGraph_UnreachableCycleIsWarning(t *testing.T) {
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions: map[string]Action{
			"a": {Next: "response.ok"},
			// b<->c form a cycle that no entry reaches
			"b": {Next: "action.c"},
			"c": {Next: "action.b"},
		},
		Responses: map[string]ResponseConfig{"ok": {Code: 200}},
	}
	ve := runGraph(cfg)
	if ve.HasErrors() {
		t.Fatalf("unreachable cycle must not be fatal, got errors=%v", ve.errors)
	}
	if countErrs[*CycleError](ve.warnings) != 1 {
		t.Fatalf("expected 1 cycle warning, got warnings=%v", ve.warnings)
	}
}

func TestGraph_DispatchIsRootNotCycle(t *testing.T) {
	// a dispatches to b; b -> a via dispatch would NOT be a main-flow cycle.
	// Here a's normal flow ends, and dispatch reaches b (so b is not orphaned).
	cfg := APIConfig{
		HttpConfig: HttpConfig{Next: "action.a"},
		Actions: map[string]Action{
			"a": {Next: "response.ok", Dispatch: []string{"action.b"}},
			"b": {Next: "response.ok"},
		},
		Responses: map[string]ResponseConfig{"ok": {Code: 200}},
	}
	ve := runGraph(cfg)
	if ve.HasErrors() {
		t.Fatalf("unexpected errors: %v", ve.errors)
	}
	if len(ve.warnings) != 0 {
		t.Fatalf("dispatch target should be reachable (no orphan), got warnings=%v", ve.warnings)
	}
}

func TestGraph_ExtraRootResolves(t *testing.T) {
	cfg := APIConfig{
		Actions:   map[string]Action{"a": {Next: "response.ok"}},
		Responses: map[string]ResponseConfig{"ok": {Code: 200}},
	}
	// trigger.next supplied as an extra root
	ve := runGraph(cfg, "action.a")
	if ve.HasErrors() || len(ve.warnings) != 0 {
		t.Fatalf("extra root should make graph clean, got errors=%v warnings=%v", ve.errors, ve.warnings)
	}
	// bad extra root -> invalid reference
	ve2 := runGraph(cfg, "action.nope")
	if countErrs[*InvalidReferenceError](ve2.errors) != 1 {
		t.Fatalf("expected bad extra root to error, got %v", ve2.errors)
	}
}
