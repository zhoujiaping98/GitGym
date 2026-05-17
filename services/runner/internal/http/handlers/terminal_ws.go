package handlers

import (
	"context"
	"errors"
	"net/http"

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
		defer conn.CloseNow()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		session, err := manager.Acquire(ctx, workspacePath, workspaceID)
		if err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}

		if err := wsjson.Write(ctx, conn, engine.TerminalServerMessage{
			Type: "ready",
			Cols: 120,
			Rows: 30,
		}); err != nil {
			return
		}

		streamDone := make(chan error, 1)
		go func() {
			streamErr := session.ReadLoop(ctx, func(chunk []byte) error {
				return wsjson.Write(ctx, conn, engine.TerminalServerMessage{
					Type: "output",
					Data: string(chunk),
				})
			})
			if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
				_ = conn.Close(websocket.StatusNormalClosure, "")
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
				if err := session.WriteInput(message.Data); err != nil {
					_ = conn.Close(websocket.StatusInternalError, err.Error())
					return
				}
			case "resize":
				if err := session.Resize(message.Cols, message.Rows); err != nil {
					_ = conn.Close(websocket.StatusInternalError, err.Error())
					return
				}
			case "ping":
			}
		}
	}
}
