package download

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

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

func (d *Download) SupportsReplica() bool {
	return true
}

func (d *Download) Config() string {
	configBytes, err := json.Marshal(d.cfg)
	if err != nil {
		return ""
	}
	return string(configBytes)
}

func New(config Config) (*Download, error) {
	return &Download{
		cfg: &config,
	}, nil
}

func (d *Download) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", d.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, err
	}

	// The workspace is the only filesystem an action may touch; the destination
	// path is interpreted relative to its root, never the host filesystem.
	ws, err := requestctx.WorkspaceFromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", plan.ErrFailure, err)
	}

	fileValue, err := requestctx.GetFileFromContext(ctx, cfg.File)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: file not found: %v", plan.ErrFailure, err)
	}
	defer fileValue.Close()

	fileName := cfg.FileName
	if fileName == "" {
		fileName = fileValue.Name
	}

	if fileName == "" {
		return nil, nil, fmt.Errorf("%w: no filename specified and original filename is empty", plan.ErrFailure)
	}

	// path.Join cleans the workspace-relative path; confinement (".." rejection)
	// is enforced by the workspace implementation, not here.
	destPath := path.Join(cfg.DestinationPath, fileName)

	if !cfg.Overwrite {
		if _, err := ws.Stat(ctx, destPath); err == nil {
			return nil, nil, fmt.Errorf("%w: file already exists: %s", plan.ErrFailure, destPath)
		}
	}

	data, err := fileValue.GetContent()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file content: %w", err)
	}

	if err := ws.Write(ctx, destPath, data); err != nil {
		return nil, nil, fmt.Errorf("failed to write file: %w", err)
	}

	logger.Debug("file downloaded successfully", zap.String("path", destPath))

	return destPath, nil, nil
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
