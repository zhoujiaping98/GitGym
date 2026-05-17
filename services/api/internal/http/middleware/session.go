package middleware

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
)

type authenticatedSessionKey struct{}

const defaultAuthenticatedUserID uint64 = 1

type AuthenticatedSession struct {
	UserID       uint64
	SessionToken string
}

func RequireSessionCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("gitgym_session")
		if err != nil || cookie.Value == "" {
			if devAuthBypassEnabled() && requestFromLoopback(r) {
				session := AuthenticatedSession{
					UserID:       defaultAuthenticatedUserID,
					SessionToken: "dev-auth-bypass",
				}
				ctx := context.WithValue(r.Context(), authenticatedSessionKey{}, session)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		session := AuthenticatedSession{
			UserID:       defaultAuthenticatedUserID,
			SessionToken: cookie.Value,
		}

		ctx := context.WithValue(r.Context(), authenticatedSessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AuthenticatedSessionFromContext(ctx context.Context) (AuthenticatedSession, bool) {
	session, ok := ctx.Value(authenticatedSessionKey{}).(AuthenticatedSession)
	return session, ok
}

func devAuthBypassEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEV_AUTH_BYPASS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func requestFromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
