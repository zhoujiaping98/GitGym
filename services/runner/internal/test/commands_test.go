package test

import (
	"os/exec"
	"strings"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func createGitWorkspace(t *testing.T) engine.Workspace {
	t.Helper()

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

	return workspace
}

func TestRunGitStatusAndCaptureSnapshot(t *testing.T) {
	workspace := createGitWorkspace(t)

	recorder := engine.NewEventRecorder()
	result, err := engine.RunCommandWithEvents(workspace.Path, "git status --short", workspace.ID, recorder)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "" {
		t.Fatalf("expected empty status output, got %q", result.Stdout)
	}
	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "command.started" {
		t.Fatalf("expected first event type command.started, got %q", events[0].Type)
	}
	if events[0].WorkspaceID != workspace.ID {
		t.Fatalf("expected workspace ID %q, got %q", workspace.ID, events[0].WorkspaceID)
	}
	if got := events[0].Payload["raw"]; got != "git status --short" {
		t.Fatalf("expected raw command payload, got %#v", got)
	}
	if events[1].Type != "command.finished" {
		t.Fatalf("expected second event type command.finished, got %q", events[1].Type)
	}
	if got := events[1].Payload["exit_code"]; got != 0 {
		t.Fatalf("expected exit code payload 0, got %#v", got)
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

func TestRunCommandRejectsEmptyInput(t *testing.T) {
	workspace := createGitWorkspace(t)

	if _, err := engine.RunCommand(workspace.Path, "   "); err == nil {
		t.Fatal("expected error for empty command input")
	}
}

func TestRunCommandSupportsQuotedArguments(t *testing.T) {
	workspace := createGitWorkspace(t)

	if _, err := engine.RunCommand(workspace.Path, `git config user.name "Quoted Name"`); err != nil {
		t.Fatalf("set git user.name: %v", err)
	}

	result, err := engine.RunCommand(workspace.Path, "git config user.name")
	if err != nil {
		t.Fatalf("get git user.name: %v", err)
	}
	if result.Stdout != "Quoted Name\n" {
		t.Fatalf("expected quoted value to round-trip, got %q", result.Stdout)
	}
}
