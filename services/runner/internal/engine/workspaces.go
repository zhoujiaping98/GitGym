package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Workspace struct {
	ID   string
	Path string
}

func CreateWorkspace(root string) (Workspace, error) {
	id := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	path := filepath.Join(root, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Workspace{}, err
	}

	if err := InitStandardTemplate(path); err != nil {
		_ = os.RemoveAll(path)
		return Workspace{}, err
	}

	return Workspace{ID: id, Path: path}, nil
}
