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

	return runCommand(workspacePath, parts)
}

func RunCommandWithEvents(workspacePath string, raw string, workspaceID string, recorder *EventRecorder) (CommandResult, error) {
	if recorder != nil {
		recorder.Record("command_started", workspaceID, map[string]any{
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
		recorder.Record("command_finished", workspaceID, payload)
	}

	return result, err
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

func parseCommand(raw string) ([]string, error) {
	var (
		parts    []string
		current  []rune
		inQuote  rune
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
		case inQuote != 0:
			if r == '\\' {
				current = append(current, r)
				sawToken = true
			} else if r == inQuote {
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

	if inQuote != 0 {
		return nil, errors.New("unterminated quoted command argument")
	}

	flush()

	if !sawToken || len(parts) == 0 {
		return nil, errors.New("command is required")
	}

	return parts, nil
}
