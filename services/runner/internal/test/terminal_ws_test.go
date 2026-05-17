package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gitgym/services/runner/internal/engine"
	httpx "gitgym/services/runner/internal/http"
	"gitgym/services/runner/internal/http/handlers"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

func TestTerminalWebSocketRejectsMalformedWorkspaceID(t *testing.T) {
	server := httptest.NewServer(httpx.NewRouter(t.TempDir()))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/internal/workspaces/not-valid/terminal")
	if err != nil {
		t.Fatalf("get terminal endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if got := payload["error"]; got != "workspace ID is malformed" {
		t.Fatalf("expected malformed workspace error, got %#v", got)
	}
}

func TestTerminalWebSocketStreamsOutputFromPTY(t *testing.T) {
	workspace := createGitWorkspace(t)
	manager := engine.NewTerminalManager()
	conn := dialTerminalWebSocket(t, workspace.ID, filepath.Dir(workspace.Path), manager)
	defer closeTerminalWebSocket(t, conn)
	t.Cleanup(func() {
		releaseManagedTerminalSocketSession(t, manager, workspace)
	})

	assertTerminalReadyFrame(t, conn)

	marker := terminalMarker("ws-output")
	want := regexp.MustCompile(regexp.QuoteMeta(marker + ":__GITGYM_WS_OUTPUT__"))

	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "input",
		Data: shellPrintLine(marker, "__GITGYM_WS_OUTPUT__"),
	}); err != nil {
		t.Fatalf("write terminal input frame: %v", err)
	}

	output := readTerminalWebSocketUntilMatch(t, conn, want)
	if !want.MatchString(output) {
		t.Fatalf("expected terminal websocket output to contain marker, got %q", output)
	}
}

func TestTerminalWebSocketForwardsInputMessagesToPTY(t *testing.T) {
	workspace := createGitWorkspace(t)
	manager := engine.NewTerminalManager()
	conn := dialTerminalWebSocket(t, workspace.ID, filepath.Dir(workspace.Path), manager)
	defer closeTerminalWebSocket(t, conn)
	t.Cleanup(func() {
		releaseManagedTerminalSocketSession(t, manager, workspace)
	})

	assertTerminalReadyFrame(t, conn)

	marker := terminalMarker("ws-input")
	want := regexp.MustCompile(regexp.QuoteMeta(marker + ":__GITGYM_WS_INPUT__"))

	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "input",
		Data: shellPrintLine(marker, "__GITGYM_WS_INPUT__"),
	}); err != nil {
		t.Fatalf("write terminal input frame: %v", err)
	}

	output := readTerminalWebSocketUntilMatch(t, conn, want)
	if !want.MatchString(output) {
		t.Fatalf("expected websocket-forwarded input to reach PTY, got %q", output)
	}
}

func TestTerminalWebSocketHandlesResizeMessages(t *testing.T) {
	workspace := createGitWorkspace(t)
	manager := engine.NewTerminalManager()
	conn := dialTerminalWebSocket(t, workspace.ID, filepath.Dir(workspace.Path), manager)
	defer closeTerminalWebSocket(t, conn)
	t.Cleanup(func() {
		releaseManagedTerminalSocketSession(t, manager, workspace)
	})

	assertTerminalReadyFrame(t, conn)

	initialCols, initialRows := readTerminalSizeFromWebSocket(t, conn)
	targetCols, targetRows := resizedDimensions(initialCols, initialRows)

	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "resize",
		Cols: targetCols,
		Rows: targetRows,
	}); err != nil {
		t.Fatalf("write resize frame: %v", err)
	}

	resizedCols, resizedRows := readTerminalSizeFromWebSocket(t, conn)
	if resizedCols != targetCols || resizedRows != targetRows {
		t.Fatalf("expected terminal size %dx%d after websocket resize, got %dx%d", targetCols, targetRows, resizedCols, resizedRows)
	}
}

func TestTerminalWebSocketClosesWhenPTYSessionEnds(t *testing.T) {
	workspace := createGitWorkspace(t)
	manager := engine.NewTerminalManager()
	conn := dialTerminalWebSocket(t, workspace.ID, filepath.Dir(workspace.Path), manager)
	t.Cleanup(func() {
		closeTerminalWebSocket(t, conn)
	})

	assertTerminalReadyFrame(t, conn)

	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "input",
		Data: shellExit(),
	}); err != nil {
		t.Fatalf("write terminal exit input: %v", err)
	}

	status := readTerminalWebSocketCloseStatus(t, conn)
	if status != websocket.StatusNormalClosure {
		t.Fatalf("expected terminal websocket close status %v after PTY exit, got %v", websocket.StatusNormalClosure, status)
	}
}

func dialTerminalWebSocket(t *testing.T, workspaceID string, workRoot string, manager *engine.TerminalManager) *websocket.Conn {
	t.Helper()

	router := chi.NewRouter()
	router.Get("/internal/workspaces/{workspaceID}/terminal", handlers.TerminalWebSocket(workRoot, manager))

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/internal/workspaces/" + workspaceID + "/terminal"
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial terminal websocket (status %d): %v", status, err)
	}

	return conn
}

func releaseManagedTerminalSocketSession(t *testing.T, manager *engine.TerminalManager, workspace engine.Workspace) {
	t.Helper()

	session, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("re-acquire terminal session for release: %v", err)
	}

	releaseTerminalSession(t, manager, session, workspace.ID)
}

func closeTerminalWebSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, net.ErrClosed) {
		t.Fatalf("close terminal websocket: %v", err)
	}
}

func assertTerminalReadyFrame(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var message engine.TerminalServerMessage
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		t.Fatalf("read ready frame: %v", err)
	}

	if message.Type != "ready" {
		t.Fatalf("expected ready frame, got %+v", message)
	}
	if message.Cols != 120 || message.Rows != 30 {
		t.Fatalf("expected ready frame size 120x30, got %dx%d", message.Cols, message.Rows)
	}
}

func readTerminalWebSocketUntilMatch(t *testing.T, conn *websocket.Conn, want *regexp.Regexp) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var builder strings.Builder
	for {
		var message engine.TerminalServerMessage
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			t.Fatalf("read terminal websocket frame: %v", err)
		}
		if message.Type != "output" {
			continue
		}
		builder.WriteString(message.Data)
		if want.MatchString(builder.String()) {
			return builder.String()
		}
	}
}

func readTerminalSizeFromWebSocket(t *testing.T, conn *websocket.Conn) (uint16, uint16) {
	t.Helper()

	marker := terminalMarker("ws-size")
	want := terminalLinePattern(marker, `(\d+)x(\d+)`)

	if err := wsjson.Write(context.Background(), conn, engine.TerminalClientMessage{
		Type: "input",
		Data: shellPrintSize(marker),
	}); err != nil {
		t.Fatalf("write terminal size request: %v", err)
	}

	output := readTerminalWebSocketUntilMatch(t, conn, want)
	match := want.FindStringSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("expected terminal size output, got %q", output)
	}

	var cols uint16
	var rows uint16
	if _, err := fmt.Sscanf(match[1], "%d", &cols); err != nil {
		t.Fatalf("parse terminal cols from %q: %v", match[1], err)
	}
	if _, err := fmt.Sscanf(match[2], "%d", &rows); err != nil {
		t.Fatalf("parse terminal rows from %q: %v", match[2], err)
	}

	return cols, rows
}

func readTerminalWebSocketCloseStatus(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		var message engine.TerminalServerMessage
		err := wsjson.Read(ctx, conn, &message)
		if err == nil {
			continue
		}

		if status := websocket.CloseStatus(err); status != -1 {
			return status
		}

		t.Fatalf("read terminal websocket close frame: %v", err)
	}
}
