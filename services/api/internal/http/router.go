package httpx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gitgym/services/api/internal/config"
	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/oauth"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	PracticeService     service.PracticeService
	RunnerClient        runner.Client
	AuthStore           service.UserStore
	AuthConfig          config.Config
	GitHubOAuthClient   oauth.GitHubOAuthClient
	InitializationError error
}

var (
	defaultAuthStoreFactoryForTestsMu sync.RWMutex
	defaultAuthStoreFactoryForTests   func() service.UserStore
	openMySQLStoreForTestsMu          sync.RWMutex
	openMySQLStoreForTests            func(string) (service.UserStore, error)
)

type practiceCatalogStore interface {
	ListPracticeTemplates(ctx context.Context) ([]service.PracticeTemplate, error)
	ListPracticeScenarios(ctx context.Context) ([]service.PracticeScenario, error)
}

type storeBackedPracticeCatalog struct {
	store practiceCatalogStore
}

func (c storeBackedPracticeCatalog) ListTemplates(ctx context.Context) ([]service.PracticeTemplate, error) {
	return c.store.ListPracticeTemplates(ctx)
}

func (c storeBackedPracticeCatalog) ListScenarios(ctx context.Context) ([]service.PracticeScenario, error) {
	templates, err := c.store.ListPracticeTemplates(ctx)
	if err != nil {
		return nil, err
	}

	scenarios, err := c.store.ListPracticeScenarios(ctx)
	if err != nil {
		return nil, err
	}

	if err := validateScenarioTemplateReferences(templates, scenarios); err != nil {
		return nil, err
	}

	return scenarios, nil
}

func (c storeBackedPracticeCatalog) TemplateByID(ctx context.Context, templateID uint64) (service.PracticeTemplate, error) {
	templates, err := c.store.ListPracticeTemplates(ctx)
	if err != nil {
		return service.PracticeTemplate{}, err
	}

	for _, template := range templates {
		if template.ID == templateID {
			return template, nil
		}
	}

	return service.PracticeTemplate{}, fmt.Errorf("%w: %d", service.ErrUnknownPracticeTemplate, templateID)
}

func (c storeBackedPracticeCatalog) ScenarioByID(ctx context.Context, scenarioID uint64) (service.PracticeScenario, error) {
	scenarios, err := c.store.ListPracticeScenarios(ctx)
	if err != nil {
		return service.PracticeScenario{}, err
	}

	for _, scenario := range scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}

	return service.PracticeScenario{}, fmt.Errorf("%w: %d", service.ErrUnknownPracticeScenario, scenarioID)
}

func validateScenarioTemplateReferences(templates []service.PracticeTemplate, scenarios []service.PracticeScenario) error {
	templateIDs := make(map[uint64]struct{}, len(templates))
	for _, template := range templates {
		templateIDs[template.ID] = struct{}{}
	}

	for _, scenario := range scenarios {
		if _, ok := templateIDs[scenario.TemplateID]; ok {
			continue
		}

		return fmt.Errorf(
			"%w: scenario %d references missing template %d",
			service.ErrPracticeServiceConfiguration,
			scenario.ID,
			scenario.TemplateID,
		)
	}

	return nil
}

func NewRouter(deps ...Dependencies) http.Handler {
	dependencies := mergeDependencies(defaultDependencies(), deps...)
	if dependencies.InitializationError != nil {
		return newInitializationErrorRouter(dependencies.InitializationError)
	}
	if dependencies.RunnerClient == nil {
		dependencies.RunnerClient = runner.NewClient(dependencies.AuthConfig.RunnerBaseURL, http.DefaultClient)
	}
	if dependencies.PracticeService == nil {
		practiceStore := practiceSessionStoreFromDependencies(dependencies)
		if practiceStore == nil {
			practiceStore = service.NewInMemoryPracticeSessionStore()
		}
		dependencies.PracticeService = service.NewPracticeService(
			practiceStore,
			dependencies.RunnerClient,
			practiceCatalogFromDependencies(dependencies),
			time.Now,
		)
	}
	if dependencies.GitHubOAuthClient == nil {
		dependencies.GitHubOAuthClient = oauth.NewGitHubOAuthClient(
			dependencies.AuthConfig.GitHubClientID,
			dependencies.AuthConfig.GitHubSecret,
			authCallbackURL(dependencies.AuthConfig.APIBaseURL),
			http.DefaultClient,
		)
	}

	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/auth/github/login", handlers.GitHubLoginWithReadiness(dependencies.GitHubOAuthClient, dependencies.AuthStore, dependencies.AuthConfig.FrontendRedirectURL))
		r.Get("/auth/github/callback", handlers.GitHubCallback(dependencies.GitHubOAuthClient, dependencies.AuthStore, dependencies.AuthConfig.FrontendRedirectURL))
		r.Post("/auth/logout", handlers.Logout(dependencies.AuthStore))

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireSessionCookie(dependencies.AuthStore))
			r.Get("/auth/me", handlers.AuthMe(dependencies.AuthStore))
			r.Get("/templates", handlers.ListPracticeTemplates(dependencies.PracticeService))
			r.Get("/practice-sessions/current", handlers.GetCurrentPracticeSession(dependencies.PracticeService))
			r.Post("/practice-sessions", handlers.CreatePracticeSession(dependencies.PracticeService))
			r.Post("/practice-sessions/{sessionId}/reset", handlers.ResetPracticeSession(dependencies.PracticeService))
			r.Get("/practice-sessions/{sessionId}/terminal", handlers.PracticeTerminalWebsocket(dependencies.PracticeService, dependencies.RunnerClient, dependencies.AuthConfig.FrontendRedirectURL))
		})
	})
	return r
}

func practiceSessionStoreFromDependencies(dependencies Dependencies) service.PracticeSessionStore {
	if dependencies.AuthStore == nil {
		return nil
	}

	practiceStore, ok := dependencies.AuthStore.(service.PracticeSessionStore)
	if !ok {
		return nil
	}

	return practiceStore
}

func practiceCatalogFromDependencies(dependencies Dependencies) service.PracticeCatalog {
	if dependencies.AuthStore != nil {
		if catalogStore, ok := dependencies.AuthStore.(practiceCatalogStore); ok {
			return storeBackedPracticeCatalog{store: catalogStore}
		}
	}

	return service.NewFallbackPracticeCatalog()
}

func defaultDependencies() Dependencies {
	authConfig := config.LoadRuntime()
	authStore, initErr := defaultAuthStore(authConfig.MySQLDSN)

	return Dependencies{
		AuthStore:           authStore,
		AuthConfig:          authConfig,
		InitializationError: initErr,
	}
}

func mergeDependencies(base Dependencies, overrides ...Dependencies) Dependencies {
	if len(overrides) == 0 {
		return base
	}

	override := overrides[0]
	if override.PracticeService != nil {
		base.PracticeService = override.PracticeService
	}
	if override.RunnerClient != nil {
		base.RunnerClient = override.RunnerClient
	}
	if override.AuthStore != nil {
		base.AuthStore = override.AuthStore
		base.InitializationError = nil
	}
	if override.GitHubOAuthClient != nil {
		base.GitHubOAuthClient = override.GitHubOAuthClient
	}
	base.AuthConfig = mergeConfig(base.AuthConfig, override.AuthConfig)
	return base
}

func mergeConfig(base config.Config, override config.Config) config.Config {
	if override.MySQLDSN != "" {
		base.MySQLDSN = override.MySQLDSN
	}
	if override.GitHubClientID != "" {
		base.GitHubClientID = override.GitHubClientID
	}
	if override.GitHubSecret != "" {
		base.GitHubSecret = override.GitHubSecret
	}
	if override.SessionSecret != "" {
		base.SessionSecret = override.SessionSecret
	}
	if override.RunnerBaseURL != "" {
		base.RunnerBaseURL = override.RunnerBaseURL
	}
	if override.APIBaseURL != "" {
		base.APIBaseURL = override.APIBaseURL
	}
	if override.FrontendRedirectURL != "" {
		base.FrontendRedirectURL = override.FrontendRedirectURL
	}
	return base
}

func authCallbackURL(apiBaseURL string) string {
	return strings.TrimRight(apiBaseURL, "/") + "/api/v1/auth/github/callback"
}

func defaultAuthStore(mysqlDSN string) (service.UserStore, error) {
	defaultAuthStoreFactoryForTestsMu.RLock()
	factory := defaultAuthStoreFactoryForTests
	defaultAuthStoreFactoryForTestsMu.RUnlock()
	if factory != nil {
		return factory(), nil
	}

	if strings.TrimSpace(mysqlDSN) != "" {
		authStore, err := openMySQLStore(mysqlDSN)
		if err != nil {
			return nil, fmt.Errorf("auth initialization failed: %w", err)
		}
		return authStore, nil
	}
	return nil, nil
}

func SetDefaultAuthStoreFactoryForTests(factory func() service.UserStore) func() {
	defaultAuthStoreFactoryForTestsMu.Lock()
	previous := defaultAuthStoreFactoryForTests
	defaultAuthStoreFactoryForTests = factory
	defaultAuthStoreFactoryForTestsMu.Unlock()

	return func() {
		defaultAuthStoreFactoryForTestsMu.Lock()
		defaultAuthStoreFactoryForTests = previous
		defaultAuthStoreFactoryForTestsMu.Unlock()
	}
}

func SetOpenMySQLFuncForTests(openFunc func(string) (service.UserStore, error)) func() {
	openMySQLStoreForTestsMu.Lock()
	previous := openMySQLStoreForTests
	openMySQLStoreForTests = openFunc
	openMySQLStoreForTestsMu.Unlock()

	return func() {
		openMySQLStoreForTestsMu.Lock()
		openMySQLStoreForTests = previous
		openMySQLStoreForTestsMu.Unlock()
	}
}

func openMySQLStore(mysqlDSN string) (service.UserStore, error) {
	openMySQLStoreForTestsMu.RLock()
	openFunc := openMySQLStoreForTests
	openMySQLStoreForTestsMu.RUnlock()
	if openFunc != nil {
		return openFunc(mysqlDSN)
	}

	db, err := store.OpenMySQL(mysqlDSN)
	if err != nil {
		return nil, err
	}
	return store.NewMySQLStore(db), nil
}

func newInitializationErrorRouter(initErr error) http.Handler {
	r := chi.NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, initErr.Error(), http.StatusServiceUnavailable)
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/", handler)
	})
	return r
}
