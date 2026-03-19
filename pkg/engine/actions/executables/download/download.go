package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type Config struct {
	File            apiconfig.FileInput `json:"file" yaml:"file"`
	DestinationPath string              `json:"destinationPath" yaml:"destinationPath"`
	FileName        string              `json:"fileName" yaml:"fileName"`
	Overwrite       bool                `json:"overwrite" yaml:"overwrite"`
}

type Download struct {
	cfg *Config
}

func (d *Download) Type() string {
	return "download"
}

func (d *Download) Config() string {
	configBytes, err := json.Marshal(d.cfg)
	if err != nil {
		return ""
	}
	return string(configBytes)
}

func New(config Config) (*Download, error) {
	if config.DestinationPath == "" {
		return nil, errors.New("destinationPath is required")
	}
	return &Download{
		cfg: &config,
	}, nil
}

func (d *Download) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", d.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, err
	}

	fileValue, err := requestctx.GetFileFromContext(ctx, cfg.File)
	if err != nil {
		return nil, fmt.Errorf("%w: file not found: %v", plan.ErrFailure, err)
	}
	defer fileValue.Close()

	fileName := cfg.FileName
	if fileName == "" {
		fileName = fileValue.Name
	}

	if fileName == "" {
		return nil, fmt.Errorf("%w: no filename specified and original filename is empty", plan.ErrFailure)
	}

	if err := os.MkdirAll(cfg.DestinationPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	fullPath := filepath.Join(cfg.DestinationPath, fileName)

	if !cfg.Overwrite {
		if _, err := os.Stat(fullPath); err == nil {
			return nil, fmt.Errorf("%w: file already exists: %s", plan.ErrFailure, fullPath)
		}
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, fileValue.GetReader()); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	logger.Debug("file downloaded successfully", zap.String("path", fullPath))

	return fullPath, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"file": {
			Type:        actions.FieldTypeFile,
			Label:       "File",
			Placeholder: "File to download",
			Required:    true,
		},
		"destinationPath": {
			Type:        actions.FieldTypeString,
			Label:       "Destination Path",
			Placeholder: "Directory path to save the file",
			Required:    true,
		},
		"fileName": {
			Type:        actions.FieldTypeString,
			Label:       "File Name",
			Placeholder: "Output filename (uses original if empty)",
			Required:    false,
		},
		"overwrite": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Overwrite",
			Placeholder: "Overwrite existing file",
			Required:    false,
			Default:     false,
		},
	}

	if err := actions.RegisterAction("download", actions.ActionRegistrationInfo{
		Name:        "Download File",
		Description: "Saves a file from the request or action output to a specified path",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating download action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
