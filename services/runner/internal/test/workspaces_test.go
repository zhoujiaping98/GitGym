package test

import (
	"os"
	"path/filepath"
	"testing"

	"gitgym/services/runner/internal/config"
	"gitgym/services/runner/internal/engine"
)

func TestCreateWorkspaceHydratesStandardTemplate(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if workspace.ID == "" {
		t.Fatal("expected workspace ID to be populated")
	}

	if workspace.ID != filepath.Base(workspace.Path) {
		t.Fatalf("expected workspace ID %q to match directory name %q", workspace.ID, filepath.Base(workspace.Path))
	}

	if workspace.Template != "standard" {
		t.Fatalf("expected template %q, got %q", "standard", workspace.Template)
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

func TestLoadConfigNormalizesWorkRootToAbsolutePath(t *testing.T) {
	original := os.Getenv("RUNNER_WORK_ROOT")
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv("RUNNER_WORK_ROOT")
			return
		}
		_ = os.Setenv("RUNNER_WORK_ROOT", original)
	})

	if err := os.Setenv("RUNNER_WORK_ROOT", "./var/custom-workspaces"); err != nil {
		t.Fatalf("set RUNNER_WORK_ROOT: %v", err)
	}

	cfg := config.Load()

	if !filepath.IsAbs(cfg.WorkRoot) {
		t.Fatalf("expected absolute work root, got %q", cfg.WorkRoot)
	}
}
