package yamlloader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Servflow/servflow/pkg/definitions"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// YAMLLoader handles loading configurations from YAML files
type YAMLLoader struct {
	apisFolder       string
	integrationsFile string
	logger           *zap.Logger
}

// NewYAMLLoader creates a new YAML loader instance
func NewYAMLLoader(apisFolder, integrationsFile string, logger *zap.Logger) *YAMLLoader {
	return &YAMLLoader{
		apisFolder:       apisFolder,
		integrationsFile: integrationsFile,
		logger:           logger,
	}
}

// FetchAPIConfigs loads API configurations from YAML files in the APIs folder
func (l *YAMLLoader) FetchAPIConfigs() ([]*apiconfig.APIConfig, error) {
	l.logger.Debug("Loading API configs from YAML files", zap.String("folder", l.apisFolder))

	if l.apisFolder == "" {
		return nil, fmt.Errorf("APIs folder not specified")
	}

	contents, err := readYAMLFilesInFolder(l.apisFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML files from API folder: %w", err)
	}

	configs := make([]*apiconfig.APIConfig, 0)
	for path, content := range contents {
		name := filepath.Base(path)
		l.logger.Debug("Parsing API config file", zap.String("file", name))

		var cfg apiconfig.APIConfig
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			l.logger.Warn("failed to unmarshal config file", zap.Error(err), zap.String("file", name))
			continue
		}

		configs = append(configs, &cfg)
	}

	l.logger.Debug("Successfully loaded API configs", zap.Int("count", len(configs)))
	return configs, nil
}

// FetchIntegrationsConfig loads integrations configuration from YAML file
func (l *YAMLLoader) FetchIntegrationsConfig() ([]apiconfig.DatasourceConfig, error) {
	l.logger.Debug("Loading integrations config from YAML file", zap.String("file", l.integrationsFile))

	if l.integrationsFile == "" {
		l.logger.Debug("No integrations file specified, returning empty config")
		return nil, nil
	}

	contents, err := os.ReadFile(l.integrationsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read integrations config file: %w", err)
	}

	type confStruct struct {
		Integrations map[string]apiconfig.DatasourceConfig `yaml:"integrations"`
	}

	var confs confStruct
	if err := yaml.Unmarshal(contents, &confs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal integrations config: %w", err)
	}

	configs := make([]apiconfig.DatasourceConfig, 0, len(confs.Integrations))
	for id, conf := range confs.Integrations {
		conf.ID = id
		configs = append(configs, conf)
	}

	l.logger.Debug("Successfully loaded integrations config", zap.Int("count", len(configs)))
	return configs, nil
}

// GetSecret retrieves a secret from environment variables
func (l *YAMLLoader) GetSecret(key string) ([]byte, error) {
	l.logger.Debug("Getting secret from environment", zap.String("key", key))

	value := os.Getenv(key)
	if value == "" {
		return nil, fmt.Errorf("secret %s not found in environment variables", key)
	}

	return []byte(value), nil
}

// readYAMLFilesInFolder reads all YAML files from a directory recursively
func readYAMLFilesInFolder(folderPath string) (map[string][]byte, error) {
	fileContents := make(map[string][]byte)

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if file has YAML extension
		ext := filepath.Ext(path)
		if !info.IsDir() && (ext == ".yml" || ext == ".yaml") {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			content, err := io.ReadAll(f)
			if err != nil {
				return err
			}

			fileContents[path] = content
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return fileContents, nil
}
