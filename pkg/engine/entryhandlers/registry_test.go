package entryhandlers

import (
	"net/http"
	"testing"
)

func noopMiddleware(_ map[string]interface{}, next http.Handler) http.Handler { return next }

func TestRegisterAndGet(t *testing.T) {
	// Use a distinct type name to avoid collisions with any real handlers
	// registered via init in the same test binary.
	const typ = "test_handler_register_get"

	if _, ok := Get(typ); ok {
		t.Fatalf("handler %q unexpectedly registered before Register", typ)
	}

	Register(typ, noopMiddleware)

	if _, ok := Get(typ); !ok {
		t.Fatalf("handler %q not found after Register", typ)
	}
	if !Has(typ) {
		t.Fatalf("Has(%q) = false, want true", typ)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	const typ = "test_handler_duplicate"
	Register(typ, noopMiddleware)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()
	Register(typ, noopMiddleware)
}

func TestRegisteredTypesSorted(t *testing.T) {
	Register("test_zeta", noopMiddleware)
	Register("test_alpha", noopMiddleware)

	types := RegisteredTypes()
	// Verify sorted order relative to the two we added.
	var alphaIdx, zetaIdx = -1, -1
	for i, tp := range types {
		switch tp {
		case "test_alpha":
			alphaIdx = i
		case "test_zeta":
			zetaIdx = i
		}
	}
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("registered types missing: %v", types)
	}
	if alphaIdx > zetaIdx {
		t.Fatalf("RegisteredTypes not sorted: %v", types)
	}
}
