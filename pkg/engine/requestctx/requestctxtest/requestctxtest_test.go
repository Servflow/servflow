package requestctxtest_test

import (
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func TestContextConforms(t *testing.T) {
	requestctxtest.Conformance(t, requestctxtest.NewContext)
}
