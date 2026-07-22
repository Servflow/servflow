package requestctx

import (
	"testing"

	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaintEscapeTracksTransformedSecret is the core case: `escape` produces a
// new string that no longer equals the raw secret, and taint tracking must
// track that output so the scrubber catches it.
func TestTaintEscapeTracksTransformedSecret(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("QUOTED", `va"lue-long-enough`)

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `{{ escape (secret "QUOTED") }}`)
	require.NoError(t, err)
	assert.Equal(t, `va\"lue-long-enough`, out) // real transform applied

	// The escaped form is tracked even though it differs from the raw value.
	assert.Equal(t, scrubMarkerPrefix+"QUOTED"+scrubMarkerSuffix, rc.Scrub(`va\"lue-long-enough`))
	// The raw form is still tracked too.
	assert.Equal(t, scrubMarkerPrefix+"QUOTED"+scrubMarkerSuffix, rc.Scrub(`va"lue-long-enough`))
}

// TestMultilinePEMScrubbedInEscapedForm reproduces the github_token leak: a V1
// JSON config embeds the PEM via {{ escape (secret ...) }}, so the resolved
// config (and the sf.config span attribute) holds the ESCAPED image of the PEM
// — \n as two chars — which taint tracking must catch.
func TestMultilinePEMScrubbedInEscapedForm(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA2mFyUnrightlyLongBase64LineGoesHere01\nQZzz9SecondBase64LineWithMoreKeyMaterialHere567890ab\n-----END RSA PRIVATE KEY-----"
	t.Setenv("GH_PEM", pem)

	rc, ctx := newTestRC(t)
	resolvedCfg, err := rc.Resolve(ctx, `{"pem":"{{ escape (secret "GH_PEM") }}","client_id":"abc"}`)
	require.NoError(t, err)
	require.Contains(t, resolvedCfg, `\n`, "escape should have encoded the newlines")

	scrubbed := rc.Scrub(resolvedCfg)
	assert.NotContains(t, scrubbed, "MIIEowIBAAKCAQEA2mFyUnrightlyLongBase64LineGoesHere01")
	assert.NotContains(t, scrubbed, "QZzz9SecondBase64LineWithMoreKeyMaterialHere567890ab")
	assert.Contains(t, scrubbed, `"client_id":"abc"`, "non-secret config must survive")

	// Raw form masked as a single marker (longest-first replacement).
	assert.Equal(t, scrubMarkerPrefix+"GH_PEM"+scrubMarkerSuffix, rc.Scrub(pem))
	// Reflowed fragments (individual lines) masked too (external-reflow backstop).
	assert.NotContains(t, rc.Scrub("saw MIIEowIBAAKCAQEA2mFyUnrightlyLongBase64LineGoesHere01 in output"), "MIIEow")
}

// TestTaintUrlqueryBuiltin proves a shadowed built-in (urlquery) routes through
// taint tracking — a live leak vector for secrets in query params.
func TestTaintUrlqueryBuiltin(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("TOK", "s p a c e/secret+value")

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `https://x?t={{ urlquery (secret "TOK") }}`)
	require.NoError(t, err)
	require.Contains(t, out, "%2F", "urlquery should url-encode the value")

	// The url-encoded form is tracked.
	assert.NotContains(t, rc.Scrub(out), "secret%2Bvalue")
	assert.Contains(t, rc.Scrub(out), scrubMarkerPrefix+"TOK"+scrubMarkerSuffix)
}

// TestTaintPrintfComposite proves a composite output (printf embedding a
// secret) is tracked, and variadic funcs are wrapped correctly.
func TestTaintPrintfComposite(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("APIKEY", "supersecretapikey")

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `{{ printf "Authorization: Bearer %s" (secret "APIKEY") }}`)
	require.NoError(t, err)
	assert.Equal(t, "Authorization: Bearer supersecretapikey", out)

	assert.NotContains(t, rc.Scrub(out), "supersecretapikey")
}

// TestTaintChain proves nested transforms compose: escape of escape.
func TestTaintChain(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("MULTI", `a"b"c-long-enough-value`)

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `{{ escape (escape (secret "MULTI")) }}`)
	require.NoError(t, err)

	// Whatever the double-escaped form is, it's tracked.
	assert.Equal(t, scrubMarkerPrefix+"MULTI"+scrubMarkerSuffix, rc.Scrub(out))
}

// TestTaintHashTracksDigest proves a one-way transform (hash) tracks its digest
// so digests of secrets don't leak either.
func TestTaintHashTracksDigest(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("PW", "supersecretpassword")

	rc, ctx := newTestRC(t)
	out, err := rc.Resolve(ctx, `{{ hash (secret "PW") }}`)
	require.NoError(t, err)
	require.NotEqual(t, "supersecretpassword", out)

	assert.Equal(t, scrubMarkerPrefix+"PW"+scrubMarkerSuffix, rc.Scrub(out))
}

// TestTaintDoesNotTrackUntransformedData proves ordinary (non-secret) data
// passing through a transform is NOT tracked — no over-masking.
func TestTaintDoesNotTrackUntransformedData(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("TOK", "supersecretvalue")

	rc, ctx := newTestRC(t)
	// Resolve a secret so the table is non-empty (HasSecrets true).
	_, err := rc.Resolve(ctx, `{{ secret "TOK" }}`)
	require.NoError(t, err)

	// escape ordinary data — its output must NOT be tracked.
	out, err := rc.Resolve(ctx, `{{ escape "just a normal config value" }}`)
	require.NoError(t, err)
	assert.Equal(t, out, rc.Scrub(out), "non-secret transform output must not be masked")
}

// TestTaintCustomRequestFunc proves functions added at request time (e.g. by an
// entry handler) are wrapped too — the user's specific concern.
func TestTaintCustomRequestFunc(t *testing.T) {
	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("TOK", "supersecretvalue")

	rc, ctx := newTestRC(t)
	// A custom transform registered after the context exists, as an entry
	// handler or trigger engine would do.
	rc.AddRequestTemplateFunctions(map[string]any{
		"wrapbrackets": func(s string) string { return "[[" + s + "]]" },
	}, true)

	out, err := rc.Resolve(ctx, `{{ wrapbrackets (secret "TOK") }}`)
	require.NoError(t, err)
	assert.Equal(t, "[[supersecretvalue]]", out)

	// The custom func's output (containing the secret) is tracked.
	assert.NotContains(t, rc.Scrub(out), "supersecretvalue")
}
