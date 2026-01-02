package server

import (
	"os"
	"path/filepath"
	"testing"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadAPIConfigsFromYAML(t *testing.T) {
	logger := zap.NewNop()

	t.Run("empty folder string returns error", func(t *testing.T) {
		configs, err := LoadAPIConfigsFromYAML("", false, logger)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "APIs folder not specified")
	})

	t.Run("empty folder returns empty configs", func(t *testing.T) {
		tempDir := t.TempDir() // Empty directory
		configs, err := LoadAPIConfigsFromYAML(tempDir, false, logger)
		require.NoError(t, err)
		assert.Len(t, configs, 0)
	})

	t.Run("non-existent folder returns error", func(t *testing.T) {
		configs, err := LoadAPIConfigsFromYAML("/non/existent/folder", false, logger)
		assert.Error(t, err)
		assert.Nil(t, configs)
	})

	t.Run("folder with valid YAML files", func(t *testing.T) {
		// Create temporary directory with test YAML files
		tempDir := t.TempDir()

		// Create a valid API config YAML file
		apiConfigYAML := `
id: test-api
http:
  listenPath: /test
  method: GET
actions:
  greet:
    type: stub
    config:
      message: "Hello World"
responses:
  success:
    statusCode: 200
    body: "{{ .actions.greet.message }}"
`
		err := os.WriteFile(filepath.Join(tempDir, "test-api.yaml"), []byte(apiConfigYAML), 0644)
		require.NoError(t, err)

		configs, err := LoadAPIConfigsFromYAML(tempDir, false, logger)
		require.NoError(t, err)
		require.Len(t, configs, 1)

		config := configs[0]
		assert.Equal(t, "test-api", config.ID)
		assert.Equal(t, "/test", config.HttpConfig.ListenPath)
		assert.Equal(t, "GET", config.HttpConfig.Method)
		assert.Contains(t, config.Actions, "greet")
	})

	t.Run("folder with invalid YAML - shouldFail false", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create an invalid YAML file
		invalidYAML := `
id: invalid-api
invalid_yaml: [unclosed array
`
		err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidYAML), 0644)
		require.NoError(t, err)

		// Should not fail but return empty configs
		configs, err := LoadAPIConfigsFromYAML(tempDir, false, logger)
		require.NoError(t, err)
		assert.Len(t, configs, 0)
	})

	t.Run("folder with invalid YAML - shouldFail true", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create an invalid YAML file
		invalidYAML := `
id: invalid-api
invalid_yaml: [unclosed array
`
		err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidYAML), 0644)
		require.NoError(t, err)

		// Should fail and return error
		configs, err := LoadAPIConfigsFromYAML(tempDir, true, logger)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "failed to unmarshal YAML file")
	})
}

func TestLoadIntegrationsConfigFromYAML(t *testing.T) {
	logger := zap.NewNop()

	t.Run("empty file path returns empty config", func(t *testing.T) {
		configs, err := LoadIntegrationsConfigFromYAML("", logger)
		require.NoError(t, err)
		assert.Nil(t, configs)
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		configs, err := LoadIntegrationsConfigFromYAML("/non/existent/file.yaml", logger)
		assert.Error(t, err)
		assert.Nil(t, configs)
	})

	t.Run("valid integrations config file", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "integrations.yaml")

		integrationsYAML := `
integrations:
  db1:
    type: mongo
    config:
      connectionString: "mongodb://localhost:27017"
      database: "testdb"
  db2:
    type: sql
    config:
      driver: "postgres"
      connectionString: "postgres://user:pass@localhost/db"
`
		err := os.WriteFile(tempFile, []byte(integrationsYAML), 0644)
		require.NoError(t, err)

		configs, err := LoadIntegrationsConfigFromYAML(tempFile, logger)
		require.NoError(t, err)
		require.Len(t, configs, 2)

		// Find configs by ID
		var db1, db2 *apiconfig.IntegrationConfig
		for i := range configs {
			if configs[i].ID == "db1" {
				db1 = &configs[i]
			} else if configs[i].ID == "db2" {
				db2 = &configs[i]
			}
		}

		require.NotNil(t, db1)
		assert.Equal(t, "db1", db1.ID)
		assert.Equal(t, "mongo", db1.Type)

		require.NotNil(t, db2)
		assert.Equal(t, "db2", db2.ID)
		assert.Equal(t, "sql", db2.Type)
	})

	t.Run("invalid integrations config file", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "invalid.yaml")

		invalidYAML := `
integrations: [unclosed array
`
		err := os.WriteFile(tempFile, []byte(invalidYAML), 0644)
		require.NoError(t, err)

		configs, err := LoadIntegrationsConfigFromYAML(tempFile, logger)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "failed to unmarshal integrations config")
	})
}

func TestReadYAMLFilesInFolder(t *testing.T) {
	t.Run("reads YAML files recursively", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create nested directory structure
		subDir := filepath.Join(tempDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		// Create YAML files
		err = os.WriteFile(filepath.Join(tempDir, "file1.yaml"), []byte("content1"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tempDir, "file2.yml"), []byte("content2"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(subDir, "file3.yaml"), []byte("content3"), 0644)
		require.NoError(t, err)

		// Create non-YAML file (should be ignored)
		err = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("not yaml"), 0644)
		require.NoError(t, err)

		contents, err := readYAMLFilesInFolder(tempDir)
		require.NoError(t, err)
		assert.Len(t, contents, 3)

		// Check that all YAML files were read
		found := make(map[string]bool)
		for path, content := range contents {
			if filepath.Base(path) == "file1.yaml" {
				assert.Equal(t, "content1", string(content))
				found["file1"] = true
			} else if filepath.Base(path) == "file2.yml" {
				assert.Equal(t, "content2", string(content))
				found["file2"] = true
			} else if filepath.Base(path) == "file3.yaml" {
				assert.Equal(t, "content3", string(content))
				found["file3"] = true
			}
		}

		assert.True(t, found["file1"])
		assert.True(t, found["file2"])
		assert.True(t, found["file3"])
	})
}
