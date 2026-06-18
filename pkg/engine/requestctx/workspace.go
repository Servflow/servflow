package requestctx

import (
	"context"
	"errors"
	"time"
)

// Workspace is the file capability an agent's actions are given. It is the only
// route through which workflow code may read or write files: a request carries
// exactly one Workspace, supplied by the host, and every path is interpreted
// relative to that workspace's root. Implementations live outside core (the host
// provides local- or object-storage-backed ones) and MUST confine all paths to
// the workspace — callers never see, and cannot address, the host filesystem.
//
// All paths use forward slashes and are relative to the workspace root. Methods
// take a context so network-backed implementations honour cancellation.
type Workspace interface {
	// Read returns the full contents of the file at path. It returns
	// ErrWorkspaceNotExist if the file is absent.
	Read(ctx context.Context, path string) ([]byte, error)
	// Write creates or overwrites the file at path, creating parent directories
	// as needed.
	Write(ctx context.Context, path string, data []byte) error
	// Stat returns metadata for the file or directory at path. It returns
	// ErrWorkspaceNotExist if nothing exists at path.
	Stat(ctx context.Context, path string) (WorkspaceEntry, error)
}

// WorkspaceEntry describes a single file or directory in a workspace. Paths are
// relative to the workspace root and use forward slashes.
type WorkspaceEntry struct {
	Path    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}

var (
	// ErrNoWorkspace is returned when an action requires a workspace but none was
	// configured for the request (the agent has no workspace assigned).
	ErrNoWorkspace = errors.New("no workspace configured for this request")
	// ErrWorkspaceNotExist is the workspace-level equivalent of fs.ErrNotExist.
	// Host implementations translate their native not-found conditions into it so
	// actions can branch with errors.Is regardless of backend.
	ErrWorkspaceNotExist = errors.New("workspace: file does not exist")
	// ErrWorkspaceInvalidPath is returned when a path escapes the workspace root
	// or is otherwise not addressable within it.
	ErrWorkspaceInvalidPath = errors.New("workspace: invalid path")
)

// WorkspaceFromContext returns the workspace configured for the request, or
// ErrNoWorkspace if the request has none. This is the helper actions call before
// touching files.
func WorkspaceFromContext(ctx context.Context) (Workspace, error) {
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	ws := reqCtx.GetWorkspace()
	if ws == nil {
		return nil, ErrNoWorkspace
	}
	return ws, nil
}

// ReadWorkspaceFile reads the file at path from the request's workspace. It is a
// convenience wrapper over WorkspaceFromContext for the common read case.
func ReadWorkspaceFile(ctx context.Context, path string) ([]byte, error) {
	ws, err := WorkspaceFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return ws.Read(ctx, path)
}

// WriteWorkspaceFile writes data to path in the request's workspace.
func WriteWorkspaceFile(ctx context.Context, path string, data []byte) error {
	ws, err := WorkspaceFromContext(ctx)
	if err != nil {
		return err
	}
	return ws.Write(ctx, path, data)
}
