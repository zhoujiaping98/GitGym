package handlers

import (
	"encoding/base64"
	"context"
	"errors"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gitgym/services/runner/internal/engine"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

func TerminalWebSocket(workRoot string, manager *engine.TerminalManager) http.HandlerFunc {
	if manager == nil {
		manager = engine.NewTerminalManager()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		var closeOnce sync.Once
		var writeMu sync.Mutex
		closeConn := func(code websocket.StatusCode, reason string) {
			closeOnce.Do(func() {
				_ = conn.Close(code, reason)
			})
		}
		writeFrame := func(message engine.TerminalServerMessage) error {
			writeMu.Lock()
			defer writeMu.Unlock()

			return wsjson.Write(ctx, conn, message)
		}

		session, err := manager.Acquire(ctx, workspacePath, workspaceID)
		if err != nil {
			closeConn(websocket.StatusInternalError, "terminal unavailable")
			return
		}

		if err := writeFrame(engine.TerminalServerMessage{
			Type: "ready",
			Cols: 120,
			Rows: 30,
		}); err != nil {
			return
		}

		streamDone := make(chan error, 1)
		completionTracker := newCommandCompletionTracker()
		exitSubmissionTracker := newShellExitSubmissionTracker()
		go func() {
			streamErr := session.ReadLoop(ctx, func(chunk []byte) error {
				output, completions := completionTracker.ingestOutput(string(chunk))
				if output != "" {
					if err := writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					}); err != nil {
						return err
					}
				}
				for _, completion := range completions {
					if err := writeFrame(engine.TerminalServerMessage{
						Type:   "status",
						Phase:  "running",
						Detail: completion.command,
					}); err != nil {
						return err
					}
					if err := writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &completion.exitCode,
					}); err != nil {
						return err
					}
				}
				return nil
			})
			switch {
			case streamErr == nil:
				waitErr := session.Wait()
				exitCode, haveExitCode := terminalExitCode(waitErr)
				output, completions := completionTracker.flush()
				if haveExitCode {
					if command, ok := exitSubmissionTracker.consume(); ok && !hasCommandCompletion(completions, command) {
						completions = append(completions, commandCompletion{
							command:  command,
							exitCode: exitCode,
						})
					}
				}
				if output != "" {
					_ = writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					})
				}
				for _, completion := range completions {
					_ = writeFrame(engine.TerminalServerMessage{
						Type:   "status",
						Phase:  "running",
						Detail: completion.command,
					})
					_ = writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &completion.exitCode,
					})
				}
				closeConn(websocket.StatusNormalClosure, "")
			case !errors.Is(streamErr, context.Canceled):
				closeConn(websocket.StatusInternalError, "terminal stream failed")
			}
			streamDone <- streamErr
		}()
		defer func() {
			cancel()
			<-streamDone
		}()

		for {
			var message engine.TerminalClientMessage
			if err := wsjson.Read(ctx, conn, &message); err != nil {
				return
			}

			switch message.Type {
			case "input":
				if command, ok := parseShellExitInput(message.Data); ok {
					exitSubmissionTracker.note(command)
				} else {
					exitSubmissionTracker.clear()
				}
				if err := session.WriteInput(message.Data); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
			case "resize":
				if err := session.Resize(message.Cols, message.Rows); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
			case "ping":
			}
		}
	}
}

func terminalExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		return 0, false
	}

	return 0, false
}

var terminalContinuationPromptLine = regexp.MustCompile("^" + regexp.QuoteMeta(engine.TerminalContinuationPromptMarker) + "\\r?$")
var terminalANSISequence = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
var terminalCommandMetadataLine = regexp.MustCompile("^" + regexp.QuoteMeta(engine.TerminalCommandExitMarker) + ":(-?\\d+):([A-Za-z0-9+/=]+)\\r?$")
var shellExitCommandPattern = regexp.MustCompile(`(?i)^(?:exit(?:\s+\d+)?|logout)$`)

type commandCompletionTracker struct {
	mu            sync.Mutex
	pendingOutput string
}

type commandCompletion struct {
	command  string
	exitCode int
}

func newCommandCompletionTracker() *commandCompletionTracker {
	return &commandCompletionTracker{}
}

func (t *commandCompletionTracker) ingestOutput(chunk string) (string, []commandCompletion) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pendingOutput += chunk
	return t.drainLocked(false)
}

func (t *commandCompletionTracker) flush() (string, []commandCompletion) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.drainLocked(true)
}

func (t *commandCompletionTracker) drainLocked(flushAll bool) (string, []commandCompletion) {
	var output strings.Builder
	completions := make([]commandCompletion, 0)

	for {
		newline := strings.IndexByte(t.pendingOutput, '\n')
		if newline < 0 {
			break
		}

		line := t.pendingOutput[:newline+1]
		t.pendingOutput = t.pendingOutput[newline+1:]

		if completion, ok := parseTerminalCommandMetadataLine(line); ok {
			completions = append(completions, completion)
			continue
		}
		if isTerminalContinuationPromptLine(line) {
			continue
		}

		output.WriteString(line)
	}

	if flushAll || !couldBeExitMarkerFragment(t.pendingOutput) {
		output.WriteString(t.pendingOutput)
		t.pendingOutput = ""
	}

	return output.String(), completions
}

func parseTerminalCommandMetadataLine(line string) (commandCompletion, bool) {
	normalized := normalizeTerminalMarkerLine(line)

	match := terminalCommandMetadataLine.FindStringSubmatch(normalized)
	if len(match) != 3 {
		return commandCompletion{}, false
	}

	exitCode, err := strconv.Atoi(match[1])
	if err != nil {
		return commandCompletion{}, false
	}

	commandBytes, err := base64.StdEncoding.DecodeString(match[2])
	if err != nil {
		return commandCompletion{}, false
	}

	command := strings.TrimSpace(string(commandBytes))
	if command == "" {
		return commandCompletion{}, false
	}

	return commandCompletion{
		command:  command,
		exitCode: exitCode,
	}, true
}

func isTerminalContinuationPromptLine(line string) bool {
	return terminalContinuationPromptLine.MatchString(normalizeTerminalMarkerLine(line))
}

func normalizeTerminalMarkerLine(line string) string {
	normalized := strings.TrimSuffix(line, "\n")
	normalized = strings.TrimSuffix(normalized, "\r")
	return terminalANSISequence.ReplaceAllString(normalized, "")
}

func couldBeExitMarkerFragment(value string) bool {
	if value == "" {
		return false
	}

	return strings.HasPrefix(engine.TerminalCommandExitMarker, value)
}

func parseShellExitInput(data string) (string, bool) {
	normalized := strings.ReplaceAll(data, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	lines := make([]string, 0, 1)
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}

	if len(lines) != 1 {
		return "", false
	}

	command := lines[0]
	if !shellExitCommandPattern.MatchString(command) {
		return "", false
	}

	return command, true
}

func hasCommandCompletion(completions []commandCompletion, command string) bool {
	for _, completion := range completions {
		if completion.command == command {
			return true
		}
	}
	return false
}

type shellExitSubmissionTracker struct {
	mu      sync.Mutex
	pending string
}

func newShellExitSubmissionTracker() *shellExitSubmissionTracker {
	return &shellExitSubmissionTracker{}
}

func (t *shellExitSubmissionTracker) note(command string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pending = command
}

func (t *shellExitSubmissionTracker) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pending = ""
}

func (t *shellExitSubmissionTracker) consume() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pending == "" {
		return "", false
	}

	command := t.pending
	t.pending = ""
	return command, true
}
