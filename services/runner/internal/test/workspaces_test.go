package test

import (
	"os"
	"path/filepath"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestCreateWorkspaceHydratesStandardTemplate(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	readmePath := filepath.Join(workspace.Path, "README.md")
	gitignorePath := filepath.Join(workspace.Path, ".gitignore")

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	if string(readme) != "# Standard Template\n" {
		t.Fatalf("unexpected README.md contents: %q", string(readme))
	}

	gitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	if string(gitignore) != ".git/\n" {
		t.Fatalf("unexpected .gitignore contents: %q", string(gitignore))
	}

	entries, err := os.ReadDir(workspace.Path)
	if err != nil {
		t.Fatalf("read workspace dir: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 template files, found %d", len(entries))
	}
}
