package engine

import (
	"bytes"
	"errors"
	"os/exec"
	"time"
)

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

	return runCommand(workspacePath, parts)
}

func RunCommandWithEvents(workspacePath string, raw string, workspaceID string, recorder *EventRecorder) (CommandResult, error) {
	if recorder != nil {
		recorder.Record("command.started", workspaceID, map[string]any{
			"raw": raw,
		})
	}

	result, err := RunCommand(workspacePath, raw)

	if recorder != nil {
		payload := map[string]any{
			"raw": raw,
		}
		if err == nil {
			payload["exit_code"] = result.ExitCode
			payload["duration_ms"] = result.DurationMS
		} else {
			payload["error"] = err.Error()
		}
		recorder.Record("command.finished", workspaceID, payload)
	}

	return result, err
}

func runCommand(workspacePath string, parts []string) (CommandResult, error) {
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workspacePath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := int(time.Since(start).Milliseconds())

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

func parseCommand(raw string) ([]string, error) {
	var (
		parts    []string
		current  []rune
		inQuote  rune
		escaping bool
		sawToken bool
	)

	flush := func() {
		if len(current) == 0 {
			return
		}
		parts = append(parts, string(current))
		current = current[:0]
	}

	for _, r := range raw {
		switch {
		case escaping:
			current = append(current, r)
			sawToken = true
			escaping = false
		case r == '\\':
			escaping = true
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				current = append(current, r)
				sawToken = true
			}
		case r == '"' || r == '\'':
			inQuote = r
			sawToken = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current = append(current, r)
			sawToken = true
		}
	}

	if escaping {
		current = append(current, '\\')
		sawToken = true
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
