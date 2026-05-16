package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCommandPreservesWindowsStylePaths(t *testing.T) {
	parts, err := parseCommand(`tool C:\repo\file.txt "D:\two words\file.txt"`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	want := []string{"tool", `C:\repo\file.txt`, `D:\two words\file.txt`}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %#v", len(want), len(parts), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("expected part %d to be %q, got %q", i, want[i], parts[i])
		}
	}
}

func TestParseCommandPreservesExplicitEmptyQuotedArgument(t *testing.T) {
	parts, err := parseCommand(`git commit -m ""`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	want := []string{"git", "commit", "-m", ""}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %#v", len(want), len(parts), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("expected part %d to be %q, got %q", i, want[i], parts[i])
		}
	}
}

func TestParseCommandHandlesEscapedQuotesInsideQuotedArgument(t *testing.T) {
	parts, err := parseCommand(`git commit -m "say \"hi\""`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	want := []string{"git", "commit", "-m", `say "hi"`}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %#v", len(want), len(parts), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("expected part %d to be %q, got %q", i, want[i], parts[i])
		}
	}
}

func TestParseCommandPreservesQuotedWindowsPathEndingWithBackslash(t *testing.T) {
	parts, err := parseCommand(`git add "C:\tmp\dir\\"`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	want := []string{"git", "add", `C:\tmp\dir\`}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %#v", len(want), len(parts), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("expected part %d to be %q, got %q", i, want[i], parts[i])
		}
	}
}

func TestRunCommandReturnsTimeoutError(t *testing.T) {
	t.Setenv("GITGYM_RUNNER_COMMAND_TIMEOUT", "10ms")

	result, err := RunCommand(t.TempDir(), "git daemon")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if result != (CommandResult{}) {
		t.Fatalf("expected zero result on timeout, got %#v", result)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestDefaultCommandTimeoutIsPositive(t *testing.T) {
	if defaultCommandTimeout <= 0 {
		t.Fatalf("expected positive default timeout, got %s", defaultCommandTimeout)
	}
}

func TestCommandTimeoutOverrideFallsBackForInvalidValue(t *testing.T) {
	t.Setenv("GITGYM_RUNNER_COMMAND_TIMEOUT", "not-a-duration")

	if got := commandTimeout(); got != defaultCommandTimeout {
		t.Fatalf("expected default timeout %s, got %s", defaultCommandTimeout, got)
	}
}

func TestCommandTimeoutOverrideUsesEnvDuration(t *testing.T) {
	t.Setenv("GITGYM_RUNNER_COMMAND_TIMEOUT", "25ms")

	if got := commandTimeout(); got != 25*time.Millisecond {
		t.Fatalf("expected override timeout 25ms, got %s", got)
	}
}

func TestRunCommandWithEventsRejectsNonGitCommand(t *testing.T) {
	recorder := NewEventRecorder()

	result, err := RunCommandWithEvents(t.TempDir(), "go version", "ws-1", recorder)
	if err == nil {
		t.Fatal("expected policy error")
	}
	if result != (CommandResult{}) {
		t.Fatalf("expected zero result on policy rejection, got %#v", result)
	}
	if !strings.Contains(err.Error(), "git") {
		t.Fatalf("expected git-only policy error, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "command_finished" {
		t.Fatalf("expected command_finished event, got %q", events[0].Type)
	}
	if got := events[0].Payload["error"]; got == nil || !strings.Contains(got.(string), "git") {
		t.Fatalf("expected git-only policy error in payload, got %#v", got)
	}
}

func TestRunCommandWithEventsRejectsEmptyInputWithoutStartedEvent(t *testing.T) {
	recorder := NewEventRecorder()

	result, err := RunCommandWithEvents(t.TempDir(), "   ", "ws-1", recorder)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if result != (CommandResult{}) {
		t.Fatalf("expected zero result on parse rejection, got %#v", result)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "command_finished" {
		t.Fatalf("expected command_finished event, got %q", events[0].Type)
	}
}

func TestRunCommandWithEventsPreSnapshotFailureDoesNotEmitStarted(t *testing.T) {
	recorder := NewEventRecorder()

	result, err := RunCommandWithEvents(t.TempDir(), "git status --short", "ws-1", recorder)
	if err == nil {
		t.Fatal("expected pre-snapshot error")
	}
	if result != (CommandResult{}) {
		t.Fatalf("expected zero result on pre-snapshot error, got %#v", result)
	}
	if !strings.Contains(err.Error(), "capture pre-run snapshot") {
		t.Fatalf("expected pre-snapshot error, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "command_finished" {
		t.Fatalf("expected command_finished event, got %q", events[0].Type)
	}
}

func TestRunCommandWithEventsPreservesExecutionOutcomeWhenPostSnapshotFails(t *testing.T) {
	workspace := createCommittedWorkspace(t)
	realGitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "git.bat")
	wrapper := "@echo off\r\n" +
		"\"" + realGitPath + "\" %*\r\n" +
		"set EXITCODE=%ERRORLEVEL%\r\n" +
		"if \"%~1\"==\"status\" if \"%~2\"==\"--short\" rmdir /s /q .git >nul 2>nul\r\n" +
		"exit /b %EXITCODE%\r\n"
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o644); err != nil {
		t.Fatalf("write git wrapper: %v", err)
	}

	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	recorder := NewEventRecorder()
	result, err := RunCommandWithEvents(workspace, "git status --short", "ws-1", recorder)
	if err == nil {
		t.Fatal("expected post-snapshot error")
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.DurationMS <= 0 {
		t.Fatalf("expected positive duration, got %d", result.DurationMS)
	}
	if !strings.Contains(err.Error(), "capture post-run snapshot") {
		t.Fatalf("expected post-snapshot error, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if got := events[1].Payload["exit_code"]; got != 0 {
		t.Fatalf("expected exit_code to be preserved, got %#v", got)
	}
	if got := events[1].Payload["duration_ms"]; got == nil {
		t.Fatal("expected duration_ms to be preserved")
	}
	if got := events[1].Payload["post_snapshot_error"]; got == nil || !strings.Contains(got.(string), "capture post-run snapshot") {
		t.Fatalf("expected post_snapshot_error payload, got %#v", got)
	}
	if got := events[1].Payload["error"]; got != nil {
		t.Fatalf("expected command error payload to remain unset, got %#v", got)
	}
}

func createCommittedWorkspace(t *testing.T) string {
	t.Helper()

	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	setupCommands := [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "GitGym Test"},
		{"config", "user.email", "test@gitgym.dev"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	}

	for _, args := range setupCommands {
		runGitCommand(t, path, args...)
	}

	return path
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
