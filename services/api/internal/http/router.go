package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.With(middleware.RequireSession).Get("/api/v1/auth/me", handlers.AuthMe())
	return r
}
