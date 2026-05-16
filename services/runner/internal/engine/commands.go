package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const defaultCommandTimeout = 30 * time.Second

type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMS int
}

func RunCommand(workspacePath string, raw string) (CommandResult, error) {
	parts, err := parseCommand(raw)
	if err != nil {
		return CommandResult{}, err
	}
	if err := validateCommandPolicy(parts); err != nil {
		return CommandResult{}, err
	}

	return runCommand(workspacePath, parts)
}

func RunCommandWithEvents(workspacePath string, raw string, workspaceID string, recorder *EventRecorder) (CommandResult, error) {
	parts, err := parseCommand(raw)
	if err != nil {
		recordCommandFinished(recorder, workspaceID, raw, CommandResult{}, false, err, nil, nil)
		return CommandResult{}, err
	}
	if err := validateCommandPolicy(parts); err != nil {
		recordCommandFinished(recorder, workspaceID, raw, CommandResult{}, false, err, nil, nil)
		return CommandResult{}, err
	}

	preSnapshot, err := CaptureSnapshot(workspacePath)
	if err != nil {
		err = fmt.Errorf("capture pre-run snapshot: %w", err)
		recordCommandFinished(recorder, workspaceID, raw, CommandResult{}, false, err, nil, nil)
		return CommandResult{}, err
	}

	recordCommandStarted(recorder, workspaceID, raw, &preSnapshot)

	result, commandErr := runCommand(workspacePath, parts)
	postSnapshot, snapshotErr := CaptureSnapshot(workspacePath)
	var postSnapshotPayload *Snapshot
	if snapshotErr != nil {
		snapshotErr = fmt.Errorf("capture post-run snapshot: %w", snapshotErr)
	} else {
		postSnapshotPayload = &postSnapshot
	}

	recordCommandFinished(recorder, workspaceID, raw, result, true, commandErr, snapshotErr, postSnapshotPayload)

	if commandErr != nil && snapshotErr != nil {
		return result, errors.Join(commandErr, snapshotErr)
	}
	if commandErr != nil {
		return result, commandErr
	}
	if snapshotErr != nil {
		return result, snapshotErr
	}
	return result, nil
}

func runCommand(workspacePath string, parts []string) (CommandResult, error) {
	timeout := commandTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = workspacePath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := int(time.Since(start).Milliseconds())

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return CommandResult{}, fmt.Errorf("command timed out after %s", timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return CommandResult{}, err
		}
	}

	return CommandResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMS: duration,
	}, nil
}

func commandTimeout() time.Duration {
	raw := os.Getenv("GITGYM_RUNNER_COMMAND_TIMEOUT")
	if raw == "" {
		return defaultCommandTimeout
	}

	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultCommandTimeout
	}

	return timeout
}

func validateCommandPolicy(parts []string) error {
	if len(parts) == 0 {
		return errors.New("command is required")
	}
	if parts[0] != "git" {
		return fmt.Errorf("only git commands are allowed: %q", parts[0])
	}
	return nil
}

func recordCommandStarted(recorder *EventRecorder, workspaceID string, raw string, preSnapshot *Snapshot) {
	if recorder == nil {
		return
	}

	payload := map[string]any{
		"raw": raw,
	}
	if preSnapshot != nil {
		payload["pre_snapshot"] = *preSnapshot
	}
	recorder.Record("command_started", workspaceID, payload)
}

func recordCommandFinished(recorder *EventRecorder, workspaceID string, raw string, result CommandResult, launched bool, commandErr error, snapshotErr error, postSnapshot *Snapshot) {
	if recorder == nil {
		return
	}

	payload := map[string]any{
		"raw": raw,
	}
	if launched {
		payload["exit_code"] = result.ExitCode
		payload["duration_ms"] = result.DurationMS
	}
	if postSnapshot != nil {
		payload["post_snapshot"] = *postSnapshot
	}
	if commandErr != nil {
		payload["error"] = commandErr.Error()
	}
	if snapshotErr != nil {
		payload["post_snapshot_error"] = snapshotErr.Error()
	}
	recorder.Record("command_finished", workspaceID, payload)
}

func parseCommand(raw string) ([]string, error) {
	var (
		parts        []string
		current      []rune
		inQuote      rune
		sawToken     bool
		tokenStarted bool
	)

	flush := func() {
		if !tokenStarted {
			return
		}
		parts = append(parts, string(current))
		current = current[:0]
		tokenStarted = false
	}

	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case inQuote != 0:
			if r == '\\' && i+1 < len(runes) && runes[i+1] == '\\' {
				current = append(current, runes[i+1])
				sawToken = true
				tokenStarted = true
				i++
			} else if r == '\\' && i+1 < len(runes) && runes[i+1] == inQuote && !quoteTerminatesToken(runes, i+1) {
				current = append(current, runes[i+1])
				sawToken = true
				tokenStarted = true
				i++
			} else if r == inQuote {
				inQuote = 0
			} else {
				current = append(current, r)
				sawToken = true
				tokenStarted = true
			}
		case r == '"' || r == '\'':
			inQuote = r
			sawToken = true
			tokenStarted = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current = append(current, r)
			sawToken = true
			tokenStarted = true
		}
	}

	if inQuote != 0 {
		return nil, errors.New("unterminated quoted command argument")
	}

	flush()

	if !sawToken || len(parts) == 0 {
		return nil, errors.New("command is required")
	}

	return parts, nil
}

func quoteTerminatesToken(runes []rune, quoteIndex int) bool {
	if quoteIndex+1 >= len(runes) {
		return true
	}

	switch runes[quoteIndex+1] {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
