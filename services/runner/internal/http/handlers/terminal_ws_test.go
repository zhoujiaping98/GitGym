package handlers

import (
	"encoding/base64"
	"strconv"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestCommandCompletionTrackerPrefersShellReportedCommandMetadata(t *testing.T) {
	completionTracker := newCommandCompletionTracker()

	output, completions := completionTracker.ingestOutput(testTerminalCommandMetadataLine(0, "python") + "\r\n")
	if output != "" {
		t.Fatalf("expected metadata marker to be removed from transcript, got %q", output)
	}
	if len(completions) != 1 {
		t.Fatalf("expected one command completion from prompt metadata, got %d", len(completions))
	}
	if completions[0].command != "python" {
		t.Fatalf("expected prompt metadata command %q, got %q", "python", completions[0].command)
	}
	if completions[0].exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", completions[0].exitCode)
	}

	output, completions = completionTracker.flush()
	if output != "" {
		t.Fatalf("expected no buffered transcript after flush, got %q", output)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no synthetic completions from prior stdin heuristics, got %#v", completions)
	}
}

func TestCommandCompletionTrackerDecodesMultilineCommandMetadata(t *testing.T) {
	completionTracker := newCommandCompletionTracker()

	command := "if ($true) {\nWrite-Output \"paste\"\n}"
	_, completions := completionTracker.ingestOutput(testTerminalCommandMetadataLine(17, command) + "\r\n")
	if len(completions) != 1 {
		t.Fatalf("expected one multiline command completion, got %d", len(completions))
	}
	if completions[0].command != command {
		t.Fatalf("expected multiline command %q, got %q", command, completions[0].command)
	}
	if completions[0].exitCode != 17 {
		t.Fatalf("expected exit code 17, got %d", completions[0].exitCode)
	}
}

func TestCommandCompletionTrackerDropsContinuationPromptMarkersFromTranscript(t *testing.T) {
	completionTracker := newCommandCompletionTracker()

	output, completions := completionTracker.ingestOutput(engine.TerminalContinuationPromptMarker + "\r\n")
	if output != "" {
		t.Fatalf("expected continuation marker to be removed from transcript, got %q", output)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no command completion from continuation marker, got %#v", completions)
	}
}

func testTerminalCommandMetadataLine(exitCode int, command string) string {
	return engine.TerminalCommandExitMarker + ":" + strconv.Itoa(exitCode) + ":" + base64.StdEncoding.EncodeToString([]byte(command))
}
