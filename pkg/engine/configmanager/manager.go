package configmanager

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/yamlloader"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// HandlerCreator is an interface for creating HTTP handlers from API configs
type HandlerCreator interface {
	CreateHandler(*apiconfig.APIConfig) (http.Handler, error)
}

// ConfigManager manages API configurations and their handlers with thread-safe updates
type ConfigManager struct {
	sync.RWMutex
	handlers     map[string]http.Handler // configID → handler
	fileToConfig map[string]string       // filepath → configID
	configs      map[string]*apiconfig.APIConfig
	yamlLoader   *yamlloader.YAMLLoader
	creator      HandlerCreator
}

// New creates a new ConfigManager
func New(yamlLoader *yamlloader.YAMLLoader, creator HandlerCreator) *ConfigManager {
	return &ConfigManager{
		handlers:     make(map[string]http.Handler),
		fileToConfig: make(map[string]string),
		configs:      make(map[string]*apiconfig.APIConfig),
		yamlLoader:   yamlLoader,
		creator:      creator,
	}
}

// LoadAllConfigs loads all configs from a folder
func (cm *ConfigManager) LoadAllConfigs(configFolder string) ([]*apiconfig.APIConfig, error) {
	logger := logging.GetLogger()
	logger.Info("Loading all configs from folder", zap.String("folder", configFolder))

	configs, err := cm.yamlLoader.FetchAPIConfigs(false)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API configs: %w", err)
	}

	cm.Lock()
	defer cm.Unlock()

	for _, config := range configs {
		handler, err := cm.creator.CreateHandler(config)
		if err != nil {
			logger.Error("Failed to create handler for config",
				zap.String("configID", config.ID),
				zap.Error(err))
			continue
		}

		cm.handlers[config.ID] = handler
		cm.configs[config.ID] = config
		logger.Info("Loaded config", zap.String("configID", config.ID))
	}

	return configs, nil
}

// LoadConfig loads a single config file
func (cm *ConfigManager) LoadConfig(filePath string) error {
	logger := logging.GetLogger()
	logger.Info("Loading config from file", zap.String("file", filePath))

	config, err := cm.loadAndValidateConfig(filePath)
	if err != nil {
		logger.Error("Failed to load config", zap.String("file", filePath), zap.Error(err))
		return err
	}

	handler, err := cm.creator.CreateHandler(config)
	if err != nil {
		logger.Error("Failed to create handler", zap.String("configID", config.ID), zap.Error(err))
		return err
	}

	cm.Lock()
	defer cm.Unlock()

	cm.handlers[config.ID] = handler
	cm.configs[config.ID] = config
	cm.fileToConfig[filePath] = config.ID

	logger.Info("Config loaded successfully", zap.String("configID", config.ID), zap.String("file", filePath))
	return nil
}

// ReloadConfig reloads a specific config file
func (cm *ConfigManager) ReloadConfig(filePath string) error {
	logger := logging.GetLogger()
	logger.Info("Reloading config from file", zap.String("file", filePath))

	config, err := cm.loadAndValidateConfig(filePath)
	if err != nil {
		logger.Error("Failed to reload config, keeping previous version",
			zap.String("file", filePath),
			zap.Error(err))
		return err
	}

	handler, err := cm.creator.CreateHandler(config)
	if err != nil {
		logger.Error("Failed to create handler, keeping previous version",
			zap.String("configID", config.ID),
			zap.Error(err))
		return err
	}

	cm.Lock()
	defer cm.Unlock()

	cm.handlers[config.ID] = handler
	cm.configs[config.ID] = config
	cm.fileToConfig[filePath] = config.ID

	logger.Info("Config reloaded successfully", zap.String("configID", config.ID), zap.String("file", filePath))
	return nil
}

// RemoveConfig removes a config
func (cm *ConfigManager) RemoveConfig(filePath string) error {
	logger := logging.GetLogger()

	cm.Lock()
	defer cm.Unlock()

	configID, ok := cm.fileToConfig[filePath]
	if !ok {
		logger.Warn("Config file not tracked", zap.String("file", filePath))
		return nil
	}

	delete(cm.handlers, configID)
	delete(cm.configs, configID)
	delete(cm.fileToConfig, filePath)

	logger.Info("Config removed", zap.String("configID", configID), zap.String("file", filePath))
	return nil
}

// GetHandler retrieves a handler by config ID (thread-safe)
func (cm *ConfigManager) GetHandler(configID string) http.Handler {
	cm.RLock()
	defer cm.RUnlock()
	return cm.handlers[configID]
}

// GetConfig retrieves a config by ID (thread-safe)
func (cm *ConfigManager) GetConfig(configID string) *apiconfig.APIConfig {
	cm.RLock()
	defer cm.RUnlock()
	return cm.configs[configID]
}

// ReloadAllConfigs reloads all currently loaded configs
func (cm *ConfigManager) ReloadAllConfigs() error {
	logger := logging.GetLogger()
	logger.Info("Reloading all configs")

	cm.RLock()
	files := make([]string, 0, len(cm.fileToConfig))
	for file := range cm.fileToConfig {
		files = append(files, file)
	}
	cm.RUnlock()

	var errors []error
	for _, file := range files {
		if err := cm.ReloadConfig(file); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		logger.Error("Some configs failed to reload", zap.Int("count", len(errors)))
		return fmt.Errorf("%d config(s) failed to reload", len(errors))
	}

	logger.Info("All configs reloaded successfully")
	return nil
}

// RegisterFile registers a file path to config ID mapping
func (cm *ConfigManager) RegisterFile(filePath string, configID string) {
	cm.Lock()
	defer cm.Unlock()
	cm.fileToConfig[filePath] = configID
}

// loadAndValidateConfig loads and validates a single config file
func (cm *ConfigManager) loadAndValidateConfig(filePath string) (*apiconfig.APIConfig, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config apiconfig.APIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}
