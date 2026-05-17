# GitGym GitHub OAuth Design

## Goal

Add a real GitHub OAuth web login flow to GitGym so local development and the future hosted product use persistent browser sessions backed by MySQL instead of the current placeholder cookie gate.

This design explicitly makes real GitHub login the default local behavior. `DEV_AUTH_BYPASS` remains available only as a loopback-only development escape hatch.

## Scope

This design covers:

- GitHub OAuth login start and callback endpoints
- persistent browser session creation and lookup
- authenticated current-user response
- logout
- local development behavior and configuration

This design does not cover:

- organization SSO requirements
- role systems or multi-user admin permissions
- refresh of GitHub access tokens for later GitHub API use
- frontend redesign beyond wiring the existing login entrypoint

## Current State

- the frontend login CTA already points at `/api/v1/auth/github/login`
- `services/api/internal/oauth/github.go` already defines the base OAuth client config
- `users` and `user_sessions` tables already exist in MySQL
- the API still uses a placeholder cookie middleware and a stub `auth/me` handler
- local development currently works through a `DEV_AUTH_BYPASS` shortcut

## Recommended Approach

Use a standard OAuth 2.0 authorization code flow with server-side session persistence in MySQL.

Why this approach:

- it matches the current GitGym architecture where the API is the browser-facing control plane
- it gives stable browser sessions across API restarts
- it reuses the existing `users` and `user_sessions` schema
- it keeps GitHub access tokens out of browser storage

## Route Surface

Add or replace these API routes:

- `GET /api/v1/auth/github/login`
  - generates OAuth state
  - redirects the browser to GitHub authorization
- `GET /api/v1/auth/github/callback`
  - verifies state
  - exchanges `code` for an access token
  - fetches the GitHub user profile
  - upserts the local user
  - creates a browser session
  - sets the `gitgym_session` cookie
  - redirects back to the frontend app
- `GET /api/v1/auth/me`
  - returns the authenticated local user as JSON
- `POST /api/v1/auth/logout`
  - revokes the current browser session
  - clears the cookie
  - returns `204 No Content`

Protected routes remain behind session middleware:

- `GET /api/v1/templates`
- `GET /api/v1/practice-sessions/current`
- `POST /api/v1/practice-sessions`
- `POST /api/v1/practice-sessions/{sessionId}/reset`
- `GET /api/v1/practice-sessions/{sessionId}/terminal`

## OAuth Flow

### Login Start

1. Browser requests `GET /api/v1/auth/github/login`
2. API generates a random OAuth `state` token
3. API stores the state in a short-lived cookie scoped to the callback flow
4. API redirects to GitHub authorization using:
   - `client_id`
   - `redirect_uri`
   - `scope=read:user user:email`
   - `state`

### Callback

1. GitHub redirects to `/api/v1/auth/github/callback?code=...&state=...`
2. API verifies that returned `state` matches the stored cookie
3. API exchanges `code` for an access token
4. API fetches the GitHub user profile
5. API fetches primary email if the profile email is absent
6. API upserts the local `users` row
7. API creates a new `user_sessions` row
8. API sets the persistent `gitgym_session` cookie
9. API clears the temporary OAuth state cookie
10. API redirects to `API_BASE_URL` or the browser app origin configured for local development

### Failure Handling

If any OAuth step fails:

- clear the temporary OAuth state cookie
- do not create a browser session
- return a clear `4xx` or `5xx` response for direct API debugging
- optionally redirect to the frontend login shell with an error query string in later UX work, but that redirect UX is not required for the first implementation

## Session Model

Use a random opaque browser session token stored in a cookie and hashed before persistence.

### Session Creation

1. Generate a cryptographically random session token
2. Hash it with the existing `HashSessionToken`
3. Insert a `user_sessions` record with:
   - `user_id`
   - `session_token_hash`
   - `user_agent`
   - `ip_address`
   - `expires_at`
4. Set `gitgym_session=<raw token>` in an `HttpOnly` cookie

### Session Lookup

For every protected request:

1. read the `gitgym_session` cookie
2. hash the raw token
3. look up the session by `session_token_hash`
4. reject if missing, revoked, or expired
5. load the corresponding local user
6. place the authenticated user and browser session in request context

### Session Revocation

Logout revokes the current row in `user_sessions` and expires the browser cookie.

## Data Layer Changes

Add store support for:

- `GetUserByGitHubID`
- `GetUserByID`
- `CreateBrowserSession`
- `GetUserSessionByTokenHash`
- `RevokeUserSession`

Upsert behavior:

- `UpsertGitHubUser` remains the entrypoint for login persistence
- after the upsert, lookup the local user row by GitHub ID to obtain the stable local user ID

## Middleware Changes

Replace the current placeholder session acceptance logic with real session lookup.

Behavior:

- default path uses the persisted browser session
- `DEV_AUTH_BYPASS=true` only works when the request originates from loopback
- if bypass is off and there is no valid cookie session, return `401`

This preserves local emergency access without turning bypass into a broad authentication hole.

## Handler Responsibilities

### `auth` handlers

- own login redirect and callback handling
- own logout
- own current-user response
- depend on:
  - GitHub OAuth config
  - a user/session store
  - cookie helpers

### middleware

- own cookie parsing and request authentication
- depend on:
  - session lookup store
  - token hashing

## Cookie Design

Use these cookies:

- `gitgym_session`
  - stores the raw opaque browser session token
  - `HttpOnly`
  - `SameSite=Lax`
  - `Path=/`
- `gitgym_oauth_state`
  - short-lived cookie used only between login start and callback
  - cleared immediately after callback handling

For local development on `127.0.0.1`, do not mark cookies `Secure`.
For hosted HTTPS environments, `Secure` should be enabled in later deployment work.

## Local Development Configuration

Required local values:

- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `API_BASE_URL`
- `RUNNER_BASE_URL`
- `MYSQL_DSN`

Local callback URL:

- `http://127.0.0.1:8080/api/v1/auth/github/callback`

Local default:

- `DEV_AUTH_BYPASS=false`

Local fallback:

- developers may temporarily set `DEV_AUTH_BYPASS=true`
- bypass remains loopback-only

## Frontend Behavior

No landing-page redesign is required.

Expected behavior:

- unauthenticated users see the existing login shell
- clicking `Continue with GitHub` starts the real OAuth redirect
- after callback and cookie creation, the browser lands back on the app
- `useCurrentSession` then drives whether the user sees the login shell or workbench

Optional follow-up:

- top bar can later show the current authenticated user from `GET /api/v1/auth/me`

That user display is not required for the first OAuth implementation.

## Security Notes

- never store raw session tokens in MySQL
- do not trust any user identifier from the frontend
- require OAuth state verification to prevent callback forgery
- keep `DEV_AUTH_BYPASS` loopback-only
- do not commit real GitHub secrets to the repository

## Testing Plan

Add or update tests for:

- login route redirects to GitHub with state
- callback rejects invalid or missing state
- callback creates a session and sets a cookie on success
- `auth/me` returns `401` without session and user JSON with session
- logout revokes the session and clears the cookie
- middleware accepts loopback bypass only when enabled
- middleware rejects bypass from non-loopback addresses

Keep browser smoke testing focused on:

- unauthenticated app shows login shell
- authenticated callback result can reach a usable session state

## Implementation Order

1. write failing auth route and middleware tests
2. add store queries and persistence methods for users and sessions
3. implement GitHub login and callback handlers
4. implement real session middleware
5. replace stub `auth/me`
6. add logout
7. validate local OAuth end-to-end with `DEV_AUTH_BYPASS=false`

## Risks and Boundaries

- the existing API still uses an in-memory practice session store; OAuth login will be real even if practice sessions remain memory-backed
- local callback correctness depends on the GitHub OAuth App being configured exactly for `127.0.0.1:8080`
- GitHub profile email can be absent; the implementation must tolerate that and optionally query the email endpoint

## Success Criteria

This work is complete when:

- local `DEV_AUTH_BYPASS=false` is the default path
- `Continue with GitHub` completes a real OAuth flow
- the API creates and validates persistent browser sessions from MySQL
- `auth/me` returns real local user data
- protected practice endpoints work with the authenticated browser session
