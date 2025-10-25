package watcher

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// ConfigReloader is an interface for reloading configs
type ConfigReloader interface {
	ReloadConfig(filePath string) error
	LoadConfig(filePath string) error
	RemoveConfig(filePath string) error
}

// Watcher watches for file system changes and triggers config reloads
type Watcher struct {
	fsWatcher      *fsnotify.Watcher
	configReloader ConfigReloader
	configFolder   string
	debounceTimers map[string]*time.Timer
	timerMutex     sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	debouncePeriod time.Duration
}

func New(configReloader ConfigReloader, configFolder string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		fsWatcher:      fsWatcher,
		configReloader: configReloader,
		configFolder:   configFolder,
		debounceTimers: make(map[string]*time.Timer),
		ctx:            ctx,
		cancel:         cancel,
		debouncePeriod: 300 * time.Millisecond,
	}

	if err := fsWatcher.Add(configFolder); err != nil {
		cancel()
		return nil, err
	}

	logging.GetLogger().Info("Watching:", zap.String("folder", configFolder))
	return w, nil
}

func (w *Watcher) Start() {
	logger := logging.GetLogger()
	logger.Info("Starting file watcher")

	go func() {
		for {
			select {
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}
				w.handleEvent(event)

			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
				logger.Error("File watcher error", zap.Error(err))

			case <-w.ctx.Done():
				logger.Info("File watcher stopped")
				return
			}
		}
	}()
}

func (w *Watcher) Stop() error {
	logger := logging.GetLogger()
	logger.Info("Stopping file watcher")

	w.cancel()

	w.timerMutex.Lock()
	for _, timer := range w.debounceTimers {
		timer.Stop()
	}
	w.timerMutex.Unlock()

	return w.fsWatcher.Close()
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	logger := logging.GetLogger()

	// Only process YAML files
	if !isYAMLFile(event.Name) {
		return
	}

	logger.Debug("File event detected",
		zap.String("file", event.Name),
		zap.String("op", event.Op.String()))

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		w.debounceReload(event.Name)

	case event.Op&fsnotify.Create == fsnotify.Create:
		w.debounceLoad(event.Name)

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		w.handleRemove(event.Name)

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		w.handleRemove(event.Name)
	}
}

// debounceReload debounces reload events to avoid multiple reloads
func (w *Watcher) debounceReload(filePath string) {
	logger := logging.GetLogger()

	w.timerMutex.Lock()
	defer w.timerMutex.Unlock()

	// If timer exists, reset it
	if timer, exists := w.debounceTimers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceTimers[filePath] = time.AfterFunc(w.debouncePeriod, func() {
		logger.Info("Config file changed, reloading", zap.String("file", filePath))
		if err := w.configReloader.ReloadConfig(filePath); err != nil {
			logger.Error("Failed to reload config", zap.String("file", filePath), zap.Error(err))
		}

		// Remove timer after execution
		w.timerMutex.Lock()
		delete(w.debounceTimers, filePath)
		w.timerMutex.Unlock()
	})
}

// debounceLoad debounces load events for new files
func (w *Watcher) debounceLoad(filePath string) {
	logger := logging.GetLogger()

	w.timerMutex.Lock()
	defer w.timerMutex.Unlock()

	// If timer exists, reset it
	if timer, exists := w.debounceTimers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceTimers[filePath] = time.AfterFunc(w.debouncePeriod, func() {
		logger.Info("New config file created, loading", zap.String("file", filePath))
		if err := w.configReloader.LoadConfig(filePath); err != nil {
			logger.Error("Failed to load config", zap.String("file", filePath), zap.Error(err))
		}

		// Remove timer after execution
		w.timerMutex.Lock()
		delete(w.debounceTimers, filePath)
		w.timerMutex.Unlock()
	})
}

// handleRemove handles file removal
func (w *Watcher) handleRemove(filePath string) {
	logger := logging.GetLogger()
	logger.Info("Config file removed", zap.String("file", filePath))

	if err := w.configReloader.RemoveConfig(filePath); err != nil {
		logger.Error("Failed to remove config", zap.String("file", filePath), zap.Error(err))
	}
}

// isYAMLFile checks if a file is a YAML file
func isYAMLFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}
