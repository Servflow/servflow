package download

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memWorkspace is an in-memory requestctx.Workspace for exercising the download
// action without touching the host filesystem. Paths are workspace-relative.
type memWorkspace struct {
	files map[string][]byte
}

func newMemWorkspace() *memWorkspace {
	return &memWorkspace{files: make(map[string][]byte)}
}

func (m *memWorkspace) Read(ctx context.Context, p string) ([]byte, error) {
	d, ok := m.files[p]
	if !ok {
		return nil, fmt.Errorf("%w: %s", requestctx.ErrWorkspaceNotExist, p)
	}
	return d, nil
}

func (m *memWorkspace) Write(ctx context.Context, p string, data []byte) error {
	m.files[p] = data
	return nil
}

func (m *memWorkspace) Delete(ctx context.Context, p string) error {
	delete(m.files, p)
	return nil
}

func (m *memWorkspace) List(ctx context.Context, prefix string) ([]requestctx.WorkspaceEntry, error) {
	return nil, nil
}

func (m *memWorkspace) Stat(ctx context.Context, p string) (requestctx.WorkspaceEntry, error) {
	if _, ok := m.files[p]; !ok {
		return requestctx.WorkspaceEntry{}, fmt.Errorf("%w: %s", requestctx.ErrWorkspaceNotExist, p)
	}
	return requestctx.WorkspaceEntry{Path: p}, nil
}

func TestDownload_Execute(t *testing.T) {
	tests := []struct {
		name           string
		destinationDir string
		setupContext   func(t *testing.T, ctx context.Context)
		setupFS        func(t *testing.T, ws *memWorkspace, destPath string)
		noWorkspace    bool
		config         Config
		expectedResult string
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful download with specified filename",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("test file content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "original.txt"))
			},
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "custom.txt",
				Overwrite: false,
			},
			expectedResult: "custom.txt",
		},
		{
			name: "successful download using original filename",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("test file content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "original.txt"))
			},
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "",
				Overwrite: false,
			},
			expectedResult: "original.txt",
		},
		{
			name: "overwrite enabled replaces existing file",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("new content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "existing.txt"))
			},
			setupFS: func(t *testing.T, ws *memWorkspace, destPath string) {
				ws.files[path.Join(destPath, "existing.txt")] = []byte("old content")
			},
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "existing.txt",
				Overwrite: true,
			},
			expectedResult: "existing.txt",
		},
		{
			name: "overwrite disabled fails when file exists",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("new content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "existing.txt"))
			},
			setupFS: func(t *testing.T, ws *memWorkspace, destPath string) {
				ws.files[path.Join(destPath, "existing.txt")] = []byte("old content")
			},
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "existing.txt",
				Overwrite: false,
			},
			expectError:   true,
			errorContains: "file already exists",
		},
		{
			name: "missing file input",
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "nonexistent"},
				FileName:  "output.txt",
				Overwrite: false,
			},
			expectError:   true,
			errorContains: "file not found",
		},
		{
			name:           "writes into nested destination directory",
			destinationDir: "nested/directory",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("test file content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "test.txt"))
			},
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "test.txt",
				Overwrite: false,
			},
			expectedResult: "test.txt",
		},
		{
			name: "no workspace configured fails",
			setupContext: func(t *testing.T, ctx context.Context) {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				file := io.NopCloser(strings.NewReader("test file content"))
				reqCtx.AddRequestFile("testfile", requestctx.NewFileValue(file, "test.txt"))
			},
			noWorkspace: true,
			config: Config{
				File:      apiconfig.FileInput{Type: apiconfig.FileInputTypeRequest, Identifier: "testfile"},
				FileName:  "test.txt",
				Overwrite: false,
			},
			expectError:   true,
			errorContains: requestctx.ErrNoWorkspace.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestctx.NewTestContext()
			ws := newMemWorkspace()

			if !tc.noWorkspace {
				reqCtx, _ := requestctx.FromContextOrError(ctx)
				reqCtx.SetWorkspace(ws)
			}

			if tc.setupContext != nil {
				tc.setupContext(t, ctx)
			}
			if tc.setupFS != nil {
				tc.setupFS(t, ws, tc.destinationDir)
			}

			cfg := tc.config
			cfg.DestinationPath = tc.destinationDir

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

			expectedPath := path.Join(tc.destinationDir, tc.expectedResult)
			assert.Equal(t, expectedPath, result)

			data, err := ws.Read(ctx, expectedPath)
			require.NoError(t, err, "file should exist in workspace")

			if tc.name == "overwrite enabled replaces existing file" {
				assert.Equal(t, "new content", string(data))
			}
		})
	}
}
