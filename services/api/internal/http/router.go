package httpx

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"gitgym/services/api/internal/config"
	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/oauth"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	PracticeService   service.PracticeService
	AuthStore         service.UserStore
	AuthConfig        config.Config
	GitHubOAuthClient oauth.GitHubOAuthClient
}

func NewRouter(deps ...Dependencies) http.Handler {
	dependencies := mergeDependencies(defaultDependencies(), deps...)
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
		r.Get("/auth/github/login", handlers.GitHubLogin(dependencies.GitHubOAuthClient))
		r.Get("/auth/github/callback", handlers.GitHubCallback(dependencies.GitHubOAuthClient, dependencies.AuthStore, dependencies.AuthConfig.FrontendRedirectURL))

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireSessionCookie(dependencies.AuthStore))
			r.Get("/auth/me", handlers.AuthMe(dependencies.AuthStore))
			r.Post("/auth/logout", handlers.Logout(dependencies.AuthStore))
			r.Get("/templates", handlers.ListPracticeTemplates(dependencies.PracticeService))
			r.Get("/practice-sessions/current", handlers.GetCurrentPracticeSession(dependencies.PracticeService))
			r.Post("/practice-sessions", handlers.CreatePracticeSession(dependencies.PracticeService))
			r.Post("/practice-sessions/{sessionId}/reset", handlers.ResetPracticeSession(dependencies.PracticeService))
			r.Get("/practice-sessions/{sessionId}/terminal", handlers.PracticeTerminalWebsocket(dependencies.PracticeService))
		})
	})
	return r
}

func defaultDependencies() Dependencies {
	authConfig := config.LoadRuntime()

	return Dependencies{
		PracticeService: service.NewPracticeService(
			service.NewInMemoryPracticeSessionStore(),
			runner.NewClient(os.Getenv("RUNNER_BASE_URL"), http.DefaultClient),
			time.Now,
		),
		AuthStore:  defaultAuthStore(authConfig.MySQLDSN),
		AuthConfig: authConfig,
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
	if override.AuthStore != nil {
		base.AuthStore = override.AuthStore
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

func defaultAuthStore(mysqlDSN string) service.UserStore {
	if strings.TrimSpace(mysqlDSN) != "" {
		if db, err := store.OpenMySQL(mysqlDSN); err == nil {
			return store.NewMySQLStore(db)
		}
	}

	return &seededAuthStore{
		userByGitHubID: map[uint64]domain.CurrentUser{
			1: {
				ID:          1,
				GitHubID:    1,
				GitHubLogin: "dev-user",
				DisplayName: "Dev User",
			},
		},
		userByID: map[uint64]domain.CurrentUser{
			1: {
				ID:          1,
				GitHubID:    1,
				GitHubLogin: "dev-user",
				DisplayName: "Dev User",
			},
		},
		sessionByTokenHash: map[string]domain.BrowserSession{
			service.HashSessionToken("session-token"): {
				ID:               1,
				UserID:           1,
				SessionTokenHash: service.HashSessionToken("session-token"),
				ExpiresAt:        time.Now().Add(24 * time.Hour).UTC(),
			},
			service.HashSessionToken("uid:42:session-token"): {
				ID:               2,
				UserID:           1,
				SessionTokenHash: service.HashSessionToken("uid:42:session-token"),
				ExpiresAt:        time.Now().Add(24 * time.Hour).UTC(),
			},
			service.HashSessionToken("123"): {
				ID:               3,
				UserID:           1,
				SessionTokenHash: service.HashSessionToken("123"),
				ExpiresAt:        time.Now().Add(24 * time.Hour).UTC(),
			},
		},
		nextUserID:    2,
		nextSessionID: 4,
	}
}

type seededAuthStore struct {
	userByGitHubID     map[uint64]domain.CurrentUser
	userByID           map[uint64]domain.CurrentUser
	sessionByTokenHash map[string]domain.BrowserSession
	nextUserID         uint64
	nextSessionID      uint64
}

func (s *seededAuthStore) UpsertGitHubUser(_ context.Context, profile service.GitHubProfile) (uint64, error) {
	if existing, ok := s.userByGitHubID[profile.ID]; ok {
		updated := existing
		updated.GitHubLogin = profile.Login
		updated.DisplayName = profile.Name
		updated.AvatarURL = stringPointer(profile.AvatarURL)
		updated.Email = stringPointer(profile.Email)
		s.userByGitHubID[profile.ID] = updated
		s.userByID[updated.ID] = updated
		return updated.ID, nil
	}

	user := domain.CurrentUser{
		ID:          s.nextUserID,
		GitHubID:    profile.ID,
		GitHubLogin: profile.Login,
		DisplayName: profile.Name,
		AvatarURL:   stringPointer(profile.AvatarURL),
		Email:       stringPointer(profile.Email),
	}
	s.userByGitHubID[profile.ID] = user
	s.userByID[user.ID] = user
	s.nextUserID++
	return user.ID, nil
}

func (s *seededAuthStore) GetUserByGitHubID(_ context.Context, githubUserID uint64) (domain.CurrentUser, error) {
	user, ok := s.userByGitHubID[githubUserID]
	if !ok {
		return domain.CurrentUser{}, service.ErrBrowserSessionNotFound
	}
	return user, nil
}

func (s *seededAuthStore) GetUserByID(_ context.Context, userID uint64) (domain.CurrentUser, error) {
	user, ok := s.userByID[userID]
	if !ok {
		return domain.CurrentUser{}, service.ErrBrowserSessionNotFound
	}
	return user, nil
}

func (s *seededAuthStore) CreateBrowserSession(_ context.Context, userID uint64, tokenHash string, userAgent string, ip string) error {
	s.sessionByTokenHash[tokenHash] = domain.BrowserSession{
		ID:               s.nextSessionID,
		UserID:           userID,
		SessionTokenHash: tokenHash,
		UserAgent:        stringPointer(userAgent),
		IPAddress:        stringPointer(ip),
		ExpiresAt:        time.Now().Add(30 * 24 * time.Hour).UTC(),
	}
	s.nextSessionID++
	return nil
}

func (s *seededAuthStore) GetBrowserSessionByTokenHash(_ context.Context, tokenHash string) (domain.BrowserSession, error) {
	session, ok := s.sessionByTokenHash[tokenHash]
	if !ok {
		return domain.BrowserSession{}, service.ErrBrowserSessionNotFound
	}
	return session, nil
}

func (s *seededAuthStore) RevokeBrowserSession(_ context.Context, tokenHash string) error {
	delete(s.sessionByTokenHash, tokenHash)
	return nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
