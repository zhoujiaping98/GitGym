package middleware

import (
	"context"
	"net/http"
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
