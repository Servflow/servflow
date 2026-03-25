package download

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownload_Execute(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(t *testing.T, ctx context.Context) context.Context
		setupFS        func(t *testing.T, destPath string)
		config         Config
		expectedResult string
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful download with specified filename",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				fileContent := "test file content"
				file := io.NopCloser(strings.NewReader(fileContent))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "original.txt"))
				return ctx
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "testfile",
				},
				FileName:  "custom.txt",
				Overwrite: false,
			},
			expectedResult: "custom.txt",
			expectError:    false,
		},
		{
			name: "successful download using original filename",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				fileContent := "test file content"
				file := io.NopCloser(strings.NewReader(fileContent))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "original.txt"))
				return ctx
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "testfile",
				},
				FileName:  "",
				Overwrite: false,
			},
			expectedResult: "original.txt",
			expectError:    false,
		},
		{
			name: "overwrite enabled replaces existing file",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				fileContent := "new content"
				file := io.NopCloser(strings.NewReader(fileContent))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "existing.txt"))
				return ctx
			},
			setupFS: func(t *testing.T, destPath string) {
				err := os.WriteFile(filepath.Join(destPath, "existing.txt"), []byte("old content"), 0644)
				require.NoError(t, err)
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "testfile",
				},
				FileName:  "existing.txt",
				Overwrite: true,
			},
			expectedResult: "existing.txt",
			expectError:    false,
		},
		{
			name: "overwrite disabled fails when file exists",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				fileContent := "new content"
				file := io.NopCloser(strings.NewReader(fileContent))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "existing.txt"))
				return ctx
			},
			setupFS: func(t *testing.T, destPath string) {
				err := os.WriteFile(filepath.Join(destPath, "existing.txt"), []byte("old content"), 0644)
				require.NoError(t, err)
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "testfile",
				},
				FileName:  "existing.txt",
				Overwrite: false,
			},
			expectError:   true,
			errorContains: "file already exists",
		},
		{
			name: "missing file input",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				return ctx
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "nonexistent",
				},
				FileName:  "output.txt",
				Overwrite: false,
			},
			expectError:   true,
			errorContains: "file not found",
		},
		{
			name: "creates destination directory",
			setupContext: func(t *testing.T, ctx context.Context) context.Context {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				fileContent := "test file content"
				file := io.NopCloser(strings.NewReader(fileContent))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "test.txt"))
				return ctx
			},
			config: Config{
				File: apiconfig.FileInput{
					Type:       apiconfig.FileInputTypeRequest,
					Identifier: "testfile",
				},
				FileName:  "test.txt",
				Overwrite: false,
			},
			expectedResult: "test.txt",
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			destPath := tc.config.DestinationPath
			if destPath == "" {
				if tc.name == "creates destination directory" {
					destPath = filepath.Join(tempDir, "nested", "directory")
				} else {
					destPath = tempDir
				}
			}

			ctx := requestctx.NewTestContext()

			if tc.setupContext != nil {
				ctx = tc.setupContext(t, ctx)
			}

			if tc.setupFS != nil {
				tc.setupFS(t, destPath)
			}

			cfg := tc.config
			cfg.DestinationPath = destPath

			download, err := New(cfg)
			require.NoError(t, err)

			result, _, err := download.Execute(ctx, download.Config())

			if tc.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, plan.ErrFailure)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			require.NoError(t, err)

			expectedPath := filepath.Join(destPath, tc.expectedResult)
			assert.Equal(t, expectedPath, result)

			_, err = os.Stat(expectedPath)
			assert.NoError(t, err, "file should exist at destination")

			if tc.name == "overwrite enabled replaces existing file" {
				content, err := os.ReadFile(expectedPath)
				require.NoError(t, err)
				assert.Equal(t, "new content", string(content))
			}
		})
	}
}
