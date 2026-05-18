package handlers

import (
	"context"
	"errors"
	"net/http"
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
		go func() {
			streamErr := session.ReadLoop(ctx, func(chunk []byte) error {
				return writeFrame(engine.TerminalServerMessage{
					Type: "output",
					Data: string(chunk),
				})
			})
			switch {
			case streamErr == nil:
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
				if err := session.WriteInput(message.Data); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
				for _, command := range commandTracker.ingest(message.Data) {
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
