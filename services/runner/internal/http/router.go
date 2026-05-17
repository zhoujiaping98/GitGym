package httpx

import (
	"net/http"

	"gitgym/services/runner/internal/engine"
	"gitgym/services/runner/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func NewRouter(workRoot string) http.Handler {
	r := chi.NewRouter()
	terminalManager := engine.NewTerminalManager()
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace(workRoot))
	r.Post("/internal/workspaces/{workspaceID}/commands", handlers.RunCommand(workRoot))
	r.Post("/internal/workspaces/{workspaceID}/reset", handlers.ResetWorkspace(workRoot))
	r.Get("/internal/workspaces/{workspaceID}/terminal", handlers.TerminalWebSocket(workRoot, terminalManager))
	return r
}
