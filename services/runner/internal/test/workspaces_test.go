package test

import (
	"os"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestCreateWorkspaceFromStandardTemplate(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := engine.InitStandardTemplate(workspace.Path); err != nil {
		t.Fatalf("init standard template: %v", err)
	}

	if _, err := os.Stat(workspace.Path + "/README.md"); err != nil {
		t.Fatalf("expected README.md to exist: %v", err)
	}
}
