package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if len(entries) != 3 {
		t.Fatalf("expected 3 workspace entries including .git, found %d", len(entries))
	}

	if _, err := os.Stat(filepath.Join(workspace.Path, ".git")); err != nil {
		t.Fatalf("expected initialized git repository: %v", err)
	}

	assertGitOutput(t, workspace.Path, []string{"branch", "--show-current"}, "main\n")
	if got := strings.TrimSpace(runGit(t, workspace.Path, "config", "user.name")); got != "GitGym Test" {
		t.Fatalf("expected git user.name GitGym Test, got %q", got)
	}
	if got := strings.TrimSpace(runGit(t, workspace.Path, "config", "user.email")); got != "test@gitgym.dev" {
		t.Fatalf("expected git user.email test@gitgym.dev, got %q", got)
	}
	if got := strings.TrimSpace(runGit(t, workspace.Path, "log", "--format=%s", "-1")); got != "Initial commit" {
		t.Fatalf("expected initial commit message, got %q", got)
	}
	if got := strings.TrimSpace(runGit(t, workspace.Path, "status", "--short")); got != "" {
		t.Fatalf("expected clean git status, got %q", got)
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

func assertGitOutput(t *testing.T, dir string, args []string, want string) {
	t.Helper()

	if got := runGit(t, dir, args...); got != want {
		t.Fatalf("expected git %s output %q, got %q", strings.Join(args, " "), want, got)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	home := t.TempDir()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"HOME="+home,
		"USERPROFILE="+home,
		"XDG_CONFIG_HOME="+home,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
