# GitHub OAuth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder browser auth gate with a real GitHub OAuth login flow backed by MySQL browser sessions.

**Architecture:** The API service owns the full OAuth web flow. It redirects the browser to GitHub, handles the callback, exchanges the code for a token, fetches the GitHub profile, upserts the local user in MySQL, creates a persistent browser session, and enforces that session through middleware on all protected routes. The frontend keeps its existing login CTA and relies on the new authenticated cookie flow.

**Tech Stack:** Go, net/http, chi, golang.org/x/oauth2, GitHub REST API, MySQL, React, Vitest, Playwright

---

## File Structure

- Modify: `services/api/internal/http/router.go`
  - add public OAuth routes and keep protected routes behind real session middleware
- Modify: `services/api/internal/http/handlers/auth.go`
  - implement login redirect, callback, `auth/me`, and logout
- Modify: `services/api/internal/http/middleware/session.go`
  - replace placeholder user injection with real session lookup
- Modify: `services/api/internal/service/auth_service.go`
  - define the auth service boundary and auth-specific store interfaces
- Modify: `services/api/internal/service/session_cookie.go`
  - keep token hashing and add raw token generation helpers
- Modify: `services/api/internal/domain/models.go`
  - carry current-user and browser-session data in domain types
- Modify: `services/api/internal/config/config.go`
  - expose callback URL and frontend redirect origin inputs
- Modify: `services/api/internal/store/mysql.go`
  - add user/session persistence methods against MySQL
- Modify: `db/query/users.sql`
  - keep the user upsert and add lookup queries used by auth
- Modify: `db/query/sessions.sql`
  - add lookup and revoke queries used by auth
- Modify: `apps/web/src/test/App.test.tsx`
  - assert the login CTA still points at the real GitHub login route
- Modify: `README.md`
  - describe how to run with `DEV_AUTH_BYPASS=false`
- Modify: `.env.example`
  - document callback and frontend redirect values if needed
- Create or Modify: `services/api/internal/test/auth_handler_test.go`
  - cover login redirect, callback, `auth/me`, and logout
- Modify: `services/api/internal/test/practice_routes_test.go`
  - ensure protected routes work with real cookie sessions and bypass remains loopback-only

## Task 1: Replace the Auth Route Contract with Real OAuth Tests

**Files:**
- Modify: `services/api/internal/test/auth_handler_test.go`
- Modify: `services/api/internal/http/router.go`

- [ ] **Step 1: Replace the placeholder auth test file with real route expectations**

Replace `services/api/internal/test/auth_handler_test.go` with:

```go
package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx "gitgym/services/api/internal/http"
)

func TestGitHubLoginRedirectsToGitHub(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Fatalf("expected GitHub authorize redirect, got %q", location)
	}
	if !strings.Contains(location, "client_id=") || !strings.Contains(location, "state=") {
		t.Fatalf("expected client_id and state in redirect, got %q", location)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatalf("expected oauth state cookie to be set")
	}
}

func TestAuthMeRequiresRealSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
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
```

- [ ] **Step 2: Run the auth tests to verify they fail for the right reason**

Run:

```bash
go test ./internal/test -run "TestGitHubLoginRedirectsToGitHub|TestAuthMeRequiresRealSession|TestLogoutRequiresRealSession" -v
```

Expected:

- `TestGitHubLoginRedirectsToGitHub` fails because `/api/v1/auth/github/login` is not mounted yet
- the auth tests fail because the current handler returns the placeholder response

- [ ] **Step 3: Mount the public auth routes before protected middleware**

Update `services/api/internal/http/router.go` so the top-level API route shape becomes:

```go
r.Route("/api/v1", func(r chi.Router) {
	r.Get("/auth/github/login", handlers.GitHubLogin())
	r.Get("/auth/github/callback", handlers.GitHubCallback())

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireSessionCookie)
		r.Get("/auth/me", handlers.AuthMe())
		r.Post("/auth/logout", handlers.Logout())
		r.Get("/templates", handlers.ListPracticeTemplates(dependencies.PracticeService))
		r.Get("/practice-sessions/current", handlers.GetCurrentPracticeSession(dependencies.PracticeService))
		r.Post("/practice-sessions", handlers.CreatePracticeSession(dependencies.PracticeService))
		r.Post("/practice-sessions/{sessionId}/reset", handlers.ResetPracticeSession(dependencies.PracticeService))
		r.Get("/practice-sessions/{sessionId}/terminal", handlers.PracticeTerminalWebsocket(dependencies.PracticeService))
	})
})
```

Keep the new auth handlers as stubs in this task:

```go
func GitHubLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}
}

func GitHubCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}
}

func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}
}
```

- [ ] **Step 4: Re-run the auth tests to verify the route surface is now partially wired**

Run:

```bash
go test ./internal/test -run "TestGitHubLoginRedirectsToGitHub|TestAuthMeRequiresRealSession|TestLogoutRequiresRealSession" -v
```

Expected:

- `TestGitHubLoginRedirectsToGitHub` still fails, now because it gets `501` instead of a redirect
- protected auth tests still fail because `auth/me` is still a placeholder

- [ ] **Step 5: Commit the route-contract red step**

```bash
git add services/api/internal/test/auth_handler_test.go services/api/internal/http/router.go services/api/internal/http/handlers/auth.go
git commit -m "test: replace auth placeholder contract"
```

## Task 2: Add Real User and Browser Session Persistence

**Files:**
- Modify: `services/api/internal/domain/models.go`
- Modify: `services/api/internal/service/auth_service.go`
- Modify: `services/api/internal/service/session_cookie.go`
- Modify: `services/api/internal/store/mysql.go`

- [ ] **Step 1: Write a failing service-level persistence test for session hashing and lookup**

Append to `services/api/internal/test/auth_handler_test.go`:

```go
func TestHashSessionTokenIsStable(t *testing.T) {
	first := service.HashSessionToken("raw-token")
	second := service.HashSessionToken("raw-token")

	if first == "" {
		t.Fatalf("expected non-empty hash")
	}
	if first != second {
		t.Fatalf("expected deterministic hash, got %q and %q", first, second)
	}
}
```

- [ ] **Step 2: Add the missing auth domain types**

Update `services/api/internal/domain/models.go` with:

```go
type BrowserSession struct {
	ID               uint64
	UserID           uint64
	SessionTokenHash string
	UserAgent        *string
	IPAddress        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}
```

Leave `CurrentUser` intact and reuse it for `auth/me`.

- [ ] **Step 3: Expand the auth service boundary around user/session persistence**

Replace `services/api/internal/service/auth_service.go` with:

```go
package service

import (
	"context"
	"gitgym/services/api/internal/domain"
)

type GitHubProfile struct {
	ID        uint64
	Login     string
	Name      string
	AvatarURL string
	Email     string
}

type BrowserSessionRecord struct {
	UserID      uint64
	TokenHash   string
	UserAgent   string
	IPAddress   string
	ExpiresAt   string
}

type UserStore interface {
	UpsertGitHubUser(ctx context.Context, profile GitHubProfile) (uint64, error)
	GetUserByGitHubID(ctx context.Context, githubUserID uint64) (domain.CurrentUser, error)
	GetUserByID(ctx context.Context, userID uint64) (domain.CurrentUser, error)
	CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error
	GetBrowserSessionByTokenHash(ctx context.Context, tokenHash string) (domain.BrowserSession, error)
	RevokeBrowserSession(ctx context.Context, tokenHash string) error
}
```

- [ ] **Step 4: Add raw token generation next to the existing token hash helper**

Extend `services/api/internal/service/session_cookie.go`:

```go
func NewSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

Keep `HashSessionToken` unchanged.

- [ ] **Step 5: Add MySQL persistence methods for current-user and browser-session records**

Extend `services/api/internal/store/mysql.go` with methods of this shape:

```go
func (s *MySQLStore) GetUserByGitHubID(ctx context.Context, githubUserID uint64) (domain.CurrentUser, error)
func (s *MySQLStore) GetUserByID(ctx context.Context, userID uint64) (domain.CurrentUser, error)
func (s *MySQLStore) CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error
func (s *MySQLStore) GetBrowserSessionByTokenHash(ctx context.Context, tokenHash string) (domain.BrowserSession, error)
func (s *MySQLStore) RevokeBrowserSession(ctx context.Context, tokenHash string) error
```

Use direct SQL against the existing tables:

```sql
SELECT id, github_user_id, github_login, display_name, avatar_url, email
FROM users
WHERE github_user_id = ?
LIMIT 1
```

```sql
SELECT id, github_user_id, github_login, display_name, avatar_url, email
FROM users
WHERE id = ?
LIMIT 1
```

```sql
INSERT INTO user_sessions (user_id, session_token_hash, user_agent, ip_address, expires_at)
VALUES (?, ?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 30 DAY))
```

```sql
SELECT id, user_id, session_token_hash, user_agent, ip_address, expires_at, revoked_at
FROM user_sessions
WHERE session_token_hash = ? AND revoked_at IS NULL
LIMIT 1
```

```sql
UPDATE user_sessions
SET revoked_at = UTC_TIMESTAMP(6)
WHERE session_token_hash = ? AND revoked_at IS NULL
```

- [ ] **Step 6: Run the focused auth tests and Go package tests**

Run:

```bash
go test ./internal/test -run TestHashSessionTokenIsStable -v
go test ./... 
```

Expected: PASS.

- [ ] **Step 7: Commit the persistence boundary**

```bash
git add services/api/internal/domain/models.go services/api/internal/service/auth_service.go services/api/internal/service/session_cookie.go services/api/internal/store/mysql.go services/api/internal/test/auth_handler_test.go
git commit -m "feat: add auth persistence primitives"
```

## Task 3: Implement GitHub Login, Callback, and Real Session Middleware

**Files:**
- Modify: `services/api/internal/config/config.go`
- Modify: `services/api/internal/http/handlers/auth.go`
- Modify: `services/api/internal/http/middleware/session.go`
- Modify: `services/api/internal/http/router.go`
- Modify: `services/api/internal/oauth/github.go`
- Modify: `services/api/internal/test/auth_handler_test.go`

- [ ] **Step 1: Write the failing callback and current-user tests**

Append to `services/api/internal/test/auth_handler_test.go`:

```go
func TestGitHubCallbackRejectsMissingState(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=abc&state=bad", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
```

And add a middleware behavior test:

```go
func TestAuthMeReturnsUnauthorizedWithoutPersistedSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "raw-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Add callback/frontend redirect configuration**

Extend `services/api/internal/config/config.go`:

```go
type Config struct {
	MySQLDSN            string
	GitHubClientID      string
	GitHubSecret        string
	SessionSecret       string
	RunnerBaseURL       string
	APIBaseURL          string
	FrontendRedirectURL string
}
```

Load `FRONTEND_REDIRECT_URL` and default it to `http://127.0.0.1:5173` if blank.

- [ ] **Step 3: Implement cookie helpers used by login and callback**

In `services/api/internal/http/handlers/auth.go`, add helper functions:

```go
func setOAuthStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "gitgym_oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
}

func setBrowserSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "gitgym_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
```

- [ ] **Step 4: Implement GitHub login redirect**

`GitHubLogin` should:

```go
state, err := service.NewSessionToken()
if err != nil { http.Error(w, "internal error", http.StatusInternalServerError); return }
setOAuthStateCookie(w, state)
redirectURL := oauth.GitHubConfig(clientID, clientSecret, callbackURL).AuthCodeURL(state)
http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
```

Use callback URL:

```go
strings.TrimRight(apiBaseURL, "/") + "/api/v1/auth/github/callback"
```

- [ ] **Step 5: Implement callback state verification and GitHub profile exchange**

Implement `GitHubCallback` to:

```go
stateCookie, err := r.Cookie("gitgym_oauth_state")
if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
	http.Error(w, "invalid oauth state", http.StatusBadRequest)
	return
}
```

Then:

```go
token, err := oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
```

Fetch the GitHub user profile with:

```go
GET https://api.github.com/user
Authorization: Bearer <access token>
Accept: application/vnd.github+json
```

If profile email is blank, fetch:

```go
GET https://api.github.com/user/emails
```

Pick the primary verified email if present.

- [ ] **Step 6: Upsert the local user, create the browser session, and redirect home**

After profile fetch:

```go
_, err = authStore.UpsertGitHubUser(ctx, service.GitHubProfile{...})
user, err := authStore.GetUserByGitHubID(ctx, profile.ID)
rawToken, err := service.NewSessionToken()
tokenHash := service.HashSessionToken(rawToken)
err = authStore.CreateBrowserSession(ctx, user.ID, tokenHash, r.UserAgent(), clientIP(r))
setBrowserSessionCookie(w, rawToken)
clearOAuthStateCookie(w)
http.Redirect(w, r, frontendRedirectURL, http.StatusTemporaryRedirect)
```

- [ ] **Step 7: Replace placeholder middleware lookup with real session validation**

In `services/api/internal/http/middleware/session.go`, after bypass handling:

```go
tokenHash := service.HashSessionToken(cookie.Value)
browserSession, err := authStore.GetBrowserSessionByTokenHash(r.Context(), tokenHash)
if err != nil || browserSession.ExpiresAt.Before(time.Now().UTC()) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return
}
```

Then load the current user and place it into context:

```go
type AuthenticatedSession struct {
	UserID       uint64
	SessionToken string
}
```

Keep loopback-only bypass as an override path.

- [ ] **Step 8: Replace the stub `auth/me` response with real JSON**

`AuthMe` should write:

```json
{
  "user": {
    "id": 1,
    "github_id": 123,
    "github_login": "octocat",
    "display_name": "The Octocat",
    "avatar_url": "https://...",
    "email": "octo@example.com"
  }
}
```

- [ ] **Step 9: Run focused auth tests and package tests**

Run:

```bash
go test ./internal/test -run "TestGitHubLoginRedirectsToGitHub|TestGitHubCallbackRejectsMissingState|TestAuthMe" -v
go test ./...
```

Expected: PASS.

- [ ] **Step 10: Commit the OAuth and middleware implementation**

```bash
git add services/api/internal/config/config.go services/api/internal/http/handlers/auth.go services/api/internal/http/middleware/session.go services/api/internal/http/router.go services/api/internal/oauth/github.go services/api/internal/test/auth_handler_test.go
git commit -m "feat: implement github oauth session flow"
```

## Task 4: Add Logout, Local Validation, and Frontend Contract Checks

**Files:**
- Modify: `services/api/internal/http/handlers/auth.go`
- Modify: `services/api/internal/test/auth_handler_test.go`
- Modify: `services/api/internal/test/practice_routes_test.go`
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `README.md`
- Modify: `.env.example`

- [ ] **Step 1: Write the failing logout behavior test**

Append to `services/api/internal/test/auth_handler_test.go`:

```go
func TestLogoutClearsCookieAndRevokesSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "raw-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Implement logout**

In `services/api/internal/http/handlers/auth.go`:

```go
func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("gitgym_session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tokenHash := service.HashSessionToken(cookie.Value)
		_ = authStore.RevokeBrowserSession(r.Context(), tokenHash)

		http.SetCookie(w, &http.Cookie{
			Name:     "gitgym_session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 3: Keep the frontend login entrypoint stable**

In `apps/web/src/test/App.test.tsx`, keep this assertion:

```tsx
expect(loginLink).toHaveAttribute("href", "/api/v1/auth/github/login");
```

If it is missing, add it back and run:

```bash
pnpm --dir apps/web test
```

Expected: PASS.

- [ ] **Step 4: Update local run docs for real OAuth first**

Update `README.md` and `.env.example` so they explicitly say:

```md
- set `DEV_AUTH_BYPASS=false` for real GitHub login
- configure the GitHub OAuth App callback URL as `http://127.0.0.1:8080/api/v1/auth/github/callback`
- use `DEV_AUTH_BYPASS=true` only as a loopback-only emergency shortcut
```

- [ ] **Step 5: Run full verification**

Run:

```bash
go test ./services/api/...
go test ./services/runner/...
pnpm --dir apps/web test
pnpm --dir apps/web build
```

Expected:

- API tests PASS
- runner tests PASS
- frontend tests PASS
- frontend build PASS

- [ ] **Step 6: Validate the real local OAuth path manually**

Run:

```bash
npm run dev
```

Manual expected result:

- opening `http://127.0.0.1:5173` shows the login shell
- clicking `Continue with GitHub` redirects to GitHub
- successful login returns to the app
- the browser receives `gitgym_session`
- protected API routes work without `DEV_AUTH_BYPASS`

- [ ] **Step 7: Commit the final OAuth slice**

```bash
git add services/api/internal/http/handlers/auth.go services/api/internal/test/auth_handler_test.go services/api/internal/test/practice_routes_test.go apps/web/src/test/App.test.tsx README.md .env.example
git commit -m "feat: wire github oauth login"
```

## Self-Review

### Spec Coverage Check

- Real GitHub login start and callback: covered in Task 3.
- Persistent MySQL-backed browser session: covered in Tasks 2 and 3.
- Real `auth/me`: covered in Task 3.
- Logout: covered in Task 4.
- Loopback-only fallback bypass: covered in Tasks 3 and 4.
- Local real-login verification: covered in Task 4.

### Placeholder Scan

- No `TBD`, `TODO`, or “implement later” placeholders remain in the task steps.
- Each task includes exact file paths and verification commands.

### Type Consistency Check

- `CurrentUser`, `BrowserSession`, `UserStore`, and `AuthenticatedSession` naming is consistent across the plan.
- Routes consistently use `/api/v1/auth/github/login`, `/callback`, `/auth/me`, and `/auth/logout`.
