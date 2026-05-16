package test

import (
	"os"
	"os/exec"
	"path/filepath"
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

	return workspace
}

func TestRunGitStatusCapturesSnapshotsInLifecycleEvents(t *testing.T) {
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
	if events[0].Type != "command_started" {
		t.Fatalf("expected first event type command_started, got %q", events[0].Type)
	}
	if events[0].WorkspaceID != workspace.ID {
		t.Fatalf("expected workspace ID %q, got %q", workspace.ID, events[0].WorkspaceID)
	}
	if got := events[0].Payload["raw"]; got != "git status --short" {
		t.Fatalf("expected raw command payload, got %#v", got)
	}
	preSnapshot, ok := events[0].Payload["pre_snapshot"].(engine.Snapshot)
	if !ok {
		t.Fatalf("expected pre_snapshot payload, got %#v", events[0].Payload["pre_snapshot"])
	}
	if preSnapshot.HeadCommit == "" {
		t.Fatal("expected pre-run head commit to be populated")
	}
	if preSnapshot.BranchName != "main" {
		t.Fatalf("expected pre-run branch main, got %q", preSnapshot.BranchName)
	}
	if len(preSnapshot.StatusSummary) != 0 {
		t.Fatalf("expected empty pre-run status summary, got %v", preSnapshot.StatusSummary)
	}
	if events[1].Type != "command_finished" {
		t.Fatalf("expected second event type command_finished, got %q", events[1].Type)
	}
	if got := events[1].Payload["exit_code"]; got != 0 {
		t.Fatalf("expected exit code payload 0, got %#v", got)
	}
	postSnapshot, ok := events[1].Payload["post_snapshot"].(engine.Snapshot)
	if !ok {
		t.Fatalf("expected post_snapshot payload, got %#v", events[1].Payload["post_snapshot"])
	}
	if postSnapshot.HeadCommit == "" {
		t.Fatal("expected post-run head commit to be populated")
	}
	if postSnapshot.BranchName != "main" {
		t.Fatalf("expected post-run branch main, got %q", postSnapshot.BranchName)
	}
	if len(postSnapshot.StatusSummary) != 0 {
		t.Fatalf("expected empty post-run status summary, got %v", postSnapshot.StatusSummary)
	}
}

func TestRunGitCommandCapturesMutatingPostSnapshot(t *testing.T) {
	workspace := createGitWorkspace(t)

	filePath := filepath.Join(workspace.Path, "notes.txt")
	if err := os.WriteFile(filePath, []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	recorder := engine.NewEventRecorder()
	result, err := engine.RunCommandWithEvents(workspace.Path, "git status --short", workspace.ID, recorder)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "notes.txt") {
		t.Fatalf("expected git status output to mention notes.txt, got %q", result.Stdout)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	preSnapshot := events[0].Payload["pre_snapshot"].(engine.Snapshot)
	if len(preSnapshot.StatusSummary) != 1 || !strings.Contains(preSnapshot.StatusSummary[0], "notes.txt") {
		t.Fatalf("expected pre snapshot to include notes.txt, got %v", preSnapshot.StatusSummary)
	}

	if err := os.WriteFile(filePath, []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("update pending file: %v", err)
	}
	cmd := exec.Command("git", "add", "notes.txt")
	cmd.Dir = workspace.Path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add notes.txt: %v\n%s", err, string(output))
	}

	mutatingRecorder := engine.NewEventRecorder()
	result, err = engine.RunCommandWithEvents(workspace.Path, "git commit -m \"Add notes.txt\"", workspace.ID, mutatingRecorder)
	if err != nil {
		t.Fatalf("run mutating command: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected commit exit code 0, got %d", result.ExitCode)
	}

	mutatingEvents := mutatingRecorder.Events()
	if len(mutatingEvents) != 2 {
		t.Fatalf("expected 2 mutating events, got %d", len(mutatingEvents))
	}

	mutatingPre := mutatingEvents[0].Payload["pre_snapshot"].(engine.Snapshot)
	if len(mutatingPre.StatusSummary) != 1 || !strings.Contains(mutatingPre.StatusSummary[0], "notes.txt") {
		t.Fatalf("expected mutating pre snapshot to include notes.txt, got %v", mutatingPre.StatusSummary)
	}

	mutatingPost := mutatingEvents[1].Payload["post_snapshot"].(engine.Snapshot)
	if len(mutatingPost.StatusSummary) != 0 {
		t.Fatalf("expected clean post snapshot after commit, got %v", mutatingPost.StatusSummary)
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

func TestRunCommandRejectsNonGitExecutables(t *testing.T) {
	workspace := createGitWorkspace(t)

	if _, err := engine.RunCommand(workspace.Path, "go version"); err == nil {
		t.Fatal("expected error for non-git command")
	}
}
