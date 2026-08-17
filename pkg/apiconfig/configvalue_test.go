package apiconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigValueRoundTrip(t *testing.T) {
	cases := map[string]struct {
		value    ConfigValue
		wantJSON string
		wantYAML string
	}{
		"literal string": {
			value:    ConfigValue{Value: "Iv1.abc"},
			wantJSON: `{"value":"Iv1.abc"}`,
			wantYAML: "value: Iv1.abc\n",
		},
		"secret ref": {
			value:    ConfigValue{Secret: "github_app_pem"},
			wantJSON: `{"secret":"github_app_pem"}`,
			wantYAML: "secret: github_app_pem\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotJSON, err := json.Marshal(tc.value)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantJSON, string(gotJSON))

			var backJSON ConfigValue
			require.NoError(t, json.Unmarshal(gotJSON, &backJSON))
			assert.Equal(t, tc.value, backJSON)

			gotYAML, err := yaml.Marshal(tc.value)
			require.NoError(t, err)
			assert.Equal(t, tc.wantYAML, string(gotYAML))

			var backYAML ConfigValue
			require.NoError(t, yaml.Unmarshal(gotYAML, &backYAML))
			assert.Equal(t, tc.value, backYAML)
		})
	}
}

// A config map decodes from both codecs into the wrapped shape.
func TestIntegrationConfigDecode(t *testing.T) {
	const raw = `{"id":"gh","type":"github_app","config":{"client_id":{"value":"Iv1.abc"},"pem":{"secret":"gh_pem"}}}`

	var cfg IntegrationConfig
	require.NoError(t, json.Unmarshal([]byte(raw), &cfg))

	assert.Equal(t, "Iv1.abc", cfg.Config["client_id"].Value)
	assert.Equal(t, "", cfg.Config["client_id"].Secret)
	assert.Equal(t, "gh_pem", cfg.Config["pem"].Secret)
	assert.Nil(t, cfg.Config["pem"].Value)
}
