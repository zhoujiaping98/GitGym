package test

import (
	"context"
	"path/filepath"
	"testing"

	"gitgym/services/runner/internal/engine"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestTerminalWebSocketEmitsFinalCommandCompletionBeforeShellExit(t *testing.T) {
	workspace := createGitWorkspace(t)
	manager := engine.NewTerminalManager()
	conn := dialTerminalWebSocket(t, workspace.ID, filepath.Dir(workspace.Path), manager)
	t.Cleanup(func() {
		closeTerminalWebSocket(t, conn)
	})

	assertTerminalReadyFrame(t, conn)

	exitCommand := shellExit()
	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "input",
		Data: exitCommand,
	}); err != nil {
		t.Fatalf("write terminal exit input: %v", err)
	}

	assertTerminalCommandCompletion(t, conn, normalizeSubmittedCommandLines([]string{exitCommand}), 0)

	status := readTerminalWebSocketCloseStatus(t, conn)
	if status != websocket.StatusNormalClosure {
		t.Fatalf("expected terminal websocket close status %v after PTY exit, got %v", websocket.StatusNormalClosure, status)
	}
}
