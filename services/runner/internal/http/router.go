package httpx

import (
	"net/http"

	"gitgym/services/runner/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func NewRouter(workRoot string) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace(workRoot))
	r.Post("/internal/workspaces/{workspaceID}/commands", handlers.RunCommand())
	r.Post("/internal/workspaces/{workspaceID}/reset", handlers.ResetWorkspace())
	return r
}
