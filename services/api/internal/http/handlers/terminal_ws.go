package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

func PracticeTerminalWebsocket(practiceService service.PracticeService, runnerClient runner.Client, frontendRedirectURL string) http.HandlerFunc {
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
		if runnerClient == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "runner client is not configured",
			})
			return
		}

		browserSocket, err := websocket.Accept(w, r, terminalAcceptOptions(frontendRedirectURL))
		if err != nil {
			return
		}
		browserConn := websocketTerminalConnection{conn: browserSocket}
		defer browserConn.Close(websocket.StatusNormalClosure, "")

		runnerConn, err := runnerClient.ConnectTerminal(r.Context(), session.RunnerRef)
		if err != nil {
			_ = browserConn.Close(websocket.StatusInternalError, "runner terminal unavailable")
			return
		}
		defer runnerConn.Close(websocket.StatusNormalClosure, "")

		bridgeCtx, cancel := context.WithCancel(r.Context())
		defer cancel()

		bridgeErr := make(chan error, 2)

		go func() {
			bridgeErr <- proxyTerminalFrames(bridgeCtx, browserConn, runnerConn)
		}()
		go func() {
			bridgeErr <- proxyTerminalFrames(bridgeCtx, runnerConn, browserConn)
		}()

		err = <-bridgeErr

		closeStatus, closeReason := terminalCloseFromError(err)

		_ = runnerConn.Close(closeStatus, closeReason)
		_ = browserConn.Close(closeStatus, closeReason)
		cancel()
	}
}

func terminalAcceptOptions(frontendRedirectURL string) *websocket.AcceptOptions {
	originPattern := terminalOriginPattern(frontendRedirectURL)
	if originPattern == "" {
		return nil
	}

	return &websocket.AcceptOptions{
		OriginPatterns: []string{originPattern},
	}
}

func terminalOriginPattern(frontendRedirectURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(frontendRedirectURL))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(parsedURL.Host)
}

func proxyTerminalFrames(ctx context.Context, src runner.TerminalConnection, dst runner.TerminalConnection) error {
	for {
		messageType, payload, err := src.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != int(websocket.MessageText) && messageType != int(websocket.MessageBinary) {
			continue
		}
		if err := dst.Write(ctx, messageType, payload); err != nil {
			return err
		}
	}
}

func terminalCloseFromError(err error) (websocket.StatusCode, string) {
	if err == nil || errors.Is(err, context.Canceled) {
		return websocket.StatusNormalClosure, ""
	}

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		return closeErr.Code, closeErr.Reason
	}

	return websocket.StatusInternalError, "terminal bridge error"
}

type websocketTerminalConnection struct {
	conn *websocket.Conn
}

func (c websocketTerminalConnection) Read(ctx context.Context) (int, []byte, error) {
	messageType, payload, err := c.conn.Read(ctx)
	return int(messageType), payload, err
}

func (c websocketTerminalConnection) Write(ctx context.Context, messageType int, payload []byte) error {
	return c.conn.Write(ctx, websocket.MessageType(messageType), payload)
}

func (c websocketTerminalConnection) Close(status websocket.StatusCode, reason string) error {
	return c.conn.Close(status, reason)
}
