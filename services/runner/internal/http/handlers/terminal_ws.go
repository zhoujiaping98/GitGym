package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

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
		go func() {
			streamErr := session.ReadLoop(ctx, func(chunk []byte) error {
				output, exitCodes := completionTracker.ingestOutput(string(chunk))
				if output != "" {
					if err := writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					}); err != nil {
						return err
					}
				}
				for _, exitCode := range exitCodes {
					if err := writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &exitCode,
					}); err != nil {
						return err
					}
				}
				return nil
			})
			switch {
			case streamErr == nil:
				output, exitCodes := completionTracker.flush()
				if output != "" {
					_ = writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					})
				}
				for _, exitCode := range exitCodes {
					_ = writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &exitCode,
					})
				}
				if exitCode, ok := terminalExitCode(session.Wait()); ok {
					_ = writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &exitCode,
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

		commandTracker := newSubmittedCommandTracker()

		for {
			var message engine.TerminalClientMessage
			if err := wsjson.Read(ctx, conn, &message); err != nil {
				return
			}

			switch message.Type {
			case "input":
				commands := commandTracker.ingest(message.Data)
				completionTracker.noteSubmittedCommands(len(commands))
				if err := session.WriteInput(message.Data); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
				for _, command := range commands {
					if err := writeFrame(engine.TerminalServerMessage{
						Type:   "status",
						Phase:  "running",
						Detail: command,
					}); err != nil {
						return
					}
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

type submittedCommandTracker struct {
	pending        []rune
	inEscapeBranch bool
}

func newSubmittedCommandTracker() *submittedCommandTracker {
	return &submittedCommandTracker{}
}

func (t *submittedCommandTracker) ingest(input string) []string {
	commands := make([]string, 0)

	for _, char := range input {
		if t.inEscapeBranch {
			if char >= 0x40 && char <= 0x7E {
				t.inEscapeBranch = false
			}
			continue
		}

		switch char {
		case 0x1B:
			t.inEscapeBranch = true
		case '\r', '\n':
			command := strings.TrimSpace(string(t.pending))
			if command != "" {
				commands = append(commands, command)
			}
			t.pending = t.pending[:0]
		case '\b', 0x7F:
			if len(t.pending) > 0 {
				t.pending = t.pending[:len(t.pending)-1]
			}
		default:
			if unicode.IsPrint(char) {
				t.pending = append(t.pending, char)
			}
		}
	}

	return commands
}

func terminalExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		return 0, false
	}

	return 0, false
}

var terminalCommandExitLine = regexp.MustCompile("^" + regexp.QuoteMeta(engine.TerminalCommandExitMarker) + ":(-?\\d+)\\r?$")
var terminalANSISequence = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type commandCompletionTracker struct {
	mu            sync.Mutex
	pendingOutput string
	pendingExits  int
}

func newCommandCompletionTracker() *commandCompletionTracker {
	return &commandCompletionTracker{}
}

func (t *commandCompletionTracker) noteSubmittedCommands(count int) {
	if count <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingExits += count
}

func (t *commandCompletionTracker) ingestOutput(chunk string) (string, []int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pendingOutput += chunk
	return t.drainLocked(false)
}

func (t *commandCompletionTracker) flush() (string, []int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.drainLocked(true)
}

func (t *commandCompletionTracker) drainLocked(flushAll bool) (string, []int) {
	var output strings.Builder
	exitCodes := make([]int, 0)

	for {
		newline := strings.IndexByte(t.pendingOutput, '\n')
		if newline < 0 {
			break
		}

		line := t.pendingOutput[:newline+1]
		t.pendingOutput = t.pendingOutput[newline+1:]

		if exitCode, ok := parseTerminalCommandExitLine(line); ok {
			if t.pendingExits > 0 {
				t.pendingExits--
				exitCodes = append(exitCodes, exitCode)
			}
			continue
		}

		output.WriteString(line)
	}

	if flushAll || !couldBeExitMarkerFragment(t.pendingOutput) {
		output.WriteString(t.pendingOutput)
		t.pendingOutput = ""
	}

	return output.String(), exitCodes
}

func parseTerminalCommandExitLine(line string) (int, bool) {
	normalized := strings.TrimSuffix(line, "\n")
	normalized = strings.TrimSuffix(normalized, "\r")
	normalized = terminalANSISequence.ReplaceAllString(normalized, "")

	match := terminalCommandExitLine.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return 0, false
	}

	exitCode, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}

	return exitCode, true
}

func couldBeExitMarkerFragment(value string) bool {
	if value == "" {
		return false
	}

	return strings.HasPrefix(engine.TerminalCommandExitMarker, value)
}
