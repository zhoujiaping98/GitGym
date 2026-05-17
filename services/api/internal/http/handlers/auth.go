package handlers

import (
	"net"
	"net/http"
	"strings"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/oauth"
	"gitgym/services/api/internal/service"
)

const (
	oauthStateCookieName     = "gitgym_oauth_state"
	browserSessionCookieName = "gitgym_session"
)

func GitHubLogin(gitHubOAuthClient oauth.GitHubOAuthClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gitHubOAuthClient == nil {
			http.Error(w, "github oauth is not configured", http.StatusInternalServerError)
			return
		}

		state, err := service.NewSessionToken()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		setOAuthStateCookie(w, state)
		http.Redirect(w, r, gitHubOAuthClient.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}
}

func GitHubCallback(gitHubOAuthClient oauth.GitHubOAuthClient, authStore service.UserStore, frontendRedirectURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gitHubOAuthClient == nil || authStore == nil {
			http.Error(w, "github oauth is not configured", http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(frontendRedirectURL) == "" {
			http.Error(w, "frontend redirect is not configured", http.StatusInternalServerError)
			return
		}

		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
			http.Error(w, "invalid oauth state", http.StatusBadRequest)
			return
		}

		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing oauth code", http.StatusBadRequest)
			return
		}

		accessToken, err := gitHubOAuthClient.ExchangeCode(r.Context(), code)
		if err != nil {
			http.Error(w, "github oauth exchange failed", http.StatusBadGateway)
			return
		}

		profile, err := gitHubOAuthClient.FetchProfile(r.Context(), accessToken)
		if err != nil {
			http.Error(w, "github profile fetch failed", http.StatusBadGateway)
			return
		}

		if _, err := authStore.UpsertGitHubUser(r.Context(), profile); err != nil {
			http.Error(w, "failed to upsert user", http.StatusInternalServerError)
			return
		}

		user, err := authStore.GetUserByGitHubID(r.Context(), profile.ID)
		if err != nil {
			http.Error(w, "failed to load user", http.StatusInternalServerError)
			return
		}

		rawToken, err := service.NewSessionToken()
		if err != nil {
			http.Error(w, "failed to create session token", http.StatusInternalServerError)
			return
		}

		if err := authStore.CreateBrowserSession(r.Context(), user.ID, service.HashSessionToken(rawToken), r.UserAgent(), clientIP(r)); err != nil {
			http.Error(w, "failed to create browser session", http.StatusInternalServerError)
			return
		}

		setBrowserSessionCookie(w, rawToken)
		clearOAuthStateCookie(w)
		http.Redirect(w, r, frontendRedirectURL, http.StatusTemporaryRedirect)
	}
}

func AuthMe(authStore service.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticatedSession, ok := middleware.AuthenticatedSessionFromContext(r.Context())
		if !ok || authenticatedSession.UserID == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "authenticated session missing from request context",
			})
			return
		}
		if authStore == nil {
			http.Error(w, "auth store is not configured", http.StatusInternalServerError)
			return
		}

		user, err := authStore.GetUserByID(r.Context(), authenticatedSession.UserID)
		if err != nil {
			http.Error(w, "failed to load current user", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":           user.ID,
				"github_id":    user.GitHubID,
				"github_login": user.GitHubLogin,
				"display_name": user.DisplayName,
				"avatar_url":   user.AvatarURL,
				"email":        user.Email,
			},
		})
	}
}

func Logout(authStore service.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(browserSessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if authStore != nil {
			_ = authStore.RevokeBrowserSession(r.Context(), service.HashSessionToken(cookie.Value))
		}

		clearBrowserSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func setOAuthStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func setBrowserSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearBrowserSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func clientIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if forwardedFor != "" {
		return forwardedFor
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
