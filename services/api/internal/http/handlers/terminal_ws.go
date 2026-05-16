package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/service"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

func PracticeTerminalWebsocket(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticatedSession, ok := middleware.AuthenticatedSessionFromContext(r.Context())
		if !ok || authenticatedSession.UserID == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "authenticated session missing from request context",
			})
			return
		}

		sessionID, err := strconv.ParseUint(chi.URLParam(r, "sessionId"), 10, 64)
		if err != nil || sessionID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid session id",
			})
			return
		}

		session, err := practiceService.PracticeSessionByID(r.Context(), authenticatedSession.UserID, sessionID)
		if err != nil {
			writeJSON(w, statusForPracticeSessionLookupError(err), map[string]any{
				"error": err.Error(),
			})
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		if err := writeTerminalLine(r.Context(), conn, fmt.Sprintf("Connected to %s at %s", session.RunnerRef, session.WorkspacePathRef)); err != nil {
			return
		}

		for {
			messageType, payload, err := conn.Read(r.Context())
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure || websocket.CloseStatus(err) == websocket.StatusGoingAway || errors.Is(err, context.Canceled) {
					return
				}
				_ = conn.Close(websocket.StatusInternalError, "read error")
				return
			}
			if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
				continue
			}
			if err := conn.Write(r.Context(), messageType, payload); err != nil {
				return
			}
		}
	}
}

func writeTerminalLine(ctx context.Context, conn *websocket.Conn, line string) error {
	return conn.Write(ctx, websocket.MessageText, []byte(line))
}
