package handlers

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/oauth"
	"gitgym/services/api/internal/service"
)

const (
	oauthStateCookieName     = "gitgym_oauth_state"
	browserSessionCookieName = "gitgym_session"
	oauthErrorQueryKey       = "oauth_error"
)

func GitHubLoginWithReadiness(gitHubOAuthClient oauth.GitHubOAuthClient, authStore service.UserStore, frontendRedirectURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireOAuthFlowReady(gitHubOAuthClient, authStore, frontendRedirectURL); err != nil {
			if redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_unavailable", false) {
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		state, err := service.NewSessionToken()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		setOAuthStateCookie(w, state, shouldUseSecureCookies(r))
		http.Redirect(w, r, gitHubOAuthClient.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}
}

func GitHubCallback(gitHubOAuthClient oauth.GitHubOAuthClient, authStore service.UserStore, frontendRedirectURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireOAuthFlowReady(gitHubOAuthClient, authStore, frontendRedirectURL); err != nil {
			if redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_unavailable", true) {
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_state_invalid", true)
			return
		}

		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_code_missing", true)
			return
		}

		accessToken, err := gitHubOAuthClient.ExchangeCode(r.Context(), code)
		if err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_exchange_failed", true)
			return
		}

		profile, err := gitHubOAuthClient.FetchProfile(r.Context(), accessToken)
		if err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_profile_failed", true)
			return
		}

		if _, err := authStore.UpsertGitHubUser(r.Context(), profile); err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_session_failed", true)
			return
		}

		user, err := authStore.GetUserByGitHubID(r.Context(), profile.ID)
		if err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_session_failed", true)
			return
		}

		rawToken, err := service.NewSessionToken()
		if err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_session_failed", true)
			return
		}

		if err := authStore.CreateBrowserSession(r.Context(), user.ID, service.HashSessionToken(rawToken), r.UserAgent(), clientIP(r)); err != nil {
			redirectToOAuthFailure(w, r, frontendRedirectURL, "oauth_session_failed", true)
			return
		}

		secureCookies := shouldUseSecureCookies(r)
		setBrowserSessionCookie(w, rawToken, secureCookies)
		clearOAuthStateCookie(w, secureCookies)
		http.Redirect(w, r, frontendRedirectURL, http.StatusTemporaryRedirect)
	}
}

func redirectToOAuthFailure(w http.ResponseWriter, r *http.Request, frontendRedirectURL string, code string, clearState bool) bool {
	redirectURL, err := oauthFailureRedirectURL(frontendRedirectURL, code)
	if err != nil {
		return false
	}

	if clearState {
		clearOAuthStateCookie(w, shouldUseSecureCookies(r))
	}

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	return true
}

func oauthFailureRedirectURL(frontendRedirectURL string, code string) (string, error) {
	if strings.TrimSpace(frontendRedirectURL) == "" || strings.TrimSpace(code) == "" {
		return "", errString("frontend redirect is not configured")
	}

	redirectURL, err := url.Parse(frontendRedirectURL)
	if err != nil {
		return "", err
	}

	query := redirectURL.Query()
	query.Set(oauthErrorQueryKey, code)
	redirectURL.RawQuery = query.Encode()
	return redirectURL.String(), nil
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

		clearBrowserSessionCookie(w, shouldUseSecureCookies(r))

		if authStore == nil {
			http.Error(w, "auth store is not configured", http.StatusInternalServerError)
			return
		}
		if err := authStore.RevokeBrowserSession(r.Context(), service.HashSessionToken(cookie.Value)); err != nil {
			http.Error(w, "failed to revoke browser session", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func setOAuthStateCookie(w http.ResponseWriter, state string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func setBrowserSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    token,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearBrowserSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    "",
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func requireOAuthFlowReady(gitHubOAuthClient oauth.GitHubOAuthClient, authStore service.UserStore, frontendRedirectURL string) error {
	if gitHubOAuthClient == nil {
		return errString("github oauth is not configured")
	}
	if authStore == nil {
		return errString("auth store is not configured")
	}
	if strings.TrimSpace(frontendRedirectURL) == "" {
		return errString("frontend redirect is not configured")
	}
	return nil
}

func shouldUseSecureCookies(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}

	forwardedProto := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	if forwardedProto == "https" {
		return true
	}

	return !isLocalRequestHost(requestHost(r))
}

type errString string

func (e errString) Error() string {
	return string(e)
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

func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if host := strings.TrimSpace(r.URL.Hostname()); host != "" {
		return host
	}
	host := strings.TrimSpace(r.Host)
	parsedHost, _, err := net.SplitHostPort(host)
	if err == nil {
		return parsedHost
	}
	return host
}

func isLocalRequestHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}
