package httpx

import (
	"net/http"
	"os"
	"time"

	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	PracticeService service.PracticeService
}

func NewRouter(deps ...Dependencies) http.Handler {
	dependencies := defaultDependencies()
	if len(deps) > 0 && deps[0].PracticeService != nil {
		dependencies.PracticeService = deps[0].PracticeService
	}

	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireSessionCookie)
			r.Get("/auth/me", handlers.AuthMe())
			r.Get("/templates", handlers.ListPracticeTemplates(dependencies.PracticeService))
			r.Post("/practice-sessions", handlers.CreatePracticeSession(dependencies.PracticeService))
			r.Get("/practice-sessions/{sessionId}/terminal", handlers.PracticeTerminalWebsocket())
		})
	})
	return r
}

func defaultDependencies() Dependencies {
	return Dependencies{
		PracticeService: service.NewPracticeService(
			service.NewInMemoryPracticeSessionStore(),
			runner.NewClient(os.Getenv("RUNNER_BASE_URL"), http.DefaultClient),
			time.Now,
		),
	}
}
