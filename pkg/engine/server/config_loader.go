package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// LoadAPIConfigsFromYAML loads API configurations from YAML files in the specified folder
// if shouldFail is true the first yaml config failed will end the run
func LoadAPIConfigsFromYAML(apisFolder string, shouldFail bool, logger *zap.Logger) ([]*apiconfig.APIConfig, error) {
	logger.Debug("Loading API configs from YAML files", zap.String("folder", apisFolder))

	if apisFolder == "" {
		return nil, fmt.Errorf("APIs folder not specified")
	}

	contents, err := readYAMLFilesInFolder(apisFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML files from API folder: %w", err)
	}

	configs := make([]*apiconfig.APIConfig, 0)
	for path, content := range contents {
		name := filepath.Base(path)
		logger.Debug("Parsing API config file", zap.String("file", name))

		var cfg apiconfig.APIConfig
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			if shouldFail {
				return nil, fmt.Errorf("failed to unmarshal YAML file %s: %w", name, err)
			}
			logger.Warn("failed to unmarshal config file", zap.Error(err), zap.String("file", name))
			continue
		}

		configs = append(configs, &cfg)
	}

	if len(configs) == 0 {
		logger.Info("No API configurations found in folder", zap.String("folder", apisFolder))
	} else {
		logger.Debug("Successfully loaded API configs", zap.Int("count", len(configs)))
	}
	return configs, nil
}

// LoadEngineConfigFromYAML loads engine configuration from YAML file
func LoadEngineConfigFromYAML(engineConfigFile string, logger *zap.Logger) (*EngineConfig, error) {
	logger.Debug("Loading engine config from YAML file", zap.String("file", engineConfigFile))

	if engineConfigFile == "" {
		logger.Debug("No engine config file specified, returning empty config")
		return &EngineConfig{}, nil
	}

	contents, err := os.ReadFile(engineConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read engine config file: %w", err)
	}

	var engineConfig EngineConfig
	if err := yaml.Unmarshal(contents, &engineConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal engine config: %w", err)
	}

	integrationCount := len(engineConfig.Integrations)
	logger.Debug("Successfully loaded engine config", zap.Int("integrations_count", integrationCount))
	return &engineConfig, nil
}

// GetIntegrationConfigs extracts integration configs from EngineConfig as a slice
func (ec *EngineConfig) GetIntegrationConfigs() []apiconfig.IntegrationConfig {
	if ec == nil || ec.Integrations == nil {
		return nil
	}

	configs := make([]apiconfig.IntegrationConfig, 0, len(ec.Integrations))
	for id, conf := range ec.Integrations {
		conf.ID = id
		configs = append(configs, conf)
	}
	return configs
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
