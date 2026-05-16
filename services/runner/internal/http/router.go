package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/runner/internal/http/handlers"
)

func NewRouter(workRoot string) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace(workRoot))
	return r
}
