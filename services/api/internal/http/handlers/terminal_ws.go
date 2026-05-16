package handlers

import "net/http"

func PracticeTerminalWebsocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": "terminal websocket not implemented",
		})
	}
}
