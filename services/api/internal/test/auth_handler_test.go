package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/config"
	"gitgym/services/api/internal/domain"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
	mysql "github.com/go-sql-driver/mysql"
)

func TestMain(m *testing.M) {
	restore := httpx.SetDefaultAuthStoreFactoryForTests(newDefaultTestAuthStore)
	code := m.Run()
	restore()
	os.Exit(code)
}

func TestGitHubLoginRedirectsToGitHub(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthConfig: config.Config{
			GitHubClientID:      "client-id",
			GitHubSecret:        "client-secret",
			APIBaseURL:          "http://127.0.0.1:8080",
			FrontendRedirectURL: "http://127.0.0.1:5173",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Fatalf("expected GitHub authorize redirect, got %q", location)
	}

	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	if redirectURL.Query().Get("client_id") != "client-id" {
		t.Fatalf("expected client_id=client-id, got %q", redirectURL.Query().Get("client_id"))
	}
	if redirectURL.Query().Get("redirect_uri") != "http://127.0.0.1:8080/api/v1/auth/github/callback" {
		t.Fatalf("unexpected redirect_uri %q", redirectURL.Query().Get("redirect_uri"))
	}
	if redirectURL.Query().Get("state") == "" {
		t.Fatalf("expected state query parameter in redirect %q", location)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected oauth state cookie to be set")
	}

	var stateCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "gitgym_oauth_state" {
			stateCookie = cookie
			break
		}
	}
	if stateCookie == nil {
		t.Fatalf("expected gitgym_oauth_state cookie, got %#v", cookies)
	}
	if stateCookie.Value == "" {
		t.Fatalf("expected oauth state cookie to have a value")
	}
	if stateCookie.Value != redirectURL.Query().Get("state") {
		t.Fatalf("expected redirect state to match cookie, got cookie=%q redirect=%q", stateCookie.Value, redirectURL.Query().Get("state"))
	}
}

func TestDefaultRouterUsesTestAuthStoreBeforeMySQLOpen(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pass@tcp(127.0.0.1:65000)/gitgym")

	openMySQLCalls := 0
	restoreOpenMySQL := httpx.SetOpenMySQLFuncForTests(func(string) (service.UserStore, error) {
		openMySQLCalls++
		return nil, errors.New("unexpected mysql open")
	})
	t.Cleanup(restoreOpenMySQL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if openMySQLCalls != 0 {
		t.Fatalf("expected test auth store seam to win before mysql open, got %d mysql open attempts", openMySQLCalls)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected default test auth store to serve auth/me, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestDefaultRouterReturnsServiceUnavailableWhenMySQLAuthInitFails(t *testing.T) {
	restoreFactory := httpx.SetDefaultAuthStoreFactoryForTests(nil)
	t.Cleanup(restoreFactory)
	t.Setenv("MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/gitgym")

	restoreOpenMySQL := httpx.SetOpenMySQLFuncForTests(func(string) (service.UserStore, error) {
		return nil, errors.New("mysql unavailable")
	})
	t.Cleanup(restoreOpenMySQL)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auth initialization failed") {
		t.Fatalf("expected init failure body, got %q", rec.Body.String())
	}
}

func TestGitHubLoginReturnsServerErrorWhenOAuthConfigIncomplete(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthConfig: config.Config{
			APIBaseURL:          "http://127.0.0.1:8080",
			FrontendRedirectURL: "http://127.0.0.1:5173",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected clear configuration error, got %q", rec.Body.String())
	}
}

func TestLoadRuntimeDefaultsFrontendRedirectForLoopbackAPI(t *testing.T) {
	t.Setenv("API_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("FRONTEND_REDIRECT_URL", "")

	cfg := config.LoadRuntime()

	if cfg.FrontendRedirectURL != "http://127.0.0.1:5173" {
		t.Fatalf("expected local frontend redirect default, got %q", cfg.FrontendRedirectURL)
	}
}

func TestLoadRuntimeDoesNotDefaultFrontendRedirectForNonLocalAPI(t *testing.T) {
	t.Setenv("API_BASE_URL", "https://api.gitgym.example")
	t.Setenv("FRONTEND_REDIRECT_URL", "")

	cfg := config.LoadRuntime()

	if cfg.FrontendRedirectURL != "" {
		t.Fatalf("expected blank frontend redirect for non-local api, got %q", cfg.FrontendRedirectURL)
	}
}

func TestGitHubCallbackRejectsMissingState(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=abc&state=bad", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthConfig: config.Config{
			GitHubClientID:      "client-id",
			GitHubSecret:        "client-secret",
			APIBaseURL:          "http://127.0.0.1:8080",
			FrontendRedirectURL: "http://127.0.0.1:5173",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGitHubCallbackReturnsServerErrorWhenOAuthConfigIncomplete(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=abc123&state=expected-state", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_oauth_state", Value: "expected-state"})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{},
		AuthConfig: config.Config{
			APIBaseURL:          "http://127.0.0.1:8080",
			FrontendRedirectURL: "http://127.0.0.1:5173",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected clear configuration error, got %q", rec.Body.String())
	}
}

func TestGitHubCallbackCreatesSessionAndRedirectsToFrontend(t *testing.T) {
	authStore := &stubUserStore{
		userByGitHubID: map[uint64]domain.CurrentUser{
			123: {
				ID:          7,
				GitHubID:    123,
				GitHubLogin: "octocat",
				DisplayName: "The Octocat",
				AvatarURL:   stringPtr("https://avatars.example/octocat.png"),
				Email:       stringPtr("octo@example.com"),
			},
		},
		userByID: map[uint64]domain.CurrentUser{
			7: {
				ID:          7,
				GitHubID:    123,
				GitHubLogin: "octocat",
				DisplayName: "The Octocat",
				AvatarURL:   stringPtr("https://avatars.example/octocat.png"),
				Email:       stringPtr("octo@example.com"),
			},
		},
	}
	oauthClient := &stubGitHubOAuthClient{
		exchangeCodeFunc: func(code string) (string, error) {
			if code != "abc123" {
				t.Fatalf("expected callback code abc123, got %q", code)
			}
			return "access-token", nil
		},
		fetchProfileFunc: func(accessToken string) (service.GitHubProfile, error) {
			if accessToken != "access-token" {
				t.Fatalf("expected access token access-token, got %q", accessToken)
			}
			return service.GitHubProfile{
				ID:        123,
				Login:     "octocat",
				Name:      "The Octocat",
				AvatarURL: "https://avatars.example/octocat.png",
				Email:     "octo@example.com",
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=abc123&state=expected-state", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_oauth_state", Value: "expected-state"})
	req.Header.Set("User-Agent", "GitGym Test")
	req.RemoteAddr = "127.0.0.1:45678"
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore:         authStore,
		GitHubOAuthClient: oauthClient,
		AuthConfig: config.Config{
			GitHubClientID:      "client-id",
			GitHubSecret:        "client-secret",
			APIBaseURL:          "http://127.0.0.1:8080",
			FrontendRedirectURL: "http://127.0.0.1:5173/app",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d with body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "http://127.0.0.1:5173/app" {
		t.Fatalf("expected frontend redirect, got %q", rec.Header().Get("Location"))
	}
	if authStore.upsertedProfile == nil {
		t.Fatalf("expected GitHub profile to be upserted")
	}
	if authStore.createdSession == nil {
		t.Fatalf("expected browser session to be created")
	}
	if authStore.createdSession.UserID != 7 {
		t.Fatalf("expected browser session user ID 7, got %d", authStore.createdSession.UserID)
	}
	if authStore.createdSession.UserAgent != "GitGym Test" {
		t.Fatalf("expected user agent to be recorded, got %q", authStore.createdSession.UserAgent)
	}
	if authStore.createdSession.IPAddress != "127.0.0.1" {
		t.Fatalf("expected loopback IP to be recorded, got %q", authStore.createdSession.IPAddress)
	}
	if authStore.createdSession.TokenHash == "" {
		t.Fatalf("expected browser session token hash to be stored")
	}

	var (
		sessionCookie      *http.Cookie
		clearedStateCookie *http.Cookie
	)
	for _, cookie := range rec.Result().Cookies() {
		switch cookie.Name {
		case "gitgym_session":
			sessionCookie = cookie
		case "gitgym_oauth_state":
			clearedStateCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected gitgym_session cookie, got %#v", rec.Result().Cookies())
	}
	if service.HashSessionToken(sessionCookie.Value) != authStore.createdSession.TokenHash {
		t.Fatalf("expected session cookie to match stored hash")
	}
	if clearedStateCookie == nil || clearedStateCookie.MaxAge >= 0 {
		t.Fatalf("expected oauth state cookie to be cleared, got %#v", clearedStateCookie)
	}
}

func TestAuthMeReturnsUnauthorizedWithoutPersistedSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "raw-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMeReturnsServerErrorWhenSessionStoreLookupFails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "raw-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{
			getBrowserSessionErr: errors.New("mysql read timeout"),
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auth store") {
		t.Fatalf("expected auth store failure body, got %q", rec.Body.String())
	}
}

func TestAuthMeDefaultRouterReturnsServerErrorWithoutAuthBacking(t *testing.T) {
	restore := httpx.SetDefaultAuthStoreFactoryForTests(nil)
	t.Cleanup(restore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected clear configuration error, got %q", rec.Body.String())
	}
}

func TestAuthMeReturnsUserJSONForPersistedSession(t *testing.T) {
	rawToken := "persisted-session-token"
	user := domain.CurrentUser{
		ID:          7,
		GitHubID:    123,
		GitHubLogin: "octocat",
		DisplayName: "The Octocat",
		AvatarURL:   stringPtr("https://avatars.example/octocat.png"),
		Email:       stringPtr("octo@example.com"),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: rawToken})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{
			userByID: map[uint64]domain.CurrentUser{
				user.ID: user,
			},
			sessionByTokenHash: map[string]domain.BrowserSession{
				service.HashSessionToken(rawToken): {
					ID:               9,
					UserID:           user.ID,
					SessionTokenHash: service.HashSessionToken(rawToken),
					ExpiresAt:        time.Now().Add(30 * time.Minute).UTC(),
				},
			},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		User struct {
			ID          uint64  `json:"id"`
			GitHubID    uint64  `json:"github_id"`
			GitHubLogin string  `json:"github_login"`
			DisplayName string  `json:"display_name"`
			AvatarURL   *string `json:"avatar_url"`
			Email       *string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal auth/me response: %v", err)
	}
	if payload.User.ID != user.ID || payload.User.GitHubID != user.GitHubID {
		t.Fatalf("unexpected auth/me payload: %+v", payload.User)
	}
	if payload.User.GitHubLogin != "octocat" || payload.User.DisplayName != "The Octocat" {
		t.Fatalf("unexpected auth/me identity payload: %+v", payload.User)
	}
}

func TestAuthMeReturnsServerErrorWhenAuthStoreUnavailableAfterBypass(t *testing.T) {
	restore := httpx.SetDefaultAuthStoreFactoryForTests(nil)
	t.Cleanup(restore)
	t.Setenv("DEV_AUTH_BYPASS", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.RemoteAddr = "127.0.0.1:45678"
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected clear configuration error, got %q", rec.Body.String())
	}
}

func TestAuthMeReturnsServerErrorWhenAuthStoreUnavailableWithSessionCookie(t *testing.T) {
	restore := httpx.SetDefaultAuthStoreFactoryForTests(nil)
	t.Cleanup(restore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "persisted-session-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected clear configuration error, got %q", rec.Body.String())
	}
}

func TestLogoutRequiresRealSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutClearsStaleCookieWithoutPersistedSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "stale-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", rec.Code, rec.Body.String())
	}

	var clearedCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "gitgym_session" {
			clearedCookie = cookie
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected stale session cookie to be cleared, got %#v", clearedCookie)
	}
}

func TestLogoutReturnsServerErrorWhenAuthStoreUnavailableButClearsCookie(t *testing.T) {
	restore := httpx.SetDefaultAuthStoreFactoryForTests(nil)
	t.Cleanup(restore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "stale-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var clearedCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "gitgym_session" {
			clearedCookie = cookie
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected session cookie to be cleared, got %#v", clearedCookie)
	}
}

func TestLogoutReturnsServerErrorWhenRevokeFailsButClearsCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "stale-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter(httpx.Dependencies{
		AuthStore: &stubUserStore{
			revokeErr: errors.New("mysql write timeout"),
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var clearedCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "gitgym_session" {
			clearedCookie = cookie
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected session cookie to be cleared, got %#v", clearedCookie)
	}
}

func TestNewSessionTokenReturnsHexAndStableHash(t *testing.T) {
	firstToken, err := service.NewSessionToken()
	if err != nil {
		t.Fatalf("expected token, got error: %v", err)
	}
	secondToken, err := service.NewSessionToken()
	if err != nil {
		t.Fatalf("expected token, got error: %v", err)
	}

	if len(firstToken) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(firstToken))
	}
	if len(secondToken) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(secondToken))
	}
	if firstToken == secondToken {
		t.Fatalf("expected unique tokens, got %q", firstToken)
	}

	firstHash := service.HashSessionToken(firstToken)
	secondHash := service.HashSessionToken(firstToken)
	if firstHash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if firstHash != secondHash {
		t.Fatalf("expected deterministic hash, got %q and %q", firstHash, secondHash)
	}
}

func TestBrowserSessionLookupQueryRequiresUnrevokedAndUnexpiredSession(t *testing.T) {
	query := store.BrowserSessionLookupQuery()

	if !strings.Contains(query, "revoked_at IS NULL") {
		t.Fatalf("expected revoked guard in query, got %q", query)
	}
	if !strings.Contains(query, "expires_at > UTC_TIMESTAMP(6)") {
		t.Fatalf("expected expiry guard in query, got %q", query)
	}
}

func TestBrowserSessionLookupErrorMapsNoRowsToStableNotFound(t *testing.T) {
	err := store.MapBrowserSessionLookupError(sql.ErrNoRows)

	if !errors.Is(err, service.ErrBrowserSessionNotFound) {
		t.Fatalf("expected browser session not found, got %v", err)
	}
}

func TestNormalizeMySQLDSNEnablesParseTimeAndUTC(t *testing.T) {
	normalized, err := store.NormalizeMySQLDSN("user:pass@tcp(localhost:3306)/gitgym")
	if err != nil {
		t.Fatalf("expected normalized dsn, got error: %v", err)
	}

	cfg, err := mysql.ParseDSN(normalized)
	if err != nil {
		t.Fatalf("expected parseable dsn, got error: %v", err)
	}
	if !cfg.ParseTime {
		t.Fatalf("expected ParseTime=true in %q", normalized)
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("expected UTC location, got %v", cfg.Loc)
	}
}

type stubUserStore struct {
	upsertedProfile      *service.GitHubProfile
	userByGitHubID       map[uint64]domain.CurrentUser
	userByID             map[uint64]domain.CurrentUser
	sessionByTokenHash   map[string]domain.BrowserSession
	createdSession       *service.BrowserSessionRecord
	revokedTokenHash     string
	getBrowserSessionErr error
	revokeErr            error
}

func (s *stubUserStore) UpsertGitHubUser(_ context.Context, profile service.GitHubProfile) (uint64, error) {
	s.upsertedProfile = &profile
	if user, ok := s.userByGitHubID[profile.ID]; ok {
		return user.ID, nil
	}
	return 0, nil
}

func (s *stubUserStore) GetUserByGitHubID(_ context.Context, githubUserID uint64) (domain.CurrentUser, error) {
	user, ok := s.userByGitHubID[githubUserID]
	if !ok {
		return domain.CurrentUser{}, sql.ErrNoRows
	}
	return user, nil
}

func (s *stubUserStore) GetUserByID(_ context.Context, userID uint64) (domain.CurrentUser, error) {
	user, ok := s.userByID[userID]
	if !ok {
		return domain.CurrentUser{}, sql.ErrNoRows
	}
	return user, nil
}

func (s *stubUserStore) CreateBrowserSession(_ context.Context, userID uint64, tokenHash string, userAgent string, ip string) error {
	s.createdSession = &service.BrowserSessionRecord{
		UserID:    userID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IPAddress: ip,
	}
	return nil
}

func (s *stubUserStore) GetBrowserSessionByTokenHash(_ context.Context, tokenHash string) (domain.BrowserSession, error) {
	if s.getBrowserSessionErr != nil {
		return domain.BrowserSession{}, s.getBrowserSessionErr
	}
	session, ok := s.sessionByTokenHash[tokenHash]
	if !ok {
		return domain.BrowserSession{}, service.ErrBrowserSessionNotFound
	}
	return session, nil
}

func (s *stubUserStore) RevokeBrowserSession(_ context.Context, tokenHash string) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revokedTokenHash = tokenHash
	return nil
}

type stubGitHubOAuthClient struct {
	exchangeCodeFunc func(code string) (string, error)
	fetchProfileFunc func(accessToken string) (service.GitHubProfile, error)
}

func (s *stubGitHubOAuthClient) AuthCodeURL(state string) string {
	values := url.Values{}
	values.Set("client_id", "stub-client")
	values.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + values.Encode()
}

func (s *stubGitHubOAuthClient) ExchangeCode(_ context.Context, code string) (string, error) {
	if s.exchangeCodeFunc == nil {
		return "", nil
	}
	return s.exchangeCodeFunc(code)
}

func (s *stubGitHubOAuthClient) FetchProfile(_ context.Context, accessToken string) (service.GitHubProfile, error) {
	if s.fetchProfileFunc == nil {
		return service.GitHubProfile{}, nil
	}
	return s.fetchProfileFunc(accessToken)
}

func stringPtr(value string) *string {
	return &value
}

func newDefaultTestAuthStore() service.UserStore {
	return &stubUserStore{
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
	}
}
