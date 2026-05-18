package handlers

import "testing"

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
