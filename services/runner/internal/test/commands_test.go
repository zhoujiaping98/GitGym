package test

import (
	"os/exec"
	"strings"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestRunGitStatusAndCaptureSnapshot(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	setupCommands := [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "GitGym Test"},
		{"config", "user.email", "test@gitgym.dev"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	}

	for _, args := range setupCommands {
		cmd := exec.Command("git", args...)
		cmd.Dir = workspace.Path
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}

	result, err := engine.RunCommand(workspace.Path, "git status --short")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "" {
		t.Fatalf("expected empty status output, got %q", result.Stdout)
	}

	snapshot, err := engine.CaptureSnapshot(workspace.Path)
	if err != nil {
		t.Fatalf("capture snapshot: %v", err)
	}
	if snapshot.HeadCommit == "" {
		t.Fatal("expected head commit to be populated")
	}
	if snapshot.BranchName != "main" {
		t.Fatalf("expected branch main, got %q", snapshot.BranchName)
	}
	if len(snapshot.StatusSummary) != 0 {
		t.Fatalf("expected empty status summary, got %v", snapshot.StatusSummary)
	}
}
