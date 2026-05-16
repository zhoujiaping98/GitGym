package engine

import (
	"errors"
	"os/exec"
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

func TestRunCommandReturnsTimeoutError(t *testing.T) {
	t.Setenv("GITGYM_RUNNER_COMMAND_TIMEOUT", "10ms")

	result, err := RunCommand(t.TempDir(), `go run ./internal/engine/testdata/sleepmain`)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if result != (CommandResult{}) {
		t.Fatalf("expected zero result on timeout, got %#v", result)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if !errors.Is(err, exec.ErrNotFound) && !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout-related error, got %v", err)
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
