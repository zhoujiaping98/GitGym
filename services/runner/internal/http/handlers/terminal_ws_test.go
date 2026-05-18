package handlers

import (
	"runtime"
	"testing"
)

func TestSubmittedCommandTrackerQueuesMultipleTopLevelCommandsFromSingleFrame(t *testing.T) {
	tracker := newSubmittedCommandTracker()

	tracker.ingest("pwd\r\nls\r\n")

	firstCompletion, ok := tracker.completeCommand(0)
	if !ok {
		t.Fatal("expected first pasted command to complete")
	}
	if firstCompletion.command != "pwd" {
		t.Fatalf("expected first command %q, got %q", "pwd", firstCompletion.command)
	}

	secondCompletion, ok := tracker.completeCommand(0)
	if !ok {
		t.Fatal("expected second pasted command to complete")
	}
	if secondCompletion.command != "ls" {
		t.Fatalf("expected second command %q, got %q", "ls", secondCompletion.command)
	}
}

func TestSubmittedCommandTrackerTreatsSameFrameMultilinePasteAsSingleCommand(t *testing.T) {
	tracker := newSubmittedCommandTracker()

	input, wantCommand := pastedMultilineCommandFixture()
	tracker.ingest(input)

	completion, ok := tracker.completeCommand(0)
	if !ok {
		t.Fatal("expected pasted multiline command to complete once closed")
	}

	if completion.command != wantCommand {
		t.Fatalf("expected combined multiline command, got %q", completion.command)
	}

	if _, ok := tracker.completeCommand(0); ok {
		t.Fatal("expected pasted multiline command to produce only one completion")
	}
}

func TestSubmittedCommandTrackerWaitsForMultilineCommandClosure(t *testing.T) {
	tracker := newSubmittedCommandTracker()

	tracker.ingest("if ($true) {\r\n")
	if _, ok := tracker.completeCommand(0); ok {
		t.Fatal("expected multiline command opener to stay pending")
	}

	tracker.ingest("Write-Output \"tracker\"\r\n")
	if _, ok := tracker.completeCommand(0); ok {
		t.Fatal("expected multiline command body to stay pending")
	}

	tracker.ingest("}\r\n")
	completion, ok := tracker.completeCommand(0)
	if !ok {
		t.Fatal("expected closed multiline command to complete")
	}

	if completion.command != "if ($true) {\nWrite-Output \"tracker\"\n}" {
		t.Fatalf("expected combined multiline command, got %q", completion.command)
	}
	if completion.exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", completion.exitCode)
	}
}

func TestIsCompleteTopLevelCommandAllowsPowerShellPaths(t *testing.T) {
	for _, command := range []string{
		`cd C:\`,
		`Write-Output "C:\"`,
	} {
		if !isCompleteTopLevelCommandForShell(powershellShellDialect, command) {
			t.Fatalf("expected PowerShell command %q to be complete", command)
		}
	}
}

func pastedMultilineCommandFixture() (string, string) {
	if runtime.GOOS == "windows" {
		return "if ($true) {\r\nWrite-Output \"paste\"\r\n}\r\n", "if ($true) {\nWrite-Output \"paste\"\n}"
	}

	return "if true; then\nprintf 'paste\\n'\nfi\n", "if true; then\nprintf 'paste\\n'\nfi"
}
