package requestctx

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRC(t *testing.T) (*RequestContext, context.Context) {
	t.Helper()
	rc := NewRequestContext("test")
	return rc, WithAggregationContext(context.Background(), rc)
}

func TestSecretResolvesToRealValueAndTracks(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MY_TOKEN", "supersecretvalue")

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `Bearer {{ secret "MY_TOKEN" }}`)
	require.NoError(t, err)

	// Real value in the resolved string: transforms and comparisons work.
	assert.Equal(t, "Bearer supersecretvalue", out)
	// ...and the value is tracked for scrubbing from the moment it resolves.
	assert.True(t, rc.HasSecrets())
}

func TestSecretTransformsApplyToRealValue(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("QUOTED", `va"lue`)

	rc, ctx := newTestRC(t)

	// escape must apply to the real value — the exact case that broke the
	// placeholder model.
	out, err := rc.Resolve(ctx, `{{ escape (secret "QUOTED") }}`)
	require.NoError(t, err)
	assert.Equal(t, `va\"lue`, out)

	// eq must compare real values.
	out, err = rc.Resolve(ctx, `{{ if eq (secret "QUOTED") "va\"lue" }}match{{ end }}`)
	require.NoError(t, err)
	assert.Equal(t, "match", out)
}

func TestSecretMissingIsEmpty(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `[{{ secret "DOES_NOT_EXIST" }}]`)
	require.NoError(t, err)
	assert.Equal(t, "[]", out)
	assert.False(t, rc.HasSecrets())
}

func TestScrubMasksTrackedValues(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MY_TOKEN", "supersecretvalue")

	rc, ctx := newTestRC(t)
	_, err := rc.Resolve(ctx, `{{ secret "MY_TOKEN" }}`)
	require.NoError(t, err)

	scrubbed := rc.Scrub("api said: supersecretvalue is invalid")
	assert.NotContains(t, scrubbed, "supersecretvalue")
	assert.Contains(t, scrubbed, scrubMarkerPrefix+"MY_TOKEN"+scrubMarkerSuffix)
}

func TestScrubNoopWithoutSecrets(t *testing.T) {
	rc, _ := newTestRC(t)
	in := "nothing to hide here"
	assert.Equal(t, in, rc.Scrub(in))
	assert.False(t, rc.HasSecrets())
}

func TestScrubValueWalksContainers(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MY_TOKEN", "supersecretvalue")

	rc, ctx := newTestRC(t)
	_, err := rc.Resolve(ctx, `{{ secret "MY_TOKEN" }}`)
	require.NoError(t, err)

	out := rc.ScrubValue(map[string]interface{}{
		"echo":   "token=supersecretvalue",
		"nested": []interface{}{"supersecretvalue", 42, map[string]interface{}{"deep": "supersecretvalue"}},
		"number": 7,
	}).(map[string]interface{})

	b := fmt.Sprintf("%v", out)
	assert.NotContains(t, b, "supersecretvalue")
	assert.Equal(t, 7, out["number"])
}

func TestShortSecretNotTracked(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("TINY", "ab")

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `{{ secret "TINY" }}`)
	require.NoError(t, err)

	assert.Equal(t, "ab", out)
	// Too short to track — replacing "ab" everywhere would corrupt unrelated
	// output (the Drone lesson).
	assert.False(t, rc.HasSecrets())
}

func TestShareSecretsWithChild(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MY_TOKEN", "supersecretvalue")
	t.Setenv("CHILD_TOKEN", "childsecretvalue")

	parent, pctx := newTestRC(t)
	_, err := parent.Resolve(pctx, `{{ secret "MY_TOKEN" }}`)
	require.NoError(t, err)

	child := NewRequestContext("child")
	parent.ShareSecretsWith(child)

	// The child's scrubbers mask values the parent resolved...
	assert.NotContains(t, child.Scrub("x supersecretvalue y"), "supersecretvalue")

	// ...and values the child resolves are masked by the parent's scrubbers.
	cctx := WithAggregationContext(context.Background(), child)
	_, err = child.Resolve(cctx, `{{ secret "CHILD_TOKEN" }}`)
	require.NoError(t, err)
	assert.NotContains(t, parent.Scrub("x childsecretvalue y"), "childsecretvalue")
}

func TestNilRequestContextSafe(t *testing.T) {
	var rc *RequestContext
	assert.Equal(t, "in", rc.Scrub("in"))
	assert.False(t, rc.HasSecrets())
	assert.Equal(t, "v", rc.ScrubValue("v"))
}

func TestSecretTableConcurrency(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MY_TOKEN", "supersecretvalue")
	t.Setenv("OTHER", "othersecretvalue")

	rc, ctx := newTestRC(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, _ = rc.Resolve(ctx, `{{ secret "MY_TOKEN" }} {{ secret "OTHER" }}`)
		}()
		go func() {
			defer wg.Done()
			rc.Scrub("supersecretvalue othersecretvalue plain")
		}()
		go func() {
			defer wg.Done()
			rc.HasSecrets()
		}()
	}
	wg.Wait()

	assert.True(t, rc.HasSecrets())
	assert.NotContains(t, rc.Scrub("supersecretvalue"), "supersecretvalue")
}
