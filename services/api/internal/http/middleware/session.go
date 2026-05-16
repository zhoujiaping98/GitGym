package middleware

import (
	"context"
	"net/http"
	"strconv"
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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		session := AuthenticatedSession{
			UserID:       authenticatedUserIDFromCookie(cookie.Value),
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

func authenticatedUserIDFromCookie(raw string) uint64 {
	if id, ok := parsePrefixedUserID(raw); ok {
		return id
	}
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id > 0 {
		return id
	}
	return defaultAuthenticatedUserID
}

func parsePrefixedUserID(raw string) (uint64, bool) {
	if !strings.HasPrefix(raw, "uid:") {
		return 0, false
	}

	remainder := strings.TrimPrefix(raw, "uid:")
	userIDText := remainder
	if idx := strings.IndexByte(remainder, ':'); idx >= 0 {
		userIDText = remainder[:idx]
	}

	userID, err := strconv.ParseUint(userIDText, 10, 64)
	if err != nil || userID == 0 {
		return 0, false
	}

	return userID, true
}
