package engine

import (
	"os"
	"path/filepath"
)

type Workspace struct {
	ID       string
	Path     string
	Template string
}

func CreateWorkspace(root string) (Workspace, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Workspace{}, err
	}

	path, err := os.MkdirTemp(root, "ws-")
	if err != nil {
		return Workspace{}, err
	}

	id := filepath.Base(path)

	if err := InitStandardTemplate(path); err != nil {
		_ = os.RemoveAll(path)
		return Workspace{}, err
	}
	if err := InitWorkspaceRepository(path); err != nil {
		_ = os.RemoveAll(path)
		return Workspace{}, err
	}

	return Workspace{ID: id, Path: path, Template: "standard"}, nil
}
